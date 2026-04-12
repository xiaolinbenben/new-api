package loadtest

import (
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/tools/test-workbench/domain"
)

type latencyWindow struct {
	mu     sync.Mutex
	values []int64
	index  int
	full   bool
}

func newLatencyWindow(size int) *latencyWindow {
	return &latencyWindow{values: make([]int64, size)}
}

func (w *latencyWindow) add(value int64) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(w.values) == 0 {
		return
	}
	w.values[w.index] = value
	w.index = (w.index + 1) % len(w.values)
	if w.index == 0 {
		w.full = true
	}
}

func (w *latencyWindow) snapshot() []int64 {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.full {
		out := make([]int64, len(w.values))
		copy(out, w.values)
		return out
	}
	out := make([]int64, w.index)
	copy(out, w.values[:w.index])
	return out
}

type timestampWindow struct {
	mu    sync.Mutex
	limit int
	items []time.Time
}

func newTimestampWindow(limit int) *timestampWindow {
	return &timestampWindow{limit: limit}
}

func (w *timestampWindow) add(value time.Time) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.items = append(w.items, value)
	if len(w.items) > w.limit {
		w.items = w.items[len(w.items)-w.limit:]
	}
}

func (w *timestampWindow) ratePerSecond(window time.Duration) float64 {
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(w.items) == 0 {
		return 0
	}
	threshold := time.Now().Add(-window)
	count := 0
	for i := len(w.items) - 1; i >= 0; i-- {
		if w.items[i].Before(threshold) {
			break
		}
		count++
	}
	return float64(count) / window.Seconds()
}

type metricsBucket struct {
	active   atomic.Int64
	requests atomic.Int64
	success  atomic.Int64
	errors   atomic.Int64
	timeouts atomic.Int64

	latencies *latencyWindow

	mu          sync.Mutex
	statusCodes map[int]int64
	errorKinds  map[string]int64
}

func newMetricsBucket() *metricsBucket {
	return &metricsBucket{
		latencies:   newLatencyWindow(2048),
		statusCodes: make(map[int]int64),
		errorKinds:  make(map[string]int64),
	}
}

func (b *metricsBucket) begin() {
	b.active.Add(1)
}

func (b *metricsBucket) finish(status int, latency int64, success bool, timeout bool, errorKind string) {
	b.active.Add(-1)
	b.requests.Add(1)
	if success {
		b.success.Add(1)
	} else {
		b.errors.Add(1)
	}
	if timeout {
		b.timeouts.Add(1)
	}
	b.latencies.add(latency)
	b.mu.Lock()
	defer b.mu.Unlock()
	if status > 0 {
		b.statusCodes[status]++
	}
	if errorKind != "" {
		b.errorKinds[errorKind]++
	}
}

func (b *metricsBucket) snapshot() domain.StageStats {
	latencies := b.latencies.snapshot()
	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })

	b.mu.Lock()
	statusCodes := make(map[string]int64, len(b.statusCodes))
	for code, count := range b.statusCodes {
		statusCodes[strconv.Itoa(code)] = count
	}
	errorKinds := make(map[string]int64, len(b.errorKinds))
	for kind, count := range b.errorKinds {
		errorKinds[kind] = count
	}
	b.mu.Unlock()

	return domain.StageStats{
		Requests:    b.requests.Load(),
		Successes:   b.success.Load(),
		Errors:      b.errors.Load(),
		Timeouts:    b.timeouts.Load(),
		Active:      b.active.Load(),
		AverageMS:   average(latencies),
		P50MS:       percentile(latencies, 0.50),
		P95MS:       percentile(latencies, 0.95),
		P99MS:       percentile(latencies, 0.99),
		StatusCodes: statusCodes,
		ErrorKinds:  errorKinds,
	}
}

type scenarioMetrics struct {
	id      string
	name    string
	mode    string
	weight  int
	enabled bool

	overall    *metricsBucket
	stageTimes *timestampWindow
	stagesMu   sync.RWMutex
	stages     map[string]*metricsBucket
}

func newScenarioMetrics(s domain.ScenarioConfig) *scenarioMetrics {
	return &scenarioMetrics{
		id:         s.ID,
		name:       s.Name,
		mode:       s.Mode,
		weight:     s.Weight,
		enabled:    s.Enabled,
		overall:    newMetricsBucket(),
		stageTimes: newTimestampWindow(4096),
		stages:     make(map[string]*metricsBucket),
	}
}

func (m *scenarioMetrics) stage(name string) *metricsBucket {
	m.stagesMu.RLock()
	stage, ok := m.stages[name]
	m.stagesMu.RUnlock()
	if ok {
		return stage
	}
	m.stagesMu.Lock()
	defer m.stagesMu.Unlock()
	if stage, ok = m.stages[name]; ok {
		return stage
	}
	stage = newMetricsBucket()
	m.stages[name] = stage
	return stage
}

