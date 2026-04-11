package main

import "time"

type workerRuntimeSnapshot struct {
	Status       string       `json:"status"`
	PID          int          `json:"pid"`
	StartedAt    time.Time    `json:"started_at"`
	UptimeSec    int64        `json:"uptime_sec"`
	PublicBase   string       `json:"public_base"`
	PprofURL     string       `json:"pprof_url,omitempty"`
	Healthy      bool         `json:"healthy"`
	LastError    string       `json:"last_error,omitempty"`
	WorkerConfig workerConfig `json:"worker_config"`
}

type controlRuntimeResponse struct {
	ControlPort int                   `json:"control_port"`
	DataDir     string                `json:"data_dir"`
	Config      configView            `json:"config"`
	Worker      workerRuntimeSnapshot `json:"worker"`
}

type summaryResponse struct {
	StartedAt      time.Time        `json:"started_at"`
	UptimeSec      int64            `json:"uptime_sec"`
	TotalRequests  int64            `json:"total_requests"`
	Successes      int64            `json:"successes"`
	Errors         int64            `json:"errors"`
	CurrentQPS     float64          `json:"current_qps"`
	ErrorRate      float64          `json:"error_rate"`
	ActiveRequests int64            `json:"active_requests"`
	ActiveSSE      int64            `json:"active_sse"`
	VideoTasks     int              `json:"video_tasks"`
	StatusCodes    map[string]int64 `json:"status_codes"`
}

type routeStatsItem struct {
	Route       string           `json:"route"`
	Requests    int64            `json:"requests"`
	Successes   int64            `json:"successes"`
	Errors      int64            `json:"errors"`
	Active      int64            `json:"active"`
	LastStatus  int              `json:"last_status"`
	AverageMS   float64          `json:"average_ms"`
	P50MS       float64          `json:"p50_ms"`
	P95MS       float64          `json:"p95_ms"`
	StatusCodes map[string]int64 `json:"status_codes"`
}

type routeStatsResponse struct {
	Items []routeStatsItem `json:"items"`
}

type requestEvent struct {
	Timestamp  time.Time `json:"timestamp"`
	Route      string    `json:"route"`
	Method     string    `json:"method"`
	Model      string    `json:"model,omitempty"`
	StatusCode int       `json:"status_code"`
	LatencyMS  int64     `json:"latency_ms"`
	Stream     bool      `json:"stream"`
	Error      string    `json:"error,omitempty"`
}

type videoTaskEvent struct {
	Timestamp       time.Time `json:"timestamp"`
	TaskID          string    `json:"task_id"`
	Route           string    `json:"route"`
	Model           string    `json:"model"`
	Status          string    `json:"status"`
	DurationSeconds float64   `json:"duration_seconds"`
	Resolution      string    `json:"resolution"`
	FPS             int       `json:"fps"`
	WillFail        bool      `json:"will_fail"`
}

type eventsResponse struct {
	Errors   []requestEvent   `json:"errors"`
	Requests []requestEvent   `json:"requests"`
	Videos   []videoTaskEvent `json:"videos"`
}
