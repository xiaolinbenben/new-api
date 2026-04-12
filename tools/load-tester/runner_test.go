package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestRunnerWeightedScheduling(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "path": r.URL.Path})
	}))
	defer server.Close()

	cfg := defaultConfig()
	cfg.Target.BaseURL = server.URL
	cfg.Run.TotalConcurrency = 3
	cfg.Run.DurationSec = 30
	cfg.Run.MaxRequests = 18
	cfg.Run.RequestTimeoutMS = 1000
	cfg.Scenarios = []scenarioConfig{
		{
			ID:               "a",
			Name:             "A",
			Enabled:          true,
			Mode:             "single",
			Weight:           2,
			Method:           "GET",
			Path:             "/a",
			ExpectedStatuses: []int{200},
		},
		{
			ID:               "b",
			Name:             "B",
			Enabled:          true,
			Mode:             "single",
			Weight:           1,
			Method:           "GET",
			Path:             "/b",
			ExpectedStatuses: []int{200},
		},
	}
	require.NoError(t, validateConfig(&cfg))

	history := newHistoryStore(t.TempDir())
	samples := newSampleStore(t.TempDir(), cfg.Sampling)
	run, err := newRunner(cfg, history, samples)
	require.NoError(t, err)

	run.start(context.Background())
	select {
	case <-run.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("runner did not finish in time")
	}

	require.Equal(t, "completed", run.status())
	require.Equal(t, int64(18), run.summary().TotalRequests)

	stats := run.scenarios().Items
	counts := map[string]int64{}
	for _, item := range stats {
		counts[item.ScenarioID] = item.Requests
	}
	require.Equal(t, int64(12), counts["a"])
	require.Equal(t, int64(6), counts["b"])
}

func TestRunnerTaskFlowSuccess(t *testing.T) {
	t.Parallel()

	var polls atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/videos":
			writeJSON(w, http.StatusOK, map[string]any{"id": "task-1"})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/videos/task-1":
			if polls.Add(1) >= 2 {
				writeJSON(w, http.StatusOK, map[string]any{"status": "completed"})
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"status": "processing"})
		default:
			writeError(w, http.StatusNotFound, "not found")
		}
	}))
	defer server.Close()

	cfg := defaultConfig()
	cfg.Target.BaseURL = server.URL
	cfg.Run.TotalConcurrency = 1
	cfg.Run.DurationSec = 10
	cfg.Run.MaxRequests = 1
	cfg.Run.RequestTimeoutMS = 1000
	cfg.Scenarios = []scenarioConfig{
		{
			ID:      "video",
			Name:    "video",
			Enabled: true,
			Preset:  "new_api_videos",
			Mode:    "task_flow",
			Weight:  1,
		},
	}
	require.NoError(t, validateConfig(&cfg))

	run, err := newRunner(cfg, newHistoryStore(t.TempDir()), newSampleStore(t.TempDir(), cfg.Sampling))
	require.NoError(t, err)

	run.start(context.Background())
	select {
	case <-run.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("runner did not finish in time")
	}

	summary := run.summary()
	require.Equal(t, int64(1), summary.TotalRequests)
	require.Equal(t, int64(1), summary.Successes)
	require.Equal(t, int64(0), summary.Errors)

	stats := run.scenarios().Items
	require.Len(t, stats, 1)
	require.Equal(t, int64(1), stats[0].Requests)
	require.Equal(t, int64(1), stats[0].Successes)
	require.Equal(t, int64(1), stats[0].StageStats["submit"].Requests)
	require.GreaterOrEqual(t, stats[0].StageStats["poll"].Requests, int64(2))
}

func TestRunnerSSEIncompleteClassified(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("data: {\"type\":\"message\"}\n\n"))
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
	}))
	defer server.Close()

	cfg := defaultConfig()
	cfg.Target.BaseURL = server.URL
	cfg.Run.TotalConcurrency = 1
	cfg.Run.DurationSec = 10
	cfg.Run.MaxRequests = 1
	cfg.Run.RequestTimeoutMS = 1000
	cfg.Scenarios = []scenarioConfig{
		{
			ID:               "broken-sse",
			Name:             "broken-sse",
			Enabled:          true,
			Mode:             "sse",
			Weight:           1,
			Method:           "POST",
			Path:             "/stream",
			Body:             `{"hello":"world"}`,
			ExpectedStatuses: []int{200},
		},
	}
	require.NoError(t, validateConfig(&cfg))

	samples := newSampleStore(t.TempDir(), cfg.Sampling)
	run, err := newRunner(cfg, newHistoryStore(t.TempDir()), samples)
	require.NoError(t, err)

	run.start(context.Background())
	select {
	case <-run.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("runner did not finish in time")
	}

	summary := run.summary()
	require.Equal(t, int64(1), summary.TotalRequests)
	require.Equal(t, int64(1), summary.Errors)
	require.Equal(t, int64(1), summary.ErrorKinds["sse_incomplete"])

	errorSamples := samples.snapshot().Errors
	require.NotEmpty(t, errorSamples)
	require.Equal(t, "sse_incomplete", errorSamples[0].ErrorKind)
}

func TestExtractJSONPathSupportsNestedValues(t *testing.T) {
	t.Parallel()

	data, err := common.Marshal(map[string]any{
		"task": map[string]any{
			"id": "task-123",
		},
	})
	require.NoError(t, err)

	value, err := extractJSONPath(data, "task.id")
	require.NoError(t, err)
	require.Equal(t, "task-123", value)
}

func TestRunnerPersistsHistory(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	}))
	defer server.Close()

	cfg := defaultConfig()
	cfg.Target.BaseURL = server.URL
	cfg.Run.TotalConcurrency = 1
	cfg.Run.DurationSec = 10
	cfg.Run.MaxRequests = 1
	cfg.Scenarios = []scenarioConfig{
		{
			ID:               "single",
			Name:             "single",
			Enabled:          true,
			Mode:             "single",
			Weight:           1,
			Method:           "GET",
			Path:             "/once",
			ExpectedStatuses: []int{200},
		},
	}
	require.NoError(t, validateConfig(&cfg))

	history := newHistoryStore(t.TempDir())
	run, err := newRunner(cfg, history, newSampleStore(t.TempDir(), cfg.Sampling))
	require.NoError(t, err)
	run.start(context.Background())
	select {
	case <-run.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("runner did not finish in time")
	}

	list, err := history.list()
	require.NoError(t, err)
	require.Len(t, list.Items, 1)
	record, err := history.get(list.Items[0].RunID)
	require.NoError(t, err)
	require.Equal(t, int64(1), record.Summary.TotalRequests)
	require.Equal(t, "completed", record.RunStatus)
	require.Equal(t, fmt.Sprintf("%s", record.RunID), list.Items[0].RunID)
}
