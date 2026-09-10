package config

import "testing"

// TestConfig_NATSURLDefaultsWhenUnset guards TASK-BE-FFT-008's addition of
// NATSURL — must default to "nats://localhost:4222" when NATS_URL is
// unset, the same convention every other NATS-connecting service already
// uses (see e.g. tenant-service/internal/config.Load).
func TestConfig_NATSURLDefaultsWhenUnset(t *testing.T) {
	t.Setenv("NATS_URL", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.NATSURL != "nats://localhost:4222" {
		t.Errorf("NATSURL: got %q, want nats://localhost:4222", cfg.NATSURL)
	}
}

// TestConfig_NATSURLRespectsEnv confirms NATS_URL is actually read, not
// hardcoded.
func TestConfig_NATSURLRespectsEnv(t *testing.T) {
	t.Setenv("NATS_URL", "nats://custom-host:4222")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.NATSURL != "nats://custom-host:4222" {
		t.Errorf("NATSURL: got %q, want nats://custom-host:4222", cfg.NATSURL)
	}
}
