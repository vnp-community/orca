//go:build integration

package postgres

import (
	"context"
	"errors"
	"sort"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Meta-tests read the live schema, so a table added by any later migration is checked without editing a list.

func requestTables(t *testing.T, conn *pgx.Conn) []string {
	t.Helper()
	rows, err := conn.Query(context.Background(), `
		SELECT c.relname FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = 'request' AND c.relkind = 'r' ORDER BY 1`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			t.Fatal(err)
		}
		out = append(out, s)
	}
	return out
}

func TestEveryRequestTableHasForcedRLS(t *testing.T) {
	f := newMigratedPostgres(t)
	// No exemptions: a table that needs one must be listed here with the reason.
	rows, err := f.admin.Query(context.Background(), `
		SELECT c.relname FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = 'request' AND c.relkind = 'r' AND (NOT c.relrowsecurity OR NOT c.relforcerowsecurity)`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		_ = rows.Scan(&name)
		t.Errorf("table %s lacks ENABLE or FORCE ROW LEVEL SECURITY", name)
	}
}

func TestEveryRequestTableHasTenantPolicyWithCheck(t *testing.T) {
	f := newMigratedPostgres(t)
	for _, table := range requestTables(t, f.admin) {
		var n int
		err := f.admin.QueryRow(context.Background(), `
			SELECT count(*) FROM pg_policies
			WHERE schemaname = 'request' AND tablename = $1 AND cmd = 'ALL'
			  AND qual LIKE '%app.tenant_id%' AND with_check LIKE '%app.tenant_id%'`, table).Scan(&n)
		if err != nil {
			t.Fatal(err)
		}
		if n == 0 {
			t.Errorf("table %s has no FOR ALL tenant policy with both USING and WITH CHECK on app.tenant_id", table)
		}
	}
}

func TestEveryRequestTableHasTenantID(t *testing.T) {
	f := newMigratedPostgres(t)
	for _, table := range requestTables(t, f.admin) {
		var nullable string
		err := f.admin.QueryRow(context.Background(), `
			SELECT is_nullable FROM information_schema.columns
			WHERE table_schema = 'request' AND table_name = $1 AND column_name = 'tenant_id'`, table).Scan(&nullable)
		if err != nil || nullable != "NO" {
			t.Errorf("table %s: tenant_id NOT NULL missing (%v, nullable=%q)", table, err, nullable)
		}
	}
}

func TestAppRoleCannotBypassRLS(t *testing.T) {
	f := newMigratedPostgres(t)
	var super, bypass bool
	if err := f.app.QueryRow(context.Background(), `SELECT rolsuper, rolbypassrls FROM pg_roles WHERE rolname = current_user`).Scan(&super, &bypass); err != nil {
		t.Fatal(err)
	}
	if super || bypass {
		t.Fatalf("the test connection must be NOSUPERUSER NOBYPASSRLS (super=%v bypass=%v), or RLS tests prove nothing", super, bypass)
	}
}

func TestAppRoleHasPrivilegesOnAllTables(t *testing.T) {
	f := newMigratedPostgres(t)
	for _, table := range requestTables(t, f.admin) {
		var ok bool
		err := f.app.QueryRow(context.Background(), `SELECT has_table_privilege(current_user, 'request.' || $1, 'SELECT,INSERT,UPDATE,DELETE')`, table).Scan(&ok)
		if err != nil || !ok {
			t.Errorf("request_app lacks SELECT/INSERT/UPDATE/DELETE on %s (a new migration must GRANT)", table)
		}
	}
}

