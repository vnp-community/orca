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
