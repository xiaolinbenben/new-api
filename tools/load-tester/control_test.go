package main

import (
	"context"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type fakeBridge struct {
	runtime workerRuntimeSnapshot
}

func (f *fakeBridge) Runtime(context.Context) (workerRuntimeSnapshot, error) {
	return f.runtime, nil
}

func (f *fakeBridge) StartRun(context.Context) error {
	return nil
}

func (f *fakeBridge) StopRun(context.Context) error {
	return nil
}

func (f *fakeBridge) Summary(context.Context) (summaryResponse, error) {
	return summaryResponse{RunStatus: "idle"}, nil
}

func (f *fakeBridge) Scenarios(context.Context) (scenarioStatsResponse, error) {
	return scenarioStatsResponse{}, nil
}

func (f *fakeBridge) Samples(context.Context) (samplesResponse, error) {
	return samplesResponse{}, nil
}

func (f *fakeBridge) History(context.Context) (historyListResponse, error) {
	return historyListResponse{}, nil
}

func (f *fakeBridge) HistoryDetail(context.Context, string) (runRecord, error) {
	return runRecord{}, nil
}

func (f *fakeBridge) Shutdown(context.Context) error {
	return nil
}

func TestControlServerRequiresSession(t *testing.T) {
	t.Parallel()

	store, err := loadConfigStore(t.TempDir() + "/config.json")
	require.NoError(t, err)
	server := newControlServer(bootstrapConfig{
		ControlPort:   18090,
		DataDir:       t.TempDir(),
		AdminPassword: "secret",
	}, store, &fakeBridge{
		runtime: workerRuntimeSnapshot{
			Status:    "running",
			Healthy:   true,
			RunStatus: "idle",
			StartedAt: time.Now(),
		},
	})
	ts := httptest.NewServer(server.handler())
	defer ts.Close()

	jar, err := cookiejar.New(nil)
	require.NoError(t, err)
	client := &http.Client{Jar: jar}

	resp, err := client.Get(ts.URL + "/api/config")
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

	runtimeResp, err := client.Get(ts.URL + "/api/runtime")
	require.NoError(t, err)
	defer runtimeResp.Body.Close()
	require.Equal(t, http.StatusOK, runtimeResp.StatusCode)

	logoutReq, err := http.NewRequest(http.MethodPost, ts.URL+"/api/admin/logout", strings.NewReader(`{}`))
	require.NoError(t, err)
	logoutReq.Header.Set("Content-Type", "application/json")
	logoutResp, err := client.Do(logoutReq)
	require.NoError(t, err)
	defer logoutResp.Body.Close()
	require.Equal(t, http.StatusOK, logoutResp.StatusCode)

	afterLogoutResp, err := client.Get(ts.URL + "/api/config")
	require.NoError(t, err)
	defer afterLogoutResp.Body.Close()
	require.Equal(t, http.StatusUnauthorized, afterLogoutResp.StatusCode)
}
