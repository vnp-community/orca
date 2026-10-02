package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/auth-service/internal/domain"
)

type idempotentAudit struct {
	rows map[string]domain.AuditEntry
	err  error
}

func (f *idempotentAudit) AppendIdempotent(_ context.Context, e domain.AuditEntry) error {
	if f.err != nil {
		return f.err
	}
	if f.rows == nil {
		f.rows = map[string]domain.AuditEntry{}
	}
	if _, dup := f.rows[e.ID]; !dup {
		f.rows[e.ID] = e
	}
	return nil
}

const (
	tOK = "11111111-1111-1111-1111-111111111111"
	uOK = "22222222-2222-2222-2222-222222222222"
	aOK = "33333333-3333-3333-3333-333333333333"
)

func validEvent() HandleMcpAuditEventInput {
	return HandleMcpAuditEventInput{TenantID: tOK, OccurredAt: time.Unix(1_700_000_000, 0), AuditID: aOK, ActorID: uOK, Action: "mcp.tool_call",
		ActorType: "agent", TargetType: "mcp_tool", TargetID: "terminal_send", Outcome: "allowed", Metadata: map[string]any{"decision": "approved"}}
}

func TestHandleMcpAuditEvent_IdempotentOnRedelivery(t *testing.T) {
	f := &idempotentAudit{}
	uc := NewHandleMcpAuditEvent(f)
	for i := 0; i < 3; i++ {
		if err := uc.Execute(context.Background(), validEvent()); err != nil {
			t.Fatal(err)
		}
	}
	got := f.rows[aOK]
	if len(f.rows) != 1 || got.ActorType != domain.ActorAgent || got.Action != "mcp.tool_call" || got.TargetID != "terminal_send" || got.Metadata["decision"] != "approved" {
		t.Fatalf("%+v", f.rows)
	}
}

func TestHandleMcpAuditEvent_PoisonIsReportedStorageErrorsRetry(t *testing.T) {
	uc := NewHandleMcpAuditEvent(&idempotentAudit{})
	mutate := map[string]func(*HandleMcpAuditEventInput){
		"bad audit id":       func(i *HandleMcpAuditEventInput) { i.AuditID = "nope" },
		"bad tenant":         func(i *HandleMcpAuditEventInput) { i.TenantID = "" },
		"non-mcp action":     func(i *HandleMcpAuditEventInput) { i.Action = "user.deactivated" },
		"bad actor type":     func(i *HandleMcpAuditEventInput) { i.ActorType = "root" },
		"empty actor type":   func(i *HandleMcpAuditEventInput) { i.ActorType = "" },
		"bad actor id":       func(i *HandleMcpAuditEventInput) { i.ActorID = "x" },
		"bad outcome":        func(i *HandleMcpAuditEventInput) { i.Outcome = "maybe" },
		"zero occurred time": func(i *HandleMcpAuditEventInput) { i.OccurredAt = time.Time{} },
	}
	for name, m := range mutate {
		in := validEvent()
		m(&in)
		if err := uc.Execute(context.Background(), in); !errors.Is(err, ErrPoisonMcpAuditEvent) {
			t.Errorf("%s: %v", name, err)
		}
	}
	boom := errors.New("db down")
	if err := NewHandleMcpAuditEvent(&idempotentAudit{err: boom}).Execute(context.Background(), validEvent()); !errors.Is(err, boom) {
		t.Fatalf("storage errors must propagate for redelivery: %v", err)
	}
}

func TestAppendAuditEntry_ActorTypeAndMetadata(t *testing.T) {
	audit := &fakeAuditRepository{}
	uc := NewAppendAuditEntry(audit, &fakeClock{now: time.Unix(0, 0)})
	err := uc.Execute(context.Background(), AppendAuditEntryInput{TenantID: "t1", Action: "mcp.x", ActorType: "agent", TargetType: "mcp_tool", TargetID: "x", MetadataJSON: `{"a":1}`})
	if err != nil || audit.entries[0].ActorType != domain.ActorAgent || audit.entries[0].Metadata["a"] != float64(1) || audit.entries[0].TargetID != "x" {
		t.Fatalf("%+v %v", audit.entries, err)
	}
	for _, in := range []AppendAuditEntryInput{
		{TenantID: "t1", Action: "a", ActorType: "root"}, {TenantID: "t1", Action: "a", MetadataJSON: "[1]"}, {TenantID: "t1", Action: "a", MetadataJSON: "{oops"},
	} {
		if err := uc.Execute(context.Background(), in); err == nil {
			t.Errorf("must reject %+v", in)
		}
	}
	// Legacy callers (no actor type) are stored as users.
	_ = uc.Execute(context.Background(), AppendAuditEntryInput{TenantID: "t1", Action: "project.update"})
	if audit.entries[len(audit.entries)-1].EffectiveActorType() != domain.ActorUser {
		t.Fatal("default actor type must be user")
	}
}

func TestAuditPageTokenRoundTrip(t *testing.T) {
	e := domain.AuditEntry{ID: "abc", OccurredAt: time.Date(2026, 10, 2, 1, 2, 3, 456789000, time.UTC)}
	at, id, err := domain.DecodeAuditKeyset(domain.EncodeAuditKeyset(e))
	if err != nil || !at.Equal(e.OccurredAt) || id != "abc" {
		t.Fatalf("%v %v %v", at, id, err)
	}
	for _, bad := range []string{"", "x", "|id", "2026-10-02T00:00:00Z|", "notatime|id"} {
		if _, _, err := domain.DecodeAuditKeyset(bad); err == nil {
			t.Errorf("%q must be rejected", bad)
		}
	}
}

func TestQueryAuditLog_RejectsUnknownMetadataKeysActorTypesAndBadTokens(t *testing.T) {
	users := newFakeUserRepository()
	seedActiveUser(t, users, fakeHasher{}, "admin1", "t1", "admin@example.com", "pw", domain.RoleAdmin)
	uc := NewQueryAuditLog(users, &fakeAuditRepository{}, &fakeOPAClient{allow: true})
	ctx := withActor(context.Background(), "t1", "admin1")
	bad := []QueryAuditLogInput{
		{TenantID: "t1", MetadataEquals: map[string]string{"actor_id'; DROP TABLE x;--": "1"}},
		{TenantID: "t1", MetadataEquals: map[string]string{"args_summary": "x"}},
		{TenantID: "t1", ActorType: "root"},
		{TenantID: "t1", NewestFirst: true, PageToken: "garbage"},
	}
	for i, in := range bad {
		if _, err := uc.Execute(ctx, in); err == nil {
			t.Errorf("case %d must be rejected", i)
		}
	}
	ok := QueryAuditLogInput{TenantID: "t1", ActorType: domain.ActorAgent, MetadataEquals: map[string]string{"decision": "denied", "client_id": "c"}, NewestFirst: true,
		PageToken: domain.EncodeAuditKeyset(domain.AuditEntry{ID: "x", OccurredAt: time.Now()})}
	if _, err := uc.Execute(ctx, ok); err != nil {
		t.Fatalf("allow-listed keys and a valid token must pass: %v", err)
	}
}