// isolationFixtures holds one INSERT per table; {tenant}, {req}, {approval} and {pack} are filled per tenant. A table without one fails the test, so
// whoever adds a table must also prove its isolation here.
var isolationFixtures = map[string]string{
	"analysis_runs":             `INSERT INTO request.analysis_runs (id, tenant_id, request_id, kind, mode, status) VALUES (gen_random_uuid(), {tenant}, {req}, 'solution', 'complete', 'running')`,
	"approval_approvers":        `INSERT INTO request.approval_approvers (approval_id, principal_kind, principal_id, tenant_id) VALUES ({approval}, 'user', 'u', {tenant})`,
	"approval_policies":         `INSERT INTO request.approval_policies (id, tenant_id, subject_type, approvers, created_by) VALUES (gen_random_uuid(), {tenant}, 'plan', '[]', 'u')`,
	"approvals":                 `INSERT INTO request.approvals (id, tenant_id, request_id, subject_type, subject_id, stage, status, requested_by, subject_digest) VALUES ({approval}, {tenant}, {req}, 'plan', 's', 'plan', 'pending', 'u', 'd')`,
	"classification_runs":       `INSERT INTO request.classification_runs (id, tenant_id, request_id, trigger_name, actor_id, status, lease_expires_at) VALUES (gen_random_uuid(), {tenant}, {req}, 't', gen_random_uuid(), 'running', now())`,
	"context_packs":             `INSERT INTO request.context_packs (id, tenant_id, request_id, stage, cp_version, input_digest, digest, budget_tokens, used_tokens, items, missing, body) VALUES ({pack}, {tenant}, {req}, 'solution', '1', repeat('a', 64), repeat('b', 64), 1, 1, '[]', '[]', '')`,
	"context_sources":           `INSERT INTO request.context_sources (id, tenant_id, source_key, kind, transport, adapter, trust, enabled_for, owner_id, created_by) VALUES (gen_random_uuid(), {tenant}, 'src', 'conventions', 'internal', 'a', 'high', '[]', gen_random_uuid(), gen_random_uuid())`,
	"evidence":                  `INSERT INTO request.evidence (id, tenant_id, request_id, context_pack_id, seq, source_id, ref, retrieved_at, freshness, trust, digest, size) VALUES (gen_random_uuid(), {tenant}, {req}, {pack}, 1, 's', 'r', now(), 'fresh', 'high', repeat('c', 64), 1)`,
	"openspec_changes":          `INSERT INTO request.openspec_changes (id, tenant_id, request_id, change_id, status, tasks_sync_state) VALUES (gen_random_uuid(), {tenant}, {req}, 'c', 'ready', 'in_sync')`,
	"outbox_events":             `INSERT INTO request.outbox_events (id, tenant_id, subject, occurred_at, version, payload) VALUES (gen_random_uuid(), {tenant}, 's', now(), 1, '{}')`,
	"processed_events":          `INSERT INTO request.processed_events (tenant_id, event_id, subject) VALUES ({tenant}, gen_random_uuid(), 's')`,
	"project_engine_settings":   `INSERT INTO request.project_engine_settings (tenant_id, project_id, solution_engine, updated_by) VALUES ({tenant}, gen_random_uuid(), 'native', gen_random_uuid())`,
	"request_audit_outbox":      `INSERT INTO request.request_audit_outbox (id, tenant_id, audit_id, action, actor_id, actor_type, target_type, target_id, outcome) VALUES (gen_random_uuid(), {tenant}, gen_random_uuid(), 'a', 'u', 'user', 'request', 'r', 'allowed')`,
	"request_counters":          `INSERT INTO request.request_counters (tenant_id, next_number) VALUES ({tenant}, 1)`,
	"request_idempotency":       `INSERT INTO request.request_idempotency (tenant_id, source_provider, source_ref, request_id) VALUES ({tenant}, 'jira', 'K-1', gen_random_uuid())`,
	"request_links":             `INSERT INTO request.request_links (tenant_id, parent_request_id, child_request_id) VALUES ({tenant}, gen_random_uuid(), gen_random_uuid())`,
	"request_return_history":    `INSERT INTO request.request_return_history (id, tenant_id, request_id, action, actor_kind) VALUES (gen_random_uuid(), {tenant}, gen_random_uuid(), 'returned', 'user')`,
	"request_security_flags":    `INSERT INTO request.request_security_flags (tenant_id, request_id) VALUES ({tenant}, gen_random_uuid())`,
	"request_type_history":      `INSERT INTO request.request_type_history (tenant_id, request_id, to_type, actor_id) VALUES ({tenant}, gen_random_uuid(), 'bug', gen_random_uuid())`,
	"request_webhook_nonces":    `INSERT INTO request.request_webhook_nonces (tenant_id, source, nonce_hash, expires_at) VALUES ({tenant}, 's', repeat('d', 64), now() + interval '10 minutes')`,
	"requests":                  `INSERT INTO request.requests (id, tenant_id, number, title, source_provider, reporter_id) VALUES ({req}, {tenant}, 1, 't', 'manual', gen_random_uuid())`,
	"solutions":                 `INSERT INTO request.solutions (id, tenant_id, request_id, options, seq) VALUES (gen_random_uuid(), {tenant}, {req}, '{}', 1)`,
	"analysis_project_gates":    `INSERT INTO request.analysis_project_gates (tenant_id, project_id) VALUES ({tenant}, gen_random_uuid())`,
	"artifact_index":            `INSERT INTO request.artifact_index (tenant_id, display_id, kind, request_id, artifact_id) VALUES ({tenant}, 'REQ-1', 'request', {req}, {req})`,
	"artifact_relations":        `INSERT INTO request.artifact_relations (id, tenant_id, request_id, rel, from_kind, from_id, to_kind, to_id) VALUES (gen_random_uuid(), {tenant}, {req}, 'derived_from', 'solution', 'a', 'request', 'b')`,
	"clarification_assignees":   `INSERT INTO request.clarification_assignees (clarification_id, tenant_id, principal_kind, principal_id) VALUES ({clar}, {tenant}, 'user', 'u')`,
	"clarification_questions":   `INSERT INTO request.clarification_questions (id, tenant_id, clarification_id, seq, question_key, kind, prompt, reason) VALUES (gen_random_uuid(), {tenant}, {clar}, 1, 'k', 'text', 'p', 'r')`,
	"clarifications":            `INSERT INTO request.clarifications (id, tenant_id, request_id, seq, source, status, resume_status, asked_request_revision, due_at, created_by) VALUES ({clar}, {tenant}, {req}, 1, 'manual', 'open', 'analyzing', 1, now(), 'u')`,
	"decision_history":          `INSERT INTO request.decision_history (id, tenant_id, decision_id, action) VALUES (gen_random_uuid(), {tenant}, {dec}, 'chosen')`,
	"decisions":                 `INSERT INTO request.decisions (id, tenant_id, request_id, seq, subject_kind, subject_id, options, status) VALUES ({dec}, {tenant}, {req}, 1, 'other', 's', '[]', 'open')`,
	"request_coverage":          `INSERT INTO request.request_coverage (id, tenant_id, request_id, plan_task_id, ac_id, task_id) VALUES (gen_random_uuid(), {tenant}, {req}, gen_random_uuid(), 'AC-1', gen_random_uuid())`,
	"request_revisions":         `INSERT INTO request.request_revisions (id, tenant_id, request_id, revision, cause, snapshot, digest, actor_kind) VALUES (gen_random_uuid(), {tenant}, {req}, 1, 'created', '{}', 'd', 'user')`,
	"execution_reconcile_state": `INSERT INTO request.execution_reconcile_state (tenant_id, request_id) VALUES ({tenant}, {req})`,
	"phase_starts":              `INSERT INTO request.phase_starts (tenant_id, phase_task_id, request_id, started_by) VALUES ({tenant}, gen_random_uuid(), {req}, gen_random_uuid())`,
	"request_checks":            `INSERT INTO request.request_checks (id, tenant_id, request_id, kind, status, source) VALUES (gen_random_uuid(), {tenant}, {req}, 'tests_after', 'passed', 'manual')`,
	"task_run_outcomes":         `INSERT INTO request.task_run_outcomes (id, tenant_id, request_id, task_id, outcome, event_id, occurred_at) VALUES (gen_random_uuid(), {tenant}, {req}, gen_random_uuid(), 'started', gen_random_uuid(), now())`,
	"tenant_settings":           `INSERT INTO request.tenant_settings (tenant_id) VALUES ({tenant})`,
	"tenant_security_settings":  `INSERT INTO request.tenant_security_settings (tenant_id) VALUES ({tenant})`,
}

