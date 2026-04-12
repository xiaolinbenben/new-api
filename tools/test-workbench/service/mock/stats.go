package mock

import (
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/tools/test-workbench/domain"
)

type eventBuffer[T any] struct {
	mu    sync.RWMutex
	limit int
	items []T
}

func newEventBuffer[T any](limit int) *eventBuffer[T] {
	return &eventBuffer[T]{limit: limit}
}

func (b *eventBuffer[T]) add(item T) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.items = append([]T{item}, b.items...)
	if len(b.items) > b.limit {
		b.items = b.items[:b.limit]
	}
}

func (b *eventBuffer[T]) snapshot() []T {
	b.mu.RLock()
	defer b.mu.RUnlock()
	out := make([]T, len(b.items))
	copy(out, b.items)
	return out
}

func (b *eventBuffer[T]) reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.items = nil
}

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

type routeMetrics struct {
	route string

	active    atomic.Int64
	requests  atomic.Int64
	successes atomic.Int64
	errors    atomic.Int64

	mu          sync.Mutex
	lastStatus  int
	latencies   *latencyWindow
	statusCodes map[int]int64
}

func newRouteMetrics(route string) *routeMetrics {
	return &routeMetrics{
		route:       route,
		latencies:   newLatencyWindow(512),
		statusCodes: make(map[int]int64),
	}
}

func (m *routeMetrics) begin() {
	m.active.Add(1)
}

func (m *routeMetrics) finish(status int, latency int64) {
	m.active.Add(-1)
	m.requests.Add(1)
	if status >= 200 && status < 400 {
		m.successes.Add(1)
	} else {
		m.errors.Add(1)
	}
	m.latencies.add(latency)
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lastStatus = status
	m.statusCodes[status]++
}

func (m *routeMetrics) snapshot() domain.RouteStatsItem {
	latencies := m.latencies.snapshot()
	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })

	m.mu.Lock()
	statusCodes := make(map[string]int64, len(m.statusCodes))
	for code, count := range m.statusCodes {
		statusCodes[strconv.Itoa(code)] = count
	}
	lastStatus := m.lastStatus
	m.mu.Unlock()

	return domain.RouteStatsItem{
		Route:       m.route,
		Requests:    m.requests.Load(),
		Successes:   m.successes.Load(),
		Errors:      m.errors.Load(),
		Active:      m.active.Load(),
		LastStatus:  lastStatus,
		AverageMS:   average(latencies),
		P50MS:       percentile(latencies, 0.50),
		P95MS:       percentile(latencies, 0.95),
		StatusCodes: statusCodes,
	}
}

type statsCollector struct {
	startedAt time.Time

	activeRequests atomic.Int64
	activeSSE      atomic.Int64
	totalRequests  atomic.Int64
	successes      atomic.Int64
	errors         atomic.Int64

	routesMu sync.RWMutex
	routes   map[string]*routeMetrics

	requests *eventBuffer[domain.RequestEvent]
	errorsEv *eventBuffer[domain.RequestEvent]
	videos   *eventBuffer[domain.VideoTaskEvent]
}

func newStatsCollector(startedAt time.Time) *statsCollector {
	return &statsCollector{
		startedAt: startedAt,
		routes:    make(map[string]*routeMetrics),
		requests:  newEventBuffer[domain.RequestEvent](200),
		errorsEv:  newEventBuffer[domain.RequestEvent](100),
		videos:    newEventBuffer[domain.VideoTaskEvent](100),
	}
}

func (s *statsCollector) begin(route string) {
	s.activeRequests.Add(1)
	s.route(route).begin()
}

func (s *statsCollector) finish(route string, status int, latency int64, event domain.RequestEvent) {
	s.activeRequests.Add(-1)
	s.totalRequests.Add(1)
	if status >= 200 && status < 400 {
		s.successes.Add(1)
	} else {
		s.errors.Add(1)
	}
	rm := s.route(route)
	rm.finish(status, latency)
	s.requests.add(event)
	if event.Error != "" || status >= 400 {
		s.errorsEv.add(event)
	}
}

func (s *statsCollector) route(route string) *routeMetrics {
	s.routesMu.RLock()
	rm, ok := s.routes[route]
	s.routesMu.RUnlock()
	if ok {
		return rm
	}
	s.routesMu.Lock()
	defer s.routesMu.Unlock()
	if rm, ok = s.routes[route]; ok {
		return rm
	}
	rm = newRouteMetrics(route)
	s.routes[route] = rm
	return rm
}

func (s *statsCollector) setSSEActive(delta int64) {
	s.activeSSE.Add(delta)
}

func (s *statsCollector) addVideoEvent(event domain.VideoTaskEvent) {
	s.videos.add(event)
}

func (s *statsCollector) summary(videoTasks int) domain.MockSummary {
	requests := s.requests.snapshot()
	lastMinute := 0
	now := time.Now()
	for _, event := range requests {
		if now.Sub(event.Timestamp) <= time.Minute {
			lastMinute++
		}
	}

	statusCodes := make(map[string]int64)
	s.routesMu.RLock()
	for _, route := range s.routes {
		snapshot := route.snapshot()
		for code, count := range snapshot.StatusCodes {
			statusCodes[code] += count
		}
	}
	s.routesMu.RUnlock()

	total := s.totalRequests.Load()
	errorsCount := s.errors.Load()
	errorRate := 0.0
	if total > 0 {
		errorRate = float64(errorsCount) / float64(total)
	}

	return domain.MockSummary{
		StartedAt:      s.startedAt,
		UptimeSec:      int64(time.Since(s.startedAt).Seconds()),
		TotalRequests:  total,
		Successes:      s.successes.Load(),
		Errors:         errorsCount,
		CurrentQPS:     float64(lastMinute) / 60,
		ErrorRate:      errorRate,
		ActiveRequests: s.activeRequests.Load(),
		ActiveSSE:      s.activeSSE.Load(),
		VideoTasks:     videoTasks,
		StatusCodes:    statusCodes,
	}
}

func (s *statsCollector) routesSnapshot() []domain.RouteStatsItem {
	s.routesMu.RLock()
	items := make([]domain.RouteStatsItem, 0, len(s.routes))
	for _, route := range s.routes {
		items = append(items, route.snapshot())
	}
	s.routesMu.RUnlock()
	sort.Slice(items, func(i, j int) bool { return items[i].Route < items[j].Route })
	return items
}

func (s *statsCollector) eventsSnapshot() domain.EventsResponse {
	return domain.EventsResponse{
		Errors:   s.errorsEv.snapshot(),
		Requests: s.requests.snapshot(),
		Videos:   s.videos.snapshot(),
	}
}

func (s *statsCollector) reset(startedAt time.Time) {
	s.startedAt = startedAt
	s.activeRequests.Store(0)
	s.activeSSE.Store(0)
	s.totalRequests.Store(0)
	s.successes.Store(0)
	s.errors.Store(0)
	s.routesMu.Lock()
	s.routes = make(map[string]*routeMetrics)
	s.routesMu.Unlock()
	s.requests.reset()
	s.errorsEv.reset()
	s.videos.reset()
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
