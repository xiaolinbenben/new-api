package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"net/http"
	_ "net/http/pprof"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

type trackingWriter struct {
	http.ResponseWriter
	status int
	bytes  int64
}

func (w *trackingWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *trackingWriter) Write(data []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	n, err := w.ResponseWriter.Write(data)
	w.bytes += int64(n)
	return n, err
}

func (w *trackingWriter) Flush() {
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (w *trackingWriter) statusCode() int {
	if w.status == 0 {
		return http.StatusOK
	}
	return w.status
}

type publicCallMeta struct {
	Route  string
	Model  string
	Stream bool
	Error  string
}

type chatCompletionRequest struct {
	Model         string                `json:"model"`
	Messages      []dto.Message         `json:"messages"`
	Prompt        any                   `json:"prompt,omitempty"`
	Stream        bool                  `json:"stream,omitempty"`
	StreamOptions *dto.StreamOptions    `json:"stream_options,omitempty"`
	Tools         []dto.ToolCallRequest `json:"tools,omitempty"`
}

type responsesRequest struct {
	Model  string           `json:"model"`
	Input  any              `json:"input,omitempty"`
	Stream bool             `json:"stream,omitempty"`
	Tools  []map[string]any `json:"tools,omitempty"`
}

type imageRequestCompat struct {
	Model          string `json:"model"`
	Prompt         string `json:"prompt"`
	Size           string `json:"size"`
	ResponseFormat string `json:"response_format"`
	N              int    `json:"n"`
}

type videoRequestCompat struct {
	Model    string  `json:"model"`
	Prompt   string  `json:"prompt"`
	Duration float64 `json:"duration"`
	Size     string  `json:"size"`
	FPS      int     `json:"fps"`
}

type chatCompletionChoice struct {
	Index        int         `json:"index"`
	Message      dto.Message `json:"message"`
	FinishReason string      `json:"finish_reason"`
}

type chatCompletionResponse struct {
	ID                string                 `json:"id"`
	Object            string                 `json:"object"`
	Created           int64                  `json:"created"`
	Model             string                 `json:"model"`
	SystemFingerprint *string                `json:"system_fingerprint,omitempty"`
	Choices           []chatCompletionChoice `json:"choices"`
	Usage             dto.Usage              `json:"usage"`
}

type completionChoice struct {
	Text         string `json:"text"`
	Index        int    `json:"index"`
	FinishReason string `json:"finish_reason"`
}

type completionResponse struct {
	ID      string             `json:"id"`
	Object  string             `json:"object"`
	Created int64              `json:"created"`
	Model   string             `json:"model"`
	Choices []completionChoice `json:"choices"`
	Usage   dto.Usage          `json:"usage"`
}

type modelObject struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

type modelsResponse struct {
	Object string        `json:"object"`
	Data   []modelObject `json:"data"`
}

type imageAsset struct {
	ID          string
	CreatedAt   time.Time
	ContentType string
	Data        []byte
}

type imageAssetStore struct {
	mu    sync.RWMutex
	items map[string]*imageAsset
}

type videoTask struct {
	ID              string
	Route           string
	Model           string
	Prompt          string
	CreatedAt       time.Time
	QueueMs         int
	RunMs           int
	DurationSeconds float64
	Resolution      string
	Width           int
	Height          int
	FPS             int
	WillFail        bool
	ContentBytes    int
	ProgressJitter  int
}

type videoTaskStore struct {
	mu    sync.RWMutex
	tasks map[string]*videoTask
}

type workerApp struct {
	cfg       mockConfig
	rng       *lockedRand
	startedAt time.Time
	stats     *statsCollector
	assets    *imageAssetStore
	tasks     *videoTaskStore
}

func newWorkerApp(cfg mockConfig) *workerApp {
	startedAt := time.Now()
	return &workerApp{
		cfg:       cfg,
		rng:       newLockedRand(cfg),
		startedAt: startedAt,
		stats:     newStatsCollector(startedAt),
		assets:    &imageAssetStore{items: make(map[string]*imageAsset)},
		tasks:     &videoTaskStore{tasks: make(map[string]*videoTask)},
	}
}

func (a *workerApp) run(ctx context.Context) error {
	mux := a.handler()
	server := &http.Server{
		Addr:              fmt.Sprintf("0.0.0.0:%d", a.cfg.Worker.Port),
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	var pprofServer *http.Server
	if a.cfg.Worker.EnablePprof {
		pprofServer = &http.Server{
			Addr:              fmt.Sprintf(":%d", a.cfg.Worker.PprofPort),
			Handler:           http.DefaultServeMux,
			ReadHeaderTimeout: 5 * time.Second,
		}
		go func() {
			_ = pprofServer.ListenAndServe()
		}()
	}

	cleanupDone := make(chan struct{})
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		defer close(cleanupDone)
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				a.assets.prune(time.Now().Add(-10 * time.Minute))
				a.tasks.prune(time.Now().Add(-30 * time.Minute))
			}
		}
	}()

	errCh := make(chan error, 1)
	go func() {
		err := server.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
		if pprofServer != nil {
			_ = pprofServer.Shutdown(shutdownCtx)
		}
		<-cleanupDone
		return nil
	case err := <-errCh:
		return err
	}
}

