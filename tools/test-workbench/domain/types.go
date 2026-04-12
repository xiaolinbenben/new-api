package domain

import (
	"encoding/json"
	"time"
)

const (
	TargetTypeInternalMock = "internal_mock"
	TargetTypeExternalHTTP = "external_http"
)

type IntRange struct {
	Min int `json:"min"`
	Max int `json:"max"`
}

type FloatRange struct {
	Min float64 `json:"min"`
	Max float64 `json:"max"`
}

type GlobalRandomConfig struct {
	Mode                string   `json:"mode"`
	Seed                int64    `json:"seed"`
	LatencyMS           IntRange `json:"latency_ms"`
	ErrorRate           float64  `json:"error_rate"`
	TooManyRequestsRate float64  `json:"too_many_requests_rate"`
	ServerErrorRate     float64  `json:"server_error_rate"`
	TimeoutRate         float64  `json:"timeout_rate"`
	DefaultTokenLength  IntRange `json:"default_token_length"`
	DefaultStreamChunks IntRange `json:"default_stream_chunks"`
}

type TextBehaviorConfig struct {
	TextTokens              IntRange `json:"text_tokens"`
	AllowStream             bool     `json:"allow_stream"`
	UsageProbability        float64  `json:"usage_probability"`
	ToolCallProbability     float64  `json:"tool_call_probability"`
	ToolCallCount           IntRange `json:"tool_call_count"`
	ToolArgumentsBytes      IntRange `json:"tool_arguments_bytes"`
	MCPToolProbability      float64  `json:"mcp_tool_probability"`
	FinalAnswerProbability  float64  `json:"final_answer_probability"`
	SystemFingerprintChance float64  `json:"system_fingerprint_probability"`
}

type ImageBehaviorConfig struct {
	Sizes             []string `json:"sizes"`
	ResponseURLRate   float64  `json:"response_url_rate"`
	ImageCount        IntRange `json:"image_count"`
	ImageBytes        IntRange `json:"image_bytes"`
	WatermarkTexts    []string `json:"watermark_texts"`
	WatermarkRate     float64  `json:"watermark_rate"`
	BackgroundPalette []string `json:"background_palette"`
}

type VideoBehaviorConfig struct {
	DurationsSeconds FloatRange `json:"durations_seconds"`
	Resolutions      []string   `json:"resolutions"`
	FPS              IntRange   `json:"fps"`
	PollIntervalMS   IntRange   `json:"poll_interval_ms"`
	FailureRate      float64    `json:"failure_rate"`
	VideoBytes       IntRange   `json:"video_bytes"`
	ProgressJitter   IntRange   `json:"progress_jitter"`
}

type MockProfileConfig struct {
	Models      []string            `json:"models"`
	EnablePprof bool                `json:"enable_pprof"`
	PprofPort   int                 `json:"pprof_port"`
	Random      GlobalRandomConfig  `json:"random"`
	Chat        TextBehaviorConfig  `json:"chat"`
	Responses   TextBehaviorConfig  `json:"responses"`
	Images      ImageBehaviorConfig `json:"images"`
	Videos      VideoBehaviorConfig `json:"videos"`
}

type SamplingConfig struct {
	MaxRequestSamples int      `json:"max_request_samples"`
	MaxErrorSamples   int      `json:"max_error_samples"`
	MaxBodyBytes      int      `json:"max_body_bytes"`
	MaskHeaders       []string `json:"mask_headers"`
}

type RunProfileConfig struct {
	TotalConcurrency int            `json:"total_concurrency"`
	RampUpSec        int            `json:"ramp_up_sec"`
	DurationSec      int            `json:"duration_sec"`
	MaxRequests      int            `json:"max_requests"`
	RequestTimeoutMS int            `json:"request_timeout_ms"`
	Sampling         SamplingConfig `json:"sampling"`
}

type ExtractorConfig struct {
	TaskIDPath     string `json:"task_id_path"`
	TaskStatusPath string `json:"task_status_path"`
}

