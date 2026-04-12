package mock

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math/rand"
	"net"
	"net/http"
	_ "net/http/pprof"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/tools/test-workbench/domain"
	"github.com/QuantumNous/new-api/types"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

type Service struct {
	mu        sync.RWMutex
	listeners map[string]*Listener
}

func NewService() *Service {
	return &Service{listeners: make(map[string]*Listener)}
}

func (s *Service) Start(ctx context.Context, env domain.Environment, profile domain.MockProfile) (domain.MockListenerRuntime, error) {
	s.mu.Lock()
	if current, ok := s.listeners[env.ID]; ok && current.runtime().Healthy {
		runtime := current.runtime()
		s.mu.Unlock()
		return runtime, nil
	}
	cfg := newListenerConfig(env, profile)
	listener := newListener(cfg)
	s.listeners[env.ID] = listener
	s.mu.Unlock()

	if err := listener.start(); err != nil {
		s.mu.Lock()
		delete(s.listeners, env.ID)
		s.mu.Unlock()
		return domain.MockListenerRuntime{}, err
	}

	go func() {
		<-listener.done()
		if listener.runtime().Healthy {
			return
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		if current, ok := s.listeners[env.ID]; ok && current == listener && !current.runtime().Healthy {
			delete(s.listeners, env.ID)
		}
	}()

	return listener.runtime(), nil
}

func (s *Service) Stop(ctx context.Context, environmentID string) error {
	s.mu.Lock()
	listener, ok := s.listeners[environmentID]
	if ok {
		delete(s.listeners, environmentID)
	}
	s.mu.Unlock()
	if !ok {
		return nil
	}
	return listener.stop(ctx)
}

func (s *Service) StopAll(ctx context.Context) error {
	s.mu.Lock()
	listeners := make([]*Listener, 0, len(s.listeners))
	for _, item := range s.listeners {
		listeners = append(listeners, item)
	}
	s.listeners = make(map[string]*Listener)
	s.mu.Unlock()

	var firstErr error
	for _, item := range listeners {
		if err := item.stop(ctx); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (s *Service) List() []domain.MockListenerRuntime {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]domain.MockListenerRuntime, 0, len(s.listeners))
	for _, item := range s.listeners {
		out = append(out, item.runtime())
	}
	slicesSortMockRuntimes(out)
	return out
}

func (s *Service) Runtime(environmentID string) (domain.MockListenerRuntime, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	item, ok := s.listeners[environmentID]
	if !ok {
		return domain.MockListenerRuntime{}, false
	}
	return item.runtime(), true
}

func (s *Service) Routes(environmentID string) ([]domain.RouteStatsItem, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	item, ok := s.listeners[environmentID]
	if !ok {
		return nil, fmt.Errorf("mock listener not running")
	}
	return item.routes(), nil
}

func (s *Service) Events(environmentID string) (domain.EventsResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	item, ok := s.listeners[environmentID]
	if !ok {
		return domain.EventsResponse{}, fmt.Errorf("mock listener not running")
	}
	return item.events(), nil
}

func (s *Service) LocalBaseURL(environmentID string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	item, ok := s.listeners[environmentID]
	if !ok {
		return "", false
	}
	return item.cfg.LocalBaseURL, true
}

type listenerConfig struct {
	EnvironmentID string
	ProjectID     string
	Name          string
	BindHost      string
	LocalBaseURL  string
	PublicBaseURL string
	ProfileID     string

	Worker struct {
		Port            int
		RequireAuth     bool
		EnablePprof     bool
		PprofPort       int
		Models          []string
		ManagementToken string
	}

	Random    domain.GlobalRandomConfig
	Chat      domain.TextBehaviorConfig
	Responses domain.TextBehaviorConfig
	Images    domain.ImageBehaviorConfig
	Videos    domain.VideoBehaviorConfig
}

func newListenerConfig(env domain.Environment, profile domain.MockProfile) listenerConfig {
	host := strings.TrimSpace(env.MockBindHost)
	if host == "" {
		host = "0.0.0.0"
	}
	advertisedHost := host
	if host == "" || host == "0.0.0.0" || host == "::" {
		advertisedHost = "127.0.0.1"
	}
	cfg := listenerConfig{
		EnvironmentID: env.ID,
		ProjectID:     env.ProjectID,
		Name:          env.Name,
		BindHost:      host,
		LocalBaseURL:  fmt.Sprintf("http://127.0.0.1:%d", env.MockPort),
		PublicBaseURL: fmt.Sprintf("http://%s:%d", advertisedHost, env.MockPort),
		ProfileID:     profile.ID,
		Random:        profile.Config.Random,
		Chat:          profile.Config.Chat,
		Responses:     profile.Config.Responses,
		Images:        profile.Config.Images,
		Videos:        profile.Config.Videos,
	}
	cfg.Worker.Port = env.MockPort
	cfg.Worker.RequireAuth = env.MockRequireAuth
	cfg.Worker.EnablePprof = profile.Config.EnablePprof
	cfg.Worker.PprofPort = profile.Config.PprofPort
	cfg.Worker.Models = append([]string(nil), profile.Config.Models...)
	cfg.Worker.ManagementToken = env.MockAuthToken
	return cfg
}

type Listener struct {
	cfg       listenerConfig
	rng       *lockedRand
	startedAt time.Time
	stats     *statsCollector
	assets    *imageAssetStore
	tasks     *videoTaskStore

	server      *http.Server
	pprofServer *http.Server
	stopCh      chan struct{}
	doneCh      chan struct{}
	healthy     atomic.Bool

	stopOnce sync.Once
}

func newListener(cfg listenerConfig) *Listener {
	startedAt := time.Now()
	return &Listener{
		cfg:       cfg,
		rng:       newLockedRand(cfg),
		startedAt: startedAt,
		stats:     newStatsCollector(startedAt),
		assets:    &imageAssetStore{items: make(map[string]*imageAsset)},
		tasks:     &videoTaskStore{tasks: make(map[string]*videoTask)},
		stopCh:    make(chan struct{}),
		doneCh:    make(chan struct{}),
	}
}

func (l *Listener) start() error {
	addr := net.JoinHostPort(l.cfg.BindHost, strconv.Itoa(l.cfg.Worker.Port))
	server := &http.Server{
		Addr:              addr,
		Handler:           l.handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	l.server = server
	l.healthy.Store(true)

	go func() {
		defer close(l.doneCh)
		if err := server.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			l.healthy.Store(false)
		}
	}()

	if l.cfg.Worker.EnablePprof {
		pprofServer := &http.Server{
			Addr:              net.JoinHostPort("127.0.0.1", strconv.Itoa(l.cfg.Worker.PprofPort)),
			Handler:           http.DefaultServeMux,
			ReadHeaderTimeout: 5 * time.Second,
		}
		l.pprofServer = pprofServer
		go func() {
			if err := pprofServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				l.healthy.Store(false)
			}
		}()
	}

	go l.cleanupLoop()
	return nil
}

