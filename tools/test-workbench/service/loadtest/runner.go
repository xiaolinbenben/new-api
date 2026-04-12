package loadtest

import (
	"bufio"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/tools/test-workbench/domain"
)

type runner struct {
	id        string
	cfg       domain.RunExecutionConfig
	startedAt time.Time

	stats   *runStatsCollector
	samples *sampleStore

	client    *http.Client
	scheduler *smoothWeightedScheduler

	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{}

	issuedRequests atomic.Int64
	stopRequested  atomic.Bool

	mu         sync.RWMutex
	runStatus  string
	finishedAt time.Time
}

func newRunner(runID string, cfg domain.RunExecutionConfig) (*runner, error) {
	scheduler, err := newSmoothWeightedScheduler(cfg.Scenarios)
	if err != nil {
		return nil, err
	}

	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          cfg.Run.TotalConcurrency * 2,
		MaxIdleConnsPerHost:   cfg.Run.TotalConcurrency * 2,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: time.Second,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: cfg.Target.InsecureSkipVerify,
		},
	}

	return &runner{
		id:        runID,
		cfg:       domain.CloneRunExecutionConfig(cfg),
		startedAt: time.Now(),
		stats:     newRunStatsCollector(time.Now(), cfg.Scenarios),
		samples:   newSampleStore(cfg.Run.Sampling),
		client:    &http.Client{Transport: transport},
		scheduler: scheduler,
		done:      make(chan struct{}),
		runStatus: "idle",
	}, nil
}

func (r *runner) start(parent context.Context) {
	r.ctx, r.cancel = context.WithCancel(parent)
	r.setStatus("running")
	go r.run()
}

func (r *runner) stop() {
	r.stopRequested.Store(true)
	if r.cancel != nil {
		r.cancel()
	}
}

func (r *runner) Done() <-chan struct{} {
	return r.done
}

func (r *runner) status() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.runStatus
}

func (r *runner) finishedTime() time.Time {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.finishedAt
}

func (r *runner) setStatus(status string) {
	r.mu.Lock()
	r.runStatus = status
	if status != "running" {
		r.finishedAt = time.Now()
	}
	r.mu.Unlock()
}

func (r *runner) summary() domain.RunSummary {
	return r.stats.summary(r.id, r.status(), r.finishedTime())
}

func (r *runner) scenarios() []domain.ScenarioStatsItem {
	return r.stats.scenariosSnapshot()
}

func (r *runner) samplesSnapshot() domain.SamplesResponse {
	return r.samples.snapshot()
}

func (r *runner) buildRecord(projectID string, environmentID string, runProfileID string) domain.RunRecord {
	finished := r.finishedTime()
	record := domain.RunRecord{
		ID:            r.id,
		ProjectID:     projectID,
		EnvironmentID: environmentID,
		RunProfileID:  runProfileID,
		Status:        r.status(),
		Config:        domain.CloneRunExecutionConfig(r.cfg),
		Summary:       r.summary(),
		Scenarios:     r.scenarios(),
		Samples:       r.samplesSnapshot(),
		StartedAt:     r.startedAt,
		CreatedAt:     r.startedAt,
		UpdatedAt:     finished,
	}
	if !finished.IsZero() {
		record.FinishedAt = &finished
	}
	return record
}

func (r *runner) run() {
	defer close(r.done)
	defer func() {
		if r.cancel != nil {
			r.cancel()
		}
	}()

	if r.cfg.Run.DurationSec > 0 {
		go func() {
			timer := time.NewTimer(time.Duration(r.cfg.Run.DurationSec) * time.Second)
			defer timer.Stop()
			select {
			case <-timer.C:
				if r.cancel != nil {
					r.cancel()
				}
			case <-r.ctx.Done():
			}
		}()
	}

	var wg sync.WaitGroup
	launchDelay := time.Duration(0)
	if r.cfg.Run.TotalConcurrency > 1 && r.cfg.Run.RampUpSec > 0 {
		launchDelay = time.Duration(r.cfg.Run.RampUpSec) * time.Second / time.Duration(r.cfg.Run.TotalConcurrency)
	}

	for i := 0; i < r.cfg.Run.TotalConcurrency; i++ {
		if i > 0 && launchDelay > 0 {
			select {
			case <-time.After(launchDelay):
			case <-r.ctx.Done():
				break
			}
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			r.workerLoop()
		}()
	}

	wg.Wait()
	if r.stopRequested.Load() {
		r.setStatus("stopped")
		return
	}
	r.setStatus("completed")
}

func (r *runner) workerLoop() {
	for {
		if r.ctx.Err() != nil {
			return
		}
		if !r.reserveLogicalRequest() {
			return
		}
		index, ok := r.scheduler.NextIndex()
		if !ok {
			return
		}
		r.executeScenario(r.cfg.Scenarios[index])
	}
}

