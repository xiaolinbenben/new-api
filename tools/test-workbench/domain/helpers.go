package domain

import (
	"fmt"
	"maps"
	"net/url"
	"slices"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

const (
	DefaultAdminHost = "127.0.0.1"
	DefaultAdminPort = 18880
	DefaultMockHost  = "0.0.0.0"
	DefaultMockPort  = 18881
)

func DefaultEnvironment() Environment {
	return Environment{
		Name:               "默认环境",
		TargetType:         TargetTypeInternalMock,
		DefaultHeaders:     map[string]string{"Content-Type": "application/json"},
		InsecureSkipVerify: false,
		MockBindHost:       DefaultMockHost,
		MockPort:           DefaultMockPort,
		MockRequireAuth:    false,
		MockAuthToken:      "workbench-token",
		AutoStart:          true,
	}
}

func DefaultMockProfileConfig() MockProfileConfig {
	return MockProfileConfig{
		Models:      []string{"gpt-4o-mini", "gpt-4o", "gpt-image-1", "sora-mini"},
		EnablePprof: false,
		PprofPort:   18885,
		Random: GlobalRandomConfig{
			Mode:                "true_random",
			Seed:                20260411,
			LatencyMS:           IntRange{Min: 20, Max: 150},
			ErrorRate:           0.02,
			TooManyRequestsRate: 0.01,
			ServerErrorRate:     0.01,
			TimeoutRate:         0.005,
			DefaultTokenLength:  IntRange{Min: 48, Max: 256},
			DefaultStreamChunks: IntRange{Min: 2, Max: 8},
		},
		Chat: TextBehaviorConfig{
			TextTokens:              IntRange{Min: 48, Max: 256},
			AllowStream:             true,
			UsageProbability:        0.95,
			ToolCallProbability:     0.35,
			ToolCallCount:           IntRange{Min: 1, Max: 3},
			ToolArgumentsBytes:      IntRange{Min: 64, Max: 384},
			MCPToolProbability:      0.15,
			FinalAnswerProbability:  0.85,
			SystemFingerprintChance: 0.75,
		},
		Responses: TextBehaviorConfig{
			TextTokens:              IntRange{Min: 48, Max: 256},
			AllowStream:             true,
			UsageProbability:        0.95,
			ToolCallProbability:     0.40,
			ToolCallCount:           IntRange{Min: 1, Max: 3},
			ToolArgumentsBytes:      IntRange{Min: 64, Max: 512},
			MCPToolProbability:      0.20,
			FinalAnswerProbability:  0.80,
			SystemFingerprintChance: 0.60,
		},
		Images: ImageBehaviorConfig{
			Sizes:             []string{"512x512", "768x768", "1024x1024", "1024x1536"},
			ResponseURLRate:   0.70,
			ImageCount:        IntRange{Min: 1, Max: 4},
			ImageBytes:        IntRange{Min: 12_000, Max: 120_000},
			WatermarkTexts:    []string{"mock", "new-api", "simulated", "fake upstream"},
			WatermarkRate:     0.85,
			BackgroundPalette: []string{"#0F172A", "#155E75", "#4A044E", "#7C2D12", "#1D4ED8"},
		},
		Videos: VideoBehaviorConfig{
			DurationsSeconds: FloatRange{Min: 3, Max: 12},
			Resolutions:      []string{"640x360", "960x540", "1280x720"},
			FPS:              IntRange{Min: 12, Max: 30},
			PollIntervalMS:   IntRange{Min: 400, Max: 2200},
			FailureRate:      0.15,
			VideoBytes:       IntRange{Min: 90_000, Max: 350_000},
			ProgressJitter:   IntRange{Min: 1, Max: 12},
		},
	}
}

func DefaultRunProfileConfig() RunProfileConfig {
	return RunProfileConfig{
		TotalConcurrency: 20,
		RampUpSec:        5,
		DurationSec:      30,
		MaxRequests:      0,
		RequestTimeoutMS: 30_000,
		Sampling: SamplingConfig{
			MaxRequestSamples: 100,
			MaxErrorSamples:   100,
			MaxBodyBytes:      4096,
			MaskHeaders: []string{
				"authorization",
				"x-api-key",
				"api-key",
				"x-mock-admin-token",
				"x-loadtest-admin-token",
			},
		},
	}
}

func DefaultScenarioConfigs() []ScenarioConfig {
	return []ScenarioConfig{
		{ID: "chat-sse", Name: "聊天补全（SSE）", Enabled: true, Preset: "new_api_chat", Mode: "sse", Weight: 5},
		{ID: "responses-sse", Name: "Responses 流式输出", Enabled: true, Preset: "new_api_responses", Mode: "sse", Weight: 3},
		{ID: "images-single", Name: "图片生成", Enabled: true, Preset: "new_api_images", Mode: "single", Weight: 1},
		{ID: "videos-task", Name: "视频任务流", Enabled: true, Preset: "new_api_videos", Mode: "task_flow", Weight: 1},
	}
}

func ValidateEnvironment(env *Environment) error {
	defaults := DefaultEnvironment()
	env.Name = strings.TrimSpace(env.Name)
	if env.Name == "" {
		env.Name = defaults.Name
	}
	env.TargetType = strings.TrimSpace(env.TargetType)
	if env.TargetType == "" {
		env.TargetType = defaults.TargetType
	}
	if env.TargetType != TargetTypeInternalMock && env.TargetType != TargetTypeExternalHTTP {
		return fmt.Errorf("target_type must be %s or %s", TargetTypeInternalMock, TargetTypeExternalHTTP)
	}
	env.ExternalBaseURL = strings.TrimRight(strings.TrimSpace(env.ExternalBaseURL), "/")
	env.DefaultHeaders = NormalizeHeaders(env.DefaultHeaders)
	if len(env.DefaultHeaders) == 0 {
		env.DefaultHeaders = maps.Clone(defaults.DefaultHeaders)
	}
	if env.TargetType == TargetTypeExternalHTTP {
		parsed, err := url.Parse(env.ExternalBaseURL)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" {
			return fmt.Errorf("external_base_url must be a valid absolute URL")
		}
	}
	if env.MockBindHost == "" {
		env.MockBindHost = defaults.MockBindHost
	}
	if err := validatePort(defaultPort(env.MockPort, defaults.MockPort), "mock_port"); err != nil {
		return err
	}
	env.MockPort = defaultPort(env.MockPort, defaults.MockPort)
	if strings.TrimSpace(env.MockAuthToken) == "" {
		env.MockAuthToken = defaults.MockAuthToken
	}
	return nil
}

