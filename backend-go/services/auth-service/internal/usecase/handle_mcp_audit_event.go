package usecase

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/services/auth-service/internal/domain"
)

// ErrPoisonMcpAuditEvent marks an event that can never succeed on retry; the
// consumer acks and drops it instead of looping on redelivery.
var ErrPoisonMcpAuditEvent = errors.New("usecase: malformed mcp audit event")

// HandleMcpAuditEventInput is the decoded orca.mcp.audit.appended payload.
// TenantID and OccurredAt come from the event envelope.
type HandleMcpAuditEventInput struct {
	TenantID   string
	OccurredAt time.Time

	AuditID    string
	ActorID    string
	Action     string
	ActorType  string
	TargetType string
	TargetID   string
	Outcome    string
	IPAddress  string
	Metadata   map[string]any
}

// HandleMcpAuditEvent appends one MCP audit event to auth.audit_log. The row
// id is the event's audit_id, so a redelivered event is a no-op.
type HandleMcpAuditEvent struct{ audit IdempotentAuditAppender }

func NewHandleMcpAuditEvent(audit IdempotentAuditAppender) *HandleMcpAuditEvent {
	return &HandleMcpAuditEvent{audit: audit}
}

func (uc *HandleMcpAuditEvent) Execute(ctx context.Context, in HandleMcpAuditEventInput) error {
	if _, err := uuid.Parse(in.AuditID); err != nil {
		return ErrPoisonMcpAuditEvent
	}
	if _, err := uuid.Parse(in.TenantID); err != nil {
		return ErrPoisonMcpAuditEvent
	}
	// Only MCP actions may enter through this stream: it must not become a
	// way to forge arbitrary security events.
	if !strings.HasPrefix(in.Action, "mcp.") {
		return ErrPoisonMcpAuditEvent
	}
	actor := domain.ActorType(in.ActorType)
	if !actor.Valid() {
		return ErrPoisonMcpAuditEvent
	}
	if in.ActorID != "" {
		if _, err := uuid.Parse(in.ActorID); err != nil {
			return ErrPoisonMcpAuditEvent
		}
	}
	entry, err := domain.NewAuditEntry(in.AuditID, in.TenantID, in.ActorID, in.Action, "", in.TargetType, in.TargetID,
		in.Metadata, domain.Outcome(in.Outcome), in.IPAddress, in.OccurredAt)
	if err != nil {
		return ErrPoisonMcpAuditEvent
	}
	return uc.audit.AppendIdempotent(ctx, entry.WithActorType(actor))
}
