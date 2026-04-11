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
		"worker": map[string]any{
			"port": 19081,
		},
		"random": map[string]any{
			"mode": "seeded",
			"seed": 123,
		},
	}
	data, err := common.Marshal(partial)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, data, 0o644))

	cfg, err := loadConfigFile(path)
	require.NoError(t, err)
	require.Equal(t, 19081, cfg.Worker.Port)
	require.Equal(t, "seeded", cfg.Random.Mode)
	require.EqualValues(t, 123, cfg.Random.Seed)
	require.NotEmpty(t, cfg.Worker.ManagementToken)
	require.NotEmpty(t, cfg.Worker.Models)
	require.Greater(t, cfg.Images.ImageBytes.Max, 0)
	require.Greater(t, cfg.Videos.VideoBytes.Max, 0)
}
