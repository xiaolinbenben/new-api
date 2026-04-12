package main

import (
	"fmt"
	"maps"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"
	"github.com/google/uuid"
)

type workerConfig struct {
	Port            int    `json:"port"`
	ManagementToken string `json:"management_token"`
}

type targetConfig struct {
	BaseURL            string            `json:"base_url"`
	Headers            map[string]string `json:"headers"`
	InsecureSkipVerify bool              `json:"insecure_skip_verify"`
}

type runConfig struct {
	TotalConcurrency int `json:"total_concurrency"`
	RampUpSec        int `json:"ramp_up_sec"`
	DurationSec      int `json:"duration_sec"`
	MaxRequests      int `json:"max_requests"`
	RequestTimeoutMS int `json:"request_timeout_ms"`
}

type samplingConfig struct {
	MaxRequestSamples int      `json:"max_request_samples"`
	MaxErrorSamples   int      `json:"max_error_samples"`
	MaxBodyBytes      int      `json:"max_body_bytes"`
	MaskHeaders       []string `json:"mask_headers"`
}

type extractorConfig struct {
	TaskIDPath     string `json:"task_id_path"`
	TaskStatusPath string `json:"task_status_path"`
}

type scenarioRequestConfig struct {
	Method           string            `json:"method"`
	Path             string            `json:"path"`
	Headers          map[string]string `json:"headers"`
	Body             string            `json:"body"`
	ExpectedStatuses []int             `json:"expected_statuses"`
}

type pollRequestConfig struct {
	Method           string            `json:"method"`
	PathTemplate     string            `json:"path_template"`
	Headers          map[string]string `json:"headers"`
	ExpectedStatuses []int             `json:"expected_statuses"`
}

type taskFlowConfig struct {
	SubmitRequest  scenarioRequestConfig `json:"submit_request"`
	PollRequest    pollRequestConfig     `json:"poll_request"`
	SuccessValues  []string              `json:"success_values"`
	FailureValues  []string              `json:"failure_values"`
	PollIntervalMS int                   `json:"poll_interval_ms"`
	MaxPolls       int                   `json:"max_polls"`
}

type scenarioConfig struct {
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
	Extractors       extractorConfig   `json:"extractors"`
	TaskFlow         taskFlowConfig    `json:"task_flow"`
}

type loadTestConfig struct {
	Version   int              `json:"version"`
	Worker    workerConfig     `json:"worker"`
	Target    targetConfig     `json:"target"`
	Run       runConfig        `json:"run"`
	Sampling  samplingConfig   `json:"sampling"`
	Scenarios []scenarioConfig `json:"scenarios"`
}

type configView struct {
	Target    targetConfig     `json:"target"`
	Run       runConfig        `json:"run"`
	Sampling  samplingConfig   `json:"sampling"`
	Scenarios []scenarioConfig `json:"scenarios"`
}

type configStore struct {
	path string
	mu   sync.RWMutex
	cfg  loadTestConfig
}

func defaultConfig() loadTestConfig {
	return loadTestConfig{
		Version: 1,
		Worker: workerConfig{
			Port:            18091,
			ManagementToken: uuid.NewString(),
		},
		Target: targetConfig{
			BaseURL: "http://127.0.0.1:3000",
			Headers: map[string]string{
				"Content-Type": "application/json",
			},
		},
		Run: runConfig{
			TotalConcurrency: 20,
			RampUpSec:        5,
			DurationSec:      30,
			MaxRequests:      0,
			RequestTimeoutMS: 30_000,
		},
		Sampling: samplingConfig{
			MaxRequestSamples: 100,
			MaxErrorSamples:   100,
			MaxBodyBytes:      4096,
			MaskHeaders:       []string{"authorization", "x-api-key", "x-mock-admin-token", "x-loadtest-admin-token"},
		},
		Scenarios: []scenarioConfig{
			{
				ID:      "chat-sse",
				Name:    "Chat Completions (SSE)",
				Enabled: true,
				Preset:  "new_api_chat",
				Mode:    "sse",
				Weight:  5,
			},
			{
				ID:      "responses-sse",
				Name:    "Responses (SSE)",
				Enabled: true,
				Preset:  "new_api_responses",
				Mode:    "sse",
				Weight:  3,
			},
			{
				ID:      "images-single",
				Name:    "Image Generation",
				Enabled: true,
				Preset:  "new_api_images",
				Mode:    "single",
				Weight:  1,
			},
			{
				ID:      "videos-task",
				Name:    "Video Task Flow",
				Enabled: true,
				Preset:  "new_api_videos",
				Mode:    "task_flow",
				Weight:  1,
			},
		},
	}
}

