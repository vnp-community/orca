//go:build integration

package postgres

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/stablyai/orca-go/common/testutil"
	"github.com/stablyai/orca-go/services/mcp-service/internal/domain"
	"github.com/stablyai/orca-go/services/mcp-service/internal/usecase"
)

const (
	userA1 = "cccccccc-0000-4000-8000-000000000001"
	userA2 = "cccccccc-0000-4000-8000-000000000002"
)

// setupAuthz applies 0001+0002 as owner and returns a repository on the
// non-superuser app role, so RLS really applies.
func setupAuthz(t *testing.T) (*Repository, *pgxpool.Pool) {
	t.Helper()
	dsn := testutil.StartPostgres(t, "mcp")
	ctx := context.Background()
	conn := adminConn(t, dsn)
	execScript(t, ctx, conn, readMigration(t, "0001_init.up.sql"))
	execScript(t, ctx, conn, readMigration(t, "0002_authorization.up.sql"))
	execScript(t, ctx, conn, fmt.Sprintf(`
		CREATE ROLE %[1]s LOGIN PASSWORD '%[2]s' NOSUPERUSER NOBYPASSRLS;
		GRANT USAGE ON SCHEMA mcp TO %[1]s;
		GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA mcp TO %[1]s;`, appRole, appPass))
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.User, cfg.ConnConfig.Password = appRole, appPass
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return New(pool), pool
}

func consent(tenantID, userID, client string, exp time.Time) domain.ConsentRequest {
	return domain.ConsentRequest{
		ID: uuid.NewString(), TenantID: tenantID, UserID: userID, ClientID: client, ClientName: "App", ClientURI: "https://app.example.com",
		RedirectURI: "https://app.example.com/cb", Scopes: []string{"orca:read", "orca:write"}, State: "s", CodeChallenge: "c",
		Resource: "https://orca.example.com/mcp", IsNewClient: true, RegisteredViaDCR: true, CreatedAt: time.Now().UTC(), ExpiresAt: exp,
	}
}

func approve(t *testing.T, r *Repository, c domain.ConsentRequest, scopes ...string) usecase.ApproveConsentResult {
	t.Helper()
	res, err := r.ApproveConsent(context.Background(), usecase.ApproveConsentInput{
		TenantID: c.TenantID, UserID: c.UserID, RequestID: c.ID, Scopes: scopes, Now: time.Now().UTC(),
		GrantID: uuid.NewString(), EventID: uuid.NewString(),
	})
	if err != nil {
		t.Fatalf("approve: %v", err)
	}
	return res
}

func outboxSubjects(t *testing.T, r *Repository) []string {
	t.Helper()
	recs, err := r.FetchUnpublished(context.Background(), 100)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, rec := range recs {
		out = append(out, rec.Subject)
	}
	return out
}

func TestAuthorizationMigration_UpDownUp(t *testing.T) {
	dsn := testutil.StartPostgres(t, "mcp")
	ctx := context.Background()
	conn := adminConn(t, dsn)
	execScript(t, ctx, conn, readMigration(t, "0001_init.up.sql"))
	up, down := readMigration(t, "0002_authorization.up.sql"), readMigration(t, "0002_authorization.down.sql")
	for _, script := range []string{up, down, up} {
		execScript(t, ctx, conn, script)
	}
	var n int
	if err := conn.QueryRow(ctx, `SELECT count(*) FROM information_schema.tables WHERE table_schema='mcp' AND table_name IN ('grants','consent_requests')`).Scan(&n); err != nil || n != 2 {
		t.Fatalf("tables after up-down-up = %d err = %v", n, err)
	}
}

func TestConsent_RoundTripAndIsolation(t *testing.T) {
	r, _ := setupAuthz(t)
	ctx := context.Background()
	c := consent(tenantA, userA1, "app-1", time.Now().Add(time.Minute))
	if err := r.CreateConsentRequest(ctx, c); err != nil {
		t.Fatal(err)
	}
	got, err := r.GetConsentRequest(ctx, tenantA, userA1, c.ID)
	if err != nil || got.ClientName != "App" || got.State != "s" || len(got.Scopes) != 2 || got.ClientURI != "https://app.example.com" || !got.IsNewClient {
		t.Fatalf("got %+v err %v", got, err)
	}
	if _, err := r.GetConsentRequest(ctx, tenantA, userA2, c.ID); err != usecase.ErrConsentRequestNotFound {
		t.Fatalf("other user: %v", err)
	}
	if _, err := r.GetConsentRequest(ctx, tenantB, userA1, c.ID); err != usecase.ErrConsentRequestNotFound {
		t.Fatalf("other tenant: %v", err)
	}
}

