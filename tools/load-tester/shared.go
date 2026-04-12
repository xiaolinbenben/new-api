package main

import "time"

type workerRuntimeSnapshot struct {
	Status       string    `json:"status"`
	PID          int       `json:"pid"`
	StartedAt    time.Time `json:"started_at"`
	UptimeSec    int64     `json:"uptime_sec"`
	Healthy      bool      `json:"healthy"`
	LastError    string    `json:"last_error,omitempty"`
	WorkerPort   int       `json:"worker_port"`
	RunID        string    `json:"run_id,omitempty"`
	RunStatus    string    `json:"run_status"`
	RunStartedAt time.Time `json:"run_started_at,omitempty"`
}

type controlRuntimeResponse struct {
	ControlPort int                   `json:"control_port"`
	DataDir     string                `json:"data_dir"`
	Config      configView            `json:"config"`
	Worker      workerRuntimeSnapshot `json:"worker"`
}

type summaryResponse struct {
	RunID          string           `json:"run_id,omitempty"`
	RunStatus      string           `json:"run_status"`
	StartedAt      time.Time        `json:"started_at,omitempty"`
	FinishedAt     time.Time        `json:"finished_at,omitempty"`
	TotalRequests  int64            `json:"total_requests"`
	Successes      int64            `json:"successes"`
	Errors         int64            `json:"errors"`
	Timeouts       int64            `json:"timeouts"`
	CurrentTPS     float64          `json:"current_tps"`
	AverageMS      float64          `json:"average_ms"`
	P50MS          float64          `json:"p50_ms"`
	P95MS          float64          `json:"p95_ms"`
	P99MS          float64          `json:"p99_ms"`
	ActiveRequests int64            `json:"active_requests"`
	StatusCodes    map[string]int64 `json:"status_codes"`
	ErrorKinds     map[string]int64 `json:"error_kinds"`
}

type stageStats struct {
	Requests    int64            `json:"requests"`
	Successes   int64            `json:"successes"`
	Errors      int64            `json:"errors"`
	Timeouts    int64            `json:"timeouts"`
	Active      int64            `json:"active"`
	AverageMS   float64          `json:"average_ms"`
	P50MS       float64          `json:"p50_ms"`
	P95MS       float64          `json:"p95_ms"`
	P99MS       float64          `json:"p99_ms"`
	StatusCodes map[string]int64 `json:"status_codes"`
	ErrorKinds  map[string]int64 `json:"error_kinds"`
}

type scenarioStatsItem struct {
	ScenarioID  string                `json:"scenario_id"`
	Name        string                `json:"name"`
	Mode        string                `json:"mode"`
	Weight      int                   `json:"weight"`
	Enabled     bool                  `json:"enabled"`
	CurrentTPS  float64               `json:"current_tps"`
	Requests    int64                 `json:"requests"`
	Successes   int64                 `json:"successes"`
	Errors      int64                 `json:"errors"`
	Timeouts    int64                 `json:"timeouts"`
	Active      int64                 `json:"active"`
	AverageMS   float64               `json:"average_ms"`
	P50MS       float64               `json:"p50_ms"`
	P95MS       float64               `json:"p95_ms"`
	P99MS       float64               `json:"p99_ms"`
	ErrorKinds  map[string]int64      `json:"error_kinds"`
	StatusCodes map[string]int64      `json:"status_codes"`
	StageStats  map[string]stageStats `json:"stage_stats"`
}

type scenarioStatsResponse struct {
	Items []scenarioStatsItem `json:"items"`
}

type bodyPreview struct {
	Text        string `json:"text,omitempty"`
	Bytes       int    `json:"bytes"`
	Truncated   bool   `json:"truncated"`
	Binary      bool   `json:"binary"`
	ContentType string `json:"content_type,omitempty"`
	JSONType    string `json:"json_type,omitempty"`
}

type sampleRequestInfo struct {
	Method  string            `json:"method"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers"`
	Body    bodyPreview       `json:"body"`
}

type sampleResponseInfo struct {
	StatusCode int               `json:"status_code"`
	Headers    map[string]string `json:"headers"`
	Body       bodyPreview       `json:"body"`
}

type sampleRecord struct {
	ID           string             `json:"id"`
	RunID        string             `json:"run_id"`
	ScenarioID   string             `json:"scenario_id"`
	ScenarioName string             `json:"scenario_name"`
	Stage        string             `json:"stage"`
	Success      bool               `json:"success"`
	ErrorKind    string             `json:"error_kind,omitempty"`
	Timestamp    time.Time          `json:"timestamp"`
	DurationMS   int64              `json:"duration_ms"`
	FirstByteMS  int64              `json:"first_byte_ms,omitempty"`
	EventCount   int                `json:"event_count,omitempty"`
	DoneReceived bool               `json:"done_received,omitempty"`
	Request      sampleRequestInfo  `json:"request"`
	Response     sampleResponseInfo `json:"response"`
}

type samplesResponse struct {
	Requests []sampleRecord `json:"requests"`
	Errors   []sampleRecord `json:"errors"`
}

type historyItem struct {
	RunID         string    `json:"run_id"`
	RunStatus     string    `json:"run_status"`
	StartedAt     time.Time `json:"started_at"`
	FinishedAt    time.Time `json:"finished_at"`
	TotalRequests int64     `json:"total_requests"`
	Successes     int64     `json:"successes"`
	Errors        int64     `json:"errors"`
	Timeouts      int64     `json:"timeouts"`
	P95MS         float64   `json:"p95_ms"`
}

type historyListResponse struct {
	Items []historyItem `json:"items"`
}

type runRecord struct {
	RunID      string              `json:"run_id"`
	RunStatus  string              `json:"run_status"`
	StartedAt  time.Time           `json:"started_at"`
	FinishedAt time.Time           `json:"finished_at"`
	Config     configView          `json:"config"`
	Summary    summaryResponse     `json:"summary"`
	Scenarios  []scenarioStatsItem `json:"scenarios"`
}
