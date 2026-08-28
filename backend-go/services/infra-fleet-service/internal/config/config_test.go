package config

import (
	"testing"
	"time"
)

func TestFleetPollIntervalFromEnv(t *testing.T) {
	tests := []struct {
		name string
		env  string
		want time.Duration
	}{
		{"unset falls back to default", "", 30 * time.Second},
		{"valid override", "60", 60 * time.Second},
		{"zero falls back to default", "0", 30 * time.Second},
		{"negative falls back to default", "-5", 30 * time.Second},
		{"unparseable falls back to default", "not-a-number", 30 * time.Second},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// t.Setenv("", "") reads the same as truly-unset in
			// fleetPollIntervalFromEnv, which only checks `raw != ""`.
			t.Setenv("FLEET_POLL_INTERVAL_SEC", tt.env)
			got := fleetPollIntervalFromEnv()
			if got != tt.want {
				t.Errorf("fleetPollIntervalFromEnv() with FLEET_POLL_INTERVAL_SEC=%q = %v, want %v", tt.env, got, tt.want)
			}
		})
	}
}
