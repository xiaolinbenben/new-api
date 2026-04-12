package main

import (
	"strings"
	"testing"
)

func TestMaskHeadersAndPreview(t *testing.T) {
	t.Parallel()

	headers := map[string]string{
		"Authorization": "Bearer secret",
		"X-Test":        "value",
	}
	masked := maskHeaders(headers, []string{"authorization"})
	if masked["Authorization"] != "***" {
		t.Fatalf("expected authorization to be masked, got %q", masked["Authorization"])
	}
	if masked["X-Test"] != "value" {
		t.Fatalf("expected x-test to remain visible")
	}

	preview := previewString(strings.Repeat("a", 20), 8, "application/json")
	if !preview.Truncated {
		t.Fatalf("expected preview to be truncated")
	}
	if preview.Bytes != 20 {
		t.Fatalf("unexpected preview bytes: %d", preview.Bytes)
	}
}
