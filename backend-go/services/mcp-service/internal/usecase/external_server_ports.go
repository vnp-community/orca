package usecase

import (
	"context"
	"time"

	"github.com/stablyai/orca-go/services/mcp-service/internal/domain"
)

// ExternalServerFilter narrows ListExternalServers. OwnerUserID, when set,
// restricts the result to scope=user servers owned by that user.
type ExternalServerFilter struct {
	Scope       string
	OwnerUserID string
}

// ProbeRecord is one observation of a server's tools.
type ProbeRecord struct {
	Digest string
	Tools  []domain.ToolInfo
	At     time.Time
	Source string // probe|health
}

// ReviewRecord is an admin decision. For approve, ExpectedDigest must equal
// the stored last_probe_digest at write time (checked atomically).
type ReviewRecord struct {
	TenantID, ServerID, ReviewerID string
	Approve                        bool
	ExpectedDigest                 string
	At                             time.Time
}

// ServerKey identifies a server across tenants for background work.
type ServerKey struct{ TenantID, ServerID string }

// ExternalServerRepository persists the registry. Every mutation takes the
// outbox events to enqueue in the same transaction. No method ever sees a
// secret value: only SecretRef pointers.
type ExternalServerRepository interface {
	GetExternalServer(ctx context.Context, tenantID, id string) (domain.ExternalServer, error)
	ListExternalServers(ctx context.Context, tenantID string, f ExternalServerFilter) ([]domain.ExternalServer, error)
	// CreateExternalServer returns domain.ErrNameConflict on a duplicate (scope, scope_id, name).
	CreateExternalServer(ctx context.Context, s domain.ExternalServer, events []domain.OutboxRecord) error
	// UpdateExternalServer rewrites spec, status and digests under optimistic
	// concurrency on s.Version and reconciles refs: kept refs keep their
	// broker pointer, new ones start unset, dropped ones are deleted.
	UpdateExternalServer(ctx context.Context, s domain.ExternalServer, events []domain.OutboxRecord) error
	SetSecretRef(ctx context.Context, tenantID, serverID string, ref domain.SecretRef, events []domain.OutboxRecord) error
	RecordProbe(ctx context.Context, tenantID, serverID string, r ProbeRecord, events []domain.OutboxRecord) error
	// ApplyReview returns domain.ErrDigestMismatch when approving a digest
	// other than the stored last probe digest.
	ApplyReview(ctx context.Context, r ReviewRecord, events []domain.OutboxRecord) (domain.ExternalServer, error)
	RecordHealth(ctx context.Context, tenantID, serverID string, h domain.Health, events []domain.OutboxRecord) error
	DeleteExternalServer(ctx context.Context, tenantID, id string, events []domain.OutboxRecord) error
	// ListServersByName returns every scope's server with one of the names.
	ListServersByName(ctx context.Context, tenantID string, names []string) ([]domain.ExternalServer, error)
	// ClaimHealthChecks atomically stamps and returns up to limit approved
	// http servers (across tenants) not checked since olderThan, so replicas
	// never probe the same server twice in one interval.
	ClaimHealthChecks(ctx context.Context, olderThan time.Time, limit int) ([]ServerKey, error)
}

// SecretBroker stores external server secrets in credential-broker
// (category mcp_external_secret). Plaintext only crosses this port.
type SecretBroker interface {
	Put(ctx context.Context, tenantID, ownerID string, value domain.SecretValue) error
	Get(ctx context.Context, tenantID, ownerID string) (domain.SecretValue, error)
	// Delete is idempotent: a missing secret is not an error.
	Delete(ctx context.Context, tenantID, ownerID string) error
}

// ProbeTarget is an already validated http endpoint plus its auth headers.
type ProbeTarget struct {
	URL     string
	Headers map[string]domain.SecretValue
}

type ProbeResult struct {
	Tools           []domain.ToolInfo
	ProtocolVersion string
}

// ToolProber lists the tools of a remote MCP server. Implementations must be
// SSRF-safe: the connection address is validated at dial time.
type ToolProber interface {
	ListTools(ctx context.Context, t ProbeTarget) (ProbeResult, error)
}

// CallTarget is an already validated http endpoint plus its auth headers.
type CallTarget struct {
	URL     string
	Headers map[string]domain.SecretValue
}

// CallResult is the text-only view of a tools/call result. Non-text content
// (image, audio, blob) is dropped; IsError is the external server's own flag.
type CallResult struct {
	Text      string
	IsError   bool
	Truncated bool
	SizeBytes int
}

type ResourceResult struct {
	Text      string
	MimeType  string
	Truncated bool
	SizeBytes int
}

// ToolCaller runs one read-only call on a remote MCP server. Implementations
// must be SSRF-safe and bound time and size. maxBytes caps the returned text.
type ToolCaller interface {
	CallTool(ctx context.Context, t CallTarget, tool string, argsJSON []byte, maxBytes int) (CallResult, error)
	ReadResource(ctx context.Context, t CallTarget, uri string, maxBytes int) (ResourceResult, error)
}

// ProfileMcpReader returns the mcp.servers[].name entries of the user's
// resolved profile (tenant-service) and the user's team ids. Everything else
// in the profile (inline command/args/env) is deliberately not exposed.
type ProfileMcpReader interface {
	ServerNames(ctx context.Context, userID string) ([]string, error)
	TeamIDs(ctx context.Context, userID string) ([]string, error)
}

// AgentTokenIssuer mints the short-lived Orca MCP token placed in an agent's
// config. The returned secret must never be logged.
type AgentTokenIssuer interface {
	Issue(ctx context.Context, userID, name string, scopes []string, ttl time.Duration) (domain.SecretValue, error)
}