func (a *workerApp) handler() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", a.handleHealth)
	mux.HandleFunc("GET /__mock/assets/images/{asset_id}", a.handleImageAsset)

	mux.HandleFunc("GET /__mock/admin/runtime", a.withAdmin(a.handleAdminRuntime))
	mux.HandleFunc("GET /__mock/admin/stats/summary", a.withAdmin(a.handleAdminSummary))
	mux.HandleFunc("GET /__mock/admin/stats/routes", a.withAdmin(a.handleAdminRoutes))
	mux.HandleFunc("GET /__mock/admin/stats/events", a.withAdmin(a.handleAdminEvents))
	mux.HandleFunc("POST /__mock/admin/stats/reset", a.withAdmin(a.handleAdminReset))

	mux.HandleFunc("GET /v1/models", a.wrapPublic("models.list", a.handleListModels))
	mux.HandleFunc("GET /v1/models/{id}", a.wrapPublic("models.get", a.handleGetModel))
	mux.HandleFunc("POST /v1/completions", a.wrapPublic("completions", a.handleCompletions))
	mux.HandleFunc("POST /v1/chat/completions", a.wrapPublic("chat.completions", a.handleChatCompletions))
	mux.HandleFunc("POST /v1/responses", a.wrapPublic("responses", a.handleResponses))
	mux.HandleFunc("POST /v1/responses/compact", a.wrapPublic("responses.compact", a.handleResponses))
	mux.HandleFunc("POST /v1/images/generations", a.wrapPublic("images.generations", a.handleImages))
	mux.HandleFunc("POST /v1/images/edits", a.wrapPublic("images.edits", a.handleImages))
	mux.HandleFunc("POST /v1/edits", a.wrapPublic("images.compat_edits", a.handleImages))
	mux.HandleFunc("POST /v1/videos", a.wrapPublic("videos.create", a.handleOpenAIVideoCreate))
	mux.HandleFunc("GET /v1/videos/{task_id}", a.wrapPublic("videos.get", a.handleOpenAIVideoGet))
	mux.HandleFunc("GET /v1/videos/{task_id}/content", a.wrapPublic("videos.content", a.handleVideoContent))
	mux.HandleFunc("POST /v1/video/generations", a.wrapPublic("video.generations.create", a.handleLegacyVideoCreate))
	mux.HandleFunc("GET /v1/video/generations/{task_id}", a.wrapPublic("video.generations.get", a.handleLegacyVideoGet))
	return mux
}

func (a *workerApp) withAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Mock-Admin-Token") != a.cfg.Worker.ManagementToken {
			writeJSON(w, http.StatusUnauthorized, map[string]any{"message": "unauthorized"})
			return
		}
		next(w, r)
	}
}

func (a *workerApp) wrapPublic(route string, next func(http.ResponseWriter, *http.Request, *publicCallMeta)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		meta := &publicCallMeta{Route: route}
		writer := &trackingWriter{ResponseWriter: w}
		a.stats.begin(route)

		defer func() {
			if recovered := recover(); recovered != nil {
				meta.Error = fmt.Sprintf("%v", recovered)
				writeOpenAIError(writer, http.StatusInternalServerError, "server_error", "mock upstream panic", "panic", "")
			}
			status := writer.statusCode()
			a.stats.finish(route, status, time.Since(started).Milliseconds(), requestEvent{
				Timestamp:  started,
				Route:      route,
				Method:     r.Method,
				Model:      meta.Model,
				StatusCode: status,
				LatencyMS:  time.Since(started).Milliseconds(),
				Stream:     meta.Stream,
				Error:      meta.Error,
			})
		}()

		if a.cfg.Worker.RequireAuth && strings.HasPrefix(r.URL.Path, "/v1/") && !hasValidAuthorization(r, a.cfg.Worker.ManagementToken) {
			meta.Error = "invalid api key"
			writeOpenAIError(writer, http.StatusUnauthorized, "authentication_error", "invalid api key", "invalid_api_key", "")
			return
		}

		latency := a.rng.intRange(a.cfg.Random.LatencyMs)
		if latency > 0 {
			time.Sleep(time.Duration(latency) * time.Millisecond)
		}

		if status, errType, msg, code, ok := a.injectedError(); ok {
			meta.Error = msg
			writeOpenAIError(writer, status, errType, msg, code, "")
			return
		}

		next(writer, r, meta)
	}
}

func (a *workerApp) injectedError() (int, string, string, string, bool) {
	switch {
	case a.rng.bool(a.cfg.Random.TimeoutRate):
		return http.StatusGatewayTimeout, "timeout_error", "mock upstream timeout", "mock_timeout", true
	case a.rng.bool(a.cfg.Random.TooManyRequestsRate):
		return http.StatusTooManyRequests, "rate_limit_error", "mock upstream rate limit", "mock_rate_limit", true
	case a.rng.bool(a.cfg.Random.ServerErrorRate):
		return http.StatusInternalServerError, "server_error", "mock upstream server error", "mock_server_error", true
	case a.rng.bool(a.cfg.Random.ErrorRate):
		return http.StatusBadRequest, "invalid_request_error", "mock invalid request", "mock_invalid_request", true
	default:
		return 0, "", "", "", false
	}
}

func (a *workerApp) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"service": "mock-upstream-worker",
		"pid":     os.Getpid(),
	})
}

func (a *workerApp) handleAdminRuntime(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, workerRuntimeSnapshot{
		Status:       "running",
		PID:          os.Getpid(),
		StartedAt:    a.startedAt,
		UptimeSec:    int64(time.Since(a.startedAt).Seconds()),
		PublicBase:   fmt.Sprintf("http://127.0.0.1:%d", a.cfg.Worker.Port),
		PprofURL:     a.pprofURL(),
		Healthy:      true,
		WorkerConfig: a.cfg.Worker,
	})
}

func (a *workerApp) handleAdminSummary(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, a.stats.summary(a.tasks.count()))
}

func (a *workerApp) handleAdminRoutes(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, a.stats.routesSnapshot())
}

func (a *workerApp) handleAdminEvents(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, a.stats.eventsSnapshot())
}