func loadConfigStore(path string) (*configStore, error) {
	cfg, err := loadConfigFile(path)
	if err != nil {
		return nil, err
	}
	return &configStore{path: path, cfg: cfg}, nil
}

func loadConfigFile(path string) (loadTestConfig, error) {
	cfg := defaultConfig()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return loadTestConfig{}, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			if err = validateConfig(&cfg); err != nil {
				return loadTestConfig{}, err
			}
			if err = saveConfigFile(path, cfg); err != nil {
				return loadTestConfig{}, err
			}
			return cfg, nil
		}
		return loadTestConfig{}, err
	}

	if len(data) > 0 {
		if err := common.Unmarshal(data, &cfg); err != nil {
			return loadTestConfig{}, err
		}
	}
	if err := validateConfig(&cfg); err != nil {
		return loadTestConfig{}, err
	}
	if err := saveConfigFile(path, cfg); err != nil {
		return loadTestConfig{}, err
	}
	return cfg, nil
}

func saveConfigFile(path string, cfg loadTestConfig) error {
	data, err := common.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func (s *configStore) get() loadTestConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return cloneConfig(s.cfg)
}

func (s *configStore) getView() configView {
	cfg := s.get()
	return configView{
		Target:    cloneTargetConfig(cfg.Target),
		Run:       cfg.Run,
		Sampling:  cloneSamplingConfig(cfg.Sampling),
		Scenarios: cloneScenarios(cfg.Scenarios),
	}
}

func (s *configStore) worker() workerConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cfg.Worker
}

func (s *configStore) updateView(view configView) (loadTestConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cfg := cloneConfig(s.cfg)
	cfg.Target = cloneTargetConfig(view.Target)
	cfg.Run = view.Run
	cfg.Sampling = cloneSamplingConfig(view.Sampling)
	cfg.Scenarios = cloneScenarios(view.Scenarios)

	if err := validateConfig(&cfg); err != nil {
		return loadTestConfig{}, err
	}
	if err := saveConfigFile(s.path, cfg); err != nil {
		return loadTestConfig{}, err
	}
	s.cfg = cfg
	return cloneConfig(cfg), nil
}

func (s *configStore) reloadFromDisk() (loadTestConfig, error) {
	cfg, err := loadConfigFile(s.path)
	if err != nil {
		return loadTestConfig{}, err
	}
	s.mu.Lock()
	s.cfg = cfg
	s.mu.Unlock()
	return cloneConfig(cfg), nil
}

