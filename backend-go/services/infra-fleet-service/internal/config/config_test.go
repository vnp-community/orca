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

func TestPositiveDurationFromEnv(t *testing.T) {
	const name = "INFRA_TEST_DURATION"
	cases := []struct {
		raw  string
		want time.Duration
	}{
		{"", time.Hour},
		{"30m", 30 * time.Minute},
		{"garbage", time.Hour},
		{"0s", time.Hour},
		{"-5m", time.Hour},
	}
	for _, tc := range cases {
		t.Setenv(name, tc.raw)
		if got := positiveDurationFromEnv(name, time.Hour); got != tc.want {
			t.Errorf("raw=%q: got %v, want %v", tc.raw, got, tc.want)
		}
	}
}
