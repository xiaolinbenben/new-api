package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/QuantumNous/new-api/common"
)

const workerPIDFileName = "worker.pid"

type workerBridge interface {
	Runtime(context.Context) (workerRuntimeSnapshot, error)
	StartRun(context.Context) error
	StopRun(context.Context) error
	Summary(context.Context) (summaryResponse, error)
	Scenarios(context.Context) (scenarioStatsResponse, error)
	Samples(context.Context) (samplesResponse, error)
	History(context.Context) (historyListResponse, error)
	HistoryDetail(context.Context, string) (runRecord, error)
	Shutdown(context.Context) error
}

type processManager struct {
	execPath   string
	dataDir    string
	configPath string
	store      *configStore

	mu        sync.Mutex
	cmd       *exec.Cmd
	lastError string
	http      *http.Client
}

func newProcessManager(execPath string, dataDir string, configPath string, store *configStore) *processManager {
	return &processManager{
		execPath:   execPath,
		dataDir:    dataDir,
		configPath: configPath,
		store:      store,
		http: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

func (m *processManager) Runtime(ctx context.Context) (workerRuntimeSnapshot, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.ensureRunningLocked(ctx); err != nil {
		return workerRuntimeSnapshot{}, err
	}
	return m.runtimeLocked(ctx)
}

func (m *processManager) StartRun(ctx context.Context) error {
	return m.doWorkerJSON(ctx, http.MethodPost, "/__loadtest/admin/run/start", strings.NewReader(`{}`), nil)
}

func (m *processManager) StopRun(ctx context.Context) error {
	return m.doWorkerJSON(ctx, http.MethodPost, "/__loadtest/admin/run/stop", strings.NewReader(`{}`), nil)
}

func (m *processManager) Summary(ctx context.Context) (summaryResponse, error) {
	var out summaryResponse
	if err := m.doWorkerJSON(ctx, http.MethodGet, "/__loadtest/admin/stats/summary", nil, &out); err != nil {
		return summaryResponse{}, err
	}
	return out, nil
}

func (m *processManager) Scenarios(ctx context.Context) (scenarioStatsResponse, error) {
	var out scenarioStatsResponse
	if err := m.doWorkerJSON(ctx, http.MethodGet, "/__loadtest/admin/stats/scenarios", nil, &out); err != nil {
		return scenarioStatsResponse{}, err
	}
	return out, nil
}

func (m *processManager) Samples(ctx context.Context) (samplesResponse, error) {
	var out samplesResponse
	if err := m.doWorkerJSON(ctx, http.MethodGet, "/__loadtest/admin/stats/samples", nil, &out); err != nil {
		return samplesResponse{}, err
	}
	return out, nil
}

func (m *processManager) History(ctx context.Context) (historyListResponse, error) {
	var out historyListResponse
	if err := m.doWorkerJSON(ctx, http.MethodGet, "/__loadtest/admin/history", nil, &out); err != nil {
		return historyListResponse{}, err
	}
	return out, nil
}

func (m *processManager) HistoryDetail(ctx context.Context, runID string) (runRecord, error) {
	var out runRecord
	if err := m.doWorkerJSON(ctx, http.MethodGet, "/__loadtest/admin/history/"+urlPathEscape(runID), nil, &out); err != nil {
		return runRecord{}, err
	}
	return out, nil
}

func (m *processManager) Shutdown(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.stopLocked(ctx)
}

func (m *processManager) ensureRunningLocked(ctx context.Context) error {
	current, _ := m.runtimeLocked(ctx)
	if current.Healthy {
		return nil
	}
	if err := m.startLocked(); err != nil && !strings.Contains(err.Error(), "already running") {
		return err
	}
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		current, _ = m.runtimeLocked(ctx)
		if current.Healthy {
			m.lastError = ""
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	if m.lastError == "" {
		m.lastError = "worker did not become healthy in time"
	}
	return errors.New(m.lastError)
}

func (m *processManager) startLocked() error {
	current, _ := m.runtimeLocked(context.Background())
	if current.Healthy {
		return fmt.Errorf("worker is already running")
	}
	if err := os.MkdirAll(m.dataDir, 0o755); err != nil {
		return err
	}
	cmd := exec.CommandContext(context.Background(), m.execPath, "worker", "--config", m.configPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		m.lastError = err.Error()
		return err
	}
	m.cmd = cmd
	if err := os.WriteFile(m.pidFilePath(), []byte(strconv.Itoa(cmd.Process.Pid)), 0o644); err != nil {
		_ = cmd.Process.Kill()
		m.cmd = nil
		return err
	}
	go func() {
		err := cmd.Wait()
		m.mu.Lock()
		defer m.mu.Unlock()
		if m.cmd == cmd {
			m.cmd = nil
		}
		if err != nil {
			m.lastError = err.Error()
		}
		_ = os.Remove(m.pidFilePath())
	}()
	return nil
}

func (m *processManager) runtimeLocked(ctx context.Context) (workerRuntimeSnapshot, error) {
	cfg := m.store.get()
	snapshot := workerRuntimeSnapshot{
		Status:     "stopped",
		Healthy:    false,
		WorkerPort: cfg.Worker.Port,
		LastError:  m.lastError,
		RunStatus:  "idle",
	}

	pid, err := m.readPID()
	if err != nil {
		return snapshot, nil
	}
	snapshot.PID = pid
	if !processExists(pid) {
		_ = os.Remove(m.pidFilePath())
		if m.cmd != nil && m.cmd.Process != nil && m.cmd.Process.Pid == pid {
			m.cmd = nil
		}
		return snapshot, nil
	}

	if err := m.doWorkerJSONLocked(ctx, http.MethodGet, "/__loadtest/admin/runtime", nil, &snapshot); err != nil {
		snapshot.Status = "starting"
		snapshot.LastError = err.Error()
		return snapshot, nil
	}
	snapshot.PID = pid
	snapshot.Healthy = true
	snapshot.Status = "running"
	return snapshot, nil
}

func (m *processManager) stopLocked(ctx context.Context) error {
	snapshot, _ := m.runtimeLocked(ctx)
	if snapshot.PID == 0 {
		return nil
	}

	process, err := os.FindProcess(snapshot.PID)
	if err != nil {
		return err
	}
	if err = process.Signal(os.Interrupt); err != nil {
		_ = process.Signal(syscall.SIGTERM)
	}

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		current, _ := m.runtimeLocked(ctx)
		if current.PID == 0 || !current.Healthy {
			_ = os.Remove(m.pidFilePath())
			m.cmd = nil
			m.lastError = ""
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}

	if err = process.Kill(); err != nil {
		return err
	}
	_ = os.Remove(m.pidFilePath())
	m.cmd = nil
	return nil
}

func (m *processManager) doWorkerJSON(ctx context.Context, method string, path string, body io.Reader, target any) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.ensureRunningLocked(ctx); err != nil {
		return err
	}
	return m.doWorkerJSONLocked(ctx, method, path, body, target)
}

func (m *processManager) doWorkerJSONLocked(ctx context.Context, method string, path string, body io.Reader, target any) error {
	cfg := m.store.get()
	req, err := http.NewRequestWithContext(ctx, method, fmt.Sprintf("http://127.0.0.1:%d%s", cfg.Worker.Port, path), body)
	if err != nil {
		return err
	}
	req.Header.Set(workerAdminHeader, cfg.Worker.ManagementToken)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := m.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		data, _ := io.ReadAll(resp.Body)
		if len(data) == 0 {
			return fmt.Errorf("worker returned status %d", resp.StatusCode)
		}
		return fmt.Errorf("worker returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	if target == nil {
		return nil
	}
	return common.DecodeJson(resp.Body, target)
}

func (m *processManager) pidFilePath() string {
	return filepath.Join(m.dataDir, workerPIDFileName)
}

func (m *processManager) readPID() (int, error) {
	data, err := os.ReadFile(m.pidFilePath())
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(data)))
}

func processExists(pid int) bool {
	if pid <= 0 {
		return false
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return process.Signal(syscall.Signal(0)) == nil
}

func urlPathEscape(value string) string {
	replacer := strings.NewReplacer("%", "%25", "/", "%2F", " ", "%20")
	return replacer.Replace(value)
}