func ValidateMockProfileConfig(cfg *MockProfileConfig) error {
	defaults := DefaultMockProfileConfig()
	if cfg.PprofPort == 0 {
		cfg.PprofPort = defaults.PprofPort
	}
	if err := validatePort(cfg.PprofPort, "pprof_port"); err != nil {
		return err
	}
	if len(cfg.Models) == 0 {
		cfg.Models = slices.Clone(defaults.Models)
	}
	cfg.Models = CompactStrings(cfg.Models)
	if err := validateIntRange(cfg.Random.LatencyMS, 0, 120_000, "random.latency_ms"); err != nil {
		return err
	}
	if err := validateIntRange(cfg.Random.DefaultTokenLength, 1, 100_000, "random.default_token_length"); err != nil {
		return err
	}
	if err := validateIntRange(cfg.Random.DefaultStreamChunks, 1, 1024, "random.default_stream_chunks"); err != nil {
		return err
	}
	if cfg.Random.Mode == "" {
		cfg.Random.Mode = defaults.Random.Mode
	}
	if cfg.Random.Mode != "true_random" && cfg.Random.Mode != "seeded" {
		return fmt.Errorf("random.mode must be true_random or seeded")
	}
	for _, item := range []struct {
		value float64
		name  string
	}{
		{cfg.Random.ErrorRate, "random.error_rate"},
		{cfg.Random.TooManyRequestsRate, "random.too_many_requests_rate"},
		{cfg.Random.ServerErrorRate, "random.server_error_rate"},
		{cfg.Random.TimeoutRate, "random.timeout_rate"},
		{cfg.Images.ResponseURLRate, "images.response_url_rate"},
		{cfg.Images.WatermarkRate, "images.watermark_rate"},
		{cfg.Videos.FailureRate, "videos.failure_rate"},
		{cfg.Chat.UsageProbability, "chat.usage_probability"},
		{cfg.Chat.ToolCallProbability, "chat.tool_call_probability"},
		{cfg.Chat.MCPToolProbability, "chat.mcp_tool_probability"},
		{cfg.Chat.FinalAnswerProbability, "chat.final_answer_probability"},
		{cfg.Chat.SystemFingerprintChance, "chat.system_fingerprint_probability"},
		{cfg.Responses.UsageProbability, "responses.usage_probability"},
		{cfg.Responses.ToolCallProbability, "responses.tool_call_probability"},
		{cfg.Responses.MCPToolProbability, "responses.mcp_tool_probability"},
		{cfg.Responses.FinalAnswerProbability, "responses.final_answer_probability"},
		{cfg.Responses.SystemFingerprintChance, "responses.system_fingerprint_probability"},
	} {
		if err := validateProbability(item.value, item.name); err != nil {
			return err
		}
	}
	if err := validateTextBehavior(&cfg.Chat, defaults.Random.DefaultTokenLength, "chat"); err != nil {
		return err
	}
	if err := validateTextBehavior(&cfg.Responses, defaults.Random.DefaultTokenLength, "responses"); err != nil {
		return err
	}
	if err := validateIntRange(cfg.Images.ImageCount, 1, 16, "images.image_count"); err != nil {
		return err
	}
	if err := validateIntRange(cfg.Images.ImageBytes, 1024, 10_000_000, "images.image_bytes"); err != nil {
		return err
	}
	cfg.Images.Sizes = CompactStrings(defaultSlice(cfg.Images.Sizes, defaults.Images.Sizes))
	cfg.Images.WatermarkTexts = CompactStrings(defaultSlice(cfg.Images.WatermarkTexts, defaults.Images.WatermarkTexts))
	cfg.Images.BackgroundPalette = CompactStrings(defaultSlice(cfg.Images.BackgroundPalette, defaults.Images.BackgroundPalette))
	if err := validateFloatRange(cfg.Videos.DurationsSeconds, 0.5, 600, "videos.durations_seconds"); err != nil {
		return err
	}
	if err := validateIntRange(cfg.Videos.FPS, 1, 120, "videos.fps"); err != nil {
		return err
	}
	if err := validateIntRange(cfg.Videos.PollIntervalMS, 100, 60_000, "videos.poll_interval_ms"); err != nil {
		return err
	}
	if err := validateIntRange(cfg.Videos.VideoBytes, 1024, 50_000_000, "videos.video_bytes"); err != nil {
		return err
	}
	if err := validateIntRange(cfg.Videos.ProgressJitter, 0, 100, "videos.progress_jitter"); err != nil {
		return err
	}
	cfg.Videos.Resolutions = CompactStrings(defaultSlice(cfg.Videos.Resolutions, defaults.Videos.Resolutions))
	return nil
}

