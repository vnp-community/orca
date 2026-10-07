package config

import (
	"testing"
)

func TestLoadCodeIntel(t *testing.T) {
	// Default
	t.Setenv("CODE_INTEL_SERVICE_ADDR", "")
	t.Setenv("CODE_INTEL_MAX_RESPONSE_BYTES", "")
	t.Setenv("CODE_INTEL_MAX_STREAMS", "")

	cfg, err := loadCodeIntel()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MaxResponseBytes != 2<<20 || cfg.MaxStreams != 500 || cfg.ServiceAddr != "" {
		t.Errorf("default mismatch: %+v", cfg)
	}

	// Invalid MaxResponseBytes
	t.Setenv("CODE_INTEL_MAX_RESPONSE_BYTES", "abc")
	if _, err := loadCodeIntel(); err == nil {
		t.Errorf("expected int parse error, got nil")
	}
	t.Setenv("CODE_INTEL_MAX_RESPONSE_BYTES", "0")
	if _, err := loadCodeIntel(); err == nil || err.Error() != "CODE_INTEL_MAX_RESPONSE_BYTES invalid" {
		t.Errorf("expected out of bounds error, got %v", err)
	}
	t.Setenv("CODE_INTEL_MAX_RESPONSE_BYTES", "4194304") // 4MiB > 3MiB
	if _, err := loadCodeIntel(); err == nil || err.Error() != "CODE_INTEL_MAX_RESPONSE_BYTES invalid" {
		t.Errorf("expected out of bounds error, got %v", err)
	}

	// Invalid MaxStreams
	t.Setenv("CODE_INTEL_MAX_RESPONSE_BYTES", "")
	t.Setenv("CODE_INTEL_MAX_STREAMS", "-1")
	if _, err := loadCodeIntel(); err == nil || err.Error() != "CODE_INTEL_MAX_STREAMS invalid" {
		t.Errorf("expected out of bounds error, got %v", err)
	}
}
