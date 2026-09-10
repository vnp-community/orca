package domain

// PortForwardStatus is a live local:remote tunnel's lifecycle state.
type PortForwardStatus string

const (
	PortForwardStatusActive PortForwardStatus = "active"
	PortForwardStatusClosed PortForwardStatus = "closed"
)

// PortForward is a live local:remote tunnel, per
// specs/backend-go/services/infra-fleet-service.md §4/§5.
type PortForward struct {
	ID           string
	TenantID     string
	ConnectionID string
	LocalPort    int
	RemotePort   int
	ProcessName  string // carries ports.detect's processName through to the
	// frontend notification — additive beyond §5's DDL
	Status PortForwardStatus
}
