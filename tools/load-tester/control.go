package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

const sessionCookieName = "load_tester_session"

type loginRequest struct {
	Password string `json:"password"`
}

type controlServer struct {
	bootstrap bootstrapConfig
	store     *configStore
	bridge    workerBridge

	mu       sync.Mutex
	sessions map[string]time.Time
}

func newControlServer(bootstrap bootstrapConfig, store *configStore, bridge workerBridge) *controlServer {
	return &controlServer{
		bootstrap: bootstrap,
		store:     store,
		bridge:    bridge,
		sessions:  make(map[string]time.Time),
	}
}

func (s *controlServer) run(ctx context.Context) error {
	server := &http.Server{
		Addr:              fmt.Sprintf(":%d", s.bootstrap.ControlPort),
		Handler:           s.handler(),
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

	fmt.Printf("\nLoad Tester 控制台已启动\n")
	fmt.Printf("访问地址: http://localhost:%d\n\n", s.bootstrap.ControlPort)

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = s.bridge.Shutdown(shutdownCtx)
		return server.Shutdown(shutdownCtx)
	case err := <-errCh:
		return err
	}
}

func (s *controlServer) handler() *http.ServeMux {
	mux := http.NewServeMux()
	registerUIRoutes(mux)
	mux.HandleFunc("POST /api/admin/login", s.handleLogin)
	mux.HandleFunc("POST /api/admin/logout", s.withSession(s.handleLogout))
	mux.HandleFunc("GET /api/config", s.withSession(s.handleGetConfig))
	mux.HandleFunc("PUT /api/config", s.withSession(s.handlePutConfig))
	mux.HandleFunc("GET /api/runtime", s.withSession(s.handleRuntime))
	mux.HandleFunc("POST /api/run/start", s.withSession(s.handleRunStart))
	mux.HandleFunc("POST /api/run/stop", s.withSession(s.handleRunStop))
	mux.HandleFunc("GET /api/stats/summary", s.withSession(s.handleSummary))
	mux.HandleFunc("GET /api/stats/scenarios", s.withSession(s.handleScenarios))
	mux.HandleFunc("GET /api/stats/samples", s.withSession(s.handleSamples))
	mux.HandleFunc("GET /api/history", s.withSession(s.handleHistory))
	mux.HandleFunc("GET /api/history/{id}", s.withSession(s.handleHistoryDetail))
	return mux
}

func (s *controlServer) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if strings.TrimSpace(req.Password) != s.bootstrap.AdminPassword {
		writeError(w, http.StatusUnauthorized, "invalid password")
		return
	}

	token := uuid.NewString()
	expiresAt := time.Now().Add(24 * time.Hour)
	s.mu.Lock()
	s.sessions[token] = expiresAt
	s.mu.Unlock()

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		Expires:  expiresAt,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	writeJSON(w, http.StatusOK, map[string]any{"message": "ok"})
}

func (s *controlServer) handleLogout(w http.ResponseWriter, r *http.Request) {
	s.clearSession(w, r)
	writeJSON(w, http.StatusOK, map[string]any{"message": "ok"})
}

func (s *controlServer) handleGetConfig(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.store.getView())
}

func (s *controlServer) handlePutConfig(w http.ResponseWriter, r *http.Request) {
	var view configView
	if err := readJSON(r, &view); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	cfg, err := s.store.updateView(view)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, configView{
		Target:    cloneTargetConfig(cfg.Target),
		Run:       cfg.Run,
		Sampling:  cloneSamplingConfig(cfg.Sampling),
		Scenarios: cloneScenarios(cfg.Scenarios),
	})
}

func (s *controlServer) handleRuntime(w http.ResponseWriter, r *http.Request) {
	runtime, err := s.bridge.Runtime(r.Context())
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, controlRuntimeResponse{
		ControlPort: s.bootstrap.ControlPort,
		DataDir:     s.bootstrap.DataDir,
		Config:      s.store.getView(),
		Worker:      runtime,
	})
}

func (s *controlServer) handleRunStart(w http.ResponseWriter, r *http.Request) {
	if err := s.bridge.StartRun(r.Context()); err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"message": "run started"})
}

func (s *controlServer) handleRunStop(w http.ResponseWriter, r *http.Request) {
	if err := s.bridge.StopRun(r.Context()); err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"message": "run stopping"})
}

func (s *controlServer) handleSummary(w http.ResponseWriter, r *http.Request) {
	summary, err := s.bridge.Summary(r.Context())
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, summary)
}

func (s *controlServer) handleScenarios(w http.ResponseWriter, r *http.Request) {
	items, err := s.bridge.Scenarios(r.Context())
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *controlServer) handleSamples(w http.ResponseWriter, r *http.Request) {
	samples, err := s.bridge.Samples(r.Context())
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, samples)
}

func (s *controlServer) handleHistory(w http.ResponseWriter, r *http.Request) {
	history, err := s.bridge.History(r.Context())
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, history)
}

func (s *controlServer) handleHistoryDetail(w http.ResponseWriter, r *http.Request) {
	record, err := s.bridge.HistoryDetail(r.Context(), r.PathValue("id"))
	if err != nil {
		if os.IsNotExist(err) {
			writeError(w, http.StatusNotFound, "history record not found")
			return
		}
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, record)
}

func (s *controlServer) withSession(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.isAuthorized(r) {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		next(w, r)
	}
}

func (s *controlServer) isAuthorized(r *http.Request) bool {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	expiresAt, ok := s.sessions[cookie.Value]
	if !ok {
		return false
	}
	if time.Now().After(expiresAt) {
		delete(s.sessions, cookie.Value)
		return false
	}
	return true
}

func (s *controlServer) clearSession(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(sessionCookieName)
	if err == nil {
		s.mu.Lock()
		delete(s.sessions, cookie.Value)
		s.mu.Unlock()
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	})
}