func (a *workerApp) handleAdminReset(w http.ResponseWriter, r *http.Request) {
	a.startedAt = time.Now()
	a.stats.reset(a.startedAt)
	a.assets.reset()
	a.tasks.reset()
	writeJSON(w, http.StatusOK, map[string]any{"message": "ok"})
}

func (a *workerApp) handleListModels(w http.ResponseWriter, r *http.Request, meta *publicCallMeta) {
	items := make([]modelObject, 0, len(a.cfg.Worker.Models))
	now := nowUnix()
	for _, modelID := range a.cfg.Worker.Models {
		items = append(items, modelObject{
			ID:      modelID,
			Object:  "model",
			Created: now,
			OwnedBy: "mock-upstream",
		})
	}
	writeJSON(w, http.StatusOK, modelsResponse{
		Object: "list",
		Data:   items,
	})
}

func (a *workerApp) handleGetModel(w http.ResponseWriter, r *http.Request, meta *publicCallMeta) {
	modelID := strings.TrimSpace(r.PathValue("id"))
	if modelID == "" {
		modelID = a.defaultModel("")
	}
	meta.Model = modelID
	writeJSON(w, http.StatusOK, modelObject{
		ID:      modelID,
		Object:  "model",
		Created: nowUnix(),
		OwnedBy: "mock-upstream",
	})
}

func (a *workerApp) handleChatCompletions(w http.ResponseWriter, r *http.Request, meta *publicCallMeta) {
	var req chatCompletionRequest
	if err := readJSON(r, &req); err != nil {
		meta.Error = err.Error()
		writeOpenAIError(w, http.StatusBadRequest, "invalid_request_error", err.Error(), "bad_json", "")
		return
	}
	req.Model = a.defaultModel(req.Model)
	meta.Model = req.Model
	meta.Stream = req.Stream && a.cfg.Chat.AllowStream

	completionTokens := a.rng.intRange(a.cfg.Chat.TextTokens)
	usage := a.makeUsage(completionTokens)
	systemFingerprint := a.systemFingerprint(a.cfg.Chat.SystemFingerprintChance)
	hasToolResult := hasChatToolResult(req.Messages)
	shouldToolCall := len(req.Tools) > 0 && !hasToolResult && a.rng.bool(a.cfg.Chat.ToolCallProbability)

	if meta.Stream {
		startSSE(w)
		a.stats.setSSEActive(1)
		defer a.stats.setSSEActive(-1)

		if shouldToolCall {
			toolCalls := a.generateToolCalls(a.cfg.Chat)
			_ = writeSSEEvent(w, a.chatStreamChunk(req.Model, systemFingerprint, dto.ChatCompletionsStreamResponseChoiceDelta{Role: "assistant", ToolCalls: toolCalls}, nil, nil))
			finish := "tool_calls"
			usagePtr := a.maybeUsage(a.cfg.Chat.UsageProbability, usage)
			_ = writeSSEEvent(w, a.chatStreamChunk(req.Model, systemFingerprint, dto.ChatCompletionsStreamResponseChoiceDelta{}, &finish, usagePtr))
			_ = writeSSEDone(w)
			return
		}

		text := a.rng.randomText(completionTokens)
		roleDelta := dto.ChatCompletionsStreamResponseChoiceDelta{Role: "assistant"}
		_ = writeSSEEvent(w, a.chatStreamChunk(req.Model, systemFingerprint, roleDelta, nil, nil))
		for _, piece := range chunkString(text, a.rng.intRange(a.cfg.Random.DefaultStreamChunks)) {
			content := piece
			_ = writeSSEEvent(w, a.chatStreamChunk(req.Model, systemFingerprint, dto.ChatCompletionsStreamResponseChoiceDelta{Content: &content}, nil, nil))
		}
		finish := "stop"
		usagePtr := a.maybeUsage(a.cfg.Chat.UsageProbability, usage)
		_ = writeSSEEvent(w, a.chatStreamChunk(req.Model, systemFingerprint, dto.ChatCompletionsStreamResponseChoiceDelta{}, &finish, usagePtr))
		_ = writeSSEDone(w)
		return
	}

	message := dto.Message{Role: "assistant"}
	finishReason := "stop"
	if shouldToolCall {
		toolCalls := a.generateToolCalls(a.cfg.Chat)
		payload, _ := common.Marshal(toolCalls)
		message.Content = ""
		message.ToolCalls = payload
		finishReason = "tool_calls"
	} else {
		message.Content = a.rng.randomText(completionTokens)
	}

	response := chatCompletionResponse{
		ID:                a.rng.randomID("chatcmpl"),
		Object:            "chat.completion",
		Created:           nowUnix(),
		Model:             req.Model,
		SystemFingerprint: systemFingerprint,
		Choices: []chatCompletionChoice{{
			Index:        0,
			Message:      message,
			FinishReason: finishReason,
		}},
		Usage: usage,
	}
	writeJSON(w, http.StatusOK, response)
}

