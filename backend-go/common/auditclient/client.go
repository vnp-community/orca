// Package auditclient is the thin gRPC client every non-auth-service uses to
// append an audit entry to auth-service's audit_log — audit_log is
// auth-service's own schema (auth-service.md §2), so no other service talks
// to its Postgres directly.
package auditclient

import (
	"context"
	"log/slog"

	authv1 "github.com/stablyai/orca-go/proto/gen/go/orca/auth/v1"
)

// Client is the shared cross-service audit-append client (TASK-BE-018),
// following the same "cross-service shared code policy" as
// common/grpcmw/common/tenant.
type Client struct {
	auth authv1.AuthServiceClient
}

// New wraps an already-dialed authv1.AuthServiceClient.
func New(auth authv1.AuthServiceClient) *Client { return &Client{auth: auth} }

// Append is best-effort: an audit-append failure is logged, never returned
// to the caller as an error — a permission-check RPC's own success/failure
// must never depend on the audit system being reachable (matches
// auth-service's own audit calls, which never roll back the primary
// operation on an audit-write failure — see update_access_policy.go's doc
// comment on PublishPolicyChange's identical non-blocking posture).
func (c *Client) Append(ctx context.Context, tenantID, actorID, action, target, outcome, ipAddress string) {
	_, err := c.auth.AppendAuditEntry(ctx, &authv1.AppendAuditEntryRequest{
		TenantId: tenantID, ActorId: actorID, Action: action, Target: target, Outcome: outcome, IpAddress: ipAddress,
	})
	if err != nil {
		slog.WarnContext(ctx, "auditclient: failed to append audit entry", "action", action, "error", err)
	}
}
