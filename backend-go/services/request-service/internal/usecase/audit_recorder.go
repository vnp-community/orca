package usecase

import (
	"context"
	"log/slog"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/common/auditclient"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// AuditEvent is what a use case reports. Metadata carries ids, kinds and counts, never content.
type AuditEvent struct {
	Action      string
	TargetType  string
	TargetID    string
	Outcome     string // allowed | denied
	RequestID   string
	SubjectType string // approvals only: decides whether approval.approve is durable
	Metadata    map[string]any
}

// BestEffortAudit sends one entry to auth-service and swallows failures (auditclient.Client).
type BestEffortAudit interface {
	AppendDetailed(ctx context.Context, e auditclient.Entry)
}

// AuditRecorder writes audit entries: durable ones through the outbox in the caller's transaction,
// the rest straight to auth-service, which may drop them when it is down (CR-REQ-035 2.5).
type AuditRecorder struct {
	sink   BestEffortAudit
	outbox AuditOutboxStore
	tx     TxRunner
	log    *slog.Logger
}

func NewAuditRecorder(sink BestEffortAudit, outbox AuditOutboxStore, tx TxRunner, log *slog.Logger) *AuditRecorder {
	if log == nil {
		log = slog.Default()
	}
	return &AuditRecorder{sink: sink, outbox: outbox, tx: tx, log: log}
}

// Record routes the event; a durable action that cannot be queued returns the error so the caller can abort.
func (r *AuditRecorder) Record(ctx context.Context, e AuditEvent) error {
	if domain.IsDurableAudit(e.Action, e.SubjectType) {
		return r.RecordDurable(ctx, e)
	}
	entry, err := r.entry(ctx, e, "")
	if err != nil {
		r.log.WarnContext(ctx, "audit event dropped", slog.String("action", e.Action), slog.Any("error", err))
		return nil // a malformed best-effort event must not fail the action it describes.
	}
	if r.sink != nil {
		r.sink.AppendDetailed(ctx, entry)
	}
	return nil
}

// RecordDurable queues the entry in the outbox; inside InTx it commits atomically with the change.
func (r *AuditRecorder) RecordDurable(ctx context.Context, e AuditEvent) error {
	auditID := uuid.NewString()
	entry, err := r.entry(ctx, e, auditID)
	if err != nil {
		return err
	}
	rec := AuditOutboxRecord{
		ID: uuid.NewString(), AuditID: auditID, TenantID: entry.TenantID, Action: entry.Action, ActorID: entry.ActorID,
		ActorType: entry.ActorType, TargetType: entry.TargetType, TargetID: entry.TargetID, Outcome: entry.Outcome,
		IPAddress: entry.IPAddress, MetadataJSON: entry.MetadataJSON,
	}
	return r.tx.InTx(ctx, func(ctx context.Context) error { return r.outbox.Enqueue(ctx, rec) })
}

// RecordDenied implements AccessAuditor for the access interceptor.
func (r *AuditRecorder) RecordDenied(ctx context.Context, d DeniedAccess) {
	_ = r.Record(ctx, AuditEvent{
		Action: domain.AuditRequestAccessDenied, TargetType: "request", TargetID: d.RequestID, Outcome: "denied", RequestID: d.RequestID,
		Metadata: map[string]any{"rpc": d.RPC, "actor_type": d.ActorType},
	})
}

func (r *AuditRecorder) entry(ctx context.Context, e AuditEvent, auditID string) (auditclient.Entry, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return auditclient.Entry{}, domain.ErrRequestTenantRequired()
	}
	meta, err := domain.MarshalAuditMetadata(e.RequestID, auditID, e.Metadata)
	if err != nil {
		return auditclient.Entry{}, err
	}
	actor, _ := tenant.UserID(ctx)
	if actor == "" {
		actor = "system"
	}
	ip, _ := tenant.ClientIP(ctx)
	outcome := e.Outcome
	if outcome == "" {
		outcome = "allowed"
	}
	target := e.TargetType
	if e.TargetID != "" {
		target += ":" + e.TargetID
	}
	return auditclient.Entry{
		TenantID: tenantID, ActorID: actor, ActorType: tenant.ActorType(ctx), Action: e.Action, Target: target,
		TargetType: e.TargetType, TargetID: e.TargetID, Outcome: outcome, IPAddress: ip, MetadataJSON: meta,
	}, nil
}
