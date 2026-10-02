//go:build integration

package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/testutil"
	"github.com/stablyai/orca-go/services/mcp-service/internal/domain"
	"github.com/stablyai/orca-go/services/mcp-service/internal/usecase"
)

var allMigrations = []string{"0001_init", "0002_authorization", "0003_tool_policies", "0004_approvals_audit_killswitch"}

// setupGovernance applies every migration as owner and returns repositories on
// TWO pools of the non-superuser app role (simulating two replicas).
func setupGovernance(t *testing.T) (r1, r2 *Repository, appPool *pgxpool.Pool) {
	t.Helper()
	dsn := testutil.StartPostgres(t, "mcp")
	ctx := context.Background()
	conn := adminConn(t, dsn)
	for _, m := range allMigrations {
		execScript(t, ctx, conn, readMigration(t, m+".up.sql"))
	}
	execScript(t, ctx, conn, fmt.Sprintf(`
		CREATE ROLE %[1]s LOGIN PASSWORD '%[2]s' NOSUPERUSER NOBYPASSRLS;
		GRANT USAGE ON SCHEMA mcp TO %[1]s;
		GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA mcp TO %[1]s;`, appRole, appPass))
	mk := func() *pgxpool.Pool {
		cfg, err := pgxpool.ParseConfig(dsn)
		if err != nil {
			t.Fatal(err)
		}
		cfg.ConnConfig.User, cfg.ConnConfig.Password = appRole, appPass
		p, err := pgxpool.NewWithConfig(ctx, cfg)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(p.Close)
		return p
	}
	p1, p2 := mk(), mk()
	return New(p1), New(p2), p1
}

func TestGovernanceMigrations_UpDownUp(t *testing.T) {
	dsn := testutil.StartPostgres(t, "mcp")
	ctx := context.Background()
	conn := adminConn(t, dsn)
	up := func() {
		for _, m := range allMigrations {
			execScript(t, ctx, conn, readMigration(t, m+".up.sql"))
		}
	}
	down := func() {
		for i := len(allMigrations) - 1; i >= 0; i-- {
			execScript(t, ctx, conn, readMigration(t, allMigrations[i]+".down.sql"))
		}
	}
	up()
	down()
	if schemaExists(t, ctx, conn) {
		t.Fatal("schema still present after down")
	}
	up()
	if !schemaExists(t, ctx, conn) {
		t.Fatal("schema missing after second up")
	}
}

func okPolicy(tenantID, tool, decision string) domain.ToolPolicy {
	return domain.ToolPolicy{ID: uuid.NewString(), Match: domain.ToolPolicyMatch{Tool: tool}, Decision: decision,
		CreatedBy: userA1, UpdatedBy: userA1, UpdatedAt: time.Now().UTC()}
}