func (l *Listener) stop(ctx context.Context) error {
	var firstErr error
	l.stopOnce.Do(func() {
		close(l.stopCh)
		l.healthy.Store(false)
		if l.server != nil {
			if err := l.server.Shutdown(ctx); err != nil && firstErr == nil {
				firstErr = err
			}
		}
		if l.pprofServer != nil {
			if err := l.pprofServer.Shutdown(ctx); err != nil && firstErr == nil {
				firstErr = err
			}
		}
	})
	select {
	case <-l.doneCh:
	case <-ctx.Done():
		if firstErr == nil {
			firstErr = ctx.Err()
		}
	}
	return firstErr
}

func (l *Listener) done() <-chan struct{} {
	return l.doneCh
}

func (l *Listener) cleanupLoop() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-l.stopCh:
			return
		case <-ticker.C:
			l.assets.prune(time.Now().Add(-10 * time.Minute))
			l.tasks.prune(time.Now().Add(-30 * time.Minute))
		}
	}
}

func (l *Listener) runtime() domain.MockListenerRuntime {
	status := "stopped"
	if l.healthy.Load() {
		status = "running"
	}
	return domain.MockListenerRuntime{
		EnvironmentID: l.cfg.EnvironmentID,
		ProjectID:     l.cfg.ProjectID,
		Name:          l.cfg.Name,
		Status:        status,
		ListenAddress: net.JoinHostPort(l.cfg.BindHost, strconv.Itoa(l.cfg.Worker.Port)),
		LocalBaseURL:  l.cfg.LocalBaseURL,
		PublicBaseURL: l.cfg.PublicBaseURL,
		RequireAuth:   l.cfg.Worker.RequireAuth,
		Healthy:       l.healthy.Load(),
		StartedAt:     l.startedAt,
		ProfileID:     l.cfg.ProfileID,
		Summary:       l.stats.summary(l.tasks.count()),
	}
}

func (l *Listener) routes() []domain.RouteStatsItem {
	return l.stats.routesSnapshot()
}

func (l *Listener) events() domain.EventsResponse {
	return l.stats.eventsSnapshot()
}

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
	QueueMS         int
	RunMS           int
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

func (l *Listener) handler() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", l.handleHealth)
	mux.HandleFunc("GET /__mock/assets/images/{asset_id}", l.handleImageAsset)

	mux.HandleFunc("GET /v1/models", l.wrapPublic("models.list", l.handleListModels))
	mux.HandleFunc("GET /v1/models/{id}", l.wrapPublic("models.get", l.handleGetModel))
	mux.HandleFunc("POST /v1/completions", l.wrapPublic("completions", l.handleCompletions))
	mux.HandleFunc("POST /v1/chat/completions", l.wrapPublic("chat.completions", l.handleChatCompletions))
	mux.HandleFunc("POST /v1/responses", l.wrapPublic("responses", l.handleResponses))
	mux.HandleFunc("POST /v1/responses/compact", l.wrapPublic("responses.compact", l.handleResponses))
	mux.HandleFunc("POST /v1/images/generations", l.wrapPublic("images.generations", l.handleImages))
	mux.HandleFunc("POST /v1/images/edits", l.wrapPublic("images.edits", l.handleImages))
	mux.HandleFunc("POST /v1/edits", l.wrapPublic("images.compat_edits", l.handleImages))
	mux.HandleFunc("POST /v1/videos", l.wrapPublic("videos.create", l.handleOpenAIVideoCreate))
	mux.HandleFunc("GET /v1/videos/{task_id}", l.wrapPublic("videos.get", l.handleOpenAIVideoGet))
	mux.HandleFunc("GET /v1/videos/{task_id}/content", l.wrapPublic("videos.content", l.handleVideoContent))
	mux.HandleFunc("POST /v1/video/generations", l.wrapPublic("video.generations.create", l.handleLegacyVideoCreate))
	mux.HandleFunc("GET /v1/video/generations/{task_id}", l.wrapPublic("video.generations.get", l.handleLegacyVideoGet))
	return mux
}

