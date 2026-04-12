package router

import (
	"io/fs"
	"net/http"
	"path"
	"strings"

	"github.com/QuantumNous/new-api/tools/test-workbench/controller"
)

func New(ctrl *controller.AdminController, uiFS fs.FS) http.Handler {
	mux := http.NewServeMux()
	ctrl.Register(mux)
	registerUIRoutes(mux, uiFS)
	return mux
}

func registerUIRoutes(mux *http.ServeMux, uiFS fs.FS) {
	distFS, err := fs.Sub(uiFS, "web/dist")
	if err != nil {
		mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "前端构建产物不存在", http.StatusServiceUnavailable)
		})
		return
	}
	fileServer := http.FileServer(http.FS(distFS))
	mux.HandleFunc("GET /{$}", serveIndex(distFS))
	mux.HandleFunc("GET /{rest...}", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}
		trimmed := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if trimmed == "." || trimmed == "" || !strings.Contains(trimmed, ".") {
			serveIndex(distFS)(w, r)
			return
		}
		if _, err := fs.Stat(distFS, trimmed); err == nil {
			fileServer.ServeHTTP(w, r)
			return
		}
		serveIndex(distFS)(w, r)
	})
}

func serveIndex(distFS fs.FS) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data, err := fs.ReadFile(distFS, "index.html")
		if err != nil {
			http.Error(w, "前端入口文件不存在", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(data)
	}
}
