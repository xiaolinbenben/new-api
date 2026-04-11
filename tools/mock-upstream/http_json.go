package main

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/types"
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
		writeOpenAIError(w, http.StatusInternalServerError, "server_error", "json marshal failed", "json_marshal_failed", "")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(data)
}

func writeOpenAIError(w http.ResponseWriter, status int, errorType string, message string, code any, param string) {
	writeJSON(w, status, map[string]any{
		"error": types.OpenAIError{
			Message: message,
			Type:    errorType,
			Param:   param,
			Code:    code,
		},
	})
}

func writeSSEEvent(w http.ResponseWriter, payload any) error {
	data, err := common.Marshal(payload)
	if err != nil {
		return err
	}
	if _, err = fmt.Fprintf(w, "data: %s\n\n", data); err != nil {
		return err
	}
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
	return nil
}

func writeSSEDone(w http.ResponseWriter) error {
	if _, err := fmt.Fprint(w, "data: [DONE]\n\n"); err != nil {
		return err
	}
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
	return nil
}

func startSSE(w http.ResponseWriter) {
	headers := w.Header()
	headers.Set("Content-Type", "text/event-stream")
	headers.Set("Cache-Control", "no-cache")
	headers.Set("Connection", "keep-alive")
	headers.Set("X-Accel-Buffering", "no")
}

func nowUnix() int64 {
	return time.Now().Unix()
}
