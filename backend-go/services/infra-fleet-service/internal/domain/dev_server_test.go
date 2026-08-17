package domain

import "testing"

func TestNewDevServer_ValidatesInvariants(t *testing.T) {
	tests := []struct {
		name     string
		tenantID string
		host     string
		mode     ConnectionMode
		wantErr  error
	}{
		{"valid relay-ssh", "t1", "10.0.0.1", ConnectionModeRelaySSH, nil},
		{"valid relay-websocket", "t1", "10.0.0.1", ConnectionModeRelayWebSocket, nil},
		{"valid direct-websocket", "t1", "10.0.0.1", ConnectionModeDirectWebSocket, nil},
		{"empty tenant", "", "10.0.0.1", ConnectionModeRelaySSH, ErrEmptyDevServerTenant},
		{"empty host", "t1", "", ConnectionModeRelaySSH, ErrEmptyHost},
		{"invalid mode", "t1", "10.0.0.1", ConnectionMode("bogus"), ErrInvalidConnectionMode},
		{"unspecified mode", "t1", "10.0.0.1", ConnectionMode(""), ErrInvalidConnectionMode},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ds, err := NewDevServer("ds1", tt.tenantID, tt.host, tt.mode)
			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("expected no error, got %v", err)
				}
				if ds.ID != "ds1" || ds.TenantID != tt.tenantID || ds.Host != tt.host || ds.Mode != tt.mode {
					t.Errorf("unexpected DevServer: %+v", ds)
				}
				return
			}
			if err != tt.wantErr {
				t.Fatalf("expected %v, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestConnectionMode_Valid(t *testing.T) {
	valid := []ConnectionMode{ConnectionModeRelaySSH, ConnectionModeRelayWebSocket, ConnectionModeDirectWebSocket}
	for _, m := range valid {
		if !m.Valid() {
			t.Errorf("expected %q to be valid", m)
		}
	}
	invalid := []ConnectionMode{"", "bogus", "RELAY-SSH"}
	for _, m := range invalid {
		if m.Valid() {
			t.Errorf("expected %q to be invalid", m)
		}
	}
}

func TestDevServer_IsZero(t *testing.T) {
	if !(DevServer{}).IsZero() {
		t.Error("expected zero-value DevServer to report IsZero")
	}
	ds, err := NewDevServer("ds1", "t1", "host", ConnectionModeRelaySSH)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ds.IsZero() {
		t.Error("expected a constructed DevServer to not report IsZero")
	}
}
