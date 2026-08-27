package domain

import (
	"errors"
	"testing"
)

func TestNewDevServer_ValidatesInvariants(t *testing.T) {
	tests := []struct {
		name        string
		tenantID    string
		host        string
		mode        ConnectionMode
		sshTargetID string
		wantErr     error
	}{
		{"valid relay-ssh", "t1", "10.0.0.1", ConnectionModeRelaySSH, "ssht1", nil},
		{"valid relay-websocket", "t1", "10.0.0.1", ConnectionModeRelayWebSocket, "", nil},
		{"valid direct-websocket", "t1", "10.0.0.1", ConnectionModeDirectWebSocket, "", nil},
		{"empty tenant", "", "10.0.0.1", ConnectionModeRelaySSH, "ssht1", ErrEmptyDevServerTenant},
		{"empty host", "t1", "", ConnectionModeRelaySSH, "ssht1", ErrEmptyHost},
		{"invalid mode", "t1", "10.0.0.1", ConnectionMode("bogus"), "", ErrInvalidConnectionMode},
		{"unspecified mode", "t1", "10.0.0.1", ConnectionMode(""), "", ErrInvalidConnectionMode},
		{"relay-ssh missing ssh target", "t1", "10.0.0.1", ConnectionModeRelaySSH, "", ErrMissingSSHTargetForRelaySSH},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ds, err := NewDevServer("ds1", tt.tenantID, tt.host, tt.mode, tt.sshTargetID, nil)
			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("expected no error, got %v", err)
				}
				if ds.ID != "ds1" || ds.TenantID != tt.tenantID || ds.Host != tt.host || ds.Mode != tt.mode || ds.SSHTargetID != tt.sshTargetID {
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

// TestNewDevServer_RelaySSHRequiresSSHTargetID is a focused regression test
// for the invariant above — relay-ssh mode with a blank SSHTargetID must
// never construct a DevServer sshconn.Connector can't dial.
func TestNewDevServer_RelaySSHRequiresSSHTargetID(t *testing.T) {
	_, err := NewDevServer("ds1", "t1", "10.0.0.1", ConnectionModeRelaySSH, "", nil)
	if !errors.Is(err, ErrMissingSSHTargetForRelaySSH) {
		t.Fatalf("expected ErrMissingSSHTargetForRelaySSH, got %v", err)
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
	ds, err := NewDevServer("ds1", "t1", "host", ConnectionModeRelaySSH, "ssht1", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ds.IsZero() {
		t.Error("expected a constructed DevServer to not report IsZero")
	}
}