func validateConfig(cfg *loadTestConfig) error {
	if cfg.Version == 0 {
		cfg.Version = 1
	}
	if cfg.Worker.ManagementToken == "" {
		cfg.Worker.ManagementToken = uuid.NewString()
	}
	if err := validatePort(cfg.Worker.Port, "worker.port"); err != nil {
		return err
	}

	cfg.Target.BaseURL = strings.TrimRight(strings.TrimSpace(cfg.Target.BaseURL), "/")
	if cfg.Target.BaseURL == "" {
		cfg.Target.BaseURL = defaultConfig().Target.BaseURL
	}
	parsed, err := url.Parse(cfg.Target.BaseURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Errorf("target.base_url must be a valid absolute URL")
	}
	cfg.Target.Headers = normalizeHeaders(cfg.Target.Headers)

	if cfg.Run.TotalConcurrency <= 0 {
		cfg.Run.TotalConcurrency = defaultConfig().Run.TotalConcurrency
	}
	if cfg.Run.TotalConcurrency > 100_000 {
		return fmt.Errorf("run.total_concurrency must be <= 100000")
	}
	if cfg.Run.RampUpSec < 0 || cfg.Run.RampUpSec > 3600 {
		return fmt.Errorf("run.ramp_up_sec must be between 0 and 3600")
	}
	if cfg.Run.DurationSec <= 0 || cfg.Run.DurationSec > 86_400 {
		return fmt.Errorf("run.duration_sec must be between 1 and 86400")
	}
	if cfg.Run.MaxRequests < 0 {
		return fmt.Errorf("run.max_requests must be >= 0")
	}
	if cfg.Run.RequestTimeoutMS < 100 || cfg.Run.RequestTimeoutMS > 600_000 {
		return fmt.Errorf("run.request_timeout_ms must be between 100 and 600000")
	}

	if cfg.Sampling.MaxRequestSamples <= 0 {
		cfg.Sampling.MaxRequestSamples = defaultConfig().Sampling.MaxRequestSamples
	}
	if cfg.Sampling.MaxErrorSamples <= 0 {
		cfg.Sampling.MaxErrorSamples = defaultConfig().Sampling.MaxErrorSamples
	}
	if cfg.Sampling.MaxBodyBytes < 256 {
		cfg.Sampling.MaxBodyBytes = defaultConfig().Sampling.MaxBodyBytes
	}
	cfg.Sampling.MaskHeaders = normalizeHeaderMasks(cfg.Sampling.MaskHeaders)

	if len(cfg.Scenarios) == 0 {
		cfg.Scenarios = defaultConfig().Scenarios
	}
	seen := make(map[string]struct{}, len(cfg.Scenarios))
	for i := range cfg.Scenarios {
		if err := validateScenario(&cfg.Scenarios[i]); err != nil {
			return fmt.Errorf("scenarios[%d]: %w", i, err)
		}
		if _, ok := seen[cfg.Scenarios[i].ID]; ok {
			return fmt.Errorf("scenario id %q must be unique", cfg.Scenarios[i].ID)
		}
		seen[cfg.Scenarios[i].ID] = struct{}{}
	}
	return nil
}

func validateScenario(s *scenarioConfig) error {
	applyScenarioPreset(s)

	s.ID = strings.TrimSpace(s.ID)
	s.Name = strings.TrimSpace(s.Name)
	s.Preset = strings.TrimSpace(s.Preset)
	s.Mode = strings.TrimSpace(strings.ToLower(s.Mode))
	s.Method = normalizeMethod(s.Method)
	s.Path = normalizePath(s.Path)
	s.Headers = normalizeHeaders(s.Headers)
	s.ExpectedStatuses = normalizeExpectedStatuses(s.ExpectedStatuses)

	if s.ID == "" {
		return fmt.Errorf("id is required")
	}
	if s.Name == "" {
		s.Name = s.ID
	}
	if s.Weight <= 0 {
		s.Weight = 1
	}
	if s.Weight > 10_000 {
		return fmt.Errorf("weight must be <= 10000")
	}
	switch s.Mode {
	case "single", "sse", "task_flow":
	default:
		return fmt.Errorf("mode must be one of single, sse, task_flow")
	}

	if s.Mode == "task_flow" {
		s.TaskFlow.SubmitRequest.Method = normalizeMethod(s.TaskFlow.SubmitRequest.Method)
		s.TaskFlow.SubmitRequest.Path = normalizePath(s.TaskFlow.SubmitRequest.Path)
		s.TaskFlow.SubmitRequest.Headers = normalizeHeaders(s.TaskFlow.SubmitRequest.Headers)
		s.TaskFlow.SubmitRequest.ExpectedStatuses = normalizeExpectedStatuses(s.TaskFlow.SubmitRequest.ExpectedStatuses)
		s.TaskFlow.PollRequest.Method = normalizeMethod(s.TaskFlow.PollRequest.Method)
		if s.TaskFlow.PollRequest.Method == "" {
			s.TaskFlow.PollRequest.Method = "GET"
		}
		s.TaskFlow.PollRequest.PathTemplate = normalizePathTemplate(s.TaskFlow.PollRequest.PathTemplate)
		s.TaskFlow.PollRequest.Headers = normalizeHeaders(s.TaskFlow.PollRequest.Headers)
		s.TaskFlow.PollRequest.ExpectedStatuses = normalizeExpectedStatuses(s.TaskFlow.PollRequest.ExpectedStatuses)
		s.Extractors.TaskIDPath = strings.TrimSpace(s.Extractors.TaskIDPath)
		s.Extractors.TaskStatusPath = strings.TrimSpace(s.Extractors.TaskStatusPath)
		if s.TaskFlow.PollIntervalMS <= 0 {
			s.TaskFlow.PollIntervalMS = 1000
		}
		if s.TaskFlow.MaxPolls <= 0 {
			s.TaskFlow.MaxPolls = 20
		}
		s.TaskFlow.SuccessValues = normalizeStringList(s.TaskFlow.SuccessValues)
		s.TaskFlow.FailureValues = normalizeStringList(s.TaskFlow.FailureValues)
		if s.TaskFlow.SubmitRequest.Method == "" || s.TaskFlow.SubmitRequest.Path == "" {
			return fmt.Errorf("task_flow.submit_request.method and path are required")
		}
		if s.TaskFlow.PollRequest.PathTemplate == "" {
			return fmt.Errorf("task_flow.poll_request.path_template is required")
		}
		if !strings.Contains(s.TaskFlow.PollRequest.PathTemplate, "{task_id}") {
			return fmt.Errorf("task_flow.poll_request.path_template must contain {task_id}")
		}
		if s.Extractors.TaskIDPath == "" || s.Extractors.TaskStatusPath == "" {
			return fmt.Errorf("extractors.task_id_path and extractors.task_status_path are required for task_flow")
		}
		return nil
	}

	if s.Method == "" {
		s.Method = "POST"
	}
	if s.Path == "" {
		return fmt.Errorf("path is required")
	}
	return nil
}