func (l *Listener) wrapPublic(route string, next func(http.ResponseWriter, *http.Request, *publicCallMeta)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		meta := &publicCallMeta{Route: route}
		writer := &trackingWriter{ResponseWriter: w}
		l.stats.begin(route)

		defer func() {
			if recovered := recover(); recovered != nil {
				meta.Error = fmt.Sprintf("%v", recovered)
				writeOpenAIError(writer, http.StatusInternalServerError, "server_error", "mock upstream panic", "panic", "")
			}
			status := writer.statusCode()
			l.stats.finish(route, status, time.Since(started).Milliseconds(), domain.RequestEvent{
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

		if l.cfg.Worker.RequireAuth && strings.HasPrefix(r.URL.Path, "/v1/") && !hasValidAuthorization(r, l.cfg.Worker.ManagementToken) {
			meta.Error = "invalid api key"
			writeOpenAIError(writer, http.StatusUnauthorized, "authentication_error", "invalid api key", "invalid_api_key", "")
			return
		}

		latency := l.rng.intRange(l.cfg.Random.LatencyMS)
		if latency > 0 {
			time.Sleep(time.Duration(latency) * time.Millisecond)
		}

		if status, errType, msg, code, ok := l.injectedError(); ok {
			meta.Error = msg
			writeOpenAIError(writer, status, errType, msg, code, "")
			return
		}

		next(writer, r, meta)
	}
}

func (l *Listener) injectedError() (int, string, string, string, bool) {
	switch {
	case l.rng.bool(l.cfg.Random.TimeoutRate):
		return http.StatusGatewayTimeout, "timeout_error", "mock upstream timeout", "mock_timeout", true
	case l.rng.bool(l.cfg.Random.TooManyRequestsRate):
		return http.StatusTooManyRequests, "rate_limit_error", "mock upstream rate limit", "mock_rate_limit", true
	case l.rng.bool(l.cfg.Random.ServerErrorRate):
		return http.StatusInternalServerError, "server_error", "mock upstream server error", "mock_server_error", true
	case l.rng.bool(l.cfg.Random.ErrorRate):
		return http.StatusBadRequest, "invalid_request_error", "mock invalid request", "mock_invalid_request", true
	default:
		return 0, "", "", "", false
	}
}

func (l *Listener) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"service": "test-workbench-mock",
	})
}

func (l *Listener) handleListModels(w http.ResponseWriter, r *http.Request, meta *publicCallMeta) {
	items := make([]modelObject, 0, len(l.cfg.Worker.Models))
	now := nowUnix()
	for _, modelID := range l.cfg.Worker.Models {
		items = append(items, modelObject{
			ID:      modelID,
			Object:  "model",
			Created: now,
			OwnedBy: "test-workbench",
		})
	}
	writeJSON(w, http.StatusOK, modelsResponse{Object: "list", Data: items})
}

func (l *Listener) handleGetModel(w http.ResponseWriter, r *http.Request, meta *publicCallMeta) {
	modelID := strings.TrimSpace(r.PathValue("id"))
	if modelID == "" {
		modelID = l.defaultModel("")
	}
	meta.Model = modelID
	writeJSON(w, http.StatusOK, modelObject{
		ID:      modelID,
		Object:  "model",
		Created: nowUnix(),
		OwnedBy: "test-workbench",
	})
}

