package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const workerAdminHeader = "X-Loadtest-Admin-Token"

type workerApp struct {
	configPath string
	store      *configStore
	history    *historyStore
	samples    *sampleStore
	startedAt  time.Time

	mu      sync.RWMutex
	current *runner
}

func newWorkerApp(configPath string, store *configStore) *workerApp {
	cfg := store.get()
	dataDir := filepath.Dir(configPath)
	return &workerApp{
		configPath: configPath,
		store:      store,
		history:    newHistoryStore(filepath.Join(dataDir, "runs")),
		samples:    newSampleStore(filepath.Join(dataDir, "samples", "current"), cfg.Sampling),
		startedAt:  time.Now(),
	}
}

func (a *workerApp) run(ctx context.Context) error {
	cfg := a.store.get()
	server := &http.Server{
		Addr:              fmt.Sprintf("127.0.0.1:%d", cfg.Worker.Port),
		Handler:           a.handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

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
		a.stopCurrentRunner()
		return server.Shutdown(shutdownCtx)
	case err := <-errCh:
		a.stopCurrentRunner()
		return err
	}
}

func (a *workerApp) handler() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /__loadtest/admin/runtime", a.withAdminToken(a.handleRuntime))
	mux.HandleFunc("POST /__loadtest/admin/run/start", a.withAdminToken(a.handleRunStart))
	mux.HandleFunc("POST /__loadtest/admin/run/stop", a.withAdminToken(a.handleRunStop))
	mux.HandleFunc("GET /__loadtest/admin/stats/summary", a.withAdminToken(a.handleSummary))
	mux.HandleFunc("GET /__loadtest/admin/stats/scenarios", a.withAdminToken(a.handleScenarios))
	mux.HandleFunc("GET /__loadtest/admin/stats/samples", a.withAdminToken(a.handleSamples))
	mux.HandleFunc("GET /__loadtest/admin/history", a.withAdminToken(a.handleHistory))
	mux.HandleFunc("GET /__loadtest/admin/history/{id}", a.withAdminToken(a.handleHistoryDetail))
	return mux
}

func (a *workerApp) withAdminToken(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := strings.TrimSpace(r.Header.Get(workerAdminHeader))
		cfg := a.store.get()
		if token == "" || token != cfg.Worker.ManagementToken {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		next(w, r)
	}
}

func (a *workerApp) handleRuntime(w http.ResponseWriter, _ *http.Request) {
	cfg, err := a.store.reloadFromDisk()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	current := a.currentRunner()
	snapshot := workerRuntimeSnapshot{
		Status:     "running",
		Healthy:    true,
		StartedAt:  a.startedAt,
		UptimeSec:  int64(time.Since(a.startedAt).Seconds()),
		WorkerPort: cfg.Worker.Port,
		RunStatus:  "idle",
	}
	if current != nil {
		snapshot.RunID = current.id
		snapshot.RunStatus = current.status()
		snapshot.RunStartedAt = current.startedAt
	}
	writeJSON(w, http.StatusOK, snapshot)
}

func (a *workerApp) handleRunStart(w http.ResponseWriter, r *http.Request) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.current != nil && a.current.status() == "running" {
		writeError(w, http.StatusConflict, "run is already active")
		return
	}

	cfg, err := a.store.reloadFromDisk()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	newRun, err := newRunner(cfg, a.history, a.samples)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	newRun.start(context.Background())
	a.current = newRun
	go func(local *runner) {
		<-local.Done()
		a.mu.Lock()
		if a.current == local {
			a.current = nil
		}
		a.mu.Unlock()
	}(newRun)

	writeJSON(w, http.StatusOK, map[string]any{
		"message": "run started",
		"run_id":  newRun.id,
	})
}

func (a *workerApp) handleRunStop(w http.ResponseWriter, _ *http.Request) {
	current := a.currentRunner()
	if current == nil {
		writeError(w, http.StatusConflict, "no active run")
		return
	}
	current.stop()
	writeJSON(w, http.StatusOK, map[string]any{"message": "run stopping"})
}

func (a *workerApp) handleSummary(w http.ResponseWriter, _ *http.Request) {
	if current := a.currentRunner(); current != nil {
		writeJSON(w, http.StatusOK, current.summary())
		return
	}
	record, ok := a.latestRecord()
	if ok {
		writeJSON(w, http.StatusOK, record.Summary)
		return
	}
	writeJSON(w, http.StatusOK, summaryResponse{
		RunStatus:   "idle",
		StatusCodes: map[string]int64{},
		ErrorKinds:  map[string]int64{},
	})
}

func (a *workerApp) handleScenarios(w http.ResponseWriter, _ *http.Request) {
	if current := a.currentRunner(); current != nil {
		writeJSON(w, http.StatusOK, current.scenarios())
		return
	}
	cfg := a.store.getView()
	items := make([]scenarioStatsItem, 0, len(cfg.Scenarios))
	for _, scenario := range cfg.Scenarios {
		items = append(items, scenarioStatsItem{
			ScenarioID:  scenario.ID,
			Name:        scenario.Name,
			Mode:        scenario.Mode,
			Weight:      scenario.Weight,
			Enabled:     scenario.Enabled,
			StatusCodes: map[string]int64{},
			ErrorKinds:  map[string]int64{},
			StageStats:  map[string]stageStats{},
		})
	}
	writeJSON(w, http.StatusOK, scenarioStatsResponse{Items: items})
}

func (a *workerApp) handleSamples(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, a.samples.snapshot())
}

func (a *workerApp) handleHistory(w http.ResponseWriter, _ *http.Request) {
	history, err := a.history.list()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, history)
}

func (a *workerApp) handleHistoryDetail(w http.ResponseWriter, r *http.Request) {
	record, err := a.history.get(r.PathValue("id"))
	if err != nil {
		if os.IsNotExist(err) {
			writeError(w, http.StatusNotFound, "history record not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, record)
}

func (a *workerApp) currentRunner() *runner {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.current
}

func (a *workerApp) latestRecord() (runRecord, bool) {
	history, err := a.history.list()
	if err != nil || len(history.Items) == 0 {
		return runRecord{}, false
	}
	record, err := a.history.get(history.Items[0].RunID)
	if err != nil {
		return runRecord{}, false
	}
	return record, true
}

func (a *workerApp) stopCurrentRunner() {
	current := a.currentRunner()
	if current == nil {
		return
	}
	current.stop()
	select {
	case <-current.Done():
	case <-time.After(3 * time.Second):
	}
}