func TestPolicies_VersioningEpochRevisionsAndIsolation(t *testing.T) {
	r, _, pool := setupGovernance(t)
	ctx := context.Background()
	defaults := domain.DefaultTenantSettings(tenantA, true, 90)
	snap, err := r.LoadPolicySnapshot(ctx, defaults)
	if err != nil || !snap.Settings.Enabled || snap.Epoch != 0 {
		t.Fatalf("%+v %v", snap, err)
	}
	p := okPolicy(tenantA, "task_create", "require_approval")
	p.Match.Roles = []string{"user"}
	ev, _ := domain.NewOutboxEvent(uuid.NewString(), domain.SubjectPolicyChanged, tenantA, time.Now(), map[string]any{"op": "create"})
	created, err := r.CreateToolPolicy(ctx, tenantA, p, []domain.OutboxRecord{ev})
	if err != nil || created.Version != 1 || len(created.Match.Roles) != 1 {
		t.Fatalf("%+v %v", created, err)
	}
	snap, _ = r.LoadPolicySnapshot(ctx, defaults)
	if snap.Epoch != 1 || len(snap.Policies) != 1 {
		t.Fatalf("epoch must bump in the same transaction: %+v", snap)
	}

	// Concurrent writers from two replicas: exactly one wins per version.
	r2 := New(pool)
	var wins, conflicts int32
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			q := created
			q.Decision, q.UpdatedAt = "deny", time.Now().UTC()
			repo := r
			if i%2 == 1 {
				repo = r2
			}
			_, err := repo.UpdateToolPolicy(ctx, tenantA, q, nil)
			var ae *apperrors.AppError
			switch {
			case err == nil:
				atomic.AddInt32(&wins, 1)
			case errors.As(err, &ae) && ae.Code == domain.CodePolicyVersionConflict:
				atomic.AddInt32(&conflicts, 1)
			default:
				t.Errorf("unexpected: %v", err)
			}
		}(i)
	}
	wg.Wait()
	if wins != 1 || conflicts != 7 {
		t.Fatalf("wins=%d conflicts=%d", wins, conflicts)
	}

	// Tenant isolation: B neither sees, updates nor deletes A's policy.
	if ps, _ := r.ListToolPolicies(ctx, tenantB); len(ps) != 0 {
		t.Fatal("tenant B sees tenant A policies")
	}
	cur := created
	cur.Version = 2
	if _, err := r.UpdateToolPolicy(ctx, tenantB, cur, nil); err == nil || !strings.Contains(err.Error(), domain.CodeNotFound) {
		t.Fatalf("cross-tenant update must be not-found: %v", err)
	}
	if err := r.DeleteToolPolicy(ctx, tenantB, created.ID, userA1, nil); err == nil {
		t.Fatal("cross-tenant delete must fail")
	}
	if err := r.DeleteToolPolicy(ctx, tenantA, created.ID, userA1, nil); err != nil {
		t.Fatal(err)
	}
	var revs int
	_ = (&Repository{pool: pool}).withTenantTx(ctx, tenantA, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT count(*) FROM mcp.tool_policy_revisions`).Scan(&revs)
	})
	if revs != 3 { // create, one winning update, delete
		t.Fatalf("revisions = %d, want 3", revs)
	}
	// CHECK constraints are the last line of defense behind domain validation.
	bad := okPolicy(tenantA, "", "deny")
	bad.Match = domain.ToolPolicyMatch{}
	if _, err := r.CreateToolPolicy(ctx, tenantA, bad, nil); err == nil {
		t.Fatal("a policy matching everything must be rejected by the CHECK")
	}
}

func TestPatchTenantSettings_PartialAndEpoch(t *testing.T) {
	r, _, _ := setupGovernance(t)
	ctx := context.Background()
	d := domain.DefaultTenantSettings(tenantA, true, 90)
	days := 14
	got, err := r.PatchTenantSettings(ctx, d, usecase.SettingsPatch{MaxTokenDays: &days}, userA1, nil)
	if err != nil || got.MaxTokenDays != 14 || !got.Enabled || got.ApprovalTTLSeconds != domain.DefaultApprovalTTL || got.UpdatedBy != userA1 {
		t.Fatalf("%+v %v", got, err)
	}
	off := false
	got, _ = r.PatchTenantSettings(ctx, d, usecase.SettingsPatch{Enabled: &off}, userA1, nil)
	if got.Enabled || got.MaxTokenDays != 14 {
		t.Fatalf("partial patch must keep other fields: %+v", got)
	}
	snap, _ := r.LoadPolicySnapshot(ctx, d)
	if snap.Epoch != 2 {
		t.Fatalf("epoch %d", snap.Epoch)
	}
	if got, _ := r.PatchTenantSettings(ctx, domain.DefaultTenantSettings(tenantB, false, 90), usecase.SettingsPatch{}, userA1, nil); got.Enabled {
		t.Fatal("tenant B must have its own row with the default it was created with")
	}
}

func approvalFixture(tenantID, userID, hash string, ttl time.Duration) domain.Approval {
	now := time.Now().UTC()
	return domain.Approval{ID: uuid.NewString(), TenantID: tenantID, UserID: userID, ClientID: "c1", ClientName: "Agent", SessionID: "s1",
		ToolName: "terminal_send", ToolTitle: "Send input", Channel: "terminal.send", Risk: "exec", ParamsHash: hash, ArgsPreview: "{}",
		Reasons: []string{"risk_default:exec=require_approval"}, CreatedAt: now, ExpiresAt: now.Add(ttl)}
}

func TestApprovals_LifecycleOwnershipAndReuse(t *testing.T) {
	r, _, _ := setupGovernance(t)
	ctx := context.Background()
	now := time.Now().UTC()
	limits := usecase.ApprovalLimits{MaxPendingPerClient: 3, MaxCreatedPerHour: 30}
	a := approvalFixture(tenantA, userA1, "sha256:aaa", time.Minute)
	got, created, err := r.FindOrCreatePendingApproval(ctx, a, limits, now)
	if err != nil || !created || got.Status != domain.ApprovalPending || got.Reasons[0] == "" {
		t.Fatalf("%+v %v", got, err)
	}
	dup := approvalFixture(tenantA, userA1, "sha256:aaa", time.Minute)
	same, created, err := r.FindOrCreatePendingApproval(ctx, dup, limits, now)
	if err != nil || created || same.ID != got.ID {
		t.Fatalf("same call must reuse the open approval: %+v created=%v err=%v", same, created, err)
	}
	if subjects := outboxSubjects(t, r); countOf(subjects, domain.SubjectApprovalRequested) != 1 {
		t.Fatalf("outbox: %v", subjects)
	}
	// Ownership, hash, then success; each failure leaves it pending.
	diag := func(user, hash string, approve bool) string {
		_, err := r.DecideApproval(ctx, usecase.DecideApprovalRepoInput{TenantID: tenantA, UserID: user, ApprovalID: got.ID, Approve: approve, ParamsHash: hash, Via: "web", Now: now})
		var ae *apperrors.AppError
		if errors.As(err, &ae) {
			return ae.Code
		}
		if err == nil {
			return "OK"
		}
		return err.Error()
	}
	if c := diag(userA2, "sha256:aaa", true); c != domain.CodeNotFound {
		t.Fatalf("other user: %s", c)
	}
	if _, err := r.DecideApproval(ctx, usecase.DecideApprovalRepoInput{TenantID: tenantB, UserID: userA1, ApprovalID: got.ID, Approve: true, ParamsHash: "sha256:aaa", Via: "web", Now: now}); err == nil || !strings.Contains(err.Error(), domain.CodeNotFound) {
		t.Fatalf("other tenant: %v", err)
	}
	if c := diag(userA1, "sha256:bbb", true); c != domain.CodeApprovalHashMismatch {
		t.Fatalf("hash: %s", c)
	}
	if c := diag(userA1, "sha256:aaa", true); c != "OK" {
		t.Fatalf("owner: %s", c)
	}
	for _, ap := range []bool{true, false} {
		if c := diag(userA1, "sha256:aaa", ap); c != domain.CodeApprovalAlreadyDecided {
			t.Fatalf("double decision: %s", c)
		}
	}
	// Expired: decided after the deadline.
	b := approvalFixture(tenantA, userA1, "sha256:expiring", time.Second)
	_, _, _ = r.FindOrCreatePendingApproval(ctx, b, limits, now)
	late := now.Add(time.Minute)
	_, err = r.DecideApproval(ctx, usecase.DecideApprovalRepoInput{TenantID: tenantA, UserID: userA1, ApprovalID: b.ID, Approve: true, ParamsHash: "sha256:expiring", Via: "web", Now: late})
	if err == nil || !strings.Contains(err.Error(), domain.CodeApprovalExpired) {
		t.Fatalf("expired: %v", err)
	}
	refs, _ := r.ListExpiredApprovalRefs(ctx, late, 10)
	if len(refs) != 1 || refs[0].ID != b.ID || refs[0].TenantID != tenantA {
		t.Fatalf("worker discovery: %+v", refs)
	}
	if ok, err := r.ExpireApproval(ctx, tenantA, b.ID, late); err != nil || !ok {
		t.Fatalf("%v %v", ok, err)
	}
	if ok, _ := r.ExpireApproval(ctx, tenantA, b.ID, late); ok {
		t.Fatal("second expiry must be a no-op")
	}
	// Floods.
	for i := 0; i < 3; i++ {
		_, _, err := r.FindOrCreatePendingApproval(ctx, approvalFixture(tenantA, userA2, fmt.Sprintf("sha256:f%d", i), time.Minute), limits, now)
		if err != nil {
			t.Fatal(err)
		}
	}
	if _, _, err := r.FindOrCreatePendingApproval(ctx, approvalFixture(tenantA, userA2, "sha256:f9", time.Minute), limits, now); !errors.Is(err, usecase.ErrApprovalFlood) {
		t.Fatalf("flood: %v", err)
	}
	// Keyset listing, newest first, owner only.
	rows, err := r.ListApprovals(ctx, usecase.ApprovalQuery{TenantID: tenantA, UserID: userA2, PendingOnly: true, Limit: 2, Now: now})
	if err != nil || len(rows) != 2 || rows[0].CreatedAt.Before(rows[1].CreatedAt) {
		t.Fatalf("%+v %v", rows, err)
	}
	next, _ := r.ListApprovals(ctx, usecase.ApprovalQuery{TenantID: tenantA, UserID: userA2, PendingOnly: true, Limit: 5, Now: now, CursorAt: rows[1].CreatedAt, CursorID: rows[1].ID})
	if len(next) != 1 {
		t.Fatalf("keyset page 2: %d", len(next))
	}
	if other, _ := r.ListApprovals(ctx, usecase.ApprovalQuery{TenantID: tenantB, UserID: userA2, Limit: 5, Now: now}); len(other) != 0 {
		t.Fatal("tenant B sees tenant A approvals")
	}
	if _, err := r.GetApproval(ctx, tenantB, got.ID); err == nil {
		t.Fatal("cross-tenant get must be not-found")
	}
}

func countOf(ss []string, s string) int {
	n := 0
	for _, x := range ss {
		if x == s {
			n++
		}
	}
	return n
}

func callFixture(tenantID, userID, hash string) domain.ToolCall {
	return domain.ToolCall{ID: uuid.NewString(), TenantID: tenantID, UserID: userID, ClientID: "c1", ClientName: "Agent", ToolName: "terminal_send",
		Channel: "terminal.send", Risk: "exec", RiskClass: "exec", ParamsHash: hash, ArgsSummary: "{}", Decision: domain.CallApproved}
}

func TestAdmit_SingleUseApprovalAcrossReplicas(t *testing.T) {
	r1, r2, _ := setupGovernance(t)
	ctx := context.Background()
	now := time.Now().UTC()
	a := approvalFixture(tenantA, userA1, "sha256:once", time.Minute)
	_, _, _ = r1.FindOrCreatePendingApproval(ctx, a, usecase.ApprovalLimits{}, now)
	if _, err := r1.DecideApproval(ctx, usecase.DecideApprovalRepoInput{TenantID: tenantA, UserID: userA1, ApprovalID: a.ID, Approve: true, ParamsHash: "sha256:once", Via: "web", Now: now}); err != nil {
		t.Fatal(err)
	}
	var admitted int32
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			repo := r1
			if i%2 == 1 {
				repo = r2
			}
			res, err := repo.AdmitToolCall(ctx, usecase.AdmitRequest{Call: callFixture(tenantA, userA1, "sha256:once"), ConsumeApprovalHash: "sha256:once", Now: time.Now().UTC(),
				Limits: usecase.RateLimits{PerMinute: map[string]int{"exec": 100}}})
			if err != nil {
				t.Error(err)
				return
			}
			if res.Admitted {
				atomic.AddInt32(&admitted, 1)
			} else if !res.ApprovalMissing {
				t.Errorf("unexpected deny: %+v", res)
			}
		}(i)
	}
	wg.Wait()
	if admitted != 1 {
		t.Fatalf("%d admissions from one approval, want 1", admitted)
	}
	// After consumption a fresh approval can be opened for the same hash.
	again, created, err := r1.FindOrCreatePendingApproval(ctx, approvalFixture(tenantA, userA1, "sha256:once", time.Minute), usecase.ApprovalLimits{}, now)
	if err != nil || !created || again.ID == a.ID {
		t.Fatalf("consumed approval must free the unique slot: %+v %v", again, err)
	}
}

func TestAdmit_RateLimitExactAcrossReplicas(t *testing.T) {
	r1, r2, _ := setupGovernance(t)
	ctx := context.Background()
	limits := usecase.RateLimits{PerMinute: map[string]int{"read": 7}, TotalPerMinute: 300, DailyPerUser: 5000}
	var ok, limited int32
	var wg sync.WaitGroup
	for i := 0; i < 25; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			repo := r1
			if i%2 == 1 {
				repo = r2
			}
			c := callFixture(tenantA, userA1, fmt.Sprintf("sha256:r%d", i))
			c.Decision, c.Risk, c.RiskClass = domain.CallAllow, "read", "read"
			res, err := repo.AdmitToolCall(ctx, usecase.AdmitRequest{Call: c, Limits: limits, Now: time.Now().UTC()})
			if err != nil {
				t.Error(err)
				return
			}
			if res.Admitted {
				atomic.AddInt32(&ok, 1)
			} else if res.DenyReason == domain.ReasonRateLimited {
				atomic.AddInt32(&limited, 1)
			}
		}(i)
	}
	wg.Wait()
	if ok != 7 || limited != 18 {
		t.Fatalf("admitted=%d limited=%d, want exactly 7 admitted", ok, limited)
	}
	// Loop detection: identical calls.
	loop := usecase.RateLimits{PerMinute: map[string]int{"read": 1000}, LoopSlowDown: 3, LoopBlock: 100}
	var reasons []string
	for i := 0; i < 5; i++ {
		c := callFixture(tenantA, userA2, "sha256:same")
		c.Decision, c.Risk, c.RiskClass = domain.CallAllow, "read", "read"
		res, _ := r1.AdmitToolCall(ctx, usecase.AdmitRequest{Call: c, Limits: loop, Now: time.Now().UTC()})
		reasons = append(reasons, res.DenyReason)
	}
	if reasons[2] != "" || reasons[3] != domain.ReasonLoopSlowDown {
		t.Fatalf("loop reasons: %v", reasons)
	}
}

func TestToolCalls_FinalizeTaintAuditReaperPurgeAndIsolation(t *testing.T) {
	r, _, _ := setupGovernance(t)
	ctx := context.Background()
	now := time.Now().UTC()
	c := callFixture(tenantA, userA1, "sha256:x")
	c.Decision, c.Risk, c.RiskClass, c.ReadUntrusted = domain.CallAllow, "read", "read", true
	res, err := r.AdmitToolCall(ctx, usecase.AdmitRequest{Call: c, Now: now})
	if err != nil || !res.Admitted {
		t.Fatalf("%+v %v", res, err)
	}
	if _, err := r.FinalizeToolCall(ctx, tenantA, userA2, c.ID, "ok", "", 5, now, time.Minute); err == nil {
		t.Fatal("another user's call must be not-found")
	}
	if _, err := r.FinalizeToolCall(ctx, tenantB, userA1, c.ID, "ok", "", 5, now, time.Minute); err == nil {
		t.Fatal("another tenant's call must be not-found")
	}
	if tainted, _ := r.IsTainted(ctx, tenantA, userA1, "c1", now); tainted {
		t.Fatal("not tainted before completion")
	}
	done, err := r.FinalizeToolCall(ctx, tenantA, userA1, c.ID, "ok", "", 5, now, time.Minute)
	if err != nil || done.State != domain.CallStateDone || done.DurationMs != 5 {
		t.Fatalf("%+v %v", done, err)
	}
	if _, err := r.FinalizeToolCall(ctx, tenantA, userA1, c.ID, "ok", "", 5, now, time.Minute); err != nil {
		t.Fatalf("completing twice must be harmless: %v", err)
	}
	if tainted, _ := r.IsTainted(ctx, tenantA, userA1, "c1", now); !tainted {
		t.Fatal("untrusted read must taint")
	}
	if tainted, _ := r.IsTainted(ctx, tenantA, userA1, "c1", now.Add(2*time.Minute)); tainted {
		t.Fatal("taint must expire")
	}
	if tainted, _ := r.IsTainted(ctx, tenantB, userA1, "c1", now); tainted {
		t.Fatal("taint is tenant scoped")
	}
	if n := countOf(outboxSubjects(t, r), domain.SubjectAuditAppended); n != 1 {
		t.Fatalf("exactly one audit event for the call, got %d", n)
	}
	// Reaper: a call stuck in "started" is interrupted once, with one audit event.
	stuck := callFixture(tenantA, userA1, "sha256:stuck")
	stuck.Decision, stuck.RiskClass = domain.CallAllow, "read"
	_, _ = r.AdmitToolCall(ctx, usecase.AdmitRequest{Call: stuck, Now: now.Add(-time.Hour)})
	refs, _ := r.ListStaleCalls(ctx, now.Add(-15*time.Minute), 10)
	if len(refs) != 1 || refs[0].ID != stuck.ID {
		t.Fatalf("stale: %+v", refs)
	}
	if ok, _ := r.InterruptCall(ctx, tenantA, stuck.ID, domain.ReasonInterrupted, now); !ok {
		t.Fatal("interrupt")
	}
	if ok, _ := r.InterruptCall(ctx, tenantA, stuck.ID, domain.ReasonInterrupted, now); ok {
		t.Fatal("interrupt twice")
	}
	if n := countOf(outboxSubjects(t, r), domain.SubjectAuditAppended); n != 2 {
		t.Fatalf("audit events: %d", n)
	}
	// Kill-switch scope interruption.
	run := callFixture(tenantA, userA1, "sha256:run")
	run.Decision, run.RiskClass = domain.CallAllow, "read"
	run.SessionID = "sess-9"
	_, _ = r.AdmitToolCall(ctx, usecase.AdmitRequest{Call: run, Now: now})
	ids, err := r.InterruptCallsInScope(ctx, tenantA, domain.KillScopeSession, "sess-9", domain.ReasonKilled, now)
	if err != nil || len(ids) != 1 || ids[0] != run.ID {
		t.Fatalf("%v %v", ids, err)
	}
	// Retention purge is cross-tenant and only removes finished rows.
	purged, err := r.PurgeFinishedCalls(ctx, now.Add(time.Hour), 100)
	if err != nil || purged != 3 {
		t.Fatalf("purged %d: %v", purged, err)
	}
}

func TestRLS_GovernanceTablesAreTenantIsolated(t *testing.T) {
	r, _, pool := setupGovernance(t)
	ctx := context.Background()
	now := time.Now().UTC()
	_, _ = r.LoadPolicySnapshot(ctx, domain.DefaultTenantSettings(tenantA, true, 90))
	_, _ = r.CreateToolPolicy(ctx, tenantA, okPolicy(tenantA, "t", "deny"), nil)
	_, _, _ = r.FindOrCreatePendingApproval(ctx, approvalFixture(tenantA, userA1, "sha256:rls", time.Minute), usecase.ApprovalLimits{}, now)
	c := callFixture(tenantA, userA1, "sha256:rls")
	c.Decision, c.RiskClass, c.ReadUntrusted = domain.CallAllow, "read", true
	_, _ = r.AdmitToolCall(ctx, usecase.AdmitRequest{Call: c, Now: now})
	_, _ = r.FinalizeToolCall(ctx, tenantA, userA1, c.ID, "ok", "", 1, now, time.Hour)
	_, _ = r.UpsertKillSwitch(ctx, domain.KillSwitchEntry{ID: uuid.NewString(), TenantID: tenantA, Scope: "tenant", Reason: "incident", Active: true, SetBy: userA1, SetAt: now}, nil)

	for _, table := range []string{"tool_policies", "tool_policy_revisions", "approvals", "kill_switches", "tool_calls", "taint"} {
		count := func(tenantID string) int {
			var n int
			err := (&Repository{pool: pool}).withTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
				return tx.QueryRow(ctx, `SELECT count(*) FROM mcp.`+table).Scan(&n) // no WHERE: only RLS filters
			})
			if err != nil {
				t.Fatalf("%s: %v", table, err)
			}
			return n
		}
		if count(tenantA) == 0 {
			t.Errorf("%s: tenant A should see its own row", table)
		}
		if n := count(tenantB); n != 0 {
			t.Errorf("%s: tenant B sees %d rows of tenant A", table, n)
		}
		err := (&Repository{pool: pool}).withTenantTx(ctx, tenantB, func(tx pgx.Tx) error {
			_, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE mcp.%s SET tenant_id = tenant_id`, table))
			return err
		})
		if err != nil {
			t.Errorf("%s: cross-tenant update should affect nothing, not fail: %v", table, err)
		}
	}
	// With no tenant context at all, and no relay opt-in, nothing is visible.
	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM mcp.approvals`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("no tenant context: n=%d err=%v", n, err)
	}
}

func TestKillSwitches_MirrorSettingsCleanupDiscoveryAndGrantScope(t *testing.T) {
	r, _, _ := setupGovernance(t)
	ctx := context.Background()
	now := time.Now().UTC()
	_, _ = r.LoadPolicySnapshot(ctx, domain.DefaultTenantSettings(tenantA, true, 90))
	e := domain.KillSwitchEntry{ID: uuid.NewString(), TenantID: tenantA, Scope: "tenant", Reason: "incident", Active: true, SetBy: userA1, SetAt: now}
	if _, err := r.UpsertKillSwitch(ctx, e, nil); err != nil {
		t.Fatal(err)
	}
	snap, _ := r.LoadPolicySnapshot(ctx, domain.DefaultTenantSettings(tenantA, true, 90))
	if !snap.Settings.KillSwitch.Active || snap.Settings.KillSwitch.Reason != "incident" || snap.Epoch != 1 {
		t.Fatalf("settings mirror: %+v", snap)
	}
	pending, err := r.PendingKillCleanups(ctx, 10)
	if err != nil || len(pending) != 1 || pending[0].TenantID != tenantA {
		t.Fatalf("worker discovery: %+v %v", pending, err)
	}
	if err := r.ClearKillCleanup(ctx, tenantA, pending[0].ID); err != nil {
		t.Fatal(err)
	}
	if p, _ := r.PendingKillCleanups(ctx, 10); len(p) != 0 {
		t.Fatal("cleanup flag must clear")
	}
	if active, _ := r.ListKillSwitches(ctx, tenantB, true); len(active) != 0 {
		t.Fatal("tenant B sees tenant A switch")
	}
	// Upsert on the same (scope,target) flips state, not adds rows.
	e.Active, e.Reason, e.SetAt = false, "all clear", now.Add(time.Second)
	_, _ = r.UpsertKillSwitch(ctx, e, nil)
	all, _ := r.ListKillSwitches(ctx, tenantA, false)
	active, _ := r.ListKillSwitches(ctx, tenantA, true)
	if len(all) != 1 || len(active) != 0 {
		t.Fatalf("all=%d active=%d", len(all), len(active))
	}
	snap, _ = r.LoadPolicySnapshot(ctx, domain.DefaultTenantSettings(tenantA, true, 90))
	if snap.Settings.KillSwitch.Active {
		t.Fatal("deactivation must mirror too")
	}
	// Grant lookups are tenant scoped.
	cr := consent(tenantA, userA1, "app-1", now.Add(time.Hour))
	_ = r.CreateConsentRequest(ctx, cr)
	res := approve(t, r, cr, "orca:read")
	if ok, _ := r.GrantExists(ctx, tenantA, res.Grant.ID); !ok {
		t.Fatal("grant exists")
	}
	if ok, _ := r.GrantExists(ctx, tenantB, res.Grant.ID); ok {
		t.Fatal("grant must be invisible to tenant B")
	}
	if ok, _ := r.GrantExists(ctx, tenantA, "not-a-uuid"); ok {
		t.Fatal("malformed id")
	}
	for _, tc := range []struct {
		scope, target string
		want          int
	}{{"tenant", "", 1}, {"client", "app-1", 1}, {"client", "other", 0}, {"grant", res.Grant.ID, 1}, {"session", "s", 0}} {
		ids, err := r.GrantIDsInScope(ctx, tenantA, tc.scope, tc.target)
		if err != nil || len(ids) != tc.want {
			t.Errorf("%s/%s: %v %v", tc.scope, tc.target, ids, err)
		}
	}
	if n, err := r.CancelApprovals(ctx, tenantA, "grant", res.Grant.ID, now); err != nil || n != 0 {
		t.Fatalf("%d %v", n, err)
	}
}

func TestCancelApprovalsByScope(t *testing.T) {
	r, _, _ := setupGovernance(t)
	ctx := context.Background()
	now := time.Now().UTC()
	for i, client := range []string{"c1", "c2"} {
		a := approvalFixture(tenantA, userA1, fmt.Sprintf("sha256:%d", i), time.Minute)
		a.ClientID = client
		_, _, _ = r.FindOrCreatePendingApproval(ctx, a, usecase.ApprovalLimits{}, now)
	}
	n, err := r.CancelApprovals(ctx, tenantA, "client", "c1", now)
	if err != nil || n != 1 {
		t.Fatalf("client scope: %d %v", n, err)
	}
	n, _ = r.CancelApprovals(ctx, tenantA, "tenant", "", now)
	if n != 1 {
		t.Fatalf("tenant scope: %d", n)
	}
	if got := countOf(outboxSubjects(t, r), domain.SubjectApprovalResolved); got != 2 {
		t.Fatalf("resolved events: %d", got)
	}
}
