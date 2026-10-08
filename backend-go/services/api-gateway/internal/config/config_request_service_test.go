package config

import "testing"

func TestConfig_RequestServiceAddr(t *testing.T) {
	t.Setenv("REQUEST_SERVICE_ADDR", "")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got, ok := cfg.OtherServiceAddrs["request-service"]; !ok || got != "" {
		t.Fatalf("empty env must leave an empty request-service addr, got %q (present=%v)", got, ok)
	}

	t.Setenv("REQUEST_SERVICE_ADDR", "request-service:9090")
	cfg, err = Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := cfg.OtherServiceAddrs["request-service"]; got != "request-service:9090" {
		t.Fatalf("request-service addr = %q", got)
	}
}
