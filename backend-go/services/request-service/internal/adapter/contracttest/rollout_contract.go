package contracttest

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

// RolloutEnv is the adapter set behind CR-REQ-024/025: the flag row, the active-source lookup and the gauge counts.
type RolloutEnv struct {
	ApprovalEnv
	Flow    usecase.FlowSettingsRepository
	Finder  usecase.ActiveSourceFinder
	Samples usecase.MetricsSampleSource
	// AgeRequest moves requests.updated_at into the past (the repository always stamps now).
	AgeRequest func(t *testing.T, requestID string, age time.Duration)
}

// RunRolloutContract covers TASK-REQ-025-01 (tenant_settings), 024-05 (lookup query) and 024-07 (gauge counts).
func RunRolloutContract(t *testing.T, newEnv func(t *testing.T) RolloutEnv) {
	env := newEnv(t)
	scenarios := []struct {
		name string
		fn   func(t *testing.T, env RolloutEnv)
	}{
		{"FlowSettingsMissingRowMeansNotSet", flowMissingRow},
		{"FlowSettingsUpsertReadAndOverwrite", flowUpsertOverwrite},
		{"FlowSettingsTenantIsolation", flowTenantIsolation},
		{"FlowSettingsRequireTenant", flowRequiresTenant},
		{"LookupMatchesSiteOrAnySite", lookupSiteMatching},
		{"LookupIgnoresFinishedRequestsAndPicksNewestActive", lookupActiveOnly},
		{"LookupTenantIsolation", lookupTenantIsolation},
		{"SamplesCountAcrossTenants", samplesAcrossTenants},
		{"AuditAndMetricsFireOnlyAfterCommit", auditOnlyAfterCommit},
	}
	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) { s.fn(t, env) })
	}
}

func flowMissingRow(t *testing.T, env RolloutEnv) {
	s, ok, err := env.Flow.Get(CtxForTenant(newTenant()))
	if err != nil || ok || s.Enabled {
		t.Fatalf("got (%+v, %v, %v), want no row", s, ok, err)
	}
}

func flowUpsertOverwrite(t *testing.T, env RolloutEnv) {
	tenantID := newTenant()
	ctx := CtxForTenant(tenantID)
	for _, enabled := range []bool{true, false, true} {
		if err := env.Flow.Upsert(ctx, enabled, "admin-1"); err != nil {
			t.Fatal(err)
		}
		s, ok, err := env.Flow.Get(ctx)
		if err != nil || !ok || s.Enabled != enabled || s.TenantID != tenantID {
			t.Fatalf("after Upsert(%v): (%+v, %v, %v)", enabled, s, ok, err)
		}
	}
}

func flowTenantIsolation(t *testing.T, env RolloutEnv) {
	a, b := newTenant(), newTenant()
	if err := env.Flow.Upsert(CtxForTenant(a), true, "admin-a"); err != nil {
		t.Fatal(err)
	}
	if s, ok, err := env.Flow.Get(CtxForTenant(b)); err != nil || ok || s.Enabled {
		t.Fatalf("tenant B sees tenant A's flag: (%+v, %v, %v)", s, ok, err)
	}
	if err := env.Flow.Upsert(CtxForTenant(b), false, "admin-b"); err != nil {
		t.Fatal(err)
	}
	if s, _, _ := env.Flow.Get(CtxForTenant(a)); !s.Enabled {
		t.Fatal("tenant B's write changed tenant A")
	}
}

func flowRequiresTenant(t *testing.T, env RolloutEnv) {
	if _, _, err := env.Flow.Get(context.Background()); err == nil {
		t.Error("Get without a tenant must fail, not return an empty row")
	}
	if err := env.Flow.Upsert(context.Background(), true, "x"); err == nil {
		t.Error("Upsert without a tenant must fail")
	}
}

func jiraRequest(t *testing.T, env RolloutEnv, ctx context.Context, site, ref string, status domain.RequestStatus) domain.Request {
	t.Helper()
	return createRequest(t, env.Env, ctx, func(r *domain.Request) {
		r.SourceProvider, r.SourceSite, r.SourceRef = domain.SourceProviderJira, site, ref
		r.Status = status
	})
}