func (r *runner) reserveLogicalRequest() bool {
	if r.cfg.Run.MaxRequests <= 0 {
		return true
	}
	maxRequests := int64(r.cfg.Run.MaxRequests)
	for {
		current := r.issuedRequests.Load()
		if current >= maxRequests {
			return false
		}
		if r.issuedRequests.CompareAndSwap(current, current+1) {
			return true
		}
	}
}

func (r *runner) executeScenario(scenario domain.ScenarioConfig) {
	r.stats.beginScenario(scenario.ID)
	success := false
	timeout := false
	errorKind := ""
	status := 0
	switch scenario.Mode {
	case "single":
		result := r.executeHTTPStage(scenario, "request", scenario.Method, scenario.Path, scenario.Headers, scenario.Body, scenario.ExpectedStatuses, false)
		success, timeout, errorKind, status = result.success, result.timeout, result.errorKind, result.statusCode
	case "sse":
		result := r.executeHTTPStage(scenario, "request", scenario.Method, scenario.Path, scenario.Headers, scenario.Body, scenario.ExpectedStatuses, true)
		success, timeout, errorKind, status = result.success, result.timeout, result.errorKind, result.statusCode
	case "task_flow":
		success, timeout, errorKind, status = r.executeTaskFlow(scenario)
	default:
		errorKind = "unsupported_mode"
	}
	r.stats.finishStage(scenario.ID, "scenario", status, 0, success, timeout, errorKind)
}

func (r *runner) executeTaskFlow(scenario domain.ScenarioConfig) (bool, bool, string, int) {
	submit := scenario.TaskFlow.SubmitRequest
	submitResult := r.executeHTTPStage(scenario, "submit", submit.Method, submit.Path, submit.Headers, submit.Body, submit.ExpectedStatuses, false)
	if !submitResult.success {
		return false, submitResult.timeout, submitResult.errorKind, submitResult.statusCode
	}

	taskID, err := extractJSONPath(submitResult.responseBody, scenario.Extractors.TaskIDPath)
	if err != nil || strings.TrimSpace(taskID) == "" {
		errorKind := "parse_error"
		submitResult.sample.Success = false
		submitResult.sample.ErrorKind = errorKind
		r.samples.addErrorOnly(submitResult.sample)
		return false, false, errorKind, submitResult.statusCode
	}

	pollStage := scenario.TaskFlow.PollRequest
	for i := 0; i < scenario.TaskFlow.MaxPolls; i++ {
		select {
		case <-r.ctx.Done():
			return false, true, "timeout", http.StatusRequestTimeout
		case <-time.After(time.Duration(scenario.TaskFlow.PollIntervalMS) * time.Millisecond):
		}

		path := strings.ReplaceAll(pollStage.PathTemplate, "{task_id}", url.PathEscape(taskID))
		result := r.executeHTTPStage(scenario, "poll", pollStage.Method, path, pollStage.Headers, "", pollStage.ExpectedStatuses, false)
		if !result.success {
			return false, result.timeout, result.errorKind, result.statusCode
		}

		state, err := extractJSONPath(result.responseBody, scenario.Extractors.TaskStatusPath)
		if err != nil {
			errorKind := "parse_error"
			result.sample.Success = false
			result.sample.ErrorKind = errorKind
			r.samples.addErrorOnly(result.sample)
			return false, false, errorKind, result.statusCode
		}
		normalizedState := strings.ToLower(strings.TrimSpace(state))
		if containsString(scenario.TaskFlow.SuccessValues, normalizedState) {
			return true, false, "", result.statusCode
		}
		if containsString(scenario.TaskFlow.FailureValues, normalizedState) {
			errorKind := "task_failed"
			result.sample.Success = false
			result.sample.ErrorKind = errorKind
			r.samples.addErrorOnly(result.sample)
			return false, false, errorKind, result.statusCode
		}
	}
	return false, true, "timeout", http.StatusRequestTimeout
}

type stageResult struct {
	success      bool
	timeout      bool
	errorKind    string
	statusCode   int
	durationMS   int64
	firstByteMS  int64
	eventCount   int
	doneReceived bool
	responseBody []byte
	sample       domain.SampleRecord
}

