package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"
	"github.com/google/uuid"
)

type intRange struct {
	Min int `json:"min"`
	Max int `json:"max"`
}

type floatRange struct {
	Min float64 `json:"min"`
	Max float64 `json:"max"`
}

type workerConfig struct {
	Port            int      `json:"port"`
	RequireAuth     bool     `json:"require_auth"`
	EnablePprof     bool     `json:"enable_pprof"`
	PprofPort       int      `json:"pprof_port"`
	Models          []string `json:"models"`
	ManagementToken string   `json:"management_token"`
}

type globalRandomConfig struct {
	Mode                string   `json:"mode"`
	Seed                int64    `json:"seed"`
	LatencyMs           intRange `json:"latency_ms"`
	ErrorRate           float64  `json:"error_rate"`
	TooManyRequestsRate float64  `json:"too_many_requests_rate"`
	ServerErrorRate     float64  `json:"server_error_rate"`
	TimeoutRate         float64  `json:"timeout_rate"`
	DefaultTokenLength  intRange `json:"default_token_length"`
	DefaultStreamChunks intRange `json:"default_stream_chunks"`
}

type textBehaviorConfig struct {
	TextTokens              intRange `json:"text_tokens"`
	AllowStream             bool     `json:"allow_stream"`
	UsageProbability        float64  `json:"usage_probability"`
	ToolCallProbability     float64  `json:"tool_call_probability"`
	ToolCallCount           intRange `json:"tool_call_count"`
	ToolArgumentsBytes      intRange `json:"tool_arguments_bytes"`
	MCPToolProbability      float64  `json:"mcp_tool_probability"`
	FinalAnswerProbability  float64  `json:"final_answer_probability"`
	SystemFingerprintChance float64  `json:"system_fingerprint_probability"`
}

type imageBehaviorConfig struct {
	Sizes             []string `json:"sizes"`
	ResponseURLRate   float64  `json:"response_url_rate"`
	ImageCount        intRange `json:"image_count"`
	ImageBytes        intRange `json:"image_bytes"`
	WatermarkTexts    []string `json:"watermark_texts"`
	WatermarkRate     float64  `json:"watermark_rate"`
	BackgroundPalette []string `json:"background_palette"`
}

type videoBehaviorConfig struct {
	DurationsSeconds floatRange `json:"durations_seconds"`
	Resolutions      []string   `json:"resolutions"`
	FPS              intRange   `json:"fps"`
	PollIntervalMs   intRange   `json:"poll_interval_ms"`
	FailureRate      float64    `json:"failure_rate"`
	VideoBytes       intRange   `json:"video_bytes"`
	ProgressJitter   intRange   `json:"progress_jitter"`
}

type mockConfig struct {
	Version   int                 `json:"version"`
	Worker    workerConfig        `json:"worker"`
	Random    globalRandomConfig  `json:"random"`
	Chat      textBehaviorConfig  `json:"chat"`
	Responses textBehaviorConfig  `json:"responses"`
	Images    imageBehaviorConfig `json:"images"`
	Videos    videoBehaviorConfig `json:"videos"`
}

type configView struct {
	Worker    workerConfig        `json:"worker"`
	Random    globalRandomConfig  `json:"random"`
	Chat      textBehaviorConfig  `json:"chat"`
	Responses textBehaviorConfig  `json:"responses"`
	Images    imageBehaviorConfig `json:"images"`
	Videos    videoBehaviorConfig `json:"videos"`
}

type configStore struct {
	path string
	mu   sync.RWMutex
	cfg  mockConfig
}