func lookupSiteMatching(t *testing.T, env RolloutEnv) {
	ctx := CtxForTenant(newTenant())
	r := jiraRequest(t, env, ctx, "https://a.atlassian.net", "ENG-1", domain.RequestStatusExecuting)
	for _, tc := range []struct {
		name, site, ref string
		found           bool
	}{
		{"exact site", "https://a.atlassian.net", "ENG-1", true},
		{"empty site matches any", "", "ENG-1", true},
		{"other site", "https://b.atlassian.net", "ENG-1", false},
		{"other ref", "https://a.atlassian.net", "ENG-2", false},
	} {
		id, ok, err := env.Finder.FindActiveBySource(ctx, "jira", tc.site, tc.ref)
		if err != nil || ok != tc.found || (ok && id != r.ID) {
			t.Errorf("%s: (%q, %v, %v), want found=%v id=%s", tc.name, id, ok, err, tc.found, r.ID)
		}
	}
	if _, ok, err := env.Finder.FindActiveBySource(ctx, "github", "", "ENG-1"); err != nil || ok {
		t.Errorf("a github lookup must not match a jira Request: %v %v", ok, err)
	}
}

func lookupActiveOnly(t *testing.T, env RolloutEnv) {
	ctx := CtxForTenant(newTenant())
	site := "https://a.atlassian.net"
	finished := jiraRequest(t, env, ctx, site, "ENG-5", domain.RequestStatusCompleted)
	if _, ok, err := env.Finder.FindActiveBySource(ctx, "jira", site, "ENG-5"); err != nil || ok {
		t.Fatalf("completed Request %s must not count as owning the issue: %v %v", finished.ID, ok, err)
	}
	cancelled := jiraRequest(t, env, ctx, site, "ENG-6", domain.RequestStatusCancelled)
	if _, ok, _ := env.Finder.FindActiveBySource(ctx, "jira", site, "ENG-6"); ok {
		t.Fatalf("cancelled Request %s must not count", cancelled.ID)
	}
	_ = jiraRequest(t, env, ctx, site, "ENG-7", domain.RequestStatusExecuting)
	time.Sleep(20 * time.Millisecond) // created_at decides which one is newest
	newer := jiraRequest(t, env, ctx, site, "ENG-7", domain.RequestStatusPlanning)
	got, ok, err := env.Finder.FindActiveBySource(ctx, "jira", site, "ENG-7")
	if err != nil || !ok || got != newer.ID {
		t.Fatalf("want the newest active Request %s, got (%q, %v, %v)", newer.ID, got, ok, err)
	}
}

func lookupTenantIsolation(t *testing.T, env RolloutEnv) {
	a, b := CtxForTenant(newTenant()), CtxForTenant(newTenant())
	jiraRequest(t, env, a, "https://a.atlassian.net", "ENG-9", domain.RequestStatusExecuting)
	if _, ok, err := env.Finder.FindActiveBySource(b, "jira", "", "ENG-9"); err != nil || ok {
		t.Fatalf("tenant B found tenant A's Request: %v %v", ok, err)
	}
	if _, _, err := env.Finder.FindActiveBySource(context.Background(), "jira", "", "ENG-9"); err == nil {
		t.Fatal("a lookup without a tenant must fail")
	}
}