type seedIDs struct{ tenant, req, approval, pack, clar, dec string }

func newSeedIDs(tenantID string) seedIDs {
	return seedIDs{tenant: tenantID, req: uuid.NewString(), approval: uuid.NewString(), pack: uuid.NewString(), clar: uuid.NewString(), dec: uuid.NewString()}
}

func (ids seedIDs) sql(table string) string {
	return strings.NewReplacer("{tenant}", "'"+ids.tenant+"'", "{req}", "'"+ids.req+"'", "{approval}", "'"+ids.approval+"'", "{pack}", "'"+ids.pack+"'", "{clar}", "'"+ids.clar+"'", "{dec}", "'"+ids.dec+"'").Replace(isolationFixtures[table])
}

// seedOrder puts parents of the in-schema foreign keys first.
func seedOrder(tables []string) []string {
	first := []string{"requests", "approvals", "approval_approvers", "context_packs", "evidence", "clarifications", "clarification_questions", "decisions", "decision_history"}
	rest := map[string]bool{}
	for _, t := range tables {
		rest[t] = true
	}
	var out []string
	for _, t := range first {
		if rest[t] {
			out = append(out, t)
			delete(rest, t)
		}
	}
	var others []string
	for t := range rest {
		others = append(others, t)
	}
	sort.Strings(others)
	return append(out, others...)
}

