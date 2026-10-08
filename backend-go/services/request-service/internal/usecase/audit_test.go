package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

type captureAudit struct {
	mu     sync.Mutex
	events []RPCAuditEvent
}

func (c *captureAudit) Record(_ context.Context, e RPCAuditEvent) {
	c.mu.Lock()
	c.events = append(c.events, e)
	c.mu.Unlock()
}

func auditCtx(user, actorType string) context.Context {
	ctx := tenant.WithTenantID(context.Background(), "t-1")
	if user != "" {
		ctx = tenant.WithUserID(ctx, user)
	}
	if actorType != "" {
		ctx = tenant.WithActorType(ctx, actorType)
	}
	return ctx
}

func eventOf(t *testing.T, subject string, payload map[string]any) domain.OutboxEvent {
	t.Helper()
	b, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	return domain.OutboxEvent{TenantID: "t-1", Subject: subject, Payload: b}
}

// One row per audited action of CR-REQ-024 section 2.8 that is derived from an event.
func TestAudit_EveryEventDerivedActionWritesExactlyOneEntry(t *testing.T) {
	cases := []struct {
		name       string
		ctx        context.Context
		subject    string
		payload    map[string]any
		action     string
		actorType  domain.AuditActorKind
		targetType string
		targetID   string
		metadata   map[string]string
	}{
		{"create by user", auditCtx("u1", ""), domain.SubjectRequestCreated, map[string]any{"request_id": "r1", "source_provider": "jira", "title": "secret title"},
			domain.ActionRequestCreate, domain.AuditActorUser, "request", "r1", map[string]string{"source_provider": "jira"}},
		{"create from mcp is an agent", auditCtx("u1", "agent"), domain.SubjectRequestCreated, map[string]any{"request_id": "r1", "source_provider": "mcp", "title": "x"},
			domain.ActionRequestCreate, domain.AuditActorAgent, "request", "r1", map[string]string{"source_provider": "mcp"}},
		{"confirm type", auditCtx("u1", ""), domain.SubjectRequestTypeConfirmed, map[string]any{"request_id": "r1", "type": "bug", "type_source": "ai", "actor_id": "u1"},
			domain.ActionRequestTypeConfirm, domain.AuditActorUser, "request", "r1", map[string]string{"type": "bug", "type_source": "ai"}},
		{"change type", auditCtx("u1", ""), domain.SubjectRequestTypeChanged, map[string]any{"request_id": "r1", "from": "bug", "to": "change_request", "actor_id": "u1", "reason": "free text"},
			domain.ActionRequestTypeChange, domain.AuditActorUser, "request", "r1", map[string]string{"from": "bug", "to": "change_request"}},
		{"change type by agent", auditCtx("u1", "agent"), domain.SubjectRequestTypeChanged, map[string]any{"request_id": "r1", "from": "bug", "to": "task", "actor_id": "u1"},
			domain.ActionRequestTypeChange, domain.AuditActorAgent, "request", "r1", map[string]string{"from": "bug", "to": "task"}},
		{"return by user", auditCtx("u1", ""), domain.SubjectRequestReturned, map[string]any{"request_id": "r1", "stage": "plan", "actor_id": "u1", "reason": "free text"},
			domain.ActionRequestReturn, domain.AuditActorUser, "request", "r1", map[string]string{"returned_from_stage": "plan"}},
		{"return by the system", auditCtx("", ""), domain.SubjectRequestReturned, map[string]any{"request_id": "r1", "stage": "task", "actor_id": ""},
			domain.ActionRequestReturn, domain.AuditActorSystem, "request", "r1", map[string]string{"returned_from_stage": "task"}},
		{"reopen", auditCtx("u1", ""), domain.SubjectRequestStatusChanged, map[string]any{"request_id": "r1", "trigger": "reopen", "actor_id": "u1", "actor_kind": "user"},
			domain.ActionRequestReopen, domain.AuditActorUser, "request", "r1", nil},
		{"cancel", auditCtx("u1", ""), domain.SubjectRequestStatusChanged, map[string]any{"request_id": "r1", "trigger": "cancel", "actor_id": "u1", "actor_kind": "user", "reason": "free text"},
			domain.ActionRequestCancel, domain.AuditActorUser, "request", "r1", nil},
		{"approve", auditCtx("u2", ""), domain.SubjectApprovalDecided, map[string]any{"approval_id": "a1", "decision": "approved", "decided_by": "u2", "subject_type": "plan", "stage": "awaiting_plan_approval"},
			domain.ActionApprovalApprove, domain.AuditActorUser, "approval", "a1", map[string]string{"subject_type": "plan", "stage": "awaiting_plan_approval"}},
		{"reject", auditCtx("u2", ""), domain.SubjectApprovalDecided, map[string]any{"approval_id": "a1", "decision": "rejected", "decided_by": "u2", "subject_type": "solution"},
			domain.ActionApprovalReject, domain.AuditActorUser, "approval", "a1", map[string]string{"subject_type": "solution"}},
		{"cancel approval", auditCtx("u2", ""), domain.SubjectApprovalDecided, map[string]any{"approval_id": "a1", "decision": "cancelled", "decided_by": "u2", "subject_type": "phase"},
			domain.ActionApprovalCancel, domain.AuditActorUser, "approval", "a1", map[string]string{"subject_type": "phase"}},
		{"expire is the system", auditCtx("", ""), domain.SubjectApprovalDecided, map[string]any{"approval_id": "a1", "decision": "expired", "subject_type": "plan"},
			domain.ActionApprovalExpire, domain.AuditActorSystem, "approval", "a1", map[string]string{"subject_type": "plan"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := &captureAudit{}
			(&AuditTap{Recorder: rec}).OnEvent(tc.ctx, eventOf(t, tc.subject, tc.payload))
			if len(rec.events) != 1 {
				t.Fatalf("%d audit entries, want exactly 1", len(rec.events))
			}
			got := rec.events[0]
			if got.Action != tc.action || got.ActorKind != tc.actorType || got.TargetType != tc.targetType || got.TargetID != tc.targetID {
				t.Fatalf("got %+v", got)
			}
			if got.Outcome != domain.AuditOutcomeAllowed || got.TenantID != "t-1" {
				t.Fatalf("outcome/tenant: %+v", got)
			}
			for k, v := range tc.metadata {
				if got.Metadata[k] != v {
					t.Errorf("metadata[%s] = %q, want %q", k, got.Metadata[k], v)
				}
			}
			for k := range got.Metadata {
				if _, want := tc.metadata[k]; !want {
					t.Errorf("unexpected metadata key %q", k)
				}
			}
			if strings.Contains(got.Target(), " ") || got.Target() != tc.targetType+":"+tc.targetID {
				t.Errorf("target %q", got.Target())
			}
		})
	}
}