func (a *workerApp) handleCompletions(w http.ResponseWriter, r *http.Request, meta *publicCallMeta) {
	var req chatCompletionRequest
	if err := readJSON(r, &req); err != nil {
		meta.Error = err.Error()
		writeOpenAIError(w, http.StatusBadRequest, "invalid_request_error", err.Error(), "bad_json", "")
		return
	}
	req.Model = a.defaultModel(req.Model)
	meta.Model = req.Model
	meta.Stream = req.Stream && a.cfg.Chat.AllowStream
	completionTokens := a.rng.intRange(a.cfg.Chat.TextTokens)
	usage := a.makeUsage(completionTokens)
	text := a.rng.randomText(completionTokens)

	if meta.Stream {
		startSSE(w)
		a.stats.setSSEActive(1)
		defer a.stats.setSSEActive(-1)
		for _, piece := range chunkString(text, a.rng.intRange(a.cfg.Random.DefaultStreamChunks)) {
			_ = writeSSEEvent(w, map[string]any{
				"id":      a.rng.randomID("cmpl"),
				"object":  "text_completion",
				"created": nowUnix(),
				"model":   req.Model,
				"choices": []map[string]any{{
					"text":          piece,
					"index":         0,
					"finish_reason": "",
				}},
			})
		}
		_ = writeSSEEvent(w, map[string]any{
			"id":      a.rng.randomID("cmpl"),
			"object":  "text_completion",
			"created": nowUnix(),
			"model":   req.Model,
			"choices": []map[string]any{{
				"text":          "",
				"index":         0,
				"finish_reason": "stop",
			}},
			"usage": a.maybeUsage(a.cfg.Chat.UsageProbability, usage),
		})
		_ = writeSSEDone(w)
		return
	}

	writeJSON(w, http.StatusOK, completionResponse{
		ID:      a.rng.randomID("cmpl"),
		Object:  "text_completion",
		Created: nowUnix(),
		Model:   req.Model,
		Choices: []completionChoice{{
			Text:         text,
			Index:        0,
			FinishReason: "stop",
		}},
		Usage: usage,
	})
}

