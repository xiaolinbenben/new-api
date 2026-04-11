package main

import (
	"context"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

type fakeBridge struct {
	runtime workerRuntimeSnapshot
	started int
	stopped int
}

func (f *fakeBridge) Runtime(context.Context) (workerRuntimeSnapshot, error) {
	return f.runtime, nil
}

func (f *fakeBridge) Start(context.Context) error {
	f.started++
	f.runtime.Status = "running"
	f.runtime.Healthy = true
	return nil
}

func (f *fakeBridge) Stop(context.Context) error {
	f.stopped++
	f.runtime.Status = "stopped"
	f.runtime.Healthy = false
	return nil
}

func (f *fakeBridge) Restart(context.Context) error {
	f.runtime.Status = "running"
	f.runtime.Healthy = true
	return nil
}

func (f *fakeBridge) Summary(context.Context) (summaryResponse, error) {
	return summaryResponse{TotalRequests: 12, CurrentQPS: 1.5}, nil
}

func (f *fakeBridge) Routes(context.Context) (routeStatsResponse, error) {
	return routeStatsResponse{Items: []routeStatsItem{{Route: "chat.completions", Requests: 3}}}, nil
}

func (f *fakeBridge) Events(context.Context) (eventsResponse, error) {
	return eventsResponse{
		Requests: []requestEvent{{Route: "chat.completions", StatusCode: 200, Timestamp: time.Now()}},
	}, nil
}

func (f *fakeBridge) ResetStats(context.Context) error {
	return nil
}

func TestControlServerAuthAndConfig(t *testing.T) {
	t.Parallel()

	store, err := loadConfigStore(t.TempDir() + "/config.json")
	require.NoError(t, err)
	bridge := &fakeBridge{
		runtime: workerRuntimeSnapshot{Status: "stopped", Healthy: false},
	}
	server := newControlServer(bootstrapConfig{
		ControlPort:   18080,
		DataDir:       t.TempDir(),
		AdminPassword: "secret",
	}, store, bridge)
	ts := httptest.NewServer(server.handler())
	defer ts.Close()

	jar, err := cookiejar.New(nil)
	require.NoError(t, err)
	client := &http.Client{Jar: jar}

	req, err := http.NewRequest(http.MethodGet, ts.URL+"/api/config", nil)
	require.NoError(t, err)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)

	loginResp, err := client.Post(ts.URL+"/api/admin/login", "application/json", strings.NewReader(`{"password":"secret"}`))
	require.NoError(t, err)
	defer loginResp.Body.Close()
	require.Equal(t, http.StatusOK, loginResp.StatusCode)

	configResp, err := client.Get(ts.URL + "/api/config")
	require.NoError(t, err)
	defer configResp.Body.Close()
	require.Equal(t, http.StatusOK, configResp.StatusCode)

	var cfg configView
	require.NoError(t, common.DecodeJson(configResp.Body, &cfg))
	cfg.Worker.Port = 18181
	cfg.Worker.Models = []string{"gpt-4o-mini", "sora-mini"}
	body, err := common.Marshal(cfg)
	require.NoError(t, err)

	updateReq, err := http.NewRequest(http.MethodPut, ts.URL+"/api/config", strings.NewReader(string(body)))
	require.NoError(t, err)
	updateReq.Header.Set("Content-Type", "application/json")
	updateResp, err := client.Do(updateReq)
	require.NoError(t, err)
	defer updateResp.Body.Close()
	require.Equal(t, http.StatusOK, updateResp.StatusCode)

	startReq, err := http.NewRequest(http.MethodPost, ts.URL+"/api/worker/start", strings.NewReader(`{}`))
	require.NoError(t, err)
	startReq.Header.Set("Content-Type", "application/json")
	startResp, err := client.Do(startReq)
	require.NoError(t, err)
	defer startResp.Body.Close()
	require.Equal(t, http.StatusOK, startResp.StatusCode)
	require.Equal(t, 1, bridge.started)

	runtimeResp, err := client.Get(ts.URL + "/api/runtime")
	require.NoError(t, err)
	defer runtimeResp.Body.Close()
	require.Equal(t, http.StatusOK, runtimeResp.StatusCode)

	var runtime controlRuntimeResponse
	require.NoError(t, common.DecodeJson(runtimeResp.Body, &runtime))
	require.Equal(t, "running", runtime.Worker.Status)
	require.Equal(t, 18181, runtime.Config.Worker.Port)

	logoutReq, err := http.NewRequest(http.MethodPost, ts.URL+"/api/admin/logout", strings.NewReader(`{}`))
	require.NoError(t, err)
	logoutReq.Header.Set("Content-Type", "application/json")
	logoutResp, err := client.Do(logoutReq)
	require.NoError(t, err)
	defer logoutResp.Body.Close()
	require.Equal(t, http.StatusOK, logoutResp.StatusCode)
}