func TestAudit_NeverCarriesTitleOrBody(t *testing.T) {
	rec := &captureAudit{}
	tap := &AuditTap{Recorder: rec}
	tap.OnEvent(auditCtx("u1", ""), eventOf(t, domain.SubjectRequestCreated, map[string]any{
		"request_id": "r1", "source_provider": "manual", "title": "TITLE-MARKER", "body": "BODY-MARKER", "reason": "REASON-MARKER"}))
	tap.OnEvent(auditCtx("u1", ""), eventOf(t, domain.SubjectRequestStatusChanged, map[string]any{
		"request_id": "r1", "trigger": "cancel", "actor_id": "u1", "reason": "REASON-MARKER", "title": "TITLE-MARKER"}))
	b, _ := json.Marshal(rec.events)
	for _, marker := range []string{"TITLE-MARKER", "BODY-MARKER", "REASON-MARKER"} {
		if strings.Contains(string(b), marker) {
			t.Fatalf("audit entries leak %s: %s", marker, b)
		}
	}
}

func TestAudit_IgnoresEventsThatAreNotDecisions(t *testing.T) {
	rec := &captureAudit{}
	tap := &AuditTap{Recorder: rec}
	ctx := auditCtx("u1", "")
	tap.OnEvent(ctx, eventOf(t, domain.SubjectRequestStatusChanged, map[string]any{"request_id": "r1", "trigger": "start_classification"}))
	tap.OnEvent(ctx, eventOf(t, domain.SubjectRequestClassified, map[string]any{"request_id": "r1"}))
	tap.OnEvent(ctx, eventOf(t, domain.SubjectApprovalRequested, map[string]any{"approval_id": "a1"}))
	tap.OnEvent(ctx, eventOf(t, domain.SubjectApprovalDecided, map[string]any{"approval_id": "a1", "decision": "unknown"}))
	tap.OnEvent(ctx, domain.OutboxEvent{Subject: domain.SubjectRequestCreated, Payload: []byte("not json")})
	if len(rec.events) != 0 {
		t.Fatalf("unexpected entries: %+v", rec.events)
	}
}

type fakeTxScope struct{ active bool }

func (f *fakeTxScope) InTx(ctx context.Context, fn func(context.Context) error) error {
	f.active = true
	defer func() { f.active = false }()
	return fn(ctx)
}
func (f *fakeTxScope) InTransaction(context.Context) bool { return f.active }

type memOutbox struct{ events []domain.OutboxEvent }

func (m *memOutbox) InsertOutboxEvent(_ context.Context, ev domain.OutboxEvent) error {
	m.events = append(m.events, ev)
	return nil
}

// A rolled-back transaction must leave no audit entry for a decision that never happened.
func TestAudit_OnlyAfterCommit(t *testing.T) {
	rec := &captureAudit{}
	tx := CommitHooks{Inner: &fakeTxScope{}}
	out := &TappedOutbox{Inner: &memOutbox{}, Taps: []OutboxTap{&AuditTap{Recorder: rec}}}
	ev := eventOf(t, domain.SubjectRequestStatusChanged, map[string]any{"request_id": "r1", "trigger": "cancel", "actor_id": "u1"})

	err := tx.InTx(auditCtx("u1", ""), func(ctx context.Context) error {
		if err := out.InsertOutboxEvent(ctx, ev); err != nil {
			return err
		}
		if len(rec.events) != 0 {
			t.Error("audit written before commit")
		}
		return errors.New("boom")
	})
	if err == nil || len(rec.events) != 0 {
		t.Fatalf("rolled back: err=%v entries=%d", err, len(rec.events))
	}

	err = tx.InTx(auditCtx("u1", ""), func(ctx context.Context) error {
		// A nested transaction joins the outer one: still nothing until the outermost commit.
		return tx.InTx(ctx, func(ctx context.Context) error { return out.InsertOutboxEvent(ctx, ev) })
	})
	if err != nil || len(rec.events) != 1 {
		t.Fatalf("committed: err=%v entries=%d", err, len(rec.events))
	}
}

func TestAfterCommit_RunsImmediatelyOutsideATransaction(t *testing.T) {
	ran := false
	AfterCommit(context.Background(), func() { ran = true })
	if !ran {
		t.Fatal("callback outside a transaction must run at once")
	}
}
