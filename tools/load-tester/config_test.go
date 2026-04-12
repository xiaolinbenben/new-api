package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestLoadConfigFileMergesDefaults(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	partial := map[string]any{
		"target": map[string]any{
			"base_url": "http://127.0.0.1:9999",
		},
		"run": map[string]any{
			"total_concurrency": 12,
			"duration_sec":      15,
		},
		"scenarios": []map[string]any{
			{
				"id":      "chat",
				"name":    "chat",
				"enabled": true,
				"preset":  "new_api_chat",
				"mode":    "sse",
				"weight":  3,
			},
		},
	}
	data, err := common.Marshal(partial)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, data, 0o644))

	cfg, err := loadConfigFile(path)
	require.NoError(t, err)
	require.Equal(t, "http://127.0.0.1:9999", cfg.Target.BaseURL)
	require.Equal(t, 12, cfg.Run.TotalConcurrency)
	require.Equal(t, 15, cfg.Run.DurationSec)
	require.NotEmpty(t, cfg.Worker.ManagementToken)
	require.Len(t, cfg.Scenarios, 1)
	require.Equal(t, "/v1/chat/completions", cfg.Scenarios[0].Path)
	require.Equal(t, "POST", cfg.Scenarios[0].Method)
	require.NotEmpty(t, cfg.Sampling.MaskHeaders)
}