func ValidateRunProfileConfig(cfg *RunProfileConfig) error {
	defaults := DefaultRunProfileConfig()
	if cfg.TotalConcurrency <= 0 {
		cfg.TotalConcurrency = defaults.TotalConcurrency
	}
	if cfg.TotalConcurrency > 100_000 {
		return fmt.Errorf("total_concurrency must be <= 100000")
	}
	if cfg.RampUpSec < 0 || cfg.RampUpSec > 3600 {
		return fmt.Errorf("ramp_up_sec must be between 0 and 3600")
	}
	if cfg.DurationSec <= 0 || cfg.DurationSec > 86_400 {
		return fmt.Errorf("duration_sec must be between 1 and 86400")
	}
	if cfg.MaxRequests < 0 {
		return fmt.Errorf("max_requests must be >= 0")
	}
	if cfg.RequestTimeoutMS < 100 || cfg.RequestTimeoutMS > 600_000 {
		return fmt.Errorf("request_timeout_ms must be between 100 and 600000")
	}
	cfg.Sampling.MaskHeaders = NormalizeHeaderMasks(cfg.Sampling.MaskHeaders)
	if cfg.Sampling.MaxRequestSamples <= 0 {
		cfg.Sampling.MaxRequestSamples = defaults.Sampling.MaxRequestSamples
	}
	if cfg.Sampling.MaxErrorSamples <= 0 {
		cfg.Sampling.MaxErrorSamples = defaults.Sampling.MaxErrorSamples
	}
	if cfg.Sampling.MaxBodyBytes < 256 {
		cfg.Sampling.MaxBodyBytes = defaults.Sampling.MaxBodyBytes
	}
	return nil
}