func defaultConfig() mockConfig {
	return mockConfig{
		Version: 1,
		Worker: workerConfig{
			Port:            18081,
			RequireAuth:     false,
			EnablePprof:     false,
			PprofPort:       18085,
			Models:          []string{"gpt-4o-mini", "gpt-4o", "gpt-image-1", "sora-mini"},
			ManagementToken: uuid.NewString(),
		},
		Random: globalRandomConfig{
			Mode:                "true_random",
			Seed:                20260411,
			LatencyMs:           intRange{Min: 20, Max: 150},
			ErrorRate:           0.02,
			TooManyRequestsRate: 0.01,
			ServerErrorRate:     0.01,
			TimeoutRate:         0.005,
			DefaultTokenLength:  intRange{Min: 48, Max: 256},
			DefaultStreamChunks: intRange{Min: 2, Max: 8},
		},
		Chat: textBehaviorConfig{
			TextTokens:              intRange{Min: 48, Max: 256},
			AllowStream:             true,
			UsageProbability:        0.95,
			ToolCallProbability:     0.35,
			ToolCallCount:           intRange{Min: 1, Max: 3},
			ToolArgumentsBytes:      intRange{Min: 64, Max: 384},
			MCPToolProbability:      0.15,
			FinalAnswerProbability:  0.85,
			SystemFingerprintChance: 0.75,
		},
		Responses: textBehaviorConfig{
			TextTokens:              intRange{Min: 48, Max: 256},
			AllowStream:             true,
			UsageProbability:        0.95,
			ToolCallProbability:     0.40,
			ToolCallCount:           intRange{Min: 1, Max: 3},
			ToolArgumentsBytes:      intRange{Min: 64, Max: 512},
			MCPToolProbability:      0.20,
			FinalAnswerProbability:  0.80,
			SystemFingerprintChance: 0.60,
		},
		Images: imageBehaviorConfig{
			Sizes:             []string{"512x512", "768x768", "1024x1024", "1024x1536"},
			ResponseURLRate:   0.70,
			ImageCount:        intRange{Min: 1, Max: 4},
			ImageBytes:        intRange{Min: 12_000, Max: 120_000},
			WatermarkTexts:    []string{"mock", "new-api", "simulated", "fake upstream"},
			WatermarkRate:     0.85,
			BackgroundPalette: []string{"#0F172A", "#155E75", "#4A044E", "#7C2D12", "#1D4ED8"},
		},
		Videos: videoBehaviorConfig{
			DurationsSeconds: floatRange{Min: 3, Max: 12},
			Resolutions:      []string{"640x360", "960x540", "1280x720"},
			FPS:              intRange{Min: 12, Max: 30},
			PollIntervalMs:   intRange{Min: 400, Max: 2200},
			FailureRate:      0.15,
			VideoBytes:       intRange{Min: 90_000, Max: 350_000},
			ProgressJitter:   intRange{Min: 1, Max: 12},
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

func loadConfigFile(path string) (mockConfig, error) {
	cfg := defaultConfig()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return mockConfig{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			if err = validateConfig(&cfg); err != nil {
				return mockConfig{}, err
			}
			if err = saveConfigFile(path, cfg); err != nil {
				return mockConfig{}, err
			}
			return cfg, nil
		}
		return mockConfig{}, err
	}
	if len(data) > 0 {
		if err := common.Unmarshal(data, &cfg); err != nil {
			return mockConfig{}, err
		}
	}
	if err := validateConfig(&cfg); err != nil {
		return mockConfig{}, err
	}
	if err := saveConfigFile(path, cfg); err != nil {
		return mockConfig{}, err
	}
	return cfg, nil
}

func saveConfigFile(path string, cfg mockConfig) error {
	data, err := common.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func (s *configStore) get() mockConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cfg
}

func (s *configStore) getView() configView {
	cfg := s.get()
	return configView{
		Worker: workerConfig{
			Port:        cfg.Worker.Port,
			RequireAuth: cfg.Worker.RequireAuth,
			EnablePprof: cfg.Worker.EnablePprof,
			PprofPort:   cfg.Worker.PprofPort,
			Models:      append([]string(nil), cfg.Worker.Models...),
		},
		Random:    cfg.Random,
		Chat:      cfg.Chat,
		Responses: cfg.Responses,
		Images:    cfg.Images,
		Videos:    cfg.Videos,
	}
}

func (s *configStore) managementToken() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cfg.Worker.ManagementToken
}

func (s *configStore) updateView(view configView) (mockConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cfg := s.cfg
	cfg.Worker.Port = view.Worker.Port
	cfg.Worker.RequireAuth = view.Worker.RequireAuth
	cfg.Worker.EnablePprof = view.Worker.EnablePprof
	cfg.Worker.PprofPort = view.Worker.PprofPort
	cfg.Worker.Models = append([]string(nil), view.Worker.Models...)
	cfg.Random = view.Random
	cfg.Chat = view.Chat
	cfg.Responses = view.Responses
	cfg.Images = view.Images
	cfg.Videos = view.Videos

	if err := validateConfig(&cfg); err != nil {
		return mockConfig{}, err
	}
	if err := saveConfigFile(s.path, cfg); err != nil {
		return mockConfig{}, err
	}
	s.cfg = cfg
	return cfg, nil
}

func validateConfig(cfg *mockConfig) error {
	if cfg.Worker.ManagementToken == "" {
		cfg.Worker.ManagementToken = uuid.NewString()
	}
	if cfg.Version == 0 {
		cfg.Version = 1
	}
	if err := validatePort(cfg.Worker.Port, "worker.port"); err != nil {
		return err
	}
	if err := validatePort(cfg.Worker.PprofPort, "worker.pprof_port"); err != nil {
		return err
	}
	if err := validateIntRange(cfg.Random.LatencyMs, 0, 120_000, "random.latency_ms"); err != nil {
		return err
	}
	if err := validateIntRange(cfg.Random.DefaultTokenLength, 1, 100_000, "random.default_token_length"); err != nil {
		return err
	}
	if err := validateIntRange(cfg.Random.DefaultStreamChunks, 1, 1024, "random.default_stream_chunks"); err != nil {
		return err
	}
	if err := validateProbability(cfg.Random.ErrorRate, "random.error_rate"); err != nil {
		return err
	}
	if err := validateProbability(cfg.Random.TooManyRequestsRate, "random.too_many_requests_rate"); err != nil {
		return err
	}
	if err := validateProbability(cfg.Random.ServerErrorRate, "random.server_error_rate"); err != nil {
		return err
	}
	if err := validateProbability(cfg.Random.TimeoutRate, "random.timeout_rate"); err != nil {
		return err
	}
	if cfg.Random.Mode == "" {
		cfg.Random.Mode = "true_random"
	}
	if cfg.Random.Mode != "true_random" && cfg.Random.Mode != "seeded" {
		return fmt.Errorf("random.mode must be true_random or seeded")
	}

	if err := validateTextBehavior(&cfg.Chat, cfg.Random.DefaultTokenLength, "chat"); err != nil {
		return err
	}
	if err := validateTextBehavior(&cfg.Responses, cfg.Random.DefaultTokenLength, "responses"); err != nil {
		return err
	}

	if len(cfg.Worker.Models) == 0 {
		cfg.Worker.Models = append([]string(nil), defaultConfig().Worker.Models...)
	}
	cfg.Worker.Models = compactStrings(cfg.Worker.Models)

	if err := validateIntRange(cfg.Images.ImageCount, 1, 16, "images.image_count"); err != nil {
		return err
	}
	if err := validateIntRange(cfg.Images.ImageBytes, 1024, 10_000_000, "images.image_bytes"); err != nil {
		return err
	}
	if err := validateProbability(cfg.Images.ResponseURLRate, "images.response_url_rate"); err != nil {
		return err
	}
	if err := validateProbability(cfg.Images.WatermarkRate, "images.watermark_rate"); err != nil {
		return err
	}
	cfg.Images.Sizes = compactStrings(cfg.Images.Sizes)
	if len(cfg.Images.Sizes) == 0 {
		cfg.Images.Sizes = append([]string(nil), defaultConfig().Images.Sizes...)
	}
	cfg.Images.WatermarkTexts = compactStrings(cfg.Images.WatermarkTexts)
	if len(cfg.Images.WatermarkTexts) == 0 {
		cfg.Images.WatermarkTexts = append([]string(nil), defaultConfig().Images.WatermarkTexts...)
	}
	cfg.Images.BackgroundPalette = compactStrings(cfg.Images.BackgroundPalette)
	if len(cfg.Images.BackgroundPalette) == 0 {
		cfg.Images.BackgroundPalette = append([]string(nil), defaultConfig().Images.BackgroundPalette...)
	}

	if err := validateFloatRange(cfg.Videos.DurationsSeconds, 0.5, 600, "videos.durations_seconds"); err != nil {
		return err
	}
	if err := validateIntRange(cfg.Videos.FPS, 1, 120, "videos.fps"); err != nil {
		return err
	}
	if err := validateIntRange(cfg.Videos.PollIntervalMs, 100, 60_000, "videos.poll_interval_ms"); err != nil {
		return err
	}
	if err := validateIntRange(cfg.Videos.VideoBytes, 1024, 50_000_000, "videos.video_bytes"); err != nil {
		return err
	}
	if err := validateIntRange(cfg.Videos.ProgressJitter, 0, 100, "videos.progress_jitter"); err != nil {
		return err
	}
	if err := validateProbability(cfg.Videos.FailureRate, "videos.failure_rate"); err != nil {
		return err
	}
	cfg.Videos.Resolutions = compactStrings(cfg.Videos.Resolutions)
	if len(cfg.Videos.Resolutions) == 0 {
		cfg.Videos.Resolutions = append([]string(nil), defaultConfig().Videos.Resolutions...)
	}
	return nil
}

func validateTextBehavior(cfg *textBehaviorConfig, fallback intRange, prefix string) error {
	if err := validateIntRange(cfg.TextTokens, fallback.Min, 100_000, prefix+".text_tokens"); err != nil {
		return err
	}
	if err := validateProbability(cfg.UsageProbability, prefix+".usage_probability"); err != nil {
		return err
	}
	if err := validateProbability(cfg.ToolCallProbability, prefix+".tool_call_probability"); err != nil {
		return err
	}
	if err := validateIntRange(cfg.ToolCallCount, 1, 32, prefix+".tool_call_count"); err != nil {
		return err
	}
	if err := validateIntRange(cfg.ToolArgumentsBytes, 8, 100_000, prefix+".tool_arguments_bytes"); err != nil {
		return err
	}
	if err := validateProbability(cfg.MCPToolProbability, prefix+".mcp_tool_probability"); err != nil {
		return err
	}
	if err := validateProbability(cfg.FinalAnswerProbability, prefix+".final_answer_probability"); err != nil {
		return err
	}
	if err := validateProbability(cfg.SystemFingerprintChance, prefix+".system_fingerprint_probability"); err != nil {
		return err
	}
	return nil
}

func validatePort(port int, name string) error {
	if port < 1 || port > 65535 {
		return fmt.Errorf("%s must be between 1 and 65535", name)
	}
	return nil
}

func validateIntRange(r intRange, minAllowed int, maxAllowed int, name string) error {
	if r.Min < minAllowed || r.Max > maxAllowed || r.Min > r.Max {
		return fmt.Errorf("%s must satisfy %d <= min <= max <= %d", name, minAllowed, maxAllowed)
	}
	return nil
}

func validateFloatRange(r floatRange, minAllowed float64, maxAllowed float64, name string) error {
	if r.Min < minAllowed || r.Max > maxAllowed || r.Min > r.Max {
		return fmt.Errorf("%s must satisfy %.2f <= min <= max <= %.2f", name, minAllowed, maxAllowed)
	}
	return nil
}

func validateProbability(v float64, name string) error {
	if v < 0 || v > 1 {
		return fmt.Errorf("%s must be between 0 and 1", name)
	}
	return nil
}

func compactStrings(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