func applyScenarioPreset(s *scenarioConfig) {
	preset, ok := presetScenario(s.Preset)
	if !ok {
		return
	}
	if s.Mode == "" {
		s.Mode = preset.Mode
	}
	if s.Method == "" {
		s.Method = preset.Method
	}
	if s.Path == "" {
		s.Path = preset.Path
	}
	if len(s.Headers) == 0 {
		s.Headers = cloneStringMap(preset.Headers)
	}
	if s.Body == "" {
		s.Body = preset.Body
	}
	if len(s.ExpectedStatuses) == 0 {
		s.ExpectedStatuses = slices.Clone(preset.ExpectedStatuses)
	}
	if s.Extractors.TaskIDPath == "" {
		s.Extractors.TaskIDPath = preset.Extractors.TaskIDPath
	}
	if s.Extractors.TaskStatusPath == "" {
		s.Extractors.TaskStatusPath = preset.Extractors.TaskStatusPath
	}
	if s.TaskFlow.SubmitRequest.Method == "" {
		s.TaskFlow.SubmitRequest.Method = preset.TaskFlow.SubmitRequest.Method
	}
	if s.TaskFlow.SubmitRequest.Path == "" {
		s.TaskFlow.SubmitRequest.Path = preset.TaskFlow.SubmitRequest.Path
	}
	if len(s.TaskFlow.SubmitRequest.Headers) == 0 {
		s.TaskFlow.SubmitRequest.Headers = cloneStringMap(preset.TaskFlow.SubmitRequest.Headers)
	}
	if s.TaskFlow.SubmitRequest.Body == "" {
		s.TaskFlow.SubmitRequest.Body = preset.TaskFlow.SubmitRequest.Body
	}
	if len(s.TaskFlow.SubmitRequest.ExpectedStatuses) == 0 {
		s.TaskFlow.SubmitRequest.ExpectedStatuses = slices.Clone(preset.TaskFlow.SubmitRequest.ExpectedStatuses)
	}
	if s.TaskFlow.PollRequest.Method == "" {
		s.TaskFlow.PollRequest.Method = preset.TaskFlow.PollRequest.Method
	}
	if s.TaskFlow.PollRequest.PathTemplate == "" {
		s.TaskFlow.PollRequest.PathTemplate = preset.TaskFlow.PollRequest.PathTemplate
	}
	if len(s.TaskFlow.PollRequest.Headers) == 0 {
		s.TaskFlow.PollRequest.Headers = cloneStringMap(preset.TaskFlow.PollRequest.Headers)
	}
	if len(s.TaskFlow.PollRequest.ExpectedStatuses) == 0 {
		s.TaskFlow.PollRequest.ExpectedStatuses = slices.Clone(preset.TaskFlow.PollRequest.ExpectedStatuses)
	}
	if s.TaskFlow.PollIntervalMS == 0 {
		s.TaskFlow.PollIntervalMS = preset.TaskFlow.PollIntervalMS
	}
	if s.TaskFlow.MaxPolls == 0 {
		s.TaskFlow.MaxPolls = preset.TaskFlow.MaxPolls
	}
	if len(s.TaskFlow.SuccessValues) == 0 {
		s.TaskFlow.SuccessValues = slices.Clone(preset.TaskFlow.SuccessValues)
	}
	if len(s.TaskFlow.FailureValues) == 0 {
		s.TaskFlow.FailureValues = slices.Clone(preset.TaskFlow.FailureValues)
	}
}

