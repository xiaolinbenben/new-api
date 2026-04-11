package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/stretchr/testify/require"
)

func TestWorkerEndpoints(t *testing.T) {
	t.Parallel()

	cfg := defaultConfig()
	cfg.Random.Mode = "seeded"
	cfg.Random.Seed = 42
	cfg.Random.ErrorRate = 0
	cfg.Random.ServerErrorRate = 0
	cfg.Random.TimeoutRate = 0
	cfg.Random.TooManyRequestsRate = 0
	cfg.Worker.RequireAuth = false
	cfg.Videos.PollIntervalMs = intRange{Min: 10, Max: 10}
	cfg.Chat.ToolCallProbability = 0
	cfg.Responses.ToolCallProbability = 0

	app := newWorkerApp(cfg)
	server := httptest.NewServer(app.handler())
	defer server.Close()

	t.Run("models", func(t *testing.T) {
		resp, err := http.Get(server.URL + "/v1/models")
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)

		var body modelsResponse
		require.NoError(t, common.DecodeJson(resp.Body, &body))
		require.NotEmpty(t, body.Data)
	})

	t.Run("chat completions", func(t *testing.T) {
		payload := `{"model":"gpt-4o-mini","messages":[{"role":"user","content":"hello"}]}`
		resp, err := http.Post(server.URL+"/v1/chat/completions", "application/json", strings.NewReader(payload))
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)

		var body chatCompletionResponse
		require.NoError(t, common.DecodeJson(resp.Body, &body))
		require.Len(t, body.Choices, 1)
		require.Equal(t, "chat.completion", body.Object)
		require.NotZero(t, body.Usage.TotalTokens)
	})

	t.Run("responses stream", func(t *testing.T) {
		payload := `{"model":"gpt-4o-mini","stream":true,"input":[{"role":"user","content":"hi"}]}`
		resp, err := http.Post(server.URL+"/v1/responses", "application/json", strings.NewReader(payload))
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)

		data, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		text := string(data)
		require.Contains(t, text, "response.output_text.delta")
		require.Contains(t, text, "response.completed")
	})

	t.Run("images url", func(t *testing.T) {
		payload := `{"model":"gpt-image-1","prompt":"a skyline","response_format":"url"}`
		resp, err := http.Post(server.URL+"/v1/images/generations", "application/json", strings.NewReader(payload))
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)

		var body dto.ImageResponse
		require.NoError(t, common.DecodeJson(resp.Body, &body))
		require.NotEmpty(t, body.Data)
		require.NotEmpty(t, body.Data[0].Url)

		assetResp, err := http.Get(body.Data[0].Url)
		require.NoError(t, err)
		defer assetResp.Body.Close()
		require.Equal(t, http.StatusOK, assetResp.StatusCode)
		assetBytes, err := io.ReadAll(assetResp.Body)
		require.NoError(t, err)
		require.NotEmpty(t, assetBytes)
	})

	t.Run("videos workflow", func(t *testing.T) {
		payload := `{"model":"sora-mini","prompt":"waves","duration":2.5,"size":"640x360","fps":12}`
		resp, err := http.Post(server.URL+"/v1/videos", "application/json", strings.NewReader(payload))
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)

		var created dto.OpenAIVideo
		require.NoError(t, common.DecodeJson(resp.Body, &created))
		require.NotEmpty(t, created.ID)

		var fetched dto.OpenAIVideo
		require.Eventually(t, func() bool {
			getResp, getErr := http.Get(server.URL + "/v1/videos/" + created.ID)
			require.NoError(t, getErr)
			defer getResp.Body.Close()
			require.Equal(t, http.StatusOK, getResp.StatusCode)
			require.NoError(t, common.DecodeJson(getResp.Body, &fetched))
			return fetched.Status == dto.VideoStatusCompleted || fetched.Status == dto.VideoStatusFailed
		}, 3*time.Second, 50*time.Millisecond)

		if fetched.Status == dto.VideoStatusCompleted {
			contentResp, err := http.Get(server.URL + "/v1/videos/" + created.ID + "/content")
			require.NoError(t, err)
			defer contentResp.Body.Close()
			require.Equal(t, http.StatusOK, contentResp.StatusCode)
			content, err := io.ReadAll(contentResp.Body)
			require.NoError(t, err)
			require.NotEmpty(t, content)
		}
	})

	t.Run("admin stats reset", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodGet, server.URL+"/__mock/admin/stats/summary", nil)
		require.NoError(t, err)
		req.Header.Set("X-Mock-Admin-Token", cfg.Worker.ManagementToken)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)

		var summary summaryResponse
		require.NoError(t, common.DecodeJson(resp.Body, &summary))
		require.Greater(t, summary.TotalRequests, int64(0))

		resetReq, err := http.NewRequest(http.MethodPost, server.URL+"/__mock/admin/stats/reset", strings.NewReader(`{}`))
		require.NoError(t, err)
		resetReq.Header.Set("X-Mock-Admin-Token", cfg.Worker.ManagementToken)
		resetReq.Header.Set("Content-Type", "application/json")
		resetResp, err := http.DefaultClient.Do(resetReq)
		require.NoError(t, err)
		defer resetResp.Body.Close()
		require.Equal(t, http.StatusOK, resetResp.StatusCode)
	})
}
