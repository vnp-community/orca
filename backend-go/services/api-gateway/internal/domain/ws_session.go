package domain

import "time"

// WSSession is ephemeral state for one live WS<->gRPC-stream bridge. Per
// specs/backend-go/services/api-gateway.md §4: "No identity beyond the
// connection's lifetime; if it drops, WSSession is discarded — nothing to
// fail over." Never persisted, never shared across replicas.
type WSSession struct {
	// ConnectionID identifies this WS connection for logging/tracing.
	ConnectionID string
	// TenantID/UserID are the identity resolved from the validated
	// session/JWT (§9) — attached to outbound gRPC metadata, never trusted
	// from the request body.
	TenantID string
	UserID   string
	// OwningService is the ServiceRegistry-resolved service this session's
	// gRPC stream is bridged to (e.g. "notification-service").
	OwningService string
	// OpenedAt is used for connection-age logging/metrics.
	OpenedAt time.Time
}
