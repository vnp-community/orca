package domain

import "testing"

func TestNewDevServerHealth_ValidatesInvariants(t *testing.T) {
	tests := []struct {
		name        string
		devServerID string
		cpu         float64
		ram         float64
		disk        float64
		latencyMS   int64
		wantErr     error
	}{
		{"valid reachable", "ds1", 12.5, 40.0, 60.0, 15, nil},
		{"valid unreachable zeros", "ds1", 0, 0, 0, 0, nil},
		{"empty dev server id", "", 0, 0, 0, 0, ErrEmptyHealthDevServerID},
		{"cpu out of range", "ds1", 150, 0, 0, 0, ErrPercentOutOfRange},
		{"cpu negative", "ds1", -1, 0, 0, 0, ErrPercentOutOfRange},
		{"ram out of range", "ds1", 0, 101, 0, 0, ErrPercentOutOfRange},
		{"disk out of range", "ds1", 0, 0, -5, 0, ErrPercentOutOfRange},
		{"negative latency", "ds1", 0, 0, 0, -1, ErrNegativeLatency},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, err := NewDevServerHealth(tt.devServerID, true, tt.cpu, tt.ram, tt.disk, tt.latencyMS)
			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("expected no error, got %v", err)
				}
				if h.DevServerID != tt.devServerID {
					t.Errorf("unexpected DevServerHealth: %+v", h)
				}
				return
			}
			if err != tt.wantErr {
				t.Fatalf("expected %v, got %v", tt.wantErr, err)
			}
		})
	}
}

// TestComputeHealthStatus covers BL-FLEET-03's threshold table verbatim —
// docs/logic/fleet/BL-FLEET-03-health-monitoring.md's Status Model.
func TestComputeHealthStatus(t *testing.T) {
	tests := []struct {
		name           string
		reachable      bool
		relayReachable bool
		cpu, ram       float64
		want           HealthStatus
	}{
		{"unreachable — ssh connect fails, relay/metrics don't matter", false, false, 0, 0, HealthStatusUnreachable},
		{"unreachable takes priority over relay/cpu/ram", false, true, 200, 200, HealthStatusUnreachable},
		{"unhealthy — ssh alive, relay handshake not", true, false, 10, 10, HealthStatusUnhealthy},
		{"unhealthy takes priority over cpu/ram thresholds", true, false, 200, 200, HealthStatusUnhealthy},
		{"healthy — cpu 79, ram 84 (just under both thresholds)", true, true, 79, 84, HealthStatusHealthy},
		{"degraded — cpu 81 (over 80 threshold)", true, true, 81, 10, HealthStatusDegraded},
		{"degraded — ram 86 (over 85 threshold)", true, true, 10, 86, HealthStatusDegraded},
		{"healthy — cpu exactly 80 (not > 80)", true, true, 80, 10, HealthStatusHealthy},
		{"healthy — ram exactly 85 (not > 85)", true, true, 10, 85, HealthStatusHealthy},
		{"healthy — both zero", true, true, 0, 0, HealthStatusHealthy},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ComputeHealthStatus(tt.reachable, tt.relayReachable, tt.cpu, tt.ram)
			if got != tt.want {
				t.Errorf("ComputeHealthStatus(%v, %v, %v, %v) = %q, want %q", tt.reachable, tt.relayReachable, tt.cpu, tt.ram, got, tt.want)
			}
		})
	}
}