func (r *runner) executeHTTPStage(scenario domain.ScenarioConfig, stage string, method string, path string, headers map[string]string, body string, expectedStatuses []int, sse bool) stageResult {
	stageStart := time.Now()
	requestURL := buildRequestURL(r.cfg.Target.BaseURL, path)
	requestHeaders := mergeHeaders(r.cfg.Target.Headers, headers)
	requestBody := body

	stageCtx, cancel := context.WithTimeout(r.ctx, time.Duration(r.cfg.Run.RequestTimeoutMS)*time.Millisecond)
	defer cancel()
	r.stats.beginStage(scenario.ID, stage)

	var requestReader io.Reader
	if requestBody != "" {
		requestReader = strings.NewReader(requestBody)
	}
	req, err := http.NewRequestWithContext(stageCtx, method, requestURL, requestReader)
	if err != nil {
		return r.finishStageFailure(scenario, stage, requestURL, requestHeaders, requestBody, time.Since(stageStart).Milliseconds(), 0, false, "network_error", nil, "")
	}
	for key, value := range requestHeaders {
		req.Header.Set(key, value)
	}

	resp, err := r.client.Do(req)
	if err != nil {
		timeout, errorKind := classifyRequestError(err)
		return r.finishStageFailure(scenario, stage, requestURL, requestHeaders, requestBody, time.Since(stageStart).Milliseconds(), 0, timeout, errorKind, nil, "")
	}
	defer resp.Body.Close()

	responseHeaders := headerMap(resp.Header)
	contentType := resp.Header.Get("Content-Type")
	if !matchesExpectedStatus(resp.StatusCode, expectedStatuses) {
		bodyBytes := readResponseBody(resp.Body, r.cfg.Run.Sampling.MaxBodyBytes*8)
		return r.finishStageFailure(scenario, stage, requestURL, requestHeaders, requestBody, time.Since(stageStart).Milliseconds(), resp.StatusCode, false, "http_error", bodyBytes, contentType, responseHeaders)
	}

	if sse {
		return r.readSSEStage(scenario, stage, requestURL, requestHeaders, requestBody, resp, stageStart)
	}

	bodyBytes := readResponseBody(resp.Body, r.cfg.Run.Sampling.MaxBodyBytes*8)
	duration := time.Since(stageStart).Milliseconds()
	sample := makeSample(r.id, scenario, stage, true, "", duration, 0, 0, false, requestURL, requestHeaders, requestBody, resp.StatusCode, responseHeaders, bodyBytes, contentType, r.cfg.Run.Sampling)
	r.stats.finishStage(scenario.ID, stage, resp.StatusCode, duration, true, false, "")
	r.samples.add(sample)
	return stageResult{success: true, statusCode: resp.StatusCode, durationMS: duration, responseBody: bodyBytes, sample: sample}
}

func (r *runner) readSSEStage(scenario domain.ScenarioConfig, stage string, requestURL string, requestHeaders map[string]string, requestBody string, resp *http.Response, stageStart time.Time) stageResult {
	reader := bufio.NewReader(resp.Body)
	var preview strings.Builder
	previewLimit := maxInt(r.cfg.Run.Sampling.MaxBodyBytes*4, 8192)
	eventCount := 0
	doneReceived := false
	firstByteMS := int64(0)

	for {
		line, err := reader.ReadString('\n')
		if len(line) > 0 {
			if firstByteMS == 0 {
				firstByteMS = time.Since(stageStart).Milliseconds()
			}
			if preview.Len() < previewLimit {
				remaining := previewLimit - preview.Len()
				if len(line) > remaining {
					preview.WriteString(line[:remaining])
				} else {
					preview.WriteString(line)
				}
			}
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "data:") {
				payload := strings.TrimSpace(strings.TrimPrefix(trimmed, "data:"))
				eventCount++
				if payload == "[DONE]" {
					doneReceived = true
				}
			}
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			timeout, errorKind := classifyRequestError(err)
			return r.finishStageFailure(scenario, stage, requestURL, requestHeaders, requestBody, time.Since(stageStart).Milliseconds(), resp.StatusCode, timeout, errorKind, []byte(preview.String()), resp.Header.Get("Content-Type"), headerMap(resp.Header))
		}
	}

	duration := time.Since(stageStart).Milliseconds()
	success := doneReceived
	errorKind := ""
	if !doneReceived {
		errorKind = "sse_incomplete"
	}
	responseHeaders := headerMap(resp.Header)
	bodyBytes := []byte(preview.String())
	sample := makeSample(r.id, scenario, stage, success, errorKind, duration, firstByteMS, eventCount, doneReceived, requestURL, requestHeaders, requestBody, resp.StatusCode, responseHeaders, bodyBytes, resp.Header.Get("Content-Type"), r.cfg.Run.Sampling)
	r.stats.finishStage(scenario.ID, stage, resp.StatusCode, duration, success, false, errorKind)
	r.samples.add(sample)
	return stageResult{
		success:      success,
		errorKind:    errorKind,
		statusCode:   resp.StatusCode,
		durationMS:   duration,
		firstByteMS:  firstByteMS,
		eventCount:   eventCount,
		doneReceived: doneReceived,
		responseBody: bodyBytes,
		sample:       sample,
	}
}