func TestRLS_NoTenantContextSeesNothing(t *testing.T) {
	r, pool := setupAuthz(t)
	ctx := context.Background()
	c := consent(tenantA, userA1, "app-1", time.Now().Add(time.Minute))
	_ = r.CreateConsentRequest(ctx, c)
	approve(t, r, c, "orca:read")

	for _, table := range []string{"mcp.grants", "mcp.consent_requests"} {
		var n int
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM `+table).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n != 0 {
			t.Fatalf("%s: %d rows visible without a tenant context", table, n)
		}
	}
}

func TestApproveConsent_ConcurrentDecisionsOnlyOneWins(t *testing.T) {
	r, _ := setupAuthz(t)
	ctx := context.Background()
	c := consent(tenantA, userA1, "app-1", time.Now().Add(time.Minute))
	if err := r.CreateConsentRequest(ctx, c); err != nil {
		t.Fatal(err)
	}

	var won, lost atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := r.ApproveConsent(ctx, usecase.ApproveConsentInput{
				TenantID: tenantA, UserID: userA1, RequestID: c.ID, Scopes: []string{"orca:read"}, Now: time.Now().UTC(),
				GrantID: uuid.NewString(), EventID: uuid.NewString(),
			})
			switch err {
			case nil:
				won.Add(1)
			case usecase.ErrConsentNotDecidable:
				lost.Add(1)
			default:
				t.Errorf("unexpected: %v", err)
			}
		}()
	}
	wg.Wait()
	if won.Load() != 1 || lost.Load() != 11 {
		t.Fatalf("won=%d lost=%d", won.Load(), lost.Load())
	}
	grants, _ := r.ListActiveGrants(ctx, tenantA, userA1)
	if len(grants) != 1 {
		t.Fatalf("%d grants, want 1", len(grants))
	}
	if subjects := outboxSubjects(t, r); len(subjects) != 1 || subjects[0] != domain.SubjectGrantCreated {
		t.Fatalf("outbox = %v, want exactly one created event", subjects)
	}
}

func TestApproveConsent_ExpiredOrWrongOwnerIsNotDecidable(t *testing.T) {
	r, _ := setupAuthz(t)
	ctx := context.Background()
	expired := consent(tenantA, userA1, "app-1", time.Now().Add(-time.Second))
	live := consent(tenantA, userA1, "app-2", time.Now().Add(time.Minute))
	_ = r.CreateConsentRequest(ctx, expired)
	_ = r.CreateConsentRequest(ctx, live)

	in := func(c domain.ConsentRequest, tenantID, userID string) usecase.ApproveConsentInput {
		return usecase.ApproveConsentInput{TenantID: tenantID, UserID: userID, RequestID: c.ID, Scopes: []string{"orca:read"},
			Now: time.Now().UTC(), GrantID: uuid.NewString(), EventID: uuid.NewString()}
	}
	for name, i := range map[string]usecase.ApproveConsentInput{
		"expired": in(expired, tenantA, userA1), "wrong user": in(live, tenantA, userA2), "wrong tenant": in(live, tenantB, userA1),
	} {
		if _, err := r.ApproveConsent(ctx, i); err != usecase.ErrConsentNotDecidable {
			t.Errorf("%s: %v", name, err)
		}
	}
	if grants, _ := r.ListActiveGrants(ctx, tenantA, ""); len(grants) != 0 {
		t.Fatal("a failed decision must leave no grant")
	}
	if len(outboxSubjects(t, r)) != 0 {
		t.Fatal("a failed decision must leave no event")
	}
}

func TestDenyConsent(t *testing.T) {
	r, _ := setupAuthz(t)
	ctx := context.Background()
	c := consent(tenantA, userA1, "app-1", time.Now().Add(time.Minute))
	_ = r.CreateConsentRequest(ctx, c)
	if _, err := r.DenyConsent(ctx, tenantA, userA2, c.ID, time.Now()); err != usecase.ErrConsentNotDecidable {
		t.Fatalf("other user denied: %v", err)
	}
	if _, err := r.DenyConsent(ctx, tenantA, userA1, c.ID, time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err := r.DenyConsent(ctx, tenantA, userA1, c.ID, time.Now()); err != usecase.ErrConsentNotDecidable {
		t.Fatalf("second decision: %v", err)
	}
	if grants, _ := r.ListActiveGrants(ctx, tenantA, ""); len(grants) != 0 {
		t.Fatal("deny created a grant")
	}
}

func TestGrants_OneActivePerUserAndClientAndUpsertSemantics(t *testing.T) {
	r, _ := setupAuthz(t)
	ctx := context.Background()
	c1 := consent(tenantA, userA1, "app-1", time.Now().Add(time.Minute))
	c2 := consent(tenantA, userA1, "app-1", time.Now().Add(time.Minute))
	_ = r.CreateConsentRequest(ctx, c1)
	_ = r.CreateConsentRequest(ctx, c2)

	first := approve(t, r, c1, "orca:read", "orca:write")
	second := approve(t, r, c2, "orca:exec")
	if !first.Created || second.Created || second.Grant.ID != first.Grant.ID {
		t.Fatalf("first=%+v second=%+v: second approval must update the same grant", first, second)
	}
	if got, _ := r.GetActiveGrant(ctx, tenantA, userA1, "app-1"); len(got.Scopes) != 1 || got.Scopes[0] != "orca:exec" {
		t.Fatalf("scopes = %v, want the last chosen set only", got.Scopes)
	}
	if subjects := outboxSubjects(t, r); len(subjects) != 2 || subjects[0] != domain.SubjectGrantCreated || subjects[1] != domain.SubjectGrantUpdated {
		t.Fatalf("outbox = %v", subjects)
	}

	// The database itself refuses a second active grant for the same triple.
	// The insert runs with the tenant set, so the only thing that can reject it
	// is the partial unique index (SQLSTATE 23505), not RLS.
	err := r.withTenantTx(ctx, tenantA, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `INSERT INTO mcp.grants (id, tenant_id, user_id, client_id, client_name, scopes, status)
			VALUES ($1, $2, $3, 'app-1', 'App', '{orca:read}', 'active')`, uuid.NewString(), tenantA, userA1)
		return err
	})
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23505" {
		t.Fatalf("want unique violation 23505 from uq_grants_active, got %v", err)
	}
}

func TestRevokeGrant_ScopingIdempotencyAndEvent(t *testing.T) {
	r, _ := setupAuthz(t)
	ctx := context.Background()
	c := consent(tenantA, userA1, "app-1", time.Now().Add(time.Minute))
	_ = r.CreateConsentRequest(ctx, c)
	g := approve(t, r, c, "orca:read").Grant
	now := time.Now().UTC()

	if _, _, err := r.RevokeGrant(ctx, tenantA, g.ID, userA2, userA2, now, uuid.NewString()); err != usecase.ErrGrantNotFound {
		t.Fatalf("other owner: %v", err)
	}
	if _, _, err := r.RevokeGrant(ctx, tenantB, g.ID, "", userA2, now, uuid.NewString()); err != usecase.ErrGrantNotFound {
		t.Fatalf("other tenant: %v", err)
	}
	got, newly, err := r.RevokeGrant(ctx, tenantA, g.ID, userA1, userA1, now, uuid.NewString())
	if err != nil || !newly || got.Status != domain.GrantRevoked || got.RevokedBy != userA1 {
		t.Fatalf("revoke = %+v newly=%v err=%v", got, newly, err)
	}
	again, newly, err := r.RevokeGrant(ctx, tenantA, g.ID, userA1, userA1, now, uuid.NewString())
	if err != nil || newly || again.Status != domain.GrantRevoked {
		t.Fatalf("second revoke = %+v newly=%v err=%v", again, newly, err)
	}
	subjects := outboxSubjects(t, r)
	if len(subjects) != 2 || subjects[1] != domain.SubjectGrantRevoked {
		t.Fatalf("outbox = %v, want created + one revoked", subjects)
	}
	if grants, _ := r.ListActiveGrants(ctx, tenantA, ""); len(grants) != 0 {
		t.Fatal("revoked grant still listed active")
	}
	if n, _ := r.CountGrantsForClient(ctx, tenantA, "app-1"); n != 1 {
		t.Fatalf("history count = %d, want 1 (revoked grants still count as 'seen')", n)
	}

	// After revoking, the user can be granted again (partial unique index).
	c2 := consent(tenantA, userA1, "app-1", time.Now().Add(time.Minute))
	_ = r.CreateConsentRequest(ctx, c2)
	if res := approve(t, r, c2, "orca:read"); !res.Created || res.Grant.ID == g.ID {
		t.Fatalf("re-grant must create a new row: %+v", res)
	}
}

func TestReconcile_CrossTenantScanSeesOnlyPendingRevocations(t *testing.T) {
	r, pool := setupAuthz(t)
	ctx := context.Background()
	ids := map[string]string{}
	for _, tenantID := range []string{tenantA, tenantB} {
		c := consent(tenantID, userA1, "app-1", time.Now().Add(time.Minute))
		_ = r.CreateConsentRequest(ctx, c)
		g := approve(t, r, c, "orca:read").Grant
		ids[tenantID] = g.ID
		if _, _, err := r.RevokeGrant(ctx, tenantID, g.ID, "", userA1, time.Now().UTC(), uuid.NewString()); err != nil {
			t.Fatal(err)
		}
	}
	// A still-active grant must never show up in the scan.
	live := consent(tenantA, userA2, "app-9", time.Now().Add(time.Minute))
	_ = r.CreateConsentRequest(ctx, live)
	approve(t, r, live, "orca:read")

	pending, err := r.ListUnpropagatedRevocations(ctx, 10)
	if err != nil || len(pending) != 2 {
		t.Fatalf("pending = %+v err = %v", pending, err)
	}
	for _, g := range pending {
		if g.Status != domain.GrantRevoked || g.RevocationPropagatedAt != nil {
			t.Fatalf("scan leaked %+v", g)
		}
	}

	if err := r.MarkRevocationPropagated(ctx, tenantA, ids[tenantA], time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	pending, _ = r.ListUnpropagatedRevocations(ctx, 10)
	if len(pending) != 1 || pending[0].TenantID != tenantB {
		t.Fatalf("after mark: %+v", pending)
	}
	// Marking with the wrong tenant is a no-op, never a cross-tenant write.
	if err := r.MarkRevocationPropagated(ctx, tenantA, ids[tenantB], time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if pending, _ = r.ListUnpropagatedRevocations(ctx, 10); len(pending) != 1 {
		t.Fatal("cross-tenant mark changed tenantB's grant")
	}

	// Without the relay opt-in the same scan sees nothing (RLS).
	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM mcp.grants WHERE status = 'revoked'`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("plain connection saw %d revoked grants (err %v)", n, err)
	}
}