func seedTenant(t *testing.T, f *pgFixture, ids seedIDs) {
	t.Helper()
	withTenantCommit(t, f.app, ids.tenant, func(tx pgx.Tx) {
		for _, table := range seedOrder(requestTables(t, f.admin)) {
			mustExec(t, tx, ids.sql(table))
		}
	})
}

func isRLSViolation(err error) bool {
	var pe *pgconn.PgError
	return errors.As(err, &pe) && pe.Code == "42501"
}

func countAs(t *testing.T, f *pgFixture, tenantID, table string) int {
	t.Helper()
	n := 0
	withTenant(t, f.app, tenantID, func(tx pgx.Tx) { n = countRows(t, tx, table) })
	return n
}

func TestEveryTableHasAnIsolationFixture(t *testing.T) {
	f := newMigratedPostgres(t)
	var missing []string
	tables := requestTables(t, f.admin)
	for _, table := range tables {
		if _, ok := isolationFixtures[table]; !ok {
			missing = append(missing, table)
		}
	}
	for table := range isolationFixtures {
		found := false
		for _, have := range tables {
			found = found || have == table
		}
		if !found {
			t.Errorf("fixture for %s but no such table (stale)", table)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Fatalf("tables without a tenant isolation fixture, add one: %s", strings.Join(missing, ", "))
	}
}

func TestTenantIsolationEveryTable(t *testing.T) {
	f := newMigratedPostgres(t)
	ctx := context.Background()
	a, b := newSeedIDs(uuid.NewString()), newSeedIDs(uuid.NewString())
	seedTenant(t, f, a)

	for _, table := range requestTables(t, f.admin) {
		if n := countAs(t, f, a.tenant, table); n < 1 {
			t.Errorf("%s: tenant A sees %d rows, want at least 1", table, n)
		}
		if n := countAs(t, f, b.tenant, table); n != 0 {
			t.Errorf("%s: tenant B sees %d of A's rows", table, n)
		}
		withTenant(t, f.app, b.tenant, func(tx pgx.Tx) {
			tag, err := tx.Exec(ctx, `UPDATE request.`+table+` SET tenant_id = tenant_id`)
			if err != nil || tag.RowsAffected() != 0 {
				t.Errorf("%s: tenant B UPDATE touched %d rows (err %v)", table, tag.RowsAffected(), err)
			}
			tag, err = tx.Exec(ctx, `DELETE FROM request.`+table)
			if err != nil || tag.RowsAffected() != 0 {
				t.Errorf("%s: tenant B DELETE removed %d rows (err %v)", table, tag.RowsAffected(), err)
			}
		})
		// WITH CHECK: tenant B cannot write a row that belongs to tenant A.
		withTenant(t, f.app, b.tenant, func(tx pgx.Tx) {
			fresh := newSeedIDs(a.tenant)
			fresh.req, fresh.approval, fresh.pack, fresh.clar, fresh.dec = a.req, a.approval, a.pack, a.clar, a.dec
			if _, err := tx.Exec(ctx, fresh.sql(table)); !isRLSViolation(err) {
				t.Errorf("%s: writing a tenant A row as tenant B must violate the policy, got %v", table, err)
			}
		})
	}
	// The same seed in tenant B works: the rows are separate, not shared.
	seedTenant(t, f, b)
	for _, table := range requestTables(t, f.admin) {
		if na, nb := countAs(t, f, a.tenant, table), countAs(t, f, b.tenant, table); na != nb {
			t.Errorf("%s: tenants A and B see %d and %d rows after identical seeds", table, na, nb)
		}
	}
}

// A transaction that never set app.tenant_id sees nothing and writes nothing: no leak, no wide-open default.
func TestForgottenSetConfigReturnsNothing(t *testing.T) {
	f := newMigratedPostgres(t)
	ctx := context.Background()
	a := newSeedIDs(uuid.NewString())
	seedTenant(t, f, a)
	for _, table := range requestTables(t, f.admin) {
		withTenant(t, f.app, "", func(tx pgx.Tx) {
			if n := countRows(t, tx, table); n != 0 {
				t.Errorf("%s: a transaction without app.tenant_id saw %d rows", table, n)
			}
			if _, err := tx.Exec(ctx, a.sql(table)); !isRLSViolation(err) {
				t.Errorf("%s: write without app.tenant_id must be refused, got %v", table, err)
			}
		})
	}
}

// The outbox relay policy exposes only unpublished rows across tenants and allows marking them published, never deleting.
func TestOutboxRelayPolicyOnlyFetchMark(t *testing.T) {
	f := newMigratedPostgres(t)
	ctx := context.Background()
	tenantA, tenantB := uuid.NewString(), uuid.NewString()
	for _, tn := range []string{tenantA, tenantB} {
		withTenantCommit(t, f.app, tn, func(tx pgx.Tx) { mustExec(t, tx, newSeedIDs(tn).sql("outbox_events")) })
	}
	tx, err := f.app.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	mustExec(t, tx, `SELECT set_config('app.relay', 'on', true)`)
	var n int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM request.outbox_events WHERE published_at IS NULL`).Scan(&n); err != nil || n != 2 {
		t.Fatalf("relay sees %d unpublished rows across tenants (err %v), want 2", n, err)
	}
	if tag, err := tx.Exec(ctx, `UPDATE request.outbox_events SET published_at = now()`); err != nil || tag.RowsAffected() != 2 {
		t.Fatalf("relay marks published: %v %v", tag, err)
	}
	if tag, err := tx.Exec(ctx, `DELETE FROM request.outbox_events`); err != nil || tag.RowsAffected() != 0 {
		t.Fatalf("relay must not delete outbox rows, deleted %d (err %v)", tag.RowsAffected(), err)
	}
	// The relay policy does not leak other tables.
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM request.requests`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("relay flag must not open requests: %d %v", n, err)
	}
}

