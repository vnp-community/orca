// Package auditclient is the thin cross-service wrapper every non-auth
// service's OPA-gated usecases (annotation-service, infra-fleet-service,
// project-service, task-service — TASK-BE-017/019..022) use to append an
// entry onto auth-service's own audit_log via its AppendAuditEntry RPC
// (auth.audit_log lives only in auth-service's database, per the
// database-per-service rule — see that RPC's proto doc comment for why
// this is the one cross-service write path onto it).
package auditclient

import (
	"context"

	authv1 "github.com/stablyai/orca-go/proto/gen/go/orca/auth/v1"
)

// Client wraps an authv1.AuthServiceClient for the one RPC every calling
// usecase needs.
type Client struct {
	auth authv1.AuthServiceClient
}

func New(auth authv1.AuthServiceClient) *Client {
	return &Client{auth: auth}
}

// Append records one audit entry — synchronous but best-effort: the RPC
// error (if any) is deliberately swallowed, never returned, so a slow or
// unreachable auth-service can never turn a real authorization decision
// into a 500 (same rationale as auth-service's own appendAuditBestEffort;
// "non-blocking" in every call site's doc comment means "never blocks the
// decision on failure," not "fired asynchronously" — the append has
// observably happened by the time this call returns). action/target follow
// auth.audit_log's existing convention (e.g. "annotation.delete",
// "annotation:ann-123"); outcome is "allowed" | "denied".
func (c *Client) Append(ctx context.Context, tenantID, actorID, action, target, outcome, ip string) {
	_, _ = c.auth.AppendAuditEntry(ctx, &authv1.AppendAuditEntryRequest{
		TenantId:  tenantID,
		ActorId:   actorID,
		Action:    action,
		Target:    target,
		Outcome:   outcome,
		IpAddress: ip,
	})
}
