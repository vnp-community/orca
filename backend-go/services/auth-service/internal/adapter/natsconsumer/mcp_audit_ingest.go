package natsconsumer

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"

	commoneventbus "github.com/stablyai/orca-go/common/eventbus"
	"github.com/stablyai/orca-go/services/auth-service/internal/usecase"
)

const (
	// McpStreamName / McpAuditSubject mirror mcp-service's outbox (looked up,
	// not created, here; auth-service must not import mcp-service packages).
	McpStreamName   = "MCP"
	McpAuditSubject = "orca.mcp.audit.appended"
	// mcpAuditConsumerName is DURABLE: an audit append must happen once
	// cluster-wide (not once per replica), and a stable name resumes from the
	// last ack across restarts. Redelivery is harmless: rows are idempotent by id.
	mcpAuditConsumerName = "auth-service-mcp-audit"
)

type mcpAuditPayload struct {
	AuditID    string         `json:"audit_id"`
	ActorID    string         `json:"actor_id"`
	Action     string         `json:"action"`
	ActorType  string         `json:"actor_type"`
	TargetType string         `json:"target_type"`
	TargetID   string         `json:"target_id"`
	Outcome    string         `json:"outcome"`
	IP         string         `json:"ip"`
	Metadata   map[string]any `json:"metadata"`
}

// McpAuditIngestConsumer ingests orca.mcp.audit.appended into the audit log.
type McpAuditIngestConsumer struct {
	handle *usecase.HandleMcpAuditEvent
	logger *slog.Logger
}

func NewMcpAudit(handle *usecase.HandleMcpAuditEvent, logger *slog.Logger) *McpAuditIngestConsumer {
	if logger == nil {
		logger = slog.Default()
	}
	return &McpAuditIngestConsumer{handle: handle, logger: logger}
}

func (c *McpAuditIngestConsumer) Run(ctx context.Context, bus *commoneventbus.Consumer) {
	if err := bus.Subscribe(ctx, McpStreamName, mcpAuditConsumerName, McpAuditSubject, c.HandleEvent); err != nil {
		c.logger.WarnContext(ctx, "mcp audit subscription ended", slog.String("stream", McpStreamName), slog.Any("error", err))
	}
}

// HandleEvent: a payload that can never succeed is logged and acked; any other
// error is returned so the message is redelivered.
func (c *McpAuditIngestConsumer) HandleEvent(ctx context.Context, event commoneventbus.Event) error {
	var p mcpAuditPayload
	if err := json.Unmarshal(event.Payload, &p); err != nil {
		c.logger.WarnContext(ctx, "natsconsumer: malformed mcp audit payload, dropping", slog.String("event_id", event.ID), slog.Any("error", err))
		return nil
	}
	err := c.handle.Execute(ctx, usecase.HandleMcpAuditEventInput{
		TenantID: event.TenantID, OccurredAt: event.OccurredAt, AuditID: p.AuditID, ActorID: p.ActorID, Action: p.Action,
		ActorType: p.ActorType, TargetType: p.TargetType, TargetID: p.TargetID, Outcome: p.Outcome, IPAddress: p.IP, Metadata: p.Metadata,
	})
	if errors.Is(err, usecase.ErrPoisonMcpAuditEvent) {
		c.logger.WarnContext(ctx, "natsconsumer: invalid mcp audit event, dropping", slog.String("event_id", event.ID))
		return nil
	}
	return err
}