// Audit delivery runs across tenants, but only reaches request_audit_outbox, and never other tenants' requests.
func TestAuditRelayPolicyIsScoped(t *testing.T) {
	f := newMigratedPostgres(t)
	ctx := context.Background()
	tenantA := uuid.NewString()
	ids := newSeedIDs(tenantA)
	withTenantCommit(t, f.app, tenantA, func(tx pgx.Tx) {
		mustExec(t, tx, ids.sql("request_audit_outbox"))
		mustExec(t, tx, ids.sql("requests"))
		mustExec(t, tx, ids.sql("request_counters"))
	})
	tx, _ := f.app.Begin(ctx)
	defer func() { _ = tx.Rollback(ctx) }()
	mustExec(t, tx, `SELECT set_config('app.relay', 'on', true)`)
	var n int
	_ = tx.QueryRow(ctx, `SELECT count(*) FROM request.request_audit_outbox`).Scan(&n)
	if n != 1 {
		t.Fatalf("audit relay sees %d rows, want 1", n)
	}
	_ = tx.QueryRow(ctx, `SELECT count(*) FROM request.requests`).Scan(&n)
	// 0092 (metrics gauge sampler) gives the shared relay flag read-only access to requests; writes stay closed.
	if n != 1 {
		t.Fatalf("relay reads requests read-only since 0092, saw %d", n)
	}
	if tag, err := tx.Exec(ctx, `UPDATE request.requests SET title = 'x'`); err != nil || tag.RowsAffected() != 0 {
		t.Fatalf("the relay must not write requests: %d %v", tag.RowsAffected(), err)
	}
	_ = tx.QueryRow(ctx, `SELECT count(*) FROM request.request_counters`).Scan(&n)
	if n != 1 {
		t.Fatalf("retention tenant listing reads request_counters under the relay flag, saw %d", n)
	}
	if tag, err := tx.Exec(ctx, `DELETE FROM request.request_audit_outbox`); err != nil || tag.RowsAffected() != 0 {
		t.Fatalf("an undelivered audit row must not be deletable by the relay: %d %v", tag.RowsAffected(), err)
	}
}