func (a *workerApp) handleResponses(w http.ResponseWriter, r *http.Request, meta *publicCallMeta) {
	var req responsesRequest
	if err := readJSON(r, &req); err != nil {
		meta.Error = err.Error()
		writeOpenAIError(w, http.StatusBadRequest, "invalid_request_error", err.Error(), "bad_json", "")
		return
	}
	req.Model = a.defaultModel(req.Model)
	meta.Model = req.Model
	meta.Stream = req.Stream && a.cfg.Responses.AllowStream

	completionTokens := a.rng.intRange(a.cfg.Responses.TextTokens)
	usage := a.makeUsage(completionTokens)
	hasToolResult := inputContainsToolResult(req.Input)
	shouldToolCall := len(req.Tools) > 0 && !hasToolResult && a.rng.bool(a.cfg.Responses.ToolCallProbability)
	responseID := a.rng.randomID("resp")

	if shouldToolCall {
		toolCalls := a.generateToolCalls(a.cfg.Responses)
		toolItem := a.responsesToolOutput(toolCalls[0], strings.HasPrefix(toolCalls[0].Function.Name, "mcp."))
		response := dto.OpenAIResponsesResponse{
			ID:                responseID,
			Object:            "response",
			CreatedAt:         int(nowUnix()),
			Status:            json.RawMessage(`"completed"`),
			Model:             req.Model,
			Output:            []dto.ResponsesOutput{toolItem},
			ParallelToolCalls: false,
			Usage:             a.maybeUsage(a.cfg.Responses.UsageProbability, usage),
		}
		if meta.Stream {
			startSSE(w)
			a.stats.setSSEActive(1)
			defer a.stats.setSSEActive(-1)
			_ = writeSSEEvent(w, dto.ResponsesStreamResponse{
				Type: dto.ResponsesOutputTypeItemAdded,
				Item: &toolItem,
			})
			_ = writeSSEEvent(w, dto.ResponsesStreamResponse{
				Type: dto.ResponsesOutputTypeItemDone,
				Item: &toolItem,
			})
			_ = writeSSEEvent(w, dto.ResponsesStreamResponse{
				Type:     "response.completed",
				Response: &response,
			})
			_ = writeSSEDone(w)
			return
		}
		writeJSON(w, http.StatusOK, response)
		return
	}

	text := a.rng.randomText(completionTokens)
	output := dto.ResponsesOutput{
		Type:   "message",
		ID:     a.rng.randomID("msg"),
		Status: "completed",
		Role:   "assistant",
		Content: []dto.ResponsesOutputContent{{
			Type:        "output_text",
			Text:        text,
			Annotations: []interface{}{},
		}},
	}
	response := dto.OpenAIResponsesResponse{
		ID:                responseID,
		Object:            "response",
		CreatedAt:         int(nowUnix()),
		Status:            json.RawMessage(`"completed"`),
		Model:             req.Model,
		Output:            []dto.ResponsesOutput{output},
		ParallelToolCalls: false,
		Usage:             a.maybeUsage(a.cfg.Responses.UsageProbability, usage),
	}
	if meta.Stream {
		startSSE(w)
		a.stats.setSSEActive(1)
		defer a.stats.setSSEActive(-1)
		for _, piece := range chunkString(text, a.rng.intRange(a.cfg.Random.DefaultStreamChunks)) {
			_ = writeSSEEvent(w, dto.ResponsesStreamResponse{
				Type:  "response.output_text.delta",
				Delta: piece,
			})
		}
		_ = writeSSEEvent(w, dto.ResponsesStreamResponse{
			Type:     "response.completed",
			Response: &response,
		})
		_ = writeSSEDone(w)
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (a *workerApp) handleImages(w http.ResponseWriter, r *http.Request, meta *publicCallMeta) {
	req, err := a.parseImageRequest(r)
	if err != nil {
		meta.Error = err.Error()
		writeOpenAIError(w, http.StatusBadRequest, "invalid_request_error", err.Error(), "bad_image_request", "")
		return
	}
	req.Model = a.defaultModel(req.Model)
	meta.Model = req.Model

	size := req.Size
	if size == "" {
		size = a.rng.pick(a.cfg.Images.Sizes, "1024x1024")
	}
	width, height := parseResolution(size, 1024, 1024)
	count := req.N
	if count <= 0 {
		count = a.rng.intRange(a.cfg.Images.ImageCount)
	}
	if count > a.cfg.Images.ImageCount.Max {
		count = a.cfg.Images.ImageCount.Max
	}

	responseFormat := req.ResponseFormat
	if responseFormat == "" {
		if a.rng.bool(a.cfg.Images.ResponseURLRate) {
			responseFormat = "url"
		} else {
			responseFormat = "b64_json"
		}
	}

	data := make([]dto.ImageData, 0, count)
	for i := 0; i < count; i++ {
		prompt := req.Prompt
		if prompt == "" {
			prompt = "mock image"
		}
		imageBytes, err := a.generateImageBytes(width, height, prompt)
		if err != nil {
			meta.Error = err.Error()
			writeOpenAIError(w, http.StatusInternalServerError, "server_error", err.Error(), "image_generation_failed", "")
			return
		}
		item := dto.ImageData{RevisedPrompt: prompt + " [simulated]"}
		if responseFormat == "b64_json" {
			item.B64Json = base64.StdEncoding.EncodeToString(imageBytes)
		} else {
			assetID := a.rng.randomID("img")
			a.assets.put(&imageAsset{
				ID:          assetID,
				CreatedAt:   time.Now(),
				ContentType: "image/png",
				Data:        imageBytes,
			})
			item.Url = fmt.Sprintf("%s/__mock/assets/images/%s", publicBaseURL(r), assetID)
		}
		data = append(data, item)
	}

	writeJSON(w, http.StatusOK, dto.ImageResponse{
		Data:    data,
		Created: nowUnix(),
	})
}

func (a *workerApp) handleImageAsset(w http.ResponseWriter, r *http.Request) {
	assetID := r.PathValue("asset_id")
	asset, ok := a.assets.get(assetID)
	if !ok {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", asset.ContentType)
	w.Header().Set("Cache-Control", "public, max-age=300")
	_, _ = w.Write(asset.Data)
}

func (a *workerApp) handleOpenAIVideoCreate(w http.ResponseWriter, r *http.Request, meta *publicCallMeta) {
	req, err := a.parseVideoRequest(r)
	if err != nil {
		meta.Error = err.Error()
		writeOpenAIError(w, http.StatusBadRequest, "invalid_request_error", err.Error(), "bad_video_request", "")
		return
	}
	task := a.createVideoTask("openai", req)
	meta.Model = task.Model
	a.stats.addVideoEvent(task.toEvent("queued"))
	writeJSON(w, http.StatusOK, task.toOpenAIVideo(publicBaseURL(r)))
}

func (a *workerApp) handleOpenAIVideoGet(w http.ResponseWriter, r *http.Request, meta *publicCallMeta) {
	taskID := r.PathValue("task_id")
	task, ok := a.tasks.get(taskID)
	if !ok {
		meta.Error = "video task not found"
		writeOpenAIError(w, http.StatusNotFound, "invalid_request_error", "video task not found", "task_not_found", "task_id")
		return
	}
	meta.Model = task.Model
	writeJSON(w, http.StatusOK, task.toOpenAIVideo(publicBaseURL(r)))
}

func (a *workerApp) handleLegacyVideoCreate(w http.ResponseWriter, r *http.Request, meta *publicCallMeta) {
	req, err := a.parseVideoRequest(r)
	if err != nil {
		meta.Error = err.Error()
		writeOpenAIError(w, http.StatusBadRequest, "invalid_request_error", err.Error(), "bad_video_request", "")
		return
	}
	task := a.createVideoTask("legacy", req)
	meta.Model = task.Model
	a.stats.addVideoEvent(task.toEvent("queued"))
	writeJSON(w, http.StatusOK, dto.VideoResponse{
		TaskId: task.ID,
		Status: "queued",
	})
}

func (a *workerApp) handleLegacyVideoGet(w http.ResponseWriter, r *http.Request, meta *publicCallMeta) {
	taskID := r.PathValue("task_id")
	task, ok := a.tasks.get(taskID)
	if !ok {
		meta.Error = "video task not found"
		writeOpenAIError(w, http.StatusNotFound, "invalid_request_error", "video task not found", "task_not_found", "task_id")
		return
	}
	meta.Model = task.Model
	status, progress, _ := task.stateAt(time.Now())
	resp := dto.VideoTaskResponse{
		TaskId: task.ID,
		Status: legacyVideoStatus(status),
	}
	if status == dto.VideoStatusCompleted {
		resp.Url = fmt.Sprintf("%s/v1/videos/%s/content", publicBaseURL(r), task.ID)
		resp.Format = "mp4"
		resp.Metadata = &dto.VideoTaskMetadata{
			Duration: task.DurationSeconds,
			Fps:      task.FPS,
			Width:    task.Width,
			Height:   task.Height,
		}
	}
	if status == dto.VideoStatusFailed {
		resp.Error = &dto.VideoTaskError{
			Code:    http.StatusInternalServerError,
			Message: "mock video generation failed",
		}
	}
	if progress > 0 && resp.Metadata == nil {
		resp.Metadata = &dto.VideoTaskMetadata{
			Duration: task.DurationSeconds,
			Fps:      task.FPS,
			Width:    task.Width,
			Height:   task.Height,
		}
	}
	writeJSON(w, http.StatusOK, resp)
}

func (a *workerApp) handleVideoContent(w http.ResponseWriter, r *http.Request, meta *publicCallMeta) {
	taskID := r.PathValue("task_id")
	task, ok := a.tasks.get(taskID)
	if !ok {
		meta.Error = "video task not found"
		writeOpenAIError(w, http.StatusNotFound, "invalid_request_error", "video task not found", "task_not_found", "task_id")
		return
	}
	meta.Model = task.Model
	status, _, _ := task.stateAt(time.Now())
	if status == dto.VideoStatusFailed {
		meta.Error = "video task failed"
		writeOpenAIError(w, http.StatusBadGateway, "server_error", "video task failed", "task_failed", "task_id")
		return
	}
	if status != dto.VideoStatusCompleted {
		meta.Error = "video task is not completed"
		writeOpenAIError(w, http.StatusConflict, "invalid_request_error", "video task is not completed", "task_not_ready", "task_id")
		return
	}

	content := a.generateVideoBytes(task.ContentBytes)
	w.Header().Set("Content-Type", "video/mp4")
	w.Header().Set("Cache-Control", "public, max-age=300")
	_, _ = w.Write(content)
}

func (a *workerApp) parseImageRequest(r *http.Request) (imageRequestCompat, error) {
	if strings.Contains(r.Header.Get("Content-Type"), "multipart/form-data") {
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			return imageRequestCompat{}, err
		}
		count, _ := strconv.Atoi(r.FormValue("n"))
		return imageRequestCompat{
			Model:          r.FormValue("model"),
			Prompt:         r.FormValue("prompt"),
			Size:           r.FormValue("size"),
			ResponseFormat: r.FormValue("response_format"),
			N:              count,
		}, nil
	}
	var req imageRequestCompat
	if err := readJSON(r, &req); err != nil {
		return imageRequestCompat{}, err
	}
	return req, nil
}

func (a *workerApp) parseVideoRequest(r *http.Request) (videoRequestCompat, error) {
	var req videoRequestCompat
	if err := readJSON(r, &req); err != nil {
		return videoRequestCompat{}, err
	}
	if req.Duration <= 0 {
		req.Duration = a.rng.floatRange(a.cfg.Videos.DurationsSeconds)
	}
	if req.Size == "" {
		req.Size = a.rng.pick(a.cfg.Videos.Resolutions, "1280x720")
	}
	if req.FPS <= 0 {
		req.FPS = a.rng.intRange(a.cfg.Videos.FPS)
	}
	return req, nil
}

func (a *workerApp) createVideoTask(route string, req videoRequestCompat) *videoTask {
	size := req.Size
	if size == "" {
		size = a.rng.pick(a.cfg.Videos.Resolutions, "1280x720")
	}
	width, height := parseResolution(size, 1280, 720)
	task := &videoTask{
		ID:              a.rng.randomID("video"),
		Route:           route,
		Model:           a.defaultModel(req.Model),
		Prompt:          req.Prompt,
		CreatedAt:       time.Now(),
		QueueMs:         a.rng.intRange(a.cfg.Videos.PollIntervalMs),
		RunMs:           a.rng.intRange(a.cfg.Videos.PollIntervalMs) * 3,
		DurationSeconds: req.Duration,
		Resolution:      size,
		Width:           width,
		Height:          height,
		FPS:             req.FPS,
		WillFail:        a.rng.bool(a.cfg.Videos.FailureRate),
		ContentBytes:    a.rng.intRange(a.cfg.Videos.VideoBytes),
		ProgressJitter:  a.rng.intRange(a.cfg.Videos.ProgressJitter),
	}
	a.tasks.put(task)
	return task
}

func (a *workerApp) makeUsage(completionTokens int) dto.Usage {
	promptTokens := maxInt(8, completionTokens/2)
	promptTokens = promptTokens + a.rng.intn(24)
	return dto.Usage{
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
		TotalTokens:      promptTokens + completionTokens,
		InputTokens:      promptTokens,
		OutputTokens:     completionTokens,
		InputTokensDetails: &dto.InputTokenDetails{
			TextTokens:  promptTokens,
			ImageTokens: 0,
			AudioTokens: 0,
		},
		PromptTokensDetails: dto.InputTokenDetails{
			TextTokens: promptTokens,
		},
		CompletionTokenDetails: dto.OutputTokenDetails{
			TextTokens:      completionTokens,
			ReasoningTokens: completionTokens / 8,
		},
	}
}

func (a *workerApp) maybeUsage(probability float64, usage dto.Usage) *dto.Usage {
	if a.rng.bool(probability) {
		return &usage
	}
	return nil
}

func (a *workerApp) generateToolCalls(cfg textBehaviorConfig) []dto.ToolCallResponse {
	count := a.rng.intRange(cfg.ToolCallCount)
	result := make([]dto.ToolCallResponse, 0, count)
	for i := 0; i < count; i++ {
		isMCP := a.rng.bool(cfg.MCPToolProbability)
		namePrefix := "tool"
		if isMCP {
			namePrefix = "mcp"
		}
		name := fmt.Sprintf("%s.lookup_%d", namePrefix, 1+a.rng.intn(9))
		result = append(result, dto.ToolCallResponse{
			Index: loIntPtr(i),
			ID:    a.rng.randomID("call"),
			Type:  "function",
			Function: dto.FunctionResponse{
				Name:      name,
				Arguments: a.randomArguments(cfg.ToolArgumentsBytes),
			},
		})
	}
	return result
}

func (a *workerApp) randomArguments(sizeRange intRange) string {
	target := a.rng.intRange(sizeRange)
	payload := map[string]any{
		"query":    a.rng.randomText(maxInt(4, target/24)),
		"limit":    1 + a.rng.intn(10),
		"trace_id": a.rng.randomID("trace"),
	}
	data, _ := common.Marshal(payload)
	for len(data) < target {
		payload[fmt.Sprintf("extra_%d", len(payload))] = a.rng.randomText(4 + a.rng.intn(12))
		data, _ = common.Marshal(payload)
	}
	return string(data)
}

func (a *workerApp) chatStreamChunk(model string, systemFingerprint *string, delta dto.ChatCompletionsStreamResponseChoiceDelta, finish *string, usage *dto.Usage) dto.ChatCompletionsStreamResponse {
	return dto.ChatCompletionsStreamResponse{
		Id:                a.rng.randomID("chatcmpl"),
		Object:            "chat.completion.chunk",
		Created:           nowUnix(),
		Model:             model,
		SystemFingerprint: systemFingerprint,
		Choices: []dto.ChatCompletionsStreamResponseChoice{{
			Delta:        delta,
			Logprobs:     nil,
			FinishReason: finish,
			Index:        0,
		}},
		Usage: usage,
	}
}

func (a *workerApp) responsesToolOutput(tool dto.ToolCallResponse, isMCP bool) dto.ResponsesOutput {
	itemType := "function_call"
	if isMCP {
		itemType = "mcp_call"
	}
	return dto.ResponsesOutput{
		Type:      itemType,
		ID:        tool.ID,
		Status:    "completed",
		CallId:    tool.ID,
		Name:      tool.Function.Name,
		Arguments: tool.Function.Arguments,
	}
}

func (a *workerApp) defaultModel(input string) string {
	if strings.TrimSpace(input) != "" {
		return strings.TrimSpace(input)
	}
	return a.rng.pick(a.cfg.Worker.Models, "gpt-4o-mini")
}

func (a *workerApp) systemFingerprint(probability float64) *string {
	if !a.rng.bool(probability) {
		return nil
	}
	value := a.rng.randomID("fp")
	return &value
}

func (a *workerApp) pprofURL() string {
	if !a.cfg.Worker.EnablePprof {
		return ""
	}
	return fmt.Sprintf("http://127.0.0.1:%d/debug/pprof/", a.cfg.Worker.PprofPort)
}

func (a *workerApp) generateImageBytes(width int, height int, prompt string) ([]byte, error) {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	palette := a.rng.shuffleStrings(a.cfg.Images.BackgroundPalette)
	bg := parseHexColor(firstOrDefault(palette, "#0F172A"))
	draw.Draw(img, img.Bounds(), &image.Uniform{C: bg}, image.Point{}, draw.Src)

	blockCount := maxInt(8, a.rng.intRange(a.cfg.Images.ImageBytes)/8000)
	for i := 0; i < blockCount; i++ {
		x0 := a.rng.intn(maxInt(1, width-1))
		y0 := a.rng.intn(maxInt(1, height-1))
		x1 := minInt(width, x0+maxInt(12, a.rng.intn(maxInt(16, width/3))))
		y1 := minInt(height, y0+maxInt(12, a.rng.intn(maxInt(16, height/3))))
		shapeColor := randomRGBA(a.rng)
		draw.Draw(img, image.Rect(x0, y0, x1, y1), &image.Uniform{C: shapeColor}, image.Point{}, draw.Over)
	}

	if a.rng.bool(a.cfg.Images.WatermarkRate) {
		label := a.rng.pick(a.cfg.Images.WatermarkTexts, "mock")
		if strings.TrimSpace(prompt) != "" {
			label = label + " | " + trimRunes(prompt, 28)
		}
		drawLabel(img, 24, height/2, label, color.White)
	}

	var buffer bytes.Buffer
	if err := png.Encode(&buffer, img); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func (a *workerApp) generateVideoBytes(size int) []byte {
	if size < 64 {
		size = 64
	}
	buffer := make([]byte, size)
	header := []byte{0x00, 0x00, 0x00, 0x18, 'f', 't', 'y', 'p', 'm', 'p', '4', '2', 0x00, 0x00, 0x00, 0x00, 'm', 'p', '4', '2', 'i', 's', 'o', 'm'}
	copy(buffer, header)
	a.rng.fillBytes(buffer[len(header):])
	return buffer
}

func hasAuthorization(r *http.Request) bool {
	return strings.TrimSpace(r.Header.Get("Authorization")) != "" ||
		strings.TrimSpace(r.Header.Get("x-api-key")) != "" ||
		strings.TrimSpace(r.Header.Get("api-key")) != ""
}

func hasValidAuthorization(r *http.Request, token string) bool {
	// Check Authorization header (Bearer token)
	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	if auth != "" {
		parts := strings.SplitN(auth, " ", 2)
		if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") && strings.TrimSpace(parts[1]) == token {
			return true
		}
		// Try exact match (some clients send just the token)
		if auth == token {
			return true
		}
	}
	// Check x-api-key header
	if strings.TrimSpace(r.Header.Get("x-api-key")) == token {
		return true
	}
	// Check api-key header
	if strings.TrimSpace(r.Header.Get("api-key")) == token {
		return true
	}
	return false
}

func hasChatToolResult(messages []dto.Message) bool {
	for _, message := range messages {
		if message.Role == "tool" || strings.TrimSpace(message.ToolCallId) != "" {
			return true
		}
	}
	return false
}

func inputContainsToolResult(input any) bool {
	switch value := input.(type) {
	case map[string]any:
		if kind, _ := value["type"].(string); kind == "function_call_output" || kind == "tool_result" {
			return true
		}
		for _, child := range value {
			if inputContainsToolResult(child) {
				return true
			}
		}
	case []any:
		for _, item := range value {
			if inputContainsToolResult(item) {
				return true
			}
		}
	case string:
		return strings.Contains(value, "function_call_output") || strings.Contains(value, "tool_result")
	}
	return false
}

func chunkString(value string, chunkCount int) []string {
	if chunkCount <= 1 || len(value) <= 1 {
		return []string{value}
	}
	runes := []rune(value)
	if chunkCount > len(runes) {
		chunkCount = len(runes)
	}
	out := make([]string, 0, chunkCount)
	step := len(runes) / chunkCount
	if step == 0 {
		step = 1
	}
	for start := 0; start < len(runes); start += step {
		end := start + step
		if len(out) == chunkCount-1 || end > len(runes) {
			end = len(runes)
		}
		out = append(out, string(runes[start:end]))
		if end == len(runes) {
			break
		}
	}
	return out
}

func publicBaseURL(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s", scheme, r.Host)
}

func parseResolution(value string, fallbackWidth int, fallbackHeight int) (int, int) {
	parts := strings.Split(strings.TrimSpace(value), "x")
	if len(parts) != 2 {
		return fallbackWidth, fallbackHeight
	}
	width, err := strconv.Atoi(parts[0])
	if err != nil || width <= 0 {
		return fallbackWidth, fallbackHeight
	}
	height, err := strconv.Atoi(parts[1])
	if err != nil || height <= 0 {
		return fallbackWidth, fallbackHeight
	}
	return width, height
}

func legacyVideoStatus(status string) string {
	switch status {
	case dto.VideoStatusCompleted:
		return "succeeded"
	case dto.VideoStatusFailed:
		return "failed"
	default:
		return status
	}
}

func parseHexColor(value string) color.Color {
	value = strings.TrimPrefix(strings.TrimSpace(value), "#")
	if len(value) != 6 {
		return color.RGBA{R: 15, G: 23, B: 42, A: 255}
	}
	r, _ := strconv.ParseUint(value[0:2], 16, 8)
	g, _ := strconv.ParseUint(value[2:4], 16, 8)
	b, _ := strconv.ParseUint(value[4:6], 16, 8)
	return color.RGBA{R: uint8(r), G: uint8(g), B: uint8(b), A: 255}
}

func randomRGBA(rng *lockedRand) color.RGBA {
	return color.RGBA{
		R: uint8(40 + rng.intn(180)),
		G: uint8(40 + rng.intn(180)),
		B: uint8(40 + rng.intn(180)),
		A: uint8(160 + rng.intn(90)),
	}
}

func drawLabel(img *image.RGBA, x int, y int, label string, fg color.Color) {
	d := &font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(fg),
		Face: basicfont.Face7x13,
		Dot:  fixed.P(x, y),
	}
	d.DrawString(label)
}

func trimRunes(value string, max int) string {
	runes := []rune(value)
	if len(runes) <= max {
		return value
	}
	return string(runes[:max])
}

func firstOrDefault(values []string, fallback string) string {
	if len(values) == 0 {
		return fallback
	}
	return values[0]
}

func minInt(a int, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a int, b int) int {
	if a > b {
		return a
	}
	return b
}

func loIntPtr(v int) *int {
	return &v
}

func (s *imageAssetStore) put(asset *imageAsset) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items[asset.ID] = asset
}

