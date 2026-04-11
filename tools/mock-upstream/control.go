package main

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

const sessionCookieName = "mock_upstream_session"

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

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
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
	mux.HandleFunc("POST /api/worker/start", s.withSession(s.handleWorkerStart))
	mux.HandleFunc("POST /api/worker/stop", s.withSession(s.handleWorkerStop))
	mux.HandleFunc("POST /api/worker/restart", s.withSession(s.handleWorkerRestart))
	mux.HandleFunc("GET /api/stats/summary", s.withSession(s.handleStatsSummary))
	mux.HandleFunc("GET /api/stats/routes", s.withSession(s.handleStatsRoutes))
	mux.HandleFunc("GET /api/stats/events", s.withSession(s.handleStatsEvents))
	mux.HandleFunc("POST /api/stats/reset", s.withSession(s.handleStatsReset))
	return mux
}

func (s *controlServer) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": err.Error()})
		return
	}
	if strings.TrimSpace(req.Password) != s.bootstrap.AdminPassword {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"message": "invalid password"})
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
		SameSite: http.SameSiteStrictMode,
	})
	writeJSON(w, http.StatusOK, map[string]any{"message": "ok"})
}

func (s *controlServer) handleLogout(w http.ResponseWriter, r *http.Request) {
	s.clearSession(w, r)
	writeJSON(w, http.StatusOK, map[string]any{"message": "ok"})
}

func (s *controlServer) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.store.getView())
}

func (s *controlServer) handlePutConfig(w http.ResponseWriter, r *http.Request) {
	var view configView
	if err := readJSON(r, &view); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": err.Error()})
		return
	}
	cfg, err := s.store.updateView(view)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, configView{
		Worker: workerConfig{
			Port:        cfg.Worker.Port,
			RequireAuth: cfg.Worker.RequireAuth,
			EnablePprof: cfg.Worker.EnablePprof,
			PprofPort:   cfg.Worker.PprofPort,
			Models:      append([]string(nil), cfg.Worker.Models...),
		},
		Random:    cfg.Random,
		Chat:      cfg.Chat,
		Responses: cfg.Responses,
		Images:    cfg.Images,
		Videos:    cfg.Videos,
	})
}

func (s *controlServer) handleRuntime(w http.ResponseWriter, r *http.Request) {
	runtime, err := s.bridge.Runtime(r.Context())
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"message": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, controlRuntimeResponse{
		ControlPort: s.bootstrap.ControlPort,
		DataDir:     s.bootstrap.DataDir,
		Config:      s.store.getView(),
		Worker:      runtime,
	})
}

func (s *controlServer) handleWorkerStart(w http.ResponseWriter, r *http.Request) {
	if err := s.bridge.Start(r.Context()); err != nil {
		writeJSON(w, http.StatusConflict, map[string]any{"message": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"message": "worker started"})
}

func (s *controlServer) handleWorkerStop(w http.ResponseWriter, r *http.Request) {
	if err := s.bridge.Stop(r.Context()); err != nil {
		writeJSON(w, http.StatusConflict, map[string]any{"message": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"message": "worker stopped"})
}

func (s *controlServer) handleWorkerRestart(w http.ResponseWriter, r *http.Request) {
	if err := s.bridge.Restart(r.Context()); err != nil {
		writeJSON(w, http.StatusConflict, map[string]any{"message": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"message": "worker restarted"})
}

func (s *controlServer) handleStatsSummary(w http.ResponseWriter, r *http.Request) {
	resp, err := s.bridge.Summary(r.Context())
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"message": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *controlServer) handleStatsRoutes(w http.ResponseWriter, r *http.Request) {
	resp, err := s.bridge.Routes(r.Context())
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"message": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *controlServer) handleStatsEvents(w http.ResponseWriter, r *http.Request) {
	resp, err := s.bridge.Events(r.Context())
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"message": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *controlServer) handleStatsReset(w http.ResponseWriter, r *http.Request) {
	if err := s.bridge.ResetStats(r.Context()); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"message": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"message": "stats reset"})
}

func (s *controlServer) withSession(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.isAuthorized(r) {
			writeJSON(w, http.StatusUnauthorized, map[string]any{"message": "unauthorized"})
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
	expireAt, ok := s.sessions[cookie.Value]
	if !ok {
		return false
	}
	if time.Now().After(expireAt) {
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
		Expires:  time.Unix(0, 0),
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})
}