func (l *Listener) handleChatCompletions(w http.ResponseWriter, r *http.Request, meta *publicCallMeta) {
	var req chatCompletionRequest
	if err := readJSON(r, &req); err != nil {
		meta.Error = err.Error()
		writeOpenAIError(w, http.StatusBadRequest, "invalid_request_error", err.Error(), "bad_json", "")
		return
	}
	req.Model = l.defaultModel(req.Model)
	meta.Model = req.Model
	meta.Stream = req.Stream && l.cfg.Chat.AllowStream

	completionTokens := l.rng.intRange(l.cfg.Chat.TextTokens)
	usage := l.makeUsage(completionTokens)
	systemFingerprint := l.systemFingerprint(l.cfg.Chat.SystemFingerprintChance)
	hasToolResult := hasChatToolResult(req.Messages)
	shouldToolCall := len(req.Tools) > 0 && !hasToolResult && l.rng.bool(l.cfg.Chat.ToolCallProbability)

	if meta.Stream {
		startSSE(w)
		l.stats.setSSEActive(1)
		defer l.stats.setSSEActive(-1)

		if shouldToolCall {
			toolCalls := l.generateToolCalls(l.cfg.Chat)
			_ = writeSSEEvent(w, l.chatStreamChunk(req.Model, systemFingerprint, dto.ChatCompletionsStreamResponseChoiceDelta{Role: "assistant", ToolCalls: toolCalls}, nil, nil))
			finish := "tool_calls"
			usagePtr := l.maybeUsage(l.cfg.Chat.UsageProbability, usage)
			_ = writeSSEEvent(w, l.chatStreamChunk(req.Model, systemFingerprint, dto.ChatCompletionsStreamResponseChoiceDelta{}, &finish, usagePtr))
			_ = writeSSEDone(w)
			return
		}

		text := l.rng.randomText(completionTokens)
		_ = writeSSEEvent(w, l.chatStreamChunk(req.Model, systemFingerprint, dto.ChatCompletionsStreamResponseChoiceDelta{Role: "assistant"}, nil, nil))
		for _, piece := range chunkString(text, l.rng.intRange(l.cfg.Random.DefaultStreamChunks)) {
			content := piece
			_ = writeSSEEvent(w, l.chatStreamChunk(req.Model, systemFingerprint, dto.ChatCompletionsStreamResponseChoiceDelta{Content: &content}, nil, nil))
		}
		finish := "stop"
		usagePtr := l.maybeUsage(l.cfg.Chat.UsageProbability, usage)
		_ = writeSSEEvent(w, l.chatStreamChunk(req.Model, systemFingerprint, dto.ChatCompletionsStreamResponseChoiceDelta{}, &finish, usagePtr))
		_ = writeSSEDone(w)
		return
	}

	message := dto.Message{Role: "assistant"}
	finishReason := "stop"
	if shouldToolCall {
		toolCalls := l.generateToolCalls(l.cfg.Chat)
		payload, _ := common.Marshal(toolCalls)
		message.Content = ""
		message.ToolCalls = payload
		finishReason = "tool_calls"
	} else {
		message.Content = l.rng.randomText(completionTokens)
	}

	writeJSON(w, http.StatusOK, chatCompletionResponse{
		ID:                l.rng.randomID("chatcmpl"),
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
	})
}

func (l *Listener) handleCompletions(w http.ResponseWriter, r *http.Request, meta *publicCallMeta) {
	var req chatCompletionRequest
	if err := readJSON(r, &req); err != nil {
		meta.Error = err.Error()
		writeOpenAIError(w, http.StatusBadRequest, "invalid_request_error", err.Error(), "bad_json", "")
		return
	}
	req.Model = l.defaultModel(req.Model)
	meta.Model = req.Model
	meta.Stream = req.Stream && l.cfg.Chat.AllowStream
	completionTokens := l.rng.intRange(l.cfg.Chat.TextTokens)
	usage := l.makeUsage(completionTokens)
	text := l.rng.randomText(completionTokens)

	if meta.Stream {
		startSSE(w)
		l.stats.setSSEActive(1)
		defer l.stats.setSSEActive(-1)
		for _, piece := range chunkString(text, l.rng.intRange(l.cfg.Random.DefaultStreamChunks)) {
			_ = writeSSEEvent(w, map[string]any{
				"id":      l.rng.randomID("cmpl"),
				"object":  "text_completion",
				"created": nowUnix(),
				"model":   req.Model,
				"choices": []map[string]any{{"text": piece, "index": 0, "finish_reason": ""}},
			})
		}
		_ = writeSSEEvent(w, map[string]any{
			"id":      l.rng.randomID("cmpl"),
			"object":  "text_completion",
			"created": nowUnix(),
			"model":   req.Model,
			"choices": []map[string]any{{"text": "", "index": 0, "finish_reason": "stop"}},
			"usage":   l.maybeUsage(l.cfg.Chat.UsageProbability, usage),
		})
		_ = writeSSEDone(w)
		return
	}

	writeJSON(w, http.StatusOK, completionResponse{
		ID:      l.rng.randomID("cmpl"),
		Object:  "text_completion",
		Created: nowUnix(),
		Model:   req.Model,
		Choices: []completionChoice{{Text: text, Index: 0, FinishReason: "stop"}},
		Usage:   usage,
	})
}