type ScenarioRequestConfig struct {
	Method           string            `json:"method"`
	Path             string            `json:"path"`
	Headers          map[string]string `json:"headers"`
	Body             string            `json:"body"`
	ExpectedStatuses []int             `json:"expected_statuses"`
}

type PollRequestConfig struct {
	Method           string            `json:"method"`
	PathTemplate     string            `json:"path_template"`
	Headers          map[string]string `json:"headers"`
	ExpectedStatuses []int             `json:"expected_statuses"`
}

type TaskFlowConfig struct {
	SubmitRequest  ScenarioRequestConfig `json:"submit_request"`
	PollRequest    PollRequestConfig     `json:"poll_request"`
	SuccessValues  []string              `json:"success_values"`
	FailureValues  []string              `json:"failure_values"`
	PollIntervalMS int                   `json:"poll_interval_ms"`
	MaxPolls       int                   `json:"max_polls"`
}

type ScenarioConfig struct {
	ID               string            `json:"id"`
	Name             string            `json:"name"`
	Enabled          bool              `json:"enabled"`
	Preset           string            `json:"preset"`
	Mode             string            `json:"mode"`
	Weight           int               `json:"weight"`
	Method           string            `json:"method"`
	Path             string            `json:"path"`
	Headers          map[string]string `json:"headers"`
	Body             string            `json:"body"`
	ExpectedStatuses []int             `json:"expected_statuses"`
	Extractors       ExtractorConfig   `json:"extractors"`
	TaskFlow         TaskFlowConfig    `json:"task_flow"`
}

type Environment struct {
	ID                 string            `json:"id"`
	ProjectID          string            `json:"project_id"`
	Name               string            `json:"name"`
	TargetType         string            `json:"target_type"`
	ExternalBaseURL    string            `json:"external_base_url"`
	DefaultHeaders     map[string]string `json:"default_headers"`
	InsecureSkipVerify bool              `json:"insecure_skip_verify"`
	MockBindHost       string            `json:"mock_bind_host"`
	MockPort           int               `json:"mock_port"`
	MockRequireAuth    bool              `json:"mock_require_auth"`
	MockAuthToken      string            `json:"mock_auth_token"`
	AutoStart          bool              `json:"auto_start"`
	DefaultMockProfile string            `json:"default_mock_profile_id"`
	DefaultRunProfile  string            `json:"default_run_profile_id"`
	CreatedAt          time.Time         `json:"created_at"`
	UpdatedAt          time.Time         `json:"updated_at"`
}

type Project struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	IsDefault   bool      `json:"is_default"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type MockProfile struct {
	ID        string            `json:"id"`
	ProjectID string            `json:"project_id"`
	Name      string            `json:"name"`
	Config    MockProfileConfig `json:"config"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
}

type RunProfile struct {
	ID        string           `json:"id"`
	ProjectID string           `json:"project_id"`
	Name      string           `json:"name"`
	Config    RunProfileConfig `json:"config"`
	CreatedAt time.Time        `json:"created_at"`
	UpdatedAt time.Time        `json:"updated_at"`
}

type Scenario struct {
	ID        string         `json:"id"`
	ProjectID string         `json:"project_id"`
	Name      string         `json:"name"`
	Config    ScenarioConfig `json:"config"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
}

type RuntimeLoadTarget struct {
	BaseURL            string            `json:"base_url"`
	Headers            map[string]string `json:"headers"`
	InsecureSkipVerify bool              `json:"insecure_skip_verify"`
}

type RunExecutionConfig struct {
	Target    RuntimeLoadTarget `json:"target"`
	Run       RunProfileConfig  `json:"run"`
	Scenarios []ScenarioConfig  `json:"scenarios"`
}

type BodyPreview struct {
	Text        string `json:"text,omitempty"`
	Bytes       int    `json:"bytes"`
	Truncated   bool   `json:"truncated"`
	Binary      bool   `json:"binary"`
	ContentType string `json:"content_type,omitempty"`
	JSONType    string `json:"json_type,omitempty"`
}

type SampleRequestInfo struct {
	Method  string            `json:"method"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers"`
	Body    BodyPreview       `json:"body"`
}

