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
	"log/slog"

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

// Entry holds the full set of fields for an audit log entry.
type Entry struct {
	TenantID     string
	ActorID      string
	ActorType    string
	Action       string
	Target       string
	TargetType   string
	TargetID     string
	Outcome      string
	IPAddress    string
	MetadataJSON string
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
//
// This is a wrapper around AppendDetailed to avoid breaking existing callers.
func (c *Client) Append(ctx context.Context, tenantID, actorID, action, target, outcome, ip string) {
	c.AppendDetailed(ctx, Entry{
		TenantID:  tenantID,
		ActorID:   actorID,
		Action:    action,
		Target:    target,
		Outcome:   outcome,
		IPAddress: ip,
	})
}

// AppendDetailed records an audit entry with all available fields (including
// actor_type, target_type, target_id, and metadata_json). Like Append, it
// swallows RPC errors.
func (c *Client) AppendDetailed(ctx context.Context, e Entry) {
	_ = c.AppendDetailedStrict(ctx, e)
}

// AppendDetailedStrict is AppendDetailed that returns the RPC error. It is for
// durable delivery (an outbox retrying until auth-service accepts the entry);
// decision paths keep using the best-effort variants above.
func (c *Client) AppendDetailedStrict(ctx context.Context, e Entry) error {
	if e.ActorType != "" && e.ActorType != "user" && e.ActorType != "agent" && e.ActorType != "system" {
		slog.Debug("auditclient: invalid ActorType dropped", "actor_type", e.ActorType)
		e.ActorType = ""
	}
	if len(e.MetadataJSON) > 4096 {
		e.MetadataJSON = `{"truncated":true}`
	}
	_, err := c.auth.AppendAuditEntry(ctx, &authv1.AppendAuditEntryRequest{
		TenantId:     e.TenantID,
		ActorId:      e.ActorID,
		ActorType:    e.ActorType,
		Action:       e.Action,
		Target:       e.Target,
		TargetType:   e.TargetType,
		TargetId:     e.TargetID,
		Outcome:      e.Outcome,
		IpAddress:    e.IPAddress,
		MetadataJson: e.MetadataJSON,
	})
	return err
}