func presetScenario(name string) (scenarioConfig, bool) {
	switch strings.TrimSpace(name) {
	case "", "raw_http":
		return scenarioConfig{}, false
	case "new_api_chat":
		return scenarioConfig{
			Mode:             "sse",
			Method:           "POST",
			Path:             "/v1/chat/completions",
			Body:             `{"model":"gpt-4o-mini","stream":true,"stream_options":{"include_usage":true},"messages":[{"role":"user","content":"hello from load tester"}]}`,
			ExpectedStatuses: []int{200},
		}, true
	case "new_api_responses":
		return scenarioConfig{
			Mode:             "sse",
			Method:           "POST",
			Path:             "/v1/responses",
			Body:             `{"model":"gpt-4o-mini","stream":true,"input":[{"role":"user","content":"hello from load tester"}]}`,
			ExpectedStatuses: []int{200},
		}, true
	case "new_api_images":
		return scenarioConfig{
			Mode:             "single",
			Method:           "POST",
			Path:             "/v1/images/generations",
			Body:             `{"model":"gpt-image-1","prompt":"a calm geometric poster","size":"512x512"}`,
			ExpectedStatuses: []int{200},
		}, true
	case "new_api_videos":
		return scenarioConfig{
			Mode: "task_flow",
			Extractors: extractorConfig{
				TaskIDPath:     "id",
				TaskStatusPath: "status",
			},
			TaskFlow: taskFlowConfig{
				SubmitRequest: scenarioRequestConfig{
					Method:           "POST",
					Path:             "/v1/videos",
					Body:             `{"model":"sora-mini","prompt":"waves over a lake","duration":2.5,"size":"640x360","fps":12}`,
					ExpectedStatuses: []int{200},
				},
				PollRequest: pollRequestConfig{
					Method:           "GET",
					PathTemplate:     "/v1/videos/{task_id}",
					ExpectedStatuses: []int{200},
				},
				SuccessValues:  []string{"completed", "succeeded", "success"},
				FailureValues:  []string{"failed", "error", "cancelled"},
				PollIntervalMS: 1000,
				MaxPolls:       20,
			},
		}, true
	default:
		return scenarioConfig{}, false
	}
}

func cloneConfig(cfg loadTestConfig) loadTestConfig {
	return loadTestConfig{
		Version:   cfg.Version,
		Worker:    cfg.Worker,
		Target:    cloneTargetConfig(cfg.Target),
		Run:       cfg.Run,
		Sampling:  cloneSamplingConfig(cfg.Sampling),
		Scenarios: cloneScenarios(cfg.Scenarios),
	}
}

func cloneTargetConfig(cfg targetConfig) targetConfig {
	return targetConfig{
		BaseURL:            cfg.BaseURL,
		Headers:            cloneStringMap(cfg.Headers),
		InsecureSkipVerify: cfg.InsecureSkipVerify,
	}
}