type SampleResponseInfo struct {
	StatusCode int               `json:"status_code"`
	Headers    map[string]string `json:"headers"`
	Body       BodyPreview       `json:"body"`
}

type SampleRecord struct {
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
	Request      SampleRequestInfo  `json:"request"`
	Response     SampleResponseInfo `json:"response"`
}

type SamplesResponse struct {
	Requests []SampleRecord `json:"requests"`
	Errors   []SampleRecord `json:"errors"`
}

type StageStats struct {
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

type RunSummary struct {
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

type ScenarioStatsItem struct {
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
	StageStats  map[string]StageStats `json:"stage_stats"`
}

type RunRecord struct {
	ID            string              `json:"id"`
	ProjectID     string              `json:"project_id"`
	EnvironmentID string              `json:"environment_id"`
	RunProfileID  string              `json:"run_profile_id"`
	Status        string              `json:"status"`
	Config        RunExecutionConfig  `json:"config"`
	Summary       RunSummary          `json:"summary"`
	Scenarios     []ScenarioStatsItem `json:"scenarios"`
	Samples       SamplesResponse     `json:"samples"`
	StartedAt     time.Time           `json:"started_at"`
	FinishedAt    *time.Time          `json:"finished_at,omitempty"`
	CreatedAt     time.Time           `json:"created_at"`
	UpdatedAt     time.Time           `json:"updated_at"`
}

type RunListItem struct {
	ID            string     `json:"id"`
	ProjectID     string     `json:"project_id"`
	EnvironmentID string     `json:"environment_id"`
	RunProfileID  string     `json:"run_profile_id"`
	Status        string     `json:"status"`
	StartedAt     time.Time  `json:"started_at"`
	FinishedAt    *time.Time `json:"finished_at,omitempty"`
	TotalRequests int64      `json:"total_requests"`
	Successes     int64      `json:"successes"`
	Errors        int64      `json:"errors"`
	Timeouts      int64      `json:"timeouts"`
	P95MS         float64    `json:"p95_ms"`
}

type MockSummary struct {
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

type RouteStatsItem struct {
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

type RequestEvent struct {
	Timestamp  time.Time `json:"timestamp"`
	Route      string    `json:"route"`
	Method     string    `json:"method"`
	Model      string    `json:"model,omitempty"`
	StatusCode int       `json:"status_code"`
	LatencyMS  int64     `json:"latency_ms"`
	Stream     bool      `json:"stream"`
	Error      string    `json:"error,omitempty"`
}

type VideoTaskEvent struct {
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

type EventsResponse struct {
	Errors   []RequestEvent   `json:"errors"`
	Requests []RequestEvent   `json:"requests"`
	Videos   []VideoTaskEvent `json:"videos"`
}

type MockListenerRuntime struct {
	EnvironmentID string      `json:"environment_id"`
	ProjectID     string      `json:"project_id"`
	Name          string      `json:"name"`
	Status        string      `json:"status"`
	ListenAddress string      `json:"listen_address"`
	LocalBaseURL  string      `json:"local_base_url"`
	PublicBaseURL string      `json:"public_base_url"`
	RequireAuth   bool        `json:"require_auth"`
	Healthy       bool        `json:"healthy"`
	StartedAt     time.Time   `json:"started_at"`
	ProfileID     string      `json:"profile_id"`
	Summary       MockSummary `json:"summary"`
}

type LoadRunRuntime struct {
	RunID         string          `json:"run_id"`
	ProjectID     string          `json:"project_id"`
	EnvironmentID string          `json:"environment_id"`
	RunProfileID  string          `json:"run_profile_id"`
	Status        string          `json:"status"`
	StartedAt     time.Time       `json:"started_at"`
	TargetBaseURL string          `json:"target_base_url"`
	Summary       RunSummary      `json:"summary"`
	Samples       SamplesResponse `json:"samples"`
}

type MetricEnvelope struct {
	Kind       string          `json:"kind"`
	ScenarioID string          `json:"scenario_id,omitempty"`
	Payload    json.RawMessage `json:"payload"`
}
