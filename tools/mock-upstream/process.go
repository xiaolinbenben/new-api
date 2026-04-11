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

const (
	workerPIDFileName = "worker.pid"
)

type workerBridge interface {
	Runtime(context.Context) (workerRuntimeSnapshot, error)
	Start(context.Context) error
	Stop(context.Context) error
	Restart(context.Context) error
	Summary(context.Context) (summaryResponse, error)
	Routes(context.Context) (routeStatsResponse, error)
	Events(context.Context) (eventsResponse, error)
	ResetStats(context.Context) error
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
			Timeout: 2 * time.Second,
		},
	}
}

func (m *processManager) Runtime(ctx context.Context) (workerRuntimeSnapshot, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.runtimeLocked(ctx)
}

func (m *processManager) Start(ctx context.Context) error {
	m.mu.Lock()
	if err := m.startLocked(ctx); err != nil {
		m.mu.Unlock()
		return err
	}
	m.mu.Unlock()
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		snapshot, err := m.Runtime(ctx)
		if err == nil && snapshot.Healthy {
			m.mu.Lock()
			m.lastError = ""
			m.mu.Unlock()
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	m.mu.Lock()
	m.lastError = "worker did not become healthy in time"
	m.mu.Unlock()
	return errors.New(m.lastError)
}

func (m *processManager) Stop(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.stopLocked(ctx)
}

func (m *processManager) Restart(ctx context.Context) error {
	m.mu.Lock()
	if err := m.stopLocked(ctx); err != nil && !strings.Contains(err.Error(), "not running") {
		m.mu.Unlock()
		return err
	}
	current, _ := m.runtimeLocked(ctx)
	if current.Healthy {
		m.mu.Unlock()
		return fmt.Errorf("worker is already running")
	}
	if err := os.MkdirAll(m.dataDir, 0o755); err != nil {
		m.mu.Unlock()
		return err
	}
	cmd := exec.CommandContext(context.Background(), m.execPath, "worker", "--config", m.configPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		m.lastError = err.Error()
		m.mu.Unlock()
		return err
	}
	m.cmd = cmd
	if err := os.WriteFile(m.pidFilePath(), []byte(strconv.Itoa(cmd.Process.Pid)), 0o644); err != nil {
		_ = cmd.Process.Kill()
		m.cmd = nil
		m.mu.Unlock()
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
	m.mu.Unlock()

	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		snapshot, err := m.Runtime(ctx)
		if err == nil && snapshot.Healthy {
			m.mu.Lock()
			m.lastError = ""
			m.mu.Unlock()
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	m.mu.Lock()
	m.lastError = "worker did not become healthy in time"
	m.mu.Unlock()
	return errors.New("worker did not become healthy in time")
}

func (m *processManager) startLocked(_ context.Context) error {
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

func (m *processManager) Summary(ctx context.Context) (summaryResponse, error) {
	var out summaryResponse
	if err := m.doWorkerJSON(ctx, http.MethodGet, "/__mock/admin/stats/summary", nil, &out); err != nil {
		return summaryResponse{}, err
	}
	return out, nil
}

func (m *processManager) Routes(ctx context.Context) (routeStatsResponse, error) {
	var out routeStatsResponse
	if err := m.doWorkerJSON(ctx, http.MethodGet, "/__mock/admin/stats/routes", nil, &out); err != nil {
		return routeStatsResponse{}, err
	}
	return out, nil
}

func (m *processManager) Events(ctx context.Context) (eventsResponse, error) {
	var out eventsResponse
	if err := m.doWorkerJSON(ctx, http.MethodGet, "/__mock/admin/stats/events", nil, &out); err != nil {
		return eventsResponse{}, err
	}
	return out, nil
}

func (m *processManager) ResetStats(ctx context.Context) error {
	return m.doWorkerJSON(ctx, http.MethodPost, "/__mock/admin/stats/reset", nil, nil)
}

func (m *processManager) runtimeLocked(ctx context.Context) (workerRuntimeSnapshot, error) {
	cfg := m.store.get()
	snapshot := workerRuntimeSnapshot{
		Status:       "stopped",
		PublicBase:   fmt.Sprintf("http://127.0.0.1:%d", cfg.Worker.Port),
		WorkerConfig: cfg.Worker,
		LastError:    m.lastError,
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

	err = m.doWorkerJSON(ctx, http.MethodGet, "/__mock/admin/runtime", nil, &snapshot)
	if err != nil {
		snapshot.Status = "starting"
		snapshot.LastError = err.Error()
		return snapshot, nil
	}
	snapshot.Healthy = true
	snapshot.Status = "running"
	snapshot.PID = pid
	return snapshot, nil
}

func (m *processManager) stopLocked(ctx context.Context) error {
	snapshot, _ := m.runtimeLocked(ctx)
	if snapshot.PID == 0 {
		return fmt.Errorf("worker is not running")
	}

	proc, err := os.FindProcess(snapshot.PID)
	if err != nil {
		return err
	}
	if err = proc.Signal(os.Interrupt); err != nil {
		_ = proc.Signal(syscall.SIGTERM)
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

	if err = proc.Kill(); err != nil {
		return err
	}
	_ = os.Remove(m.pidFilePath())
	m.cmd = nil
	return nil
}

func (m *processManager) doWorkerJSON(ctx context.Context, method string, path string, body io.Reader, target any) error {
	cfg := m.store.get()
	url := fmt.Sprintf("http://127.0.0.1:%d%s", cfg.Worker.Port, path)
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return err
	}
	req.Header.Set("X-Mock-Admin-Token", cfg.Worker.ManagementToken)
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
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, err
	}
	return pid, nil
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
