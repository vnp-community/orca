package domain

import (
	"sync"
	"time"
)

// CollectorMetrics records in-process metrics for CodeIntel collector invocations.
type CollectorMetrics struct {
	mu               sync.RWMutex
	callsTotal       map[string]int64
	durationSeconds  map[string]float64
	reconnectWaitSec float64
	truncatedTotal   map[string]int64
}

// NewCollectorMetrics initializes a new CollectorMetrics instance.
func NewCollectorMetrics() *CollectorMetrics {
	return &CollectorMetrics{
		callsTotal:      make(map[string]int64),
		durationSeconds: make(map[string]float64),
		truncatedTotal:  make(map[string]int64),
	}
}

// RecordCall tracks codeintel_collector_calls_total{method, outcome}.
func (m *CollectorMetrics) RecordCall(method, outcome string) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	key := method + ":" + outcome
	m.callsTotal[key]++
}

// RecordDuration tracks codeintel_collector_duration_seconds{method}.
func (m *CollectorMetrics) RecordDuration(method string, d time.Duration) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.durationSeconds[method] += d.Seconds()
}

// RecordReconnectWait tracks codeintel_collector_reconnect_wait_seconds.
func (m *CollectorMetrics) RecordReconnectWait(d time.Duration) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.reconnectWaitSec += d.Seconds()
}

// RecordTruncated tracks codeintel_collector_truncated_total{view}.
func (m *CollectorMetrics) RecordTruncated(view string) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.truncatedTotal[view]++
}

// GetCallCount returns the recorded count for a method and outcome.
func (m *CollectorMetrics) GetCallCount(method, outcome string) int64 {
	if m == nil {
		return 0
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.callsTotal[method+":"+outcome]
}

// GetTruncatedCount returns the recorded truncation count for a view.
func (m *CollectorMetrics) GetTruncatedCount(view string) int64 {
	if m == nil {
		return 0
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.truncatedTotal[view]
}