func (r *runner) finishStageFailure(scenario domain.ScenarioConfig, stage string, requestURL string, requestHeaders map[string]string, requestBody string, duration int64, status int, timeout bool, errorKind string, responseBody []byte, contentType string, responseHeaders ...map[string]string) stageResult {
	headers := map[string]string{}
	if len(responseHeaders) > 0 {
		headers = responseHeaders[0]
	}
	sample := makeSample(r.id, scenario, stage, false, errorKind, duration, 0, 0, false, requestURL, requestHeaders, requestBody, status, headers, responseBody, contentType, r.cfg.Run.Sampling)
	r.stats.finishStage(scenario.ID, stage, status, duration, false, timeout, errorKind)
	r.samples.add(sample)
	return stageResult{success: false, timeout: timeout, errorKind: errorKind, statusCode: status, durationMS: duration, responseBody: responseBody, sample: sample}
}

type smoothWeightedScheduler struct {
	mu      sync.Mutex
	nodes   []schedulerNode
	total   int
	enabled int
}

type schedulerNode struct {
	index           int
	weight          int
	currentWeight   int
	effectiveWeight int
	enabled         bool
}

func newSmoothWeightedScheduler(items []domain.ScenarioConfig) (*smoothWeightedScheduler, error) {
	nodes := make([]schedulerNode, 0, len(items))
	total := 0
	enabled := 0
	for index, item := range items {
		weight := item.Weight
		if weight <= 0 {
			weight = 1
		}
		node := schedulerNode{
			index:           index,
			weight:          weight,
			effectiveWeight: weight,
			enabled:         item.Enabled,
		}
		nodes = append(nodes, node)
		if item.Enabled {
			total += weight
			enabled++
		}
	}
	if enabled == 0 {
		return nil, fmt.Errorf("at least one scenario must be enabled")
	}
	return &smoothWeightedScheduler{nodes: nodes, total: total, enabled: enabled}, nil
}

func (s *smoothWeightedScheduler) NextIndex() (int, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.enabled == 0 {
		return 0, false
	}
	best := -1
	for i := range s.nodes {
		if !s.nodes[i].enabled {
			continue
		}
		s.nodes[i].currentWeight += s.nodes[i].effectiveWeight
		if best == -1 || s.nodes[i].currentWeight > s.nodes[best].currentWeight {
			best = i
		}
	}
	if best == -1 {
		return 0, false
	}
	s.nodes[best].currentWeight -= s.total
	return s.nodes[best].index, true
}

func buildRequestURL(baseURL string, path string) string {
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return path
	}
	return strings.TrimRight(baseURL, "/") + "/" + strings.TrimLeft(path, "/")
}

func mergeHeaders(base map[string]string, override map[string]string) map[string]string {
	out := domain.CloneStringMap(base)
	for key, value := range override {
		out[key] = value
	}
	return out
}

func matchesExpectedStatus(status int, expected []int) bool {
	for _, item := range expected {
		if item == status {
			return true
		}
	}
	return false
}

func classifyRequestError(err error) (bool, string) {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return true, "timeout"
	case errors.Is(err, context.Canceled):
		return true, "timeout"
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true, "timeout"
	}
	return false, "network_error"
}

func readResponseBody(reader io.Reader, limit int) []byte {
	if limit <= 0 {
		limit = 8192
	}
	data, _ := io.ReadAll(io.LimitReader(reader, int64(limit)))
	return data
}

func extractJSONPath(data []byte, path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", fmt.Errorf("json path is empty")
	}
	var value any
	if err := common.Unmarshal(data, &value); err != nil {
		return "", err
	}
	current := value
	for _, part := range strings.Split(path, ".") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		switch typed := current.(type) {
		case map[string]any:
			next, ok := typed[part]
			if !ok {
				return "", fmt.Errorf("json path %q not found", path)
			}
			current = next
		case []any:
			index := 0
			_, err := fmt.Sscanf(part, "%d", &index)
			if err != nil || index < 0 || index >= len(typed) {
				return "", fmt.Errorf("json array index %q invalid for path %q", part, path)
			}
			current = typed[index]
		default:
			return "", fmt.Errorf("json path %q not found", path)
		}
	}
	switch typed := current.(type) {
	case string:
		return typed, nil
	case float64:
		return fmt.Sprintf("%v", typed), nil
	case bool:
		if typed {
			return "true", nil
		}
		return "false", nil
	default:
		data, err := common.Marshal(typed)
		if err != nil {
			return "", err
		}
		return strings.Trim(string(data), `"`), nil
	}
}

func containsString(values []string, target string) bool {
	target = strings.ToLower(strings.TrimSpace(target))
	for _, value := range values {
		if strings.ToLower(strings.TrimSpace(value)) == target {
			return true
		}
	}
	return false
}

func maxInt(a int, b int) int {
	if a > b {
		return a
	}
	return b
}
