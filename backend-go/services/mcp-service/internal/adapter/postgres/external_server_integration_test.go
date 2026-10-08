//go:build integration

package postgres

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/common/testutil"
	"github.com/stablyai/orca-go/services/mcp-service/internal/domain"
	"github.com/stablyai/orca-go/services/mcp-service/internal/usecase"
)

// externalSetup applies 0001 (schema, outbox) and 0007 and returns repositories
// over the non-superuser app role (RLS applies) and the owner.
func externalSetup(t *testing.T) (*Repository, *pgxpool.Pool, *pgxpool.Pool) {
	t.Helper()
	dsn := testutil.StartPostgres(t, "mcp")
	ctx := context.Background()
	conn := adminConn(t, dsn)
	execScript(t, ctx, conn, readMigration(t, "0001_init.up.sql"))
	execScript(t, ctx, conn, readMigration(t, "0007_external_servers.up.sql"))
	execScript(t, ctx, conn, `
		CREATE ROLE mcp_app LOGIN PASSWORD 'mcp_app_pw' NOSUPERUSER NOBYPASSRLS;
		GRANT USAGE ON SCHEMA mcp TO mcp_app;
		GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA mcp TO mcp_app;`)
	repo, appPool, adminPool := poolsFor(t, dsn)
	return repo, appPool, adminPool
}