func ValidateScenarioConfig(s *ScenarioConfig) error {
	ApplyScenarioPreset(s)
	s.ID = strings.TrimSpace(s.ID)
	s.Name = strings.TrimSpace(s.Name)
	s.Preset = strings.TrimSpace(s.Preset)
	s.Mode = strings.TrimSpace(strings.ToLower(s.Mode))
	s.Method = NormalizeMethod(s.Method)
	s.Path = NormalizePath(s.Path)
	s.Headers = NormalizeHeaders(s.Headers)
	s.ExpectedStatuses = NormalizeExpectedStatuses(s.ExpectedStatuses)
	if s.ID == "" {
		return fmt.Errorf("scenario id is required")
	}
	if s.Name == "" {
		s.Name = s.ID
	}
	if s.Weight <= 0 {
		s.Weight = 1
	}
	if s.Weight > 10_000 {
		return fmt.Errorf("scenario weight must be <= 10000")
	}
	switch s.Mode {
	case "single", "sse", "task_flow":
	default:
		return fmt.Errorf("scenario mode must be one of single, sse, task_flow")
	}
	if s.Mode != "task_flow" {
		if s.Method == "" {
			s.Method = "POST"
		}
		if s.Path == "" {
			return fmt.Errorf("scenario path is required")
		}
		return nil
	}
	s.TaskFlow.SubmitRequest.Method = NormalizeMethod(s.TaskFlow.SubmitRequest.Method)
	s.TaskFlow.SubmitRequest.Path = NormalizePath(s.TaskFlow.SubmitRequest.Path)
	s.TaskFlow.SubmitRequest.Headers = NormalizeHeaders(s.TaskFlow.SubmitRequest.Headers)
	s.TaskFlow.SubmitRequest.ExpectedStatuses = NormalizeExpectedStatuses(s.TaskFlow.SubmitRequest.ExpectedStatuses)
	s.TaskFlow.PollRequest.Method = NormalizeMethod(s.TaskFlow.PollRequest.Method)
	if s.TaskFlow.PollRequest.Method == "" {
		s.TaskFlow.PollRequest.Method = "GET"
	}
	s.TaskFlow.PollRequest.PathTemplate = NormalizePath(s.TaskFlow.PollRequest.PathTemplate)
	s.TaskFlow.PollRequest.Headers = NormalizeHeaders(s.TaskFlow.PollRequest.Headers)
	s.TaskFlow.PollRequest.ExpectedStatuses = NormalizeExpectedStatuses(s.TaskFlow.PollRequest.ExpectedStatuses)
	s.TaskFlow.SuccessValues = NormalizeStringList(s.TaskFlow.SuccessValues)
	s.TaskFlow.FailureValues = NormalizeStringList(s.TaskFlow.FailureValues)
	if s.TaskFlow.PollIntervalMS <= 0 {
		s.TaskFlow.PollIntervalMS = 1000
	}
	if s.TaskFlow.MaxPolls <= 0 {
		s.TaskFlow.MaxPolls = 20
	}
	if s.TaskFlow.SubmitRequest.Method == "" || s.TaskFlow.SubmitRequest.Path == "" {
		return fmt.Errorf("task_flow.submit_request.method and path are required")
	}
	if s.TaskFlow.PollRequest.PathTemplate == "" || !strings.Contains(s.TaskFlow.PollRequest.PathTemplate, "{task_id}") {
		return fmt.Errorf("task_flow.poll_request.path_template must contain {task_id}")
	}
	if strings.TrimSpace(s.Extractors.TaskIDPath) == "" || strings.TrimSpace(s.Extractors.TaskStatusPath) == "" {
		return fmt.Errorf("task_flow extractors are required")
	}
	return nil
}