func (s *imageAssetStore) get(id string) (*imageAsset, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	asset, ok := s.items[id]
	return asset, ok
}

func (s *imageAssetStore) prune(cutoff time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, asset := range s.items {
		if asset.CreatedAt.Before(cutoff) {
			delete(s.items, id)
		}
	}
}

func (s *imageAssetStore) reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items = make(map[string]*imageAsset)
}

func (s *videoTaskStore) put(task *videoTask) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tasks[task.ID] = task
}

func (s *videoTaskStore) get(id string) (*videoTask, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	task, ok := s.tasks[id]
	return task, ok
}

func (s *videoTaskStore) count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.tasks)
}

func (s *videoTaskStore) prune(cutoff time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, task := range s.tasks {
		if task.CreatedAt.Before(cutoff) {
			delete(s.tasks, id)
		}
	}
}

func (s *videoTaskStore) reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tasks = make(map[string]*videoTask)
}

func (t *videoTask) stateAt(now time.Time) (string, int, int64) {
	elapsed := now.Sub(t.CreatedAt)
	if elapsed <= time.Duration(t.QueueMs)*time.Millisecond {
		progress := minInt(8+t.ProgressJitter, 12)
		return dto.VideoStatusQueued, progress, 0
	}
	totalRuntime := time.Duration(t.QueueMs+t.RunMs) * time.Millisecond
	if elapsed < totalRuntime {
		runElapsed := elapsed - time.Duration(t.QueueMs)*time.Millisecond
		progress := int(float64(runElapsed) / float64(time.Duration(t.RunMs)*time.Millisecond) * 100)
		progress = minInt(99, maxInt(10, progress+t.ProgressJitter))
		return dto.VideoStatusInProgress, progress, 0
	}
	completedAt := t.CreatedAt.Add(totalRuntime).Unix()
	if t.WillFail {
		return dto.VideoStatusFailed, 100, completedAt
	}
	return dto.VideoStatusCompleted, 100, completedAt
}