func poolsFor(t *testing.T, dsn string) (*Repository, *pgxpool.Pool, *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()
	acfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	admin, err := pgxpool.NewWithConfig(ctx, acfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(admin.Close)
	pcfg, _ := pgxpool.ParseConfig(dsn)
	pcfg.ConnConfig.User, pcfg.ConnConfig.Password = "mcp_app", "mcp_app_pw"
	app, err := pgxpool.NewWithConfig(ctx, pcfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(app.Close)
	return New(app), app, admin
}

func newServer(tenant string, name string) domain.ExternalServer {
	now := time.Now().UTC().Truncate(time.Microsecond)
	return domain.ExternalServer{
		ID: uuid.NewString(), TenantID: tenant, Scope: domain.ScopeTenant, ScopeID: tenant, Name: name, Transport: domain.TransportHTTP,
		URL: "https://x.example.com/mcp", Args: []string{}, Status: domain.StatusPendingReview, SpecDigest: "spec1", CreatedBy: "admin", Version: 1,
		CreatedAt: now, UpdatedAt: now, HeaderRefs: []domain.SecretRef{{Kind: "header", Name: "X-Api-Key"}},
	}
}

func TestExternalServers_MigrationUpDownUp(t *testing.T) {
	dsn := testutil.StartPostgres(t, "mcp")
	ctx := context.Background()
	conn := adminConn(t, dsn)
	execScript(t, ctx, conn, readMigration(t, "0001_init.up.sql"))
	up, down := readMigration(t, "0007_external_servers.up.sql"), readMigration(t, "0007_external_servers.down.sql")
	count := func() int {
		var n int
		if err := conn.QueryRow(ctx, `SELECT count(*) FROM information_schema.tables WHERE table_schema='mcp' AND table_name LIKE 'external_server%'`).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	execScript(t, ctx, conn, up)
	if count() != 3 {
		t.Fatalf("tables after up: %d", count())
	}
	execScript(t, ctx, conn, down)
	if count() != 0 {
		t.Fatal("tables remain after down")
	}
	execScript(t, ctx, conn, up)
	if count() != 3 {
		t.Fatal("tables missing after second up")
	}
}

func TestExternalServers_NoColumnCanHoldASecretValue(t *testing.T) {
	_, _, admin := externalSetup(t)
	rows, err := admin.Query(context.Background(), `SELECT table_name, column_name FROM information_schema.columns
		WHERE table_schema='mcp' AND table_name LIKE 'external_server%'`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var tbl, col string
		_ = rows.Scan(&tbl, &col)
		for _, bad := range []string{"secret", "value", "plaintext", "cipher", "password", "token"} {
			if strings.Contains(col, bad) && col != "broker_owner_id" {
				t.Errorf("%s.%s looks like it could hold a secret", tbl, col)
			}
		}
	}
}

func TestExternalServers_RLSCrossTenantIsNotFound(t *testing.T) {
	repo, app, _ := externalSetup(t)
	ctx := context.Background()
	s := newServer(tenantA, "srv")
	if err := repo.CreateExternalServer(ctx, s, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.GetExternalServer(ctx, tenantB, s.ID); err == nil || !strings.Contains(err.Error(), "MCP_NOT_FOUND") {
		t.Fatalf("cross-tenant get: %v", err)
	}
	if l, _ := repo.ListExternalServers(ctx, tenantB, usecase.ExternalServerFilter{}); len(l) != 0 {
		t.Fatal("cross-tenant list must be empty")
	}
	if err := repo.DeleteExternalServer(ctx, tenantB, s.ID, nil); err == nil {
		t.Fatal("cross-tenant delete must not succeed")
	}
	// without any tenant context a raw query sees nothing
	var n int
	if err := app.QueryRow(ctx, `SELECT count(*) FROM mcp.external_servers`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("no tenant context: n=%d err=%v", n, err)
	}
	// a tenant cannot insert rows for another tenant
	other := newServer(tenantB, "evil")
	other.TenantID = tenantB
	tx, _ := app.Begin(ctx)
	defer tx.Rollback(ctx)
	_, _ = tx.Exec(ctx, `SELECT set_config('app.tenant_id', $1, true)`, tenantA)
	if _, err := tx.Exec(ctx, `INSERT INTO mcp.external_servers (id, tenant_id, scope, scope_id, name, transport, url, spec_digest, created_by)
		VALUES ($1,$2,'tenant',$2,'x','http','https://a','d','u')`, uuid.NewString(), tenantB); err == nil {
		t.Fatal("RLS WITH CHECK must reject a foreign tenant_id")
	}
}

func TestExternalServers_CRUDRefsReviewAndConflict(t *testing.T) {
	repo, _, admin := externalSetup(t)
	ctx := context.Background()
	s := newServer(tenantA, "srv")
	if err := repo.CreateExternalServer(ctx, s, nil); err != nil {
		t.Fatal(err)
	}
	if err := repo.CreateExternalServer(ctx, newServer(tenantA, "srv"), nil); err == nil || !strings.Contains(err.Error(), domain.CodeServerNameConflict) {
		t.Fatalf("duplicate name: %v", err)
	}
	now := time.Now().UTC()
	if err := repo.SetSecretRef(ctx, tenantA, s.ID, domain.SecretRef{Kind: "header", Name: "X-Api-Key", BrokerOwnerID: "mcp:o", SetBy: "u", SetAt: &now}, nil); err != nil {
		t.Fatal(err)
	}
	got, _ := repo.GetExternalServer(ctx, tenantA, s.ID)
	if len(got.HeaderRefs) != 1 || !got.HeaderRefs[0].HasSecret() {
		t.Fatalf("%+v", got.HeaderRefs)
	}

	// review: digest is checked atomically against the stored probe
	if _, err := repo.ApplyReview(ctx, usecase.ReviewRecord{TenantID: tenantA, ServerID: s.ID, ReviewerID: "a", Approve: true, ExpectedDigest: "d1", At: now}, nil); err == nil || !strings.Contains(err.Error(), domain.CodeServerDigestMismatch) {
		t.Fatalf("no probe yet: %v", err)
	}
	tools := []domain.ToolInfo{{Name: "echo", Description: strings.Repeat("x", 5000)}}
	if err := repo.RecordProbe(ctx, tenantA, s.ID, usecase.ProbeRecord{Digest: "d1", Tools: tools, At: now, Source: "probe"}, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.ApplyReview(ctx, usecase.ReviewRecord{TenantID: tenantA, ServerID: s.ID, ReviewerID: "a", Approve: true, ExpectedDigest: "stale", At: now}, nil); err == nil {
		t.Fatal("stale digest must be rejected")
	}
	appr, err := repo.ApplyReview(ctx, usecase.ReviewRecord{TenantID: tenantA, ServerID: s.ID, ReviewerID: "a", Approve: true, ExpectedDigest: "d1", At: now}, nil)
	if err != nil || appr.Status != domain.StatusApproved || appr.ApprovedDigest != "d1" || len(appr.ApprovedTools) != 1 || len(appr.ApprovedTools[0].Description) != maxStoredDescription {
		t.Fatalf("%+v %v", appr, err)
	}
	var decision string
	if err := admin.QueryRow(ctx, `SELECT decision FROM mcp.external_server_tools_history WHERE server_id=$1`, s.ID).Scan(&decision); err != nil || decision != "approved" {
		t.Fatalf("history decision %q %v", decision, err)
	}

	// spec change: pending again, probe cleared, dropped ref pruned, version CAS enforced
	next := appr
	next.URL, next.Status, next.SpecDigest, next.LastProbeDigest = "https://y.example.com/mcp", domain.StatusPendingReview, "spec2", ""
	next.HeaderRefs = nil
	if err := repo.UpdateExternalServer(ctx, next, nil); err != nil {
		t.Fatal(err)
	}
	if err := repo.UpdateExternalServer(ctx, next, nil); err == nil {
		t.Fatal("stale version must fail")
	}
	after, _ := repo.GetExternalServer(ctx, tenantA, s.ID)
	if after.Status != domain.StatusPendingReview || after.LastProbeDigest != "" || after.ApprovedDigest != "d1" || len(after.HeaderRefs) != 0 || after.Version != 3 {
		t.Fatalf("status=%s probe=%q approved=%q refs=%d version=%d", after.Status, after.LastProbeDigest, after.ApprovedDigest, len(after.HeaderRefs), after.Version)
	}
	if _, err := repo.ApplyReview(ctx, usecase.ReviewRecord{TenantID: tenantA, ServerID: s.ID, ReviewerID: "a", Approve: false, At: now}, nil); err != nil {
		t.Fatal(err)
	}
	if err := repo.DeleteExternalServer(ctx, tenantA, s.ID, nil); err != nil {
		t.Fatal(err)
	}
	var refs int
	_ = admin.QueryRow(ctx, `SELECT count(*) FROM mcp.external_server_secret_refs`).Scan(&refs)
	if refs != 0 {
		t.Fatal("refs must cascade")
	}
}

func TestExternalServers_ClaimHealthChecksAcrossTenantsOncePerInterval(t *testing.T) {
	repo, _, admin := externalSetup(t)
	ctx := context.Background()
	a, b, c := newServer(tenantA, "a"), newServer(tenantB, "b"), newServer(tenantA, "pending")
	for _, s := range []domain.ExternalServer{a, b, c} {
		if err := repo.CreateExternalServer(ctx, s, nil); err != nil {
			t.Fatal(err)
		}
	}
	for _, id := range []string{a.ID, b.ID} {
		if _, err := admin.Exec(ctx, `UPDATE mcp.external_servers SET status='approved' WHERE id=$1`, id); err != nil {
			t.Fatal(err)
		}
	}
	keys, err := repo.ClaimHealthChecks(ctx, time.Now().Add(-time.Minute), 10)
	if err != nil || len(keys) != 2 {
		t.Fatalf("keys=%v err=%v", keys, err)
	}
	again, _ := repo.ClaimHealthChecks(ctx, time.Now().Add(-time.Minute), 10)
	if len(again) != 0 {
		t.Fatalf("already claimed within the interval: %v", again)
	}
}

type stubToolCaller struct{ calls int }

func (s *stubToolCaller) CallTool(context.Context, usecase.CallTarget, string, []byte, int) (usecase.CallResult, error) {
	s.calls++
	return usecase.CallResult{Text: "pong", SizeBytes: 4}, nil
}

func (s *stubToolCaller) ReadResource(context.Context, usecase.CallTarget, string, int) (usecase.ResourceResult, error) {
	s.calls++
	return usecase.ResourceResult{Text: "doc"}, nil
}

type fixedClock struct{}

func (fixedClock) Now() time.Time { return time.Now().UTC() }

func TestExternalServers_CallExternalToolRespectsTenantRLS(t *testing.T) {
	repo, _, admin := externalSetup(t)
	ctx := context.Background()
	s := newServer(tenantA, "srv")
	s.HeaderRefs = nil
	if err := repo.CreateExternalServer(ctx, s, nil); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	tools := []domain.ToolInfo{{Name: "ping", Description: "d"}}
	if err := repo.RecordProbe(ctx, tenantA, s.ID, usecase.ProbeRecord{Digest: "d1", Tools: tools, At: now, Source: "probe"}, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.ApplyReview(ctx, usecase.ReviewRecord{TenantID: tenantA, ServerID: s.ID, ReviewerID: "a", Approve: true, ExpectedDigest: "d1", At: now}, nil); err != nil {
		t.Fatal(err)
	}
	caller := &stubToolCaller{}
	cl := usecase.NewExternalServerClient(repo, nil, caller, repo, fixedClock{})

	if out, err := cl.CallTool(tenant.WithTenantID(ctx, tenantA), usecase.CallToolInput{ServerID: s.ID, Tool: "ping"}); err != nil || out.Text != "pong" {
		t.Fatalf("tenant A: %+v %v", out, err)
	}
	if _, err := cl.CallTool(tenant.WithTenantID(ctx, tenantB), usecase.CallToolInput{ServerID: s.ID, Tool: "ping"}); err == nil || !strings.Contains(err.Error(), domain.CodeNotFound) {
		t.Fatalf("tenant B must not reach tenant A's server: %v", err)
	}
	if _, err := cl.ReadResource(tenant.WithTenantID(ctx, tenantB), usecase.ReadResourceInput{ServerID: s.ID, URI: "file:///x"}); err == nil || !strings.Contains(err.Error(), domain.CodeNotFound) {
		t.Fatalf("tenant B read: %v", err)
	}
	if caller.calls != 1 {
		t.Fatalf("external server contacted %d times, want 1", caller.calls)
	}
	var n int
	if err := admin.QueryRow(ctx, `SELECT count(*) FROM mcp.outbox_events WHERE tenant_id = $1 AND payload->>'action' = $2`, tenantA, domain.AuditActionExternalCall).Scan(&n); err != nil || n != 1 {
		t.Fatalf("audit rows for tenant A: n=%d err=%v", n, err)
	}
	if err := admin.QueryRow(ctx, `SELECT count(*) FROM mcp.outbox_events WHERE tenant_id = $1`, tenantB).Scan(&n); err != nil || n != 0 {
		t.Fatalf("tenant B must have no events: n=%d err=%v", n, err)
	}
}
