//go:build integration

package postgres

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// withTenant runs fn in a transaction scoped like InTx does (set_config, transaction-local).
func withTenant(t *testing.T, pool *pgxpool.Pool, tenantID string, fn func(tx pgx.Tx)) {
	t.Helper()
	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if tenantID != "" {
		if _, err := tx.Exec(ctx, `SELECT set_config('app.tenant_id', $1, true)`, tenantID); err != nil {
			t.Fatal(err)
		}
	}
	fn(tx)
}

func countRows(t *testing.T, tx pgx.Tx, table string) int {
	t.Helper()
	var n int
	if err := tx.QueryRow(context.Background(), `SELECT count(*) FROM request.`+table).Scan(&n); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return n
}

func TestPostgres_RLS_DirectSQLCannotSeeOtherTenant(t *testing.T) {
	f := newMigratedPostgres(t)
	ctx := context.Background()
	tenantA, tenantB := uuid.NewString(), uuid.NewString()
	reqA, reqB := uuid.NewString(), uuid.NewString()

	// Seed one row per tenant in tables that matter, through the non-superuser role.
	for tenantID, reqID := range map[string]string{tenantA: reqA, tenantB: reqB} {
		withTenantCommit(t, f.app, tenantID, func(tx pgx.Tx) {
			mustExec(t, tx, `INSERT INTO request.requests (id, tenant_id, number, title, source_provider, status, urgency, reporter_id)
				VALUES ($1, $2, 1, 't', 'manual', 'new', 'normal', gen_random_uuid())`, reqID, tenantID)
			mustExec(t, tx, `INSERT INTO request.analysis_runs (id, tenant_id, request_id, kind, mode, status)
				VALUES (gen_random_uuid(), $1, $2, 'solution', 'complete', 'running')`, tenantID, reqID)
			mustExec(t, tx, `INSERT INTO request.outbox_events (id, tenant_id, subject, occurred_at, version, payload)
				VALUES (gen_random_uuid(), $1, 'orca.request.request.created', now(), 1, '{}')`, tenantID)
		})
	}

	for _, table := range []string{"requests", "analysis_runs", "outbox_events"} {
		withTenant(t, f.app, tenantA, func(tx pgx.Tx) {
			if n := countRows(t, tx, table); n != 1 {
				t.Errorf("%s: tenant A sees %d rows, want 1", table, n)
			}
		})
		// No tenant set: zero rows and no cast error (NULLIF guards the empty GUC).
		withTenant(t, f.app, "", func(tx pgx.Tx) {
			if n := countRows(t, tx, table); n != 0 {
				t.Errorf("%s: no-tenant session sees %d rows, want 0", table, n)
			}
		})
	}

	// WITH CHECK: tenant A cannot write a row that belongs to B.
	for name, stmt := range map[string]string{
		"requests": `INSERT INTO request.requests (id, tenant_id, number, title, source_provider, status, urgency, reporter_id)
			VALUES (gen_random_uuid(), $1, 9, 't', 'manual', 'new', 'normal', gen_random_uuid())`,
		"analysis_runs": `INSERT INTO request.analysis_runs (id, tenant_id, request_id, kind, mode, status)
			VALUES (gen_random_uuid(), $1, $2, 'solution', 'complete', 'running')`,
	} {
		withTenant(t, f.app, tenantA, func(tx pgx.Tx) {
			args := []any{tenantB}
			if name == "analysis_runs" {
				args = append(args, reqA)
			}
			if _, err := tx.Exec(ctx, stmt, args...); err == nil {
				t.Errorf("%s: tenant A inserted a row for tenant B", name)
			}
		})
	}

	// Updates and deletes silently match nothing across tenants.
	withTenant(t, f.app, tenantA, func(tx pgx.Tx) {
		tag, err := tx.Exec(ctx, `UPDATE request.requests SET title = 'pwned' WHERE id = $1`, reqB)
		if err != nil || tag.RowsAffected() != 0 {
			t.Errorf("cross-tenant update affected %d rows (err %v)", tag.RowsAffected(), err)
		}
		tag, err = tx.Exec(ctx, `DELETE FROM request.analysis_runs WHERE request_id = $1`, reqB)
		if err != nil || tag.RowsAffected() != 0 {
			t.Errorf("cross-tenant delete affected %d rows (err %v)", tag.RowsAffected(), err)
		}
	})

	// The role really is subject to RLS (guards against running these checks as a superuser).
	var super, bypass bool
	if err := f.admin.QueryRow(ctx, `SELECT rolsuper, rolbypassrls FROM pg_roles WHERE rolname = $1`, appRole).Scan(&super, &bypass); err != nil || super || bypass {
		t.Fatalf("app role must be NOSUPERUSER NOBYPASSRLS: super=%v bypass=%v err=%v", super, bypass, err)
	}
}

func withTenantCommit(t *testing.T, pool *pgxpool.Pool, tenantID string, fn func(tx pgx.Tx)) {
	t.Helper()
	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	mustExec(t, tx, `SELECT set_config('app.tenant_id', $1, true)`, tenantID)
	fn(tx)
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
}

func mustExec(t *testing.T, tx pgx.Tx, sql string, args ...any) {
	t.Helper()
	if _, err := tx.Exec(context.Background(), sql, args...); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
}
