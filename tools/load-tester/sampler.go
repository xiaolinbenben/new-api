package main

import (
	"bufio"
	"bytes"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
)

type sampleBuffer struct {
	limit int
	items []sampleRecord
}

func newSampleBuffer(limit int) *sampleBuffer {
	return &sampleBuffer{limit: limit}
}

func (b *sampleBuffer) add(item sampleRecord) {
	b.items = append([]sampleRecord{item}, b.items...)
	if len(b.items) > b.limit {
		b.items = b.items[:b.limit]
	}
}

func (b *sampleBuffer) snapshot() []sampleRecord {
	out := make([]sampleRecord, len(b.items))
	copy(out, b.items)
	return out
}

type sampleStore struct {
	dir       string
	maskHeads []string

	mu       sync.Mutex
	requests *sampleBuffer
	errors   *sampleBuffer
}

func newSampleStore(dir string, sampling samplingConfig) *sampleStore {
	return &sampleStore{
		dir:       dir,
		maskHeads: normalizeHeaderMasks(sampling.MaskHeaders),
		requests:  newSampleBuffer(sampling.MaxRequestSamples),
		errors:    newSampleBuffer(sampling.MaxErrorSamples),
	}
}

func (s *sampleStore) reset(sampling samplingConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.maskHeads = normalizeHeaderMasks(sampling.MaskHeaders)
	s.requests = newSampleBuffer(sampling.MaxRequestSamples)
	s.errors = newSampleBuffer(sampling.MaxErrorSamples)
	if err := os.RemoveAll(s.dir); err != nil {
		return err
	}
	return os.MkdirAll(s.dir, 0o755)
}

func (s *sampleStore) add(item sampleRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.requests.add(item)
	if !item.Success || item.ErrorKind != "" {
		s.errors.add(item)
	}
	return s.persistLocked()
}

func (s *sampleStore) addErrorOnly(item sampleRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.errors.add(item)
	return s.persistLocked()
}

func (s *sampleStore) snapshot() samplesResponse {
	s.mu.Lock()
	defer s.mu.Unlock()
	return samplesResponse{
		Requests: s.requests.snapshot(),
		Errors:   s.errors.snapshot(),
	}
}

func (s *sampleStore) persistLocked() error {
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return err
	}
	if err := writeJSONL(filepath.Join(s.dir, "requests.jsonl"), s.requests.snapshot()); err != nil {
		return err
	}
	return writeJSONL(filepath.Join(s.dir, "errors.jsonl"), s.errors.snapshot())
}

func writeJSONL(path string, items []sampleRecord) error {
	var buffer bytes.Buffer
	writer := bufio.NewWriter(&buffer)
	for _, item := range items {
		data, err := common.Marshal(item)
		if err != nil {
			return err
		}
		if _, err = writer.Write(data); err != nil {
			return err
		}
		if err = writer.WriteByte('\n'); err != nil {
			return err
		}
	}
	if err := writer.Flush(); err != nil {
		return err
	}
	return os.WriteFile(path, buffer.Bytes(), 0o644)
}

func maskHeaders(headers map[string]string, masks []string) map[string]string {
	out := make(map[string]string, len(headers))
	if len(headers) == 0 {
		return out
	}
	maskSet := make(map[string]struct{}, len(masks))
	for _, header := range masks {
		maskSet[strings.ToLower(header)] = struct{}{}
	}
	for key, value := range headers {
		if _, ok := maskSet[strings.ToLower(key)]; ok {
			out[key] = "***"
			continue
		}
		out[key] = value
	}
	return out
}

func previewString(value string, limit int, contentType string) bodyPreview {
	data := []byte(value)
	preview := bodyPreview{
		Bytes:       len(data),
		ContentType: contentType,
	}
	if limit <= 0 {
		limit = 1024
	}
	if looksBinary(data, contentType) {
		preview.Binary = true
		return preview
	}
	if len(data) > limit {
		preview.Truncated = true
		data = data[:limit]
	}
	preview.Text = string(data)
	trimmed := bytes.TrimSpace([]byte(preview.Text))
	if len(trimmed) > 0 {
		preview.JSONType = common.GetJsonType(trimmed)
	}
	return preview
}

func previewBytes(data []byte, limit int, contentType string) bodyPreview {
	preview := bodyPreview{
		Bytes:       len(data),
		ContentType: contentType,
	}
	if limit <= 0 {
		limit = 1024
	}
	if looksBinary(data, contentType) {
		preview.Binary = true
		if len(data) > limit {
			preview.Truncated = true
		}
		return preview
	}
	if len(data) > limit {
		preview.Truncated = true
		data = data[:limit]
	}
	preview.Text = string(data)
	trimmed := bytes.TrimSpace([]byte(preview.Text))
	if len(trimmed) > 0 {
		preview.JSONType = common.GetJsonType(trimmed)
	}
	return preview
}

func headerMap(headers http.Header) map[string]string {
	out := make(map[string]string, len(headers))
	for key, values := range headers {
		out[key] = strings.Join(values, ", ")
	}
	return out
}

func looksBinary(data []byte, contentType string) bool {
	contentType = strings.ToLower(strings.TrimSpace(contentType))
	if strings.Contains(contentType, "application/json") ||
		strings.Contains(contentType, "text/") ||
		strings.Contains(contentType, "application/xml") ||
		strings.Contains(contentType, "application/javascript") ||
		strings.Contains(contentType, "application/x-www-form-urlencoded") ||
		strings.Contains(contentType, "text/event-stream") {
		return false
	}
	if len(data) == 0 {
		return false
	}
	if !utf8.Valid(data) {
		return true
	}
	for _, b := range data {
		if b == 0 {
			return true
		}
	}
	return false
}

func makeSample(runID string, scenario scenarioConfig, stage string, success bool, errorKind string, durationMS int64, firstByteMS int64, eventCount int, doneReceived bool, requestURL string, requestHeaders map[string]string, requestBody string, responseStatus int, responseHeaders map[string]string, responseBody []byte, responseContentType string, sampling samplingConfig) sampleRecord {
	return sampleRecord{
		ID:           fmt.Sprintf("%s-%s-%d", scenario.ID, stage, timeNowUnixNano()),
		RunID:        runID,
		ScenarioID:   scenario.ID,
		ScenarioName: scenario.Name,
		Stage:        stage,
		Success:      success,
		ErrorKind:    errorKind,
		Timestamp:    time.Now(),
		DurationMS:   durationMS,
		FirstByteMS:  firstByteMS,
		EventCount:   eventCount,
		DoneReceived: doneReceived,
		Request: sampleRequestInfo{
			Method:  stageMethod(scenario, stage),
			URL:     requestURL,
			Headers: maskHeaders(requestHeaders, sampling.MaskHeaders),
			Body:    previewString(requestBody, sampling.MaxBodyBytes, requestHeaders["Content-Type"]),
		},
		Response: sampleResponseInfo{
			StatusCode: responseStatus,
			Headers:    maskHeaders(responseHeaders, sampling.MaskHeaders),
			Body:       previewBytes(responseBody, sampling.MaxBodyBytes, responseContentType),
		},
	}
}

func stageMethod(scenario scenarioConfig, stage string) string {
	if scenario.Mode == "task_flow" {
		if stage == "submit" {
			return scenario.TaskFlow.SubmitRequest.Method
		}
		if stage == "poll" {
			return scenario.TaskFlow.PollRequest.Method
		}
	}
	return scenario.Method
}

func timeNowUnixNano() int64 {
	return time.Now().UnixNano()
}
