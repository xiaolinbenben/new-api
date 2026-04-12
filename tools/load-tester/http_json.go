package main

import (
	"errors"
	"net/http"

	"github.com/QuantumNous/new-api/common"
)

func readJSON(r *http.Request, target any) error {
	if r.Body == nil {
		return errors.New("empty request body")
	}
	return common.DecodeJson(r.Body, target)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	data, err := common.Marshal(payload)
	if err != nil {
		http.Error(w, "json marshal failed", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(data)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]any{"message": message})
}
