package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	commonconfig "github.com/stablyai/orca-go/common/config"
)

type Config struct {
	commonconfig.Base
	InfraFleetServiceAddr   string
	AgentCallTimeout        time.Duration
	MaxInflightPerDevServer int
	AgentMaxResultBytes     int64
	ReconnectWait           time.Duration

	// Cache configuration (SOL-022)
	HeadProbeTTL                time.Duration
	CollectTimeout              time.Duration
	SnapshotTTL                 time.Duration
	SnapshotKeepCommits         int
	SnapshotMaxBytes            int64
	CacheMaxBytesPerBinding     int64
	CacheMaxBytesPerTenant      int64
	SymbolLRUEntries            int
	SymbolLRUTTL                time.Duration
	MaintenanceInterval         time.Duration
}

func Load() (Config, error) {
	base, err := commonconfig.LoadBase("code-intel-service")
	if err != nil {
		return Config{}, err
	}

	timeout, err := durationEnv("CODEINTEL_AGENT_CALL_TIMEOUT", 95*time.Second)
	if err != nil {
		return Config{}, err
	}

	reconnectWait, err := durationEnv("CODEINTEL_RECONNECT_WAIT", 20*time.Second)
	if err != nil {
		return Config{}, err
	}

	maxInflight, err := intEnv("CODEINTEL_MAX_INFLIGHT_PER_DEVSERVER", 4)
	if err != nil {
		return Config{}, err
	}

	maxResultBytes, err := int64Env("CODEINTEL_AGENT_MAX_RESULT_BYTES", 12*1024*1024) // 12 MiB
	if err != nil {
		return Config{}, err
	}

	headProbeTTL, err := durationEnv("CODEINTEL_HEAD_PROBE_TTL", 15*time.Second)
	if err != nil {
		return Config{}, err
	}
	collectTimeout, err := durationEnv("CODEINTEL_COLLECT_TIMEOUT", 100*time.Second)
	if err != nil {
		return Config{}, err
	}
	snapshotTTL, err := durationEnv("CODEINTEL_SNAPSHOT_TTL", 168*time.Hour)
	if err != nil {
		return Config{}, err
	}
	keepCommits, err := intEnv("CODEINTEL_SNAPSHOT_KEEP_COMMITS", 3)
	if err != nil {
		return Config{}, err
	}
	snapshotMaxBytes, err := int64Env("CODEINTEL_SNAPSHOT_MAX_BYTES", 3*1024*1024)
	if err != nil {
		return Config{}, err
	}
	cacheMaxBytesPerBinding, err := int64Env("CODEINTEL_CACHE_MAX_BYTES_PER_BINDING", 64*1024*1024)
	if err != nil {
		return Config{}, err
	}
	cacheMaxBytesPerTenant, err := int64Env("CODEINTEL_CACHE_MAX_BYTES_PER_TENANT", 512*1024*1024)
	if err != nil {
		return Config{}, err
	}
	symbolLRUEntries, err := intEnv("CODEINTEL_SYMBOL_LRU_ENTRIES", 64)
	if err != nil {
		return Config{}, err
	}
	symbolLRUTTL, err := durationEnv("CODEINTEL_SYMBOL_LRU_TTL", 5*time.Minute)
	if err != nil {
		return Config{}, err
	}
	maintenanceInterval, err := durationEnv("CODEINTEL_MAINTENANCE_INTERVAL", 10*time.Minute)
	if err != nil {
		return Config{}, err
	}

	return Config{
		Base:                    base,
		InfraFleetServiceAddr:   commonconfig.StringEnv("INFRA_FLEET_SERVICE_ADDR", "localhost:50051"),
		AgentCallTimeout:        timeout,
		MaxInflightPerDevServer: maxInflight,
		AgentMaxResultBytes:     maxResultBytes,
		ReconnectWait:           reconnectWait,
		HeadProbeTTL:            headProbeTTL,
		CollectTimeout:          collectTimeout,
		SnapshotTTL:             snapshotTTL,
		SnapshotKeepCommits:     keepCommits,
		SnapshotMaxBytes:        snapshotMaxBytes,
		CacheMaxBytesPerBinding: cacheMaxBytesPerBinding,
		CacheMaxBytesPerTenant:  cacheMaxBytesPerTenant,
		SymbolLRUEntries:        symbolLRUEntries,
		SymbolLRUTTL:            symbolLRUTTL,
		MaintenanceInterval:     maintenanceInterval,
	}, nil
}

func intEnv(key string, def int) (int, error) {
	v := os.Getenv(key)
	if v == "" {
		return def, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("config: invalid int for %s=%q: %w", key, v, err)
	}
	return n, nil
}

func int64Env(key string, def int64) (int64, error) {
	v := os.Getenv(key)
	if v == "" {
		return def, nil
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("config: invalid int64 for %s=%q: %w", key, v, err)
	}
	return n, nil
}

func durationEnv(key string, def time.Duration) (time.Duration, error) {
	v := os.Getenv(key)
	if v == "" {
		return def, nil
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return 0, fmt.Errorf("config: invalid duration for %s=%q: %w", key, v, err)
	}
	return d, nil
}
