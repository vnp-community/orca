package domain

import "time"

// Event subjects of the external server registry (BE-MCP-SOL-014 section I).
// Payloads carry ids, names and counts only: never a secret value or token.
const (
	SubjectExternalServerCreated      = "orca.mcp.externalserver.created"
	SubjectExternalServerUpdated      = "orca.mcp.externalserver.updated"
	SubjectExternalServerDeleted      = "orca.mcp.externalserver.deleted"
	SubjectExternalServerReviewed     = "orca.mcp.externalserver.reviewed"
	SubjectExternalServerSecretSet    = "orca.mcp.externalserver.secret_set"
	SubjectExternalServerToolsChanged = "orca.mcp.externalserver.tools_changed"
	SubjectExternalServerHealth       = "orca.mcp.externalserver.health_changed"
	SubjectAgentConfigResolved        = "orca.mcp.agentconfig.resolved"
)

// Audit actions for the registry.
const (
	AuditActionExternalServerUpsert    = "mcp.external_server.upsert"
	AuditActionExternalServerDelete    = "mcp.external_server.delete"
	AuditActionExternalServerSecretSet = "mcp.external_server.secret_set"
	AuditActionExternalServerProbe     = "mcp.external_server.probe"
	AuditActionExternalServerReview    = "mcp.external_server.review"
	AuditActionExternalCall            = "mcp.external.call"
	AuditActionExternalRead            = "mcp.external.read"
)

// NewExternalCallAuditEvent audits an external call made by an internal
// service. actorType is "user", "agent" or "system" (auth-service CHECK); meta
// must hold ids, sizes and digests only, never arguments or content.
func NewExternalCallAuditEvent(eventID, auditID, tenantID, actorID, actorType, action, serverID, outcome string, at time.Time, meta map[string]any) (OutboxRecord, error) {
	if meta == nil {
		meta = map[string]any{}
	}
	return NewOutboxEvent(eventID, SubjectAuditAppended, tenantID, at, map[string]any{
		"audit_id": auditID, "actor_id": actorID, "action": action, "actor_type": actorType,
		"target_type": "mcp_external_server", "target_id": serverID, "outcome": outcome, "metadata": meta,
	})
}

// Action is something a caller does to a registry entry.
type Action string

const (
	ActionWrite  Action = "write"  // upsert, setSecret, delete
	ActionProbe  Action = "probe"  // owner of a user-scope server may probe it
	ActionReview Action = "review" // admin only
)

// Actor is the authenticated caller (from gRPC metadata, never the body).
type Actor struct{ UserID, Role string }

func (a Actor) IsAdmin() bool { return a.Role == RoleAdmin }

// CanSee is the list filter: admins see everything, others only their
// own user-scope servers.
func (a Actor) CanSee(scope, scopeID string) bool {
	return a.IsAdmin() || (scope == ScopeUser && scopeID == a.UserID && a.UserID != "")
}

// Authorize applies the permission matrix to an existing entry. A caller who
// may not even see the entry gets ErrNotFound (never reveals existence); a
// caller who sees it but lacks the role gets ErrNotAdmin.
func (a Actor) Authorize(act Action, scope, scopeID string) error {
	if a.IsAdmin() {
		return nil
	}
	if !a.CanSee(scope, scopeID) {
		return ErrNotFound()
	}
	if act == ActionReview {
		return ErrNotAdmin()
	}
	return nil // owner of a user-scope server: write, probe
}

// AuthorizeCreate decides who may create in a scope. Non-admins can only
// create user-scope servers, and the scope id is forced to themselves.
func (a Actor) AuthorizeCreate(scope string) error {
	if a.IsAdmin() || scope == ScopeUser {
		return nil
	}
	return ErrNotAdmin()
}