func samplesAcrossTenants(t *testing.T, env RolloutEnv) {
	ctx := context.Background()
	stuckBefore, err := env.Samples.CountStuck(ctx, "classifying", time.Now().Add(-time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	outboxBefore, err := env.Samples.CountOutboxPending(ctx)
	if err != nil {
		t.Fatal(err)
	}
	approvalsBefore, err := env.Samples.PendingApprovalsBySubject(ctx)
	if err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 2; i++ { // two tenants: the counts must span them
		tctx := CtxForTenant(newTenant())
		old := createRequest(t, env.Env, tctx, func(r *domain.Request) { r.Status = domain.RequestStatusClassifying })
		env.AgeRequest(t, old.ID, 10*time.Minute)
		createRequest(t, env.Env, tctx, func(r *domain.Request) { r.Status = domain.RequestStatusClassifying }) // fresh: not stuck
		ev, err := usecase.NewOutboxEvent(tctx, domain.SubjectRequestCreated, map[string]any{"request_id": old.ID})
		if err != nil {
			t.Fatal(err)
		}
		if err := env.Tx.InTx(tctx, func(txCtx context.Context) error { return env.Outbox.InsertOutboxEvent(txCtx, ev) }); err != nil {
			t.Fatal(err)
		}
		tenantID, _ := tenant.TenantID(tctx)
		seedApproval(t, env.ApprovalEnv, tenantID, nil)
	}

	stuckAfter, _ := env.Samples.CountStuck(ctx, "classifying", time.Now().Add(-time.Minute))
	if stuckAfter-stuckBefore != 2 {
		t.Errorf("stuck classifying grew by %d, want 2 (only the aged request of each tenant)", stuckAfter-stuckBefore)
	}
	if n, _ := env.Samples.CountStuck(ctx, "classifying", time.Now().Add(-time.Hour)); n > stuckAfter {
		t.Errorf("a longer threshold cannot count more requests: %d > %d", n, stuckAfter)
	}
	if outboxAfter, _ := env.Samples.CountOutboxPending(ctx); outboxAfter-outboxBefore != 2 {
		t.Errorf("outbox pending grew by %d, want 2", outboxAfter-outboxBefore)
	}
	approvalsAfter, err := env.Samples.PendingApprovalsBySubject(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if approvalsAfter["plan"]-approvalsBefore["plan"] != 2 {
		t.Errorf("pending plan approvals grew by %d, want 2", approvalsAfter["plan"]-approvalsBefore["plan"])
	}
}

type auditSink struct {
	mu      sync.Mutex
	entries []usecase.RPCAuditEvent
}

func (a *auditSink) Record(_ context.Context, e usecase.RPCAuditEvent) {
	a.mu.Lock()
	a.entries = append(a.entries, e)
	a.mu.Unlock()
}

func (a *auditSink) count() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return len(a.entries)
}

// A rolled-back cancel must leave no audit entry and no status change; a committed one leaves exactly one.
func auditOnlyAfterCommit(t *testing.T, env RolloutEnv) {
	sink := &auditSink{}
	hooked := usecase.CommitHooks{Inner: env.TxScope}
	out := &usecase.TappedOutbox{Inner: env.Outbox, Taps: []usecase.OutboxTap{&usecase.AuditTap{Recorder: sink}}}
	tr := usecase.NewTransitionRequest(env.Requests, hooked, out)
	cancel := usecase.NewCancelRequest(env.Requests, tr, env.Returns, noopCanceller{}, &toggleGuard{}, hooked)

	tenantID := newTenant()
	actor := uuid.NewString()
	ctx := tenant.WithUserID(CtxForTenant(tenantID), actor)
	r := createRequest(t, env.Env, ctx, func(r *domain.Request) { r.Status, r.Type = domain.RequestStatusExecuting, domain.RequestTypeTask })

	boom := errors.New("boom")
	err := hooked.InTx(ctx, func(txCtx context.Context) error {
		if _, err := cancel.Execute(txCtx, usecase.CancelInput{RequestID: r.ID, Reason: "no longer needed", ActorID: actor}); err != nil {
			return err
		}
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("want the forced error, got %v", err)
	}
	if sink.count() != 0 {
		t.Fatalf("%d audit entries from a rolled-back transaction", sink.count())
	}
	if got, _ := env.Requests.Get(ctx, r.ID); got.Status != domain.RequestStatusExecuting {
		t.Fatalf("status %s after rollback, want executing", got.Status)
	}

	if _, err := cancel.Execute(ctx, usecase.CancelInput{RequestID: r.ID, Reason: "no longer needed", ActorID: actor}); err != nil {
		t.Fatal(err)
	}
	if sink.count() != 1 || sink.entries[0].Action != domain.ActionRequestCancel || sink.entries[0].TargetID != r.ID ||
		sink.entries[0].TenantID != tenantID || sink.entries[0].ActorID != actor {
		t.Fatalf("committed cancel: %+v", sink.entries)
	}
}
