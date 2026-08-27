// Package domain holds infra-fleet-service's entities and value objects. Per
// specs/backend-go/architecture/03-clean-architecture-guidelines.md, this
// package has zero imports outside stdlib + other domain/ packages — no
// database, no gRPC, no framework.
package domain

import "errors"

// ConnectionMode is the transport this service uses to reach a DevServer —
// mirrors orca.infrafleet.v1.ConnectionMode and TS's provider registry
// transport axis (direct-websocket / relay-websocket / relay-ssh), see
// specs/backend-go/services/infra-fleet-service.md §4.
type ConnectionMode string

const (
	ConnectionModeRelaySSH        ConnectionMode = "relay-ssh"
	ConnectionModeRelayWebSocket  ConnectionMode = "relay-websocket"
	ConnectionModeDirectWebSocket ConnectionMode = "direct-websocket"
)

// Valid reports whether m is one of the known enum values.
func (m ConnectionMode) Valid() bool {
	switch m {
	case ConnectionModeRelaySSH, ConnectionModeRelayWebSocket, ConnectionModeDirectWebSocket:
		return true
	default:
		return false
	}
}

var (
	// ErrEmptyDevServerTenant is returned when TenantID is empty — a dev
	// server with no owning tenant is never a valid domain state.
	ErrEmptyDevServerTenant = errors.New("domain: tenant_id is required")
	// ErrEmptyHost guards against registering a dev server nobody can reach.
	ErrEmptyHost = errors.New("domain: host is required")
	// ErrInvalidConnectionMode is returned when mode isn't one of the known
	// transport modes.
	ErrInvalidConnectionMode = errors.New("domain: invalid connection mode")
	// ErrMissingSSHTargetForRelaySSH is returned when mode is
	// ConnectionModeRelaySSH but SSHTargetID is empty — relay-ssh's
	// cert-based SSH auth (sshconn.Connector) has no target to dial without
	// it, see specs/backend-go/services/infra-fleet-service.md §9.
	ErrMissingSSHTargetForRelaySSH = errors.New("domain: ssh_target_id is required when mode is relay-ssh")
)

// DevServer is a registered dev host: which tenant owns it, how to reach it
// (host), and which transport mode this service uses to talk to it.
// SSHTargetID links to the SshTarget sshconn.Connector dials for cert-based
// SSH auth — required for relay-ssh mode, empty for the other two modes. See
// specs/backend-go/services/infra-fleet-service.md §4 — this scaffold's
// DevServer is the proto-sized subset of the design doc's fuller entity
// (display name, bootstrap status, agent version are not modeled here; see
// this service's README "Known gaps").
type DevServer struct {
	ID          string
	TenantID    string
	Host        string
	Mode        ConnectionMode
	SSHTargetID string
}

// NewDevServer constructs a DevServer, enforcing the invariants a record
// must satisfy to be meaningful — this is where "infra-fleet-service owns
// this data's correctness" actually lives, not scattered validation in the
// gRPC handler.
func NewDevServer(id, tenantID, host string, mode ConnectionMode, sshTargetID string) (DevServer, error) {
	if tenantID == "" {
		return DevServer{}, ErrEmptyDevServerTenant
	}
	if host == "" {
		return DevServer{}, ErrEmptyHost
	}
	if !mode.Valid() {
		return DevServer{}, ErrInvalidConnectionMode
	}
	if mode == ConnectionModeRelaySSH && sshTargetID == "" {
		return DevServer{}, ErrMissingSSHTargetForRelaySSH
	}
	return DevServer{ID: id, TenantID: tenantID, Host: host, Mode: mode, SSHTargetID: sshTargetID}, nil
}

// IsZero reports whether ds is the zero-value DevServer — used by
// ResolveConnection's not-found branch to signal "no dev server", distinct
// from a real DevServer with a coincidentally empty field.
func (ds DevServer) IsZero() bool {
	return ds == DevServer{}
}
