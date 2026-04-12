package main

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed ui/*
var uiFiles embed.FS

func registerUIRoutes(mux *http.ServeMux) {
	rootFS, _ := fs.Sub(uiFiles, "ui")
	fileServer := http.FileServer(http.FS(rootFS))
	mux.Handle("/assets/", http.StripPrefix("/assets/", fileServer))
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		data, err := uiFiles.ReadFile("ui/index.html")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(data)
	})
}
