package usecase

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/auth-service/internal/domain"
)

// HandleSSHConnectedEventInput is the decoded shape of infra-fleet-service's
// orca.infrafleet.ssh.connected outbox event (see that service's
// internal/usecase/establish_connection.go's sshConnectedPayload) plus the
// tenant/occurred-at fields carried on the eventbus.Event envelope rather
// than duplicated in the payload — internal/adapter/natsconsumer is
// responsible for assembling this from both.
type HandleSSHConnectedEventInput struct {
	TenantID     string
	ActorUserID  string // may be empty — see domain.AuditEntry's doc comment
	ConnectionID string
	Host         string
	OccurredAt   time.Time
}

// HandleSSHConnectedEvent appends an "ssh.connect" entry to auth-service's
// own audit log for an SSH connection established by infra-fleet-service —
// per auth-service.md's "own + ingested from other services' outbox
// streams" framing (TASK-AUTH-05-08). Constructor-injects AuditRepository
// only, mirroring NewQueryAuditLog's narrow-port style — this usecase has
// no authorization check of its own (see
// internal/adapter/natsconsumer/audit_ingest.go's doc comment on the trust
// boundary: only services inside the mesh can publish this subject).
type HandleSSHConnectedEvent struct {
	audit AuditRepository
}

func NewHandleSSHConnectedEvent(audit AuditRepository) *HandleSSHConnectedEvent {
	return &HandleSSHConnectedEvent{audit: audit}
}

func (uc *HandleSSHConnectedEvent) Execute(ctx context.Context, in HandleSSHConnectedEventInput) error {
	entry, err := domain.NewAuditEntry(
		uuid.NewString(), in.TenantID, in.ActorUserID,
		"ssh.connect", "ssh_host", in.Host,
		map[string]any{"connectionId": in.ConnectionID}, "", in.OccurredAt,
	)
	if err != nil {
		return apperrors.New(apperrors.KindInvalidArgument, "AUTH_SSH_CONNECTED_EVENT_INVALID", "failed to construct audit entry for ssh.connect event", err)
	}
	if err := uc.audit.Append(ctx, entry); err != nil {
		return apperrors.New(apperrors.KindInternal, "AUTH_SSH_CONNECTED_AUDIT_APPEND_FAILED", "failed to append ssh.connect audit entry", err)
	}
	return nil
}