func (l *Listener) handleResponses(w http.ResponseWriter, r *http.Request, meta *publicCallMeta) {
	var req responsesRequest
	if err := readJSON(r, &req); err != nil {
		meta.Error = err.Error()
		writeOpenAIError(w, http.StatusBadRequest, "invalid_request_error", err.Error(), "bad_json", "")
		return
	}
	req.Model = l.defaultModel(req.Model)
	meta.Model = req.Model
	meta.Stream = req.Stream && l.cfg.Responses.AllowStream

	completionTokens := l.rng.intRange(l.cfg.Responses.TextTokens)
	usage := l.makeUsage(completionTokens)
	hasToolResult := inputContainsToolResult(req.Input)
	shouldToolCall := len(req.Tools) > 0 && !hasToolResult && l.rng.bool(l.cfg.Responses.ToolCallProbability)
	responseID := l.rng.randomID("resp")

	if shouldToolCall {
		toolCalls := l.generateToolCalls(l.cfg.Responses)
		toolItem := l.responsesToolOutput(toolCalls[0], strings.HasPrefix(toolCalls[0].Function.Name, "mcp."))
		response := dto.OpenAIResponsesResponse{
			ID:                responseID,
			Object:            "response",
			CreatedAt:         int(nowUnix()),
			Status:            json.RawMessage(`"completed"`),
			Model:             req.Model,
			Output:            []dto.ResponsesOutput{toolItem},
			ParallelToolCalls: false,
			Usage:             l.maybeUsage(l.cfg.Responses.UsageProbability, usage),
		}
		if meta.Stream {
			startSSE(w)
			l.stats.setSSEActive(1)
			defer l.stats.setSSEActive(-1)
			_ = writeSSEEvent(w, dto.ResponsesStreamResponse{Type: dto.ResponsesOutputTypeItemAdded, Item: &toolItem})
			_ = writeSSEEvent(w, dto.ResponsesStreamResponse{Type: dto.ResponsesOutputTypeItemDone, Item: &toolItem})
			_ = writeSSEEvent(w, dto.ResponsesStreamResponse{Type: "response.completed", Response: &response})
			_ = writeSSEDone(w)
			return
		}
		writeJSON(w, http.StatusOK, response)
		return
	}

	text := l.rng.randomText(completionTokens)
	output := dto.ResponsesOutput{
		Type:   "message",
		ID:     l.rng.randomID("msg"),
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
		Usage:             l.maybeUsage(l.cfg.Responses.UsageProbability, usage),
	}
	if meta.Stream {
		startSSE(w)
		l.stats.setSSEActive(1)
		defer l.stats.setSSEActive(-1)
		for _, piece := range chunkString(text, l.rng.intRange(l.cfg.Random.DefaultStreamChunks)) {
			_ = writeSSEEvent(w, dto.ResponsesStreamResponse{Type: "response.output_text.delta", Delta: piece})
		}
		_ = writeSSEEvent(w, dto.ResponsesStreamResponse{Type: "response.completed", Response: &response})
		_ = writeSSEDone(w)
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (l *Listener) handleImages(w http.ResponseWriter, r *http.Request, meta *publicCallMeta) {
	req, err := l.parseImageRequest(r)
	if err != nil {
		meta.Error = err.Error()
		writeOpenAIError(w, http.StatusBadRequest, "invalid_request_error", err.Error(), "bad_image_request", "")
		return
	}
	req.Model = l.defaultModel(req.Model)
	meta.Model = req.Model

	size := req.Size
	if size == "" {
		size = l.rng.pick(l.cfg.Images.Sizes, "1024x1024")
	}
	width, height := parseResolution(size, 1024, 1024)
	count := req.N
	if count <= 0 {
		count = l.rng.intRange(l.cfg.Images.ImageCount)
	}
	if count > l.cfg.Images.ImageCount.Max {
		count = l.cfg.Images.ImageCount.Max
	}

	responseFormat := req.ResponseFormat
	if responseFormat == "" {
		if l.rng.bool(l.cfg.Images.ResponseURLRate) {
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
		imageBytes, err := l.generateImageBytes(width, height, prompt)
		if err != nil {
			meta.Error = err.Error()
			writeOpenAIError(w, http.StatusInternalServerError, "server_error", err.Error(), "image_generation_failed", "")
			return
		}
		item := dto.ImageData{RevisedPrompt: prompt + " [simulated]"}
		if responseFormat == "b64_json" {
			item.B64Json = base64.StdEncoding.EncodeToString(imageBytes)
		} else {
			assetID := l.rng.randomID("img")
			l.assets.put(&imageAsset{ID: assetID, CreatedAt: time.Now(), ContentType: "image/png", Data: imageBytes})
			item.Url = fmt.Sprintf("%s/__mock/assets/images/%s", publicBaseURL(r), assetID)
		}
		data = append(data, item)
	}

	writeJSON(w, http.StatusOK, dto.ImageResponse{Data: data, Created: nowUnix()})
}

func (l *Listener) handleImageAsset(w http.ResponseWriter, r *http.Request) {
	assetID := r.PathValue("asset_id")
	asset, ok := l.assets.get(assetID)
	if !ok {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", asset.ContentType)
	w.Header().Set("Cache-Control", "public, max-age=300")
	_, _ = w.Write(asset.Data)
}

func (l *Listener) handleOpenAIVideoCreate(w http.ResponseWriter, r *http.Request, meta *publicCallMeta) {
	req, err := l.parseVideoRequest(r)
	if err != nil {
		meta.Error = err.Error()
		writeOpenAIError(w, http.StatusBadRequest, "invalid_request_error", err.Error(), "bad_video_request", "")
		return
	}
	task := l.createVideoTask("openai", req)
	meta.Model = task.Model
	l.stats.addVideoEvent(task.toEvent("queued"))
	writeJSON(w, http.StatusOK, task.toOpenAIVideo(publicBaseURL(r)))
}

func (l *Listener) handleOpenAIVideoGet(w http.ResponseWriter, r *http.Request, meta *publicCallMeta) {
	taskID := r.PathValue("task_id")
	task, ok := l.tasks.get(taskID)
	if !ok {
		meta.Error = "video task not found"
		writeOpenAIError(w, http.StatusNotFound, "invalid_request_error", "video task not found", "task_not_found", "task_id")
		return
	}
	meta.Model = task.Model
	writeJSON(w, http.StatusOK, task.toOpenAIVideo(publicBaseURL(r)))
}

func (l *Listener) handleLegacyVideoCreate(w http.ResponseWriter, r *http.Request, meta *publicCallMeta) {
	req, err := l.parseVideoRequest(r)
	if err != nil {
		meta.Error = err.Error()
		writeOpenAIError(w, http.StatusBadRequest, "invalid_request_error", err.Error(), "bad_video_request", "")
		return
	}
	task := l.createVideoTask("legacy", req)
	meta.Model = task.Model
	l.stats.addVideoEvent(task.toEvent("queued"))
	writeJSON(w, http.StatusOK, dto.VideoResponse{TaskId: task.ID, Status: "queued"})
}

func (l *Listener) handleLegacyVideoGet(w http.ResponseWriter, r *http.Request, meta *publicCallMeta) {
	taskID := r.PathValue("task_id")
	task, ok := l.tasks.get(taskID)
	if !ok {
		meta.Error = "video task not found"
		writeOpenAIError(w, http.StatusNotFound, "invalid_request_error", "video task not found", "task_not_found", "task_id")
		return
	}
	meta.Model = task.Model
	status, progress, _ := task.stateAt(time.Now())
	resp := dto.VideoTaskResponse{TaskId: task.ID, Status: legacyVideoStatus(status)}
	if status == dto.VideoStatusCompleted {
		resp.Url = fmt.Sprintf("%s/v1/videos/%s/content", publicBaseURL(r), task.ID)
		resp.Format = "mp4"
		resp.Metadata = &dto.VideoTaskMetadata{Duration: task.DurationSeconds, Fps: task.FPS, Width: task.Width, Height: task.Height}
	}
	if status == dto.VideoStatusFailed {
		resp.Error = &dto.VideoTaskError{Code: http.StatusInternalServerError, Message: "mock video generation failed"}
	}
	if progress > 0 && resp.Metadata == nil {
		resp.Metadata = &dto.VideoTaskMetadata{Duration: task.DurationSeconds, Fps: task.FPS, Width: task.Width, Height: task.Height}
	}
	writeJSON(w, http.StatusOK, resp)
}

func (l *Listener) handleVideoContent(w http.ResponseWriter, r *http.Request, meta *publicCallMeta) {
	taskID := r.PathValue("task_id")
	task, ok := l.tasks.get(taskID)
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

	content := l.generateVideoBytes(task.ContentBytes)
	w.Header().Set("Content-Type", "video/mp4")
	w.Header().Set("Cache-Control", "public, max-age=300")
	_, _ = w.Write(content)
}

func (l *Listener) parseImageRequest(r *http.Request) (imageRequestCompat, error) {
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

func (l *Listener) parseVideoRequest(r *http.Request) (videoRequestCompat, error) {
	var req videoRequestCompat
	if err := readJSON(r, &req); err != nil {
		return videoRequestCompat{}, err
	}
	if req.Duration <= 0 {
		req.Duration = l.rng.floatRange(l.cfg.Videos.DurationsSeconds)
	}
	if req.Size == "" {
		req.Size = l.rng.pick(l.cfg.Videos.Resolutions, "1280x720")
	}
	if req.FPS <= 0 {
		req.FPS = l.rng.intRange(l.cfg.Videos.FPS)
	}
	return req, nil
}

func (l *Listener) createVideoTask(route string, req videoRequestCompat) *videoTask {
	size := req.Size
	if size == "" {
		size = l.rng.pick(l.cfg.Videos.Resolutions, "1280x720")
	}
	width, height := parseResolution(size, 1280, 720)
	task := &videoTask{
		ID:              l.rng.randomID("video"),
		Route:           route,
		Model:           l.defaultModel(req.Model),
		Prompt:          req.Prompt,
		CreatedAt:       time.Now(),
		QueueMS:         l.rng.intRange(l.cfg.Videos.PollIntervalMS),
		RunMS:           l.rng.intRange(l.cfg.Videos.PollIntervalMS) * 3,
		DurationSeconds: req.Duration,
		Resolution:      size,
		Width:           width,
		Height:          height,
		FPS:             req.FPS,
		WillFail:        l.rng.bool(l.cfg.Videos.FailureRate),
		ContentBytes:    l.rng.intRange(l.cfg.Videos.VideoBytes),
		ProgressJitter:  l.rng.intRange(l.cfg.Videos.ProgressJitter),
	}
	l.tasks.put(task)
	return task
}

func (l *Listener) makeUsage(completionTokens int) dto.Usage {
	promptTokens := maxInt(8, completionTokens/2)
	promptTokens += l.rng.intn(24)
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
		PromptTokensDetails: dto.InputTokenDetails{TextTokens: promptTokens},
		CompletionTokenDetails: dto.OutputTokenDetails{
			TextTokens:      completionTokens,
			ReasoningTokens: completionTokens / 8,
		},
	}
}

func (l *Listener) maybeUsage(probability float64, usage dto.Usage) *dto.Usage {
	if l.rng.bool(probability) {
		return &usage
	}
	return nil
}

func (l *Listener) generateToolCalls(cfg domain.TextBehaviorConfig) []dto.ToolCallResponse {
	count := l.rng.intRange(cfg.ToolCallCount)
	result := make([]dto.ToolCallResponse, 0, count)
	for i := 0; i < count; i++ {
		isMCP := l.rng.bool(cfg.MCPToolProbability)
		namePrefix := "tool"
		if isMCP {
			namePrefix = "mcp"
		}
		name := fmt.Sprintf("%s.lookup_%d", namePrefix, 1+l.rng.intn(9))
		result = append(result, dto.ToolCallResponse{
			Index: loIntPtr(i),
			ID:    l.rng.randomID("call"),
			Type:  "function",
			Function: dto.FunctionResponse{
				Name:      name,
				Arguments: l.randomArguments(cfg.ToolArgumentsBytes),
			},
		})
	}
	return result
}

func (l *Listener) randomArguments(sizeRange domain.IntRange) string {
	target := l.rng.intRange(sizeRange)
	payload := map[string]any{
		"query":    l.rng.randomText(maxInt(4, target/24)),
		"limit":    1 + l.rng.intn(10),
		"trace_id": l.rng.randomID("trace"),
	}
	data, _ := common.Marshal(payload)
	for len(data) < target {
		payload[fmt.Sprintf("extra_%d", len(payload))] = l.rng.randomText(4 + l.rng.intn(12))
		data, _ = common.Marshal(payload)
	}
	return string(data)
}

func (l *Listener) chatStreamChunk(model string, systemFingerprint *string, delta dto.ChatCompletionsStreamResponseChoiceDelta, finish *string, usage *dto.Usage) dto.ChatCompletionsStreamResponse {
	return dto.ChatCompletionsStreamResponse{
		Id:                l.rng.randomID("chatcmpl"),
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

func (l *Listener) responsesToolOutput(tool dto.ToolCallResponse, isMCP bool) dto.ResponsesOutput {
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

func (l *Listener) defaultModel(input string) string {
	if strings.TrimSpace(input) != "" {
		return strings.TrimSpace(input)
	}
	return l.rng.pick(l.cfg.Worker.Models, "gpt-4o-mini")
}

func (l *Listener) systemFingerprint(probability float64) *string {
	if !l.rng.bool(probability) {
		return nil
	}
	value := l.rng.randomID("fp")
	return &value
}

func (l *Listener) generateImageBytes(width int, height int, prompt string) ([]byte, error) {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	palette := l.rng.shuffleStrings(l.cfg.Images.BackgroundPalette)
	bg := parseHexColor(firstOrDefault(palette, "#0F172A"))
	draw.Draw(img, img.Bounds(), &image.Uniform{C: bg}, image.Point{}, draw.Src)

	blockCount := maxInt(8, l.rng.intRange(l.cfg.Images.ImageBytes)/8000)
	for i := 0; i < blockCount; i++ {
		x0 := l.rng.intn(maxInt(1, width-1))
		y0 := l.rng.intn(maxInt(1, height-1))
		x1 := minInt(width, x0+maxInt(12, l.rng.intn(maxInt(16, width/3))))
		y1 := minInt(height, y0+maxInt(12, l.rng.intn(maxInt(16, height/3))))
		shapeColor := randomRGBA(l.rng)
		draw.Draw(img, image.Rect(x0, y0, x1, y1), &image.Uniform{C: shapeColor}, image.Point{}, draw.Over)
	}

	if l.rng.bool(l.cfg.Images.WatermarkRate) {
		label := l.rng.pick(l.cfg.Images.WatermarkTexts, "mock")
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

func (l *Listener) generateVideoBytes(size int) []byte {
	if size < 64 {
		size = 64
	}
	buffer := make([]byte, size)
	header := []byte{0x00, 0x00, 0x00, 0x18, 'f', 't', 'y', 'p', 'm', 'p', '4', '2', 0x00, 0x00, 0x00, 0x00, 'm', 'p', '4', '2', 'i', 's', 'o', 'm'}
	copy(buffer, header)
	l.rng.fillBytes(buffer[len(header):])
	return buffer
}

type lockedRand struct {
	mu  sync.Mutex
	rnd *rand.Rand
}

func newLockedRand(cfg listenerConfig) *lockedRand {
	seed := time.Now().UnixNano()
	if cfg.Random.Mode == "seeded" {
		seed = cfg.Random.Seed
	}
	return &lockedRand{rnd: rand.New(rand.NewSource(seed))}
}

func (r *lockedRand) intn(max int) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	if max <= 0 {
		return 0
	}
	return r.rnd.Intn(max)
}

func (r *lockedRand) intRange(v domain.IntRange) int {
	if v.Max <= v.Min {
		return v.Min
	}
	return v.Min + r.intn(v.Max-v.Min+1)
}

func (r *lockedRand) floatRange(v domain.FloatRange) float64 {
	if v.Max <= v.Min {
		return v.Min
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return v.Min + r.rnd.Float64()*(v.Max-v.Min)
}

func (r *lockedRand) bool(probability float64) bool {
	if probability <= 0 {
		return false
	}
	if probability >= 1 {
		return true
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.rnd.Float64() < probability
}

func (r *lockedRand) pick(values []string, fallback string) string {
	if len(values) == 0 {
		return fallback
	}
	return values[r.intn(len(values))]
}

func (r *lockedRand) shuffleStrings(values []string) []string {
	out := append([]string(nil), values...)
	r.mu.Lock()
	defer r.mu.Unlock()
	r.rnd.Shuffle(len(out), func(i, j int) { out[i], out[j] = out[j], out[i] })
	return out
}

func (r *lockedRand) fillBytes(buf []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()
	_, _ = r.rnd.Read(buf)
}

func (r *lockedRand) randomID(prefix string) string {
	return fmt.Sprintf("%s_%08x", prefix, r.intn(1<<30))
}

func (r *lockedRand) randomText(tokens int) string {
	if tokens <= 0 {
		return ""
	}
	words := []string{
		"mock", "new", "api", "stream", "token", "latency", "vector", "image", "video", "assistant",
		"upstream", "simulated", "payload", "random", "system", "response", "tool", "context", "model", "reasoning",
		"gateway", "worker", "control", "pressure", "cluster", "session", "trace", "result", "object", "message",
	}
	var builder strings.Builder
	for i := 0; i < tokens; i++ {
		if i > 0 {
			if i%17 == 0 {
				builder.WriteString(". ")
			} else {
				builder.WriteByte(' ')
			}
		}
		builder.WriteString(words[r.intn(len(words))])
		if i%31 == 0 && i > 0 {
			builder.WriteString(fmt.Sprintf("-%d", 100+r.intn(900)))
		}
	}
	if !strings.HasSuffix(builder.String(), ".") {
		builder.WriteByte('.')
	}
	return builder.String()
}

func readJSON(r *http.Request, target any) error {
	if r.Body == nil {
		return errors.New("empty request body")
	}
	return common.DecodeJson(r.Body, target)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	data, err := common.Marshal(payload)
	if err != nil {
		writeOpenAIError(w, http.StatusInternalServerError, "server_error", "json marshal failed", "json_marshal_failed", "")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(data)
}

func writeOpenAIError(w http.ResponseWriter, status int, errorType string, message string, code any, param string) {
	writeJSON(w, status, map[string]any{
		"error": types.OpenAIError{Message: message, Type: errorType, Param: param, Code: code},
	})
}

func writeSSEEvent(w http.ResponseWriter, payload any) error {
	data, err := common.Marshal(payload)
	if err != nil {
		return err
	}
	if _, err = fmt.Fprintf(w, "data: %s\n\n", data); err != nil {
		return err
	}
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
	return nil
}

func writeSSEDone(w http.ResponseWriter) error {
	if _, err := fmt.Fprint(w, "data: [DONE]\n\n"); err != nil {
		return err
	}
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
	return nil
}

func startSSE(w http.ResponseWriter) {
	headers := w.Header()
	headers.Set("Content-Type", "text/event-stream")
	headers.Set("Cache-Control", "no-cache")
	headers.Set("Connection", "keep-alive")
	headers.Set("X-Accel-Buffering", "no")
}

func nowUnix() int64 {
	return time.Now().Unix()
}

func hasValidAuthorization(r *http.Request, token string) bool {
	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	if auth != "" {
		parts := strings.SplitN(auth, " ", 2)
		if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") && strings.TrimSpace(parts[1]) == token {
			return true
		}
		if auth == token {
			return true
		}
	}
	return strings.TrimSpace(r.Header.Get("x-api-key")) == token || strings.TrimSpace(r.Header.Get("api-key")) == token
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
	d := &font.Drawer{Dst: img, Src: image.NewUniform(fg), Face: basicfont.Face7x13, Dot: fixed.P(x, y)}
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

func (t *videoTask) stateAt(now time.Time) (string, int, int64) {
	elapsed := now.Sub(t.CreatedAt)
	if elapsed <= time.Duration(t.QueueMS)*time.Millisecond {
		progress := minInt(8+t.ProgressJitter, 12)
		return dto.VideoStatusQueued, progress, 0
	}
	totalRuntime := time.Duration(t.QueueMS+t.RunMS) * time.Millisecond
	if elapsed < totalRuntime {
		runElapsed := elapsed - time.Duration(t.QueueMS)*time.Millisecond
		progress := int(float64(runElapsed) / float64(time.Duration(t.RunMS)*time.Millisecond) * 100)
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
		video.Error = &dto.OpenAIVideoError{Message: "mock video generation failed", Code: "mock_video_failed"}
	}
	return video
}

func (t *videoTask) toEvent(status string) domain.VideoTaskEvent {
	return domain.VideoTaskEvent{
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

func slicesSortMockRuntimes(items []domain.MockListenerRuntime) {
	sort.Slice(items, func(i, j int) bool {
		if items[i].StartedAt.Equal(items[j].StartedAt) {
			return items[i].EnvironmentID < items[j].EnvironmentID
		}
		return items[i].StartedAt.Before(items[j].StartedAt)
	})
}