func cloneSamplingConfig(cfg samplingConfig) samplingConfig {
	return samplingConfig{
		MaxRequestSamples: cfg.MaxRequestSamples,
		MaxErrorSamples:   cfg.MaxErrorSamples,
		MaxBodyBytes:      cfg.MaxBodyBytes,
		MaskHeaders:       slices.Clone(cfg.MaskHeaders),
	}
}

func cloneScenarios(items []scenarioConfig) []scenarioConfig {
	out := make([]scenarioConfig, len(items))
	for i, item := range items {
		out[i] = scenarioConfig{
			ID:               item.ID,
			Name:             item.Name,
			Enabled:          item.Enabled,
			Preset:           item.Preset,
			Mode:             item.Mode,
			Weight:           item.Weight,
			Method:           item.Method,
			Path:             item.Path,
			Headers:          cloneStringMap(item.Headers),
			Body:             item.Body,
			ExpectedStatuses: slices.Clone(item.ExpectedStatuses),
			Extractors:       item.Extractors,
			TaskFlow: taskFlowConfig{
				SubmitRequest: scenarioRequestConfig{
					Method:           item.TaskFlow.SubmitRequest.Method,
					Path:             item.TaskFlow.SubmitRequest.Path,
					Headers:          cloneStringMap(item.TaskFlow.SubmitRequest.Headers),
					Body:             item.TaskFlow.SubmitRequest.Body,
					ExpectedStatuses: slices.Clone(item.TaskFlow.SubmitRequest.ExpectedStatuses),
				},
				PollRequest: pollRequestConfig{
					Method:           item.TaskFlow.PollRequest.Method,
					PathTemplate:     item.TaskFlow.PollRequest.PathTemplate,
					Headers:          cloneStringMap(item.TaskFlow.PollRequest.Headers),
					ExpectedStatuses: slices.Clone(item.TaskFlow.PollRequest.ExpectedStatuses),
				},
				SuccessValues:  slices.Clone(item.TaskFlow.SuccessValues),
				FailureValues:  slices.Clone(item.TaskFlow.FailureValues),
				PollIntervalMS: item.TaskFlow.PollIntervalMS,
				MaxPolls:       item.TaskFlow.MaxPolls,
			},
		}
	}
	return out
}

func cloneStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return map[string]string{}
	}
	return maps.Clone(in)
}

func normalizeHeaders(headers map[string]string) map[string]string {
	out := make(map[string]string, len(headers))
	for key, value := range headers {
		trimmedKey := strings.TrimSpace(key)
		if trimmedKey == "" {
			continue
		}
		out[trimmedKey] = strings.TrimSpace(value)
	}
	return out
}

func normalizeHeaderMasks(values []string) []string {
	if len(values) == 0 {
		return slices.Clone(defaultConfig().Sampling.MaskHeaders)
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.ToLower(strings.TrimSpace(value))
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	if len(out) == 0 {
		return slices.Clone(defaultConfig().Sampling.MaskHeaders)
	}
	return out
}

func normalizeExpectedStatuses(values []int) []int {
	if len(values) == 0 {
		return []int{200}
	}
	out := make([]int, 0, len(values))
	seen := make(map[int]struct{}, len(values))
	for _, value := range values {
		if value < 100 || value > 599 {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	if len(out) == 0 {
		return []int{200}
	}
	return out
}

func normalizeStringList(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.ToLower(strings.TrimSpace(value))
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}

func normalizeMethod(value string) string {
	value = strings.ToUpper(strings.TrimSpace(value))
	switch value {
	case "":
		return ""
	case "GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS":
		return value
	default:
		return value
	}
}

func normalizePath(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://") {
		return value
	}
	if !strings.HasPrefix(value, "/") {
		return "/" + value
	}
	return value
}

func normalizePathTemplate(value string) string {
	return normalizePath(value)
}

func validatePort(port int, field string) error {
	if port <= 0 || port > 65535 {
		return fmt.Errorf("%s must be between 1 and 65535", field)
	}
	return nil
}