func (m *scenarioMetrics) snapshot() domain.ScenarioStatsItem {
	stageStatsMap := make(map[string]domain.StageStats)
	statusCodes := make(map[string]int64)

	m.stagesMu.RLock()
	for stageName, bucket := range m.stages {
		snapshot := bucket.snapshot()
		stageStatsMap[stageName] = snapshot
		for code, count := range snapshot.StatusCodes {
			statusCodes[code] += count
		}
	}
	m.stagesMu.RUnlock()

	overall := m.overall.snapshot()
	return domain.ScenarioStatsItem{
		ScenarioID:  m.id,
		Name:        m.name,
		Mode:        m.mode,
		Weight:      m.weight,
		Enabled:     m.enabled,
		CurrentTPS:  m.stageTimes.ratePerSecond(10 * time.Second),
		Requests:    overall.Requests,
		Successes:   overall.Successes,
		Errors:      overall.Errors,
		Timeouts:    overall.Timeouts,
		Active:      overall.Active,
		AverageMS:   overall.AverageMS,
		P50MS:       overall.P50MS,
		P95MS:       overall.P95MS,
		P99MS:       overall.P99MS,
		ErrorKinds:  overall.ErrorKinds,
		StatusCodes: statusCodes,
		StageStats:  stageStatsMap,
	}
}

type runStatsCollector struct {
	startedAt time.Time
	overall   *metricsBucket
	times     *timestampWindow

	mu        sync.RWMutex
	scenarios map[string]*scenarioMetrics
}

func newRunStatsCollector(startedAt time.Time, scenarios []domain.ScenarioConfig) *runStatsCollector {
	items := make(map[string]*scenarioMetrics, len(scenarios))
	for _, scenario := range scenarios {
		items[scenario.ID] = newScenarioMetrics(scenario)
	}
	return &runStatsCollector{
		startedAt: startedAt,
		overall:   newMetricsBucket(),
		times:     newTimestampWindow(4096),
		scenarios: items,
	}
}

func (c *runStatsCollector) beginScenario(scenarioID string) {
	c.overall.begin()
	if item := c.scenario(scenarioID); item != nil {
		item.overall.begin()
	}
}

func (c *runStatsCollector) beginStage(scenarioID string, stage string) {
	if item := c.scenario(scenarioID); item != nil {
		item.stage(stage).begin()
	}
}

func (c *runStatsCollector) finishStage(scenarioID string, stage string, status int, latency int64, success bool, timeout bool, errorKind string) {
	c.times.add(time.Now())
	c.overall.finish(status, latency, success, timeout, errorKind)
	item := c.scenario(scenarioID)
	if item == nil {
		return
	}
	item.stageTimes.add(time.Now())
	item.overall.finish(status, latency, success, timeout, errorKind)
	item.stage(stage).finish(status, latency, success, timeout, errorKind)
}

func (c *runStatsCollector) scenario(id string) *scenarioMetrics {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.scenarios[id]
}

func (c *runStatsCollector) summary(runID string, status string, finishedAt time.Time) domain.RunSummary {
	snapshot := c.overall.snapshot()
	return domain.RunSummary{
		RunID:          runID,
		RunStatus:      status,
		StartedAt:      c.startedAt,
		FinishedAt:     finishedAt,
		TotalRequests:  snapshot.Requests,
		Successes:      snapshot.Successes,
		Errors:         snapshot.Errors,
		Timeouts:       snapshot.Timeouts,
		CurrentTPS:     c.times.ratePerSecond(10 * time.Second),
		AverageMS:      snapshot.AverageMS,
		P50MS:          snapshot.P50MS,
		P95MS:          snapshot.P95MS,
		P99MS:          snapshot.P99MS,
		ActiveRequests: snapshot.Active,
		StatusCodes:    snapshot.StatusCodes,
		ErrorKinds:     snapshot.ErrorKinds,
	}
}

func (c *runStatsCollector) scenariosSnapshot() []domain.ScenarioStatsItem {
	c.mu.RLock()
	defer c.mu.RUnlock()
	items := make([]domain.ScenarioStatsItem, 0, len(c.scenarios))
	for _, scenario := range c.scenarios {
		items = append(items, scenario.snapshot())
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ScenarioID < items[j].ScenarioID })
	return items
}

func percentile(values []int64, fraction float64) float64 {
	if len(values) == 0 {
		return 0
	}
	index := int(float64(len(values)-1) * fraction)
	if index < 0 {
		index = 0
	}
	if index >= len(values) {
		index = len(values) - 1
	}
	return float64(values[index])
}

func average(values []int64) float64 {
	if len(values) == 0 {
		return 0
	}
	var total int64
	for _, value := range values {
		total += value
	}
	return float64(total) / float64(len(values))
}
