package config

import (
	"fmt"
	"time"
)

// SessionConfig configures the session reaper (BE-MCP-SOL-004).
// Env: MCP_SESSION_IDLE_TTL (default 30m), MCP_SESSION_REAP_INTERVAL (default 1m).
type SessionConfig struct {
	IdleTTL        time.Duration
	ReapInterval   time.Duration
	ReapBatchLimit int
}

func LoadSessions() (SessionConfig, error) {
	ttl, err := durationEnv("MCP_SESSION_IDLE_TTL", 30*time.Minute)
	if err != nil {
		return SessionConfig{}, err
	}
	every, err := durationEnv("MCP_SESSION_REAP_INTERVAL", time.Minute)
	if err != nil {
		return SessionConfig{}, err
	}
	if ttl < time.Minute {
		return SessionConfig{}, fmt.Errorf("config: MCP_SESSION_IDLE_TTL must be >= 1m")
	}
	return SessionConfig{IdleTTL: ttl, ReapInterval: every, ReapBatchLimit: 200}, nil
}
