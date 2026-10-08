package usecase

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// AuditTap turns committed Request and Approval events into audit entries (CR-REQ-024 section 2.8).
// Deriving them from events gives one entry per decision whichever path made it (RPC, sweeper, handler),
// and the payloads it reads never carry title, body or Solution content.
type AuditTap struct {
	Recorder RPCAuditRecorder
	Log      *slog.Logger
}

var _ OutboxTap = (*AuditTap)(nil)

// auditFields is the union of the payload keys the tap reads.
type auditFields struct {
	RequestID       string `json:"request_id"`
	ApprovalID      string `json:"approval_id"`
	SourceProvider  string `json:"source_provider"`
	Type            string `json:"type"`
	TypeSource      string `json:"type_source"`
	From            string `json:"from"`
	To              string `json:"to"`
	Trigger         string `json:"trigger"`
	Stage           string `json:"stage"`
	Decision        string `json:"decision"`
	SubjectType     string `json:"subject_type"`
	ActorID         string `json:"actor_id"`
	ActorKind       string `json:"actor_kind"`
	DecidedBy       string `json:"decided_by"`
	ParentRequestID string `json:"parent_request_id"`
}

func (t *AuditTap) OnEvent(ctx context.Context, ev domain.OutboxEvent) {
	if t.Recorder == nil {
		return
	}
	var f auditFields
	if err := json.Unmarshal(ev.Payload, &f); err != nil {
		t.warn(ctx, ev.Subject, err)
		return
	}
	out, ok := t.event(ctx, ev, f)
	if !ok {
		return
	}
	out.TenantID = ev.TenantID
	out.Outcome = domain.AuditOutcomeAllowed
	t.Recorder.Record(ctx, out)
}

func (t *AuditTap) warn(ctx context.Context, subject string, err error) {
	if t.Log != nil {
		t.Log.WarnContext(ctx, "audit tap: unreadable event payload", slog.String("subject", subject), slog.Any("error", err))
	}
}

func (t *AuditTap) event(ctx context.Context, ev domain.OutboxEvent, f auditFields) (RPCAuditEvent, bool) {
	req := func(action string, actorID string, kind domain.AuditActorKind, md map[string]string) (RPCAuditEvent, bool) {
		return RPCAuditEvent{ActorID: actorID, ActorKind: kind, Action: action, TargetType: "request", TargetID: f.RequestID, Metadata: md}, f.RequestID != ""
	}
	switch ev.Subject {
	case domain.SubjectRequestCreated:
		actor, kind := ctxActor(ctx)
		if f.SourceProvider == string(domain.SourceProviderMCP) {
			kind = domain.AuditActorAgent
		}
		return req(domain.ActionRequestCreate, actor, kind, map[string]string{"source_provider": f.SourceProvider})
	case domain.SubjectRequestTypeConfirmed:
		return req(domain.ActionRequestTypeConfirm, f.ActorID, domain.AuditActorUser, map[string]string{"type": f.Type, "type_source": f.TypeSource})
	case domain.SubjectRequestTypeChanged:
		actor, kind := ctxActor(ctx)
		if f.ActorID != "" {
			actor = f.ActorID
		}
		return req(domain.ActionRequestTypeChange, actor, kind, map[string]string{"from": f.From, "to": f.To})
	case domain.SubjectRequestReturned:
		actor, kind := ctxActor(ctx)
		if f.ActorID != "" {
			actor = f.ActorID
		} else {
			kind = domain.AuditActorSystem
		}
		return req(domain.ActionRequestReturn, actor, kind, map[string]string{"returned_from_stage": f.Stage})
	case domain.SubjectRequestStatusChanged:
		switch domain.Trigger(f.Trigger) {
		case domain.TriggerReopen:
			return req(domain.ActionRequestReopen, f.ActorID, domain.AuditActorUser, nil)
		case domain.TriggerCancel:
			return req(domain.ActionRequestCancel, f.ActorID, auditKind(f.ActorKind), nil)
		}
	case domain.SubjectApprovalDecided:
		return approvalAudit(ctx, f)
	}
	return RPCAuditEvent{}, false
}

func approvalAudit(ctx context.Context, f auditFields) (RPCAuditEvent, bool) {
	action := map[string]string{"approved": domain.ActionApprovalApprove, "rejected": domain.ActionApprovalReject,
		"cancelled": domain.ActionApprovalCancel, "expired": domain.ActionApprovalExpire}[f.Decision]
	if action == "" || f.ApprovalID == "" {
		return RPCAuditEvent{}, false
	}
	md := map[string]string{"subject_type": f.SubjectType}
	if f.Stage != "" {
		md["stage"] = f.Stage
	}
	if action == domain.ActionApprovalExpire {
		return RPCAuditEvent{ActorKind: domain.AuditActorSystem, Action: action, TargetType: "approval", TargetID: f.ApprovalID, Metadata: md}, true
	}
	actor, kind := ctxActor(ctx)
	if f.DecidedBy != "" {
		actor = f.DecidedBy
	}
	return RPCAuditEvent{ActorID: actor, ActorKind: kind, Action: action, TargetType: "approval", TargetID: f.ApprovalID, Metadata: md}, true
}

// ctxActor reads the caller from the request context; a context with no user is the system.
func ctxActor(ctx context.Context) (string, domain.AuditActorKind) {
	id, _ := tenant.UserID(ctx)
	if id == "" {
		return "", domain.AuditActorSystem
	}
	return id, auditKind(tenant.ActorType(ctx))
}

// auditKind maps ActorKind values (user|ai|agent|system) to the audit_log actor_type enum.
func auditKind(k string) domain.AuditActorKind {
	switch k {
	case "agent", "ai":
		return domain.AuditActorAgent
	case "system":
		return domain.AuditActorSystem
	}
	return domain.AuditActorUser
}