func ApplyScenarioPreset(s *ScenarioConfig) {
	preset, ok := presetScenario(strings.TrimSpace(s.Preset))
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
		s.Headers = CloneStringMap(preset.Headers)
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
		s.TaskFlow.SubmitRequest.Headers = CloneStringMap(preset.TaskFlow.SubmitRequest.Headers)
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
		s.TaskFlow.PollRequest.Headers = CloneStringMap(preset.TaskFlow.PollRequest.Headers)
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

func CloneRunExecutionConfig(cfg RunExecutionConfig) RunExecutionConfig {
	return RunExecutionConfig{
		Target: RuntimeLoadTarget{
			BaseURL:            cfg.Target.BaseURL,
			Headers:            CloneStringMap(cfg.Target.Headers),
			InsecureSkipVerify: cfg.Target.InsecureSkipVerify,
		},
		Run: RunProfileConfig{
			TotalConcurrency: cfg.Run.TotalConcurrency,
			RampUpSec:        cfg.Run.RampUpSec,
			DurationSec:      cfg.Run.DurationSec,
			MaxRequests:      cfg.Run.MaxRequests,
			RequestTimeoutMS: cfg.Run.RequestTimeoutMS,
			Sampling: SamplingConfig{
				MaxRequestSamples: cfg.Run.Sampling.MaxRequestSamples,
				MaxErrorSamples:   cfg.Run.Sampling.MaxErrorSamples,
				MaxBodyBytes:      cfg.Run.Sampling.MaxBodyBytes,
				MaskHeaders:       slices.Clone(cfg.Run.Sampling.MaskHeaders),
			},
		},
		Scenarios: CloneScenarioConfigs(cfg.Scenarios),
	}
}

func CloneScenarioConfigs(items []ScenarioConfig) []ScenarioConfig {
	out := make([]ScenarioConfig, len(items))
	for i, item := range items {
		out[i] = ScenarioConfig{
			ID:               item.ID,
			Name:             item.Name,
			Enabled:          item.Enabled,
			Preset:           item.Preset,
			Mode:             item.Mode,
			Weight:           item.Weight,
			Method:           item.Method,
			Path:             item.Path,
			Headers:          CloneStringMap(item.Headers),
			Body:             item.Body,
			ExpectedStatuses: slices.Clone(item.ExpectedStatuses),
			Extractors:       item.Extractors,
			TaskFlow: TaskFlowConfig{
				SubmitRequest: ScenarioRequestConfig{
					Method:           item.TaskFlow.SubmitRequest.Method,
					Path:             item.TaskFlow.SubmitRequest.Path,
					Headers:          CloneStringMap(item.TaskFlow.SubmitRequest.Headers),
					Body:             item.TaskFlow.SubmitRequest.Body,
					ExpectedStatuses: slices.Clone(item.TaskFlow.SubmitRequest.ExpectedStatuses),
				},
				PollRequest: PollRequestConfig{
					Method:           item.TaskFlow.PollRequest.Method,
					PathTemplate:     item.TaskFlow.PollRequest.PathTemplate,
					Headers:          CloneStringMap(item.TaskFlow.PollRequest.Headers),
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

func CloneStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return map[string]string{}
	}
	return maps.Clone(in)
}

func NormalizeHeaders(headers map[string]string) map[string]string {
	out := make(map[string]string, len(headers))
	for key, value := range headers {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		out[key] = strings.TrimSpace(value)
	}
	return out
}

func NormalizeHeaderMasks(values []string) []string {
	defaults := DefaultRunProfileConfig().Sampling.MaskHeaders
	if len(values) == 0 {
		return slices.Clone(defaults)
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
		return slices.Clone(defaults)
	}
	return out
}

func NormalizeExpectedStatuses(values []int) []int {
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

func NormalizeStringList(values []string) []string {
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

func NormalizeMethod(value string) string {
	return strings.ToUpper(strings.TrimSpace(value))
}

func NormalizePath(value string) string {
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

func CompactStrings(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}

func MarshalToString(value any) (string, error) {
	data, err := common.Marshal(value)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func UnmarshalString(data string, target any) error {
	if strings.TrimSpace(data) == "" {
		return nil
	}
	return common.UnmarshalJsonStr(data, target)
}

func defaultPort(value int, fallback int) int {
	if value > 0 {
		return value
	}
	return fallback
}

func defaultSlice(values []string, fallback []string) []string {
	if len(values) > 0 {
		return values
	}
	return fallback
}

func validateTextBehavior(cfg *TextBehaviorConfig, fallback IntRange, prefix string) error {
	if err := validateIntRange(cfg.TextTokens, fallback.Min, 100_000, prefix+".text_tokens"); err != nil {
		return err
	}
	if err := validateIntRange(cfg.ToolCallCount, 1, 32, prefix+".tool_call_count"); err != nil {
		return err
	}
	if err := validateIntRange(cfg.ToolArgumentsBytes, 8, 100_000, prefix+".tool_arguments_bytes"); err != nil {
		return err
	}
	return nil
}

func validatePort(port int, field string) error {
	if port < 1 || port > 65535 {
		return fmt.Errorf("%s must be between 1 and 65535", field)
	}
	return nil
}

func validateIntRange(value IntRange, minAllowed int, maxAllowed int, name string) error {
	if value.Min < minAllowed || value.Max > maxAllowed || value.Min > value.Max {
		return fmt.Errorf("%s must satisfy %d <= min <= max <= %d", name, minAllowed, maxAllowed)
	}
	return nil
}

func validateFloatRange(value FloatRange, minAllowed float64, maxAllowed float64, name string) error {
	if value.Min < minAllowed || value.Max > maxAllowed || value.Min > value.Max {
		return fmt.Errorf("%s must satisfy %.2f <= min <= max <= %.2f", name, minAllowed, maxAllowed)
	}
	return nil
}

func validateProbability(value float64, name string) error {
	if value < 0 || value > 1 {
		return fmt.Errorf("%s must be between 0 and 1", name)
	}
	return nil
}

func presetScenario(name string) (ScenarioConfig, bool) {
	switch strings.TrimSpace(name) {
	case "", "raw_http":
		return ScenarioConfig{}, false
	case "new_api_chat":
		return ScenarioConfig{
			Mode:             "sse",
			Method:           "POST",
			Path:             "/v1/chat/completions",
			Body:             `{"model":"gpt-4o-mini","stream":true,"stream_options":{"include_usage":true},"messages":[{"role":"user","content":"hello from load tester"}]}`,
			ExpectedStatuses: []int{200},
		}, true
	case "new_api_responses":
		return ScenarioConfig{
			Mode:             "sse",
			Method:           "POST",
			Path:             "/v1/responses",
			Body:             `{"model":"gpt-4o-mini","stream":true,"input":[{"role":"user","content":"hello from load tester"}]}`,
			ExpectedStatuses: []int{200},
		}, true
	case "new_api_images":
		return ScenarioConfig{
			Mode:             "single",
			Method:           "POST",
			Path:             "/v1/images/generations",
			Body:             `{"model":"gpt-image-1","prompt":"a calm geometric poster","size":"512x512"}`,
			ExpectedStatuses: []int{200},
		}, true
	case "new_api_videos":
		return ScenarioConfig{
			Mode: "task_flow",
			Extractors: ExtractorConfig{
				TaskIDPath:     "id",
				TaskStatusPath: "status",
			},
			TaskFlow: TaskFlowConfig{
				SubmitRequest: ScenarioRequestConfig{
					Method:           "POST",
					Path:             "/v1/videos",
					Body:             `{"model":"sora-mini","prompt":"waves over a lake","duration":2.5,"size":"640x360","fps":12}`,
					ExpectedStatuses: []int{200},
				},
				PollRequest: PollRequestConfig{
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
		return ScenarioConfig{}, false
	}
}