func (t *videoTask) toOpenAIVideo(base string) *dto.OpenAIVideo {
	status, progress, completedAt := t.stateAt(time.Now())
	video := dto.NewOpenAIVideo()
	video.ID = t.ID
	video.TaskID = t.ID
	video.Model = t.Model
	video.Status = status
	video.Progress = progress
	video.CreatedAt = t.CreatedAt.Unix()
	video.CompletedAt = completedAt
	video.Seconds = fmt.Sprintf("%.1f", t.DurationSeconds)
	video.Size = t.Resolution
	video.SetMetadata("fps", t.FPS)
	video.SetMetadata("width", t.Width)
	video.SetMetadata("height", t.Height)
	video.SetMetadata("url", fmt.Sprintf("%s/v1/videos/%s/content", base, t.ID))
	if status == dto.VideoStatusFailed {
		video.Error = &dto.OpenAIVideoError{
			Message: "mock video generation failed",
			Code:    "mock_video_failed",
		}
	}
	return video
}

func (t *videoTask) toEvent(status string) videoTaskEvent {
	return videoTaskEvent{
		Timestamp:       time.Now(),
		TaskID:          t.ID,
		Route:           t.Route,
		Model:           t.Model,
		Status:          status,
		DurationSeconds: t.DurationSeconds,
		Resolution:      t.Resolution,
		FPS:             t.FPS,
		WillFail:        t.WillFail,
	}
}
