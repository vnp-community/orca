// Package natsconsumer wires auth-service's HandleSSHConnectedEvent usecase
// to NATS JetStream via common/eventbus.Consumer — the audit-ingestion half
// of TASK-AUTH-05-08's "infra-fleet-service publishes, auth-service
// ingests" split. See auth-service.md's "own + ingested from other
// services' outbox streams" framing.
package natsconsumer

import (
	"context"
	"encoding/json"
	"log/slog"

	commoneventbus "github.com/stablyai/orca-go/common/eventbus"
	"github.com/stablyai/orca-go/services/auth-service/internal/usecase"
)

const (
	// InfraFleetStreamName is the JetStream stream infra-fleet-service's
	// outbox relay publishes to (see that service's cmd/server/main.go
	// EnsureStream call) — looked up, not created, by Subscribe below.
	InfraFleetStreamName = "INFRAFLEET"
	// SSHConnectedSubject mirrors infra-fleet-service's
	// usecase.SSHConnectedSubject exactly; kept as a separate constant
	// here (not imported) since auth-service must not depend on
	// infra-fleet-service's Go packages — services communicate only over
	// the wire, per architecture/03-clean-architecture-guidelines.md.
	SSHConnectedSubject = "orca.infrafleet.ssh.connected"
	// sshConnectAuditConsumerName is the DURABLE JetStream consumer name
	// this subscription uses. Durable (commoneventbus.Consumer.Subscribe),
	// not ephemeral (SubscribeEphemeral): an audit-log append must happen
	// exactly once cluster-wide, not once per auth-service replica — every
	// OTHER consumer in this codebase uses SubscribeEphemeral for
	// per-replica fan-out (see notification-service/internal/adapter/eventbus's
	// doc comment for that case), which is the wrong choice here. The name
	// must stay stable across restarts/deploys so JetStream resumes from
	// the last acked position instead of replaying the whole backlog.
	sshConnectAuditConsumerName = "auth-service-ssh-connect-audit"
)

// sshConnectedPayload is SSHConnectedSubject's JSON payload shape — must
// stay structurally compatible with infra-fleet-service's
// usecase.sshConnectedPayload. TenantID/OccurredAt are read off the
// eventbus.Event envelope instead (see HandleSSHConnectedEventInput's doc
// comment), not decoded from this struct.
type sshConnectedPayload struct {
	ActorUserID  string `json:"actor_user_id"`
	ConnectionID string `json:"connection_id"`
	Host         string `json:"host"`
}

// AuditIngestConsumer subscribes to infra-fleet-service's ssh.connect
// outbox stream and appends each event to auth-service's own audit log via
// HandleSSHConnectedEvent. Trust boundary: the NATS subject is only
// publishable by services inside the mesh (mTLS + default-deny
// NetworkPolicy, per architecture/07-security-architecture.md), so this
// consumer does not re-authenticate the event's claimed actor beyond what
// infra-fleet-service already validated when the connection was
// established.
type AuditIngestConsumer struct {
	handle *usecase.HandleSSHConnectedEvent
	logger *slog.Logger
}

// New constructs an AuditIngestConsumer. A nil logger falls back to
// slog.Default(), matching common/outbox.Relay's convention.
func New(handle *usecase.HandleSSHConnectedEvent, logger *slog.Logger) *AuditIngestConsumer {
	if logger == nil {
		logger = slog.Default()
	}
	return &AuditIngestConsumer{handle: handle, logger: logger}
}

// Run subscribes the durable consumer and blocks until ctx is cancelled or
// the subscription ends (e.g. the INFRAFLEET stream doesn't exist yet
// because infra-fleet-service hasn't started/registered it) — logged as a
// warning, not fatal to this process's startup, matching every other NATS
// consumer's graceful-degradation posture in this codebase.
func (c *AuditIngestConsumer) Run(ctx context.Context, bus *commoneventbus.Consumer) {
	if err := bus.Subscribe(ctx, InfraFleetStreamName, sshConnectAuditConsumerName, SSHConnectedSubject, c.handleEvent); err != nil {
		c.logger.WarnContext(ctx, "eventbus subject subscription ended",
			slog.String("stream", InfraFleetStreamName), slog.String("subject", SSHConnectedSubject), slog.Any("error", err))
	}
}

// handleEvent decodes one delivered ssh.connect event and hands it to
// HandleSSHConnectedEvent. A malformed payload is logged and dropped
// (returns nil, which Acks the message) rather than left for redelivery —
// a payload that fails to unmarshal will never succeed on retry, so NAK'ing
// it would only spin the consumer loop forever on a poison message.
func (c *AuditIngestConsumer) handleEvent(ctx context.Context, event commoneventbus.Event) error {
	var payload sshConnectedPayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		c.logger.WarnContext(ctx, "natsconsumer: malformed ssh.connect event payload, dropping",
			slog.String("event_id", event.ID), slog.Any("error", err))
		return nil
	}
	return c.handle.Execute(ctx, usecase.HandleSSHConnectedEventInput{
		TenantID:     event.TenantID,
		ActorUserID:  payload.ActorUserID,
		ConnectionID: payload.ConnectionID,
		Host:         payload.Host,
		OccurredAt:   event.OccurredAt,
	})
}