func TestListAndCounts_TenantScoped(t *testing.T) {
	r, _ := setupAuthz(t)
	ctx := context.Background()
	mk := func(tenantID, userID, client string) {
		c := consent(tenantID, userID, client, time.Now().Add(time.Minute))
		_ = r.CreateConsentRequest(ctx, c)
		approve(t, r, c, "orca:read")
	}
	mk(tenantA, userA1, "app-1")
	mk(tenantA, userA2, "app-1")
	mk(tenantA, userA1, "app-2")
	mk(tenantB, userA1, "app-1")

	if all, _ := r.ListActiveGrants(ctx, tenantA, ""); len(all) != 3 {
		t.Fatalf("tenant list = %d, want 3", len(all))
	}
	if mine, _ := r.ListActiveGrants(ctx, tenantA, userA1); len(mine) != 2 {
		t.Fatalf("user list = %d, want 2", len(mine))
	}
	counts, _ := r.CountActiveGrantsByClient(ctx, tenantA)
	if counts["app-1"] != 2 || counts["app-2"] != 1 {
		t.Fatalf("counts = %v", counts)
	}
	if _, err := r.GetActiveGrant(ctx, tenantB, userA2, "app-1"); err != usecase.ErrGrantNotFound {
		t.Fatalf("cross-tenant get: %v", err)
	}
}
