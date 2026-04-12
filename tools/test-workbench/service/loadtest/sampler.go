package loadtest

import (
	"bytes"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/tools/test-workbench/domain"
)

type sampleBuffer struct {
	limit int
	items []domain.SampleRecord
}

func newSampleBuffer(limit int) *sampleBuffer {
	return &sampleBuffer{limit: limit}
}

func (b *sampleBuffer) add(item domain.SampleRecord) {
	b.items = append([]domain.SampleRecord{item}, b.items...)
	if len(b.items) > b.limit {
		b.items = b.items[:b.limit]
	}
}

func (b *sampleBuffer) snapshot() []domain.SampleRecord {
	out := make([]domain.SampleRecord, len(b.items))
	copy(out, b.items)
	return out
}

type sampleStore struct {
	maskHeads []string

	mu       sync.Mutex
	requests *sampleBuffer
	errors   *sampleBuffer
}

func newSampleStore(sampling domain.SamplingConfig) *sampleStore {
	return &sampleStore{
		maskHeads: domain.NormalizeHeaderMasks(sampling.MaskHeaders),
		requests:  newSampleBuffer(sampling.MaxRequestSamples),
		errors:    newSampleBuffer(sampling.MaxErrorSamples),
	}
}

func (s *sampleStore) add(item domain.SampleRecord) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.requests.add(item)
	if !item.Success || item.ErrorKind != "" {
		s.errors.add(item)
	}
}

func (s *sampleStore) addErrorOnly(item domain.SampleRecord) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.errors.add(item)
}

func (s *sampleStore) snapshot() domain.SamplesResponse {
	s.mu.Lock()
	defer s.mu.Unlock()
	return domain.SamplesResponse{
		Requests: s.requests.snapshot(),
		Errors:   s.errors.snapshot(),
	}
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

func previewString(value string, limit int, contentType string) domain.BodyPreview {
	data := []byte(value)
	preview := domain.BodyPreview{Bytes: len(data), ContentType: contentType}
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

func previewBytes(data []byte, limit int, contentType string) domain.BodyPreview {
	preview := domain.BodyPreview{Bytes: len(data), ContentType: contentType}
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

func makeSample(runID string, scenario domain.ScenarioConfig, stage string, success bool, errorKind string, durationMS int64, firstByteMS int64, eventCount int, doneReceived bool, requestURL string, requestHeaders map[string]string, requestBody string, responseStatus int, responseHeaders map[string]string, responseBody []byte, responseContentType string, sampling domain.SamplingConfig) domain.SampleRecord {
	return domain.SampleRecord{
		ID:           fmt.Sprintf("%s-%s-%d", scenario.ID, stage, time.Now().UnixNano()),
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
		Request: domain.SampleRequestInfo{
			Method:  stageMethod(scenario, stage),
			URL:     requestURL,
			Headers: maskHeaders(requestHeaders, sampling.MaskHeaders),
			Body:    previewString(requestBody, sampling.MaxBodyBytes, requestHeaders["Content-Type"]),
		},
		Response: domain.SampleResponseInfo{
			StatusCode: responseStatus,
			Headers:    maskHeaders(responseHeaders, sampling.MaskHeaders),
			Body:       previewBytes(responseBody, sampling.MaxBodyBytes, responseContentType),
		},
	}
}

func stageMethod(scenario domain.ScenarioConfig, stage string) string {
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
