//go:build integration

package mysql

import (
	"database/sql"
	"testing"

	"github.com/stablyai/orca-go/services/request-service/internal/adapter/contracttest"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

func (f *myFixture) execAdmin(t *testing.T, query string, args ...any) {
	t.Helper()
	if _, err := f.admin.Exec(query, args...); err != nil {
		t.Fatalf("%s: %v", query, err)
	}
}

func (f *myFixture) securityEnv() contracttest.SecurityEnv {
	base := New(f.db)
	return contracttest.SecurityEnv{
		Base:      f.contractEnv(),
		Audit:     NewAuditOutboxRepository(base),
		Nonces:    NewWebhookNonceRepository(base),
		Flags:     NewSecurityFlagRepository(base),
		Counts:    NewConcurrencyCountRepository(base),
		Retention: NewRetentionRepository(base),
		SeedRun: func(t *testing.T, tenantID, requestID, status string) {
			f.execAdmin(t, `INSERT INTO analysis_runs (id, tenant_id, request_id, kind, mode, status) VALUES (UUID(), ?, ?, 'solution', 'complete', ?)`, tenantID, requestID, status)
		},
		SeedSettings: func(t *testing.T, tenantID string, days int) {
			f.execAdmin(t, `INSERT INTO tenant_security_settings (tenant_id, request_retention_days) VALUES (?, ?)`, tenantID, days)
		},
		SeedContent: func(t *testing.T, tenantID, requestID string) {
			f.execAdmin(t, `UPDATE requests SET title = 'SECRET title', body = 'SECRET body', classification_reason = 'SECRET', return_reason = 'SECRET',
				source_url = 'https://secret.example/1', source_hints = '{"labels":["SECRET"]}',
				acceptance_criteria = '["SECRET"]', type_fields = '{"k":"SECRET"}' WHERE tenant_id = ? AND id = ?`, tenantID, requestID)
			f.execAdmin(t, `INSERT INTO request_type_history (tenant_id, request_id, to_type, actor_id, reason) VALUES (?, ?, 'bug', UUID(), 'SECRET')`, tenantID, requestID)
			f.execAdmin(t, `INSERT INTO request_return_history (id, tenant_id, request_id, action, actor_kind, reason) VALUES (UUID(), ?, ?, 'returned', 'user', 'SECRET')`, tenantID, requestID)
			f.execAdmin(t, `INSERT INTO solutions (id, tenant_id, request_id, options, seq) VALUES (UUID(), ?, ?, '{"o":"SECRET"}', 1)`, tenantID, requestID)
			f.execAdmin(t, `INSERT INTO analysis_runs (id, tenant_id, request_id, kind, mode, status, raw_output, error_message, feedback) VALUES (UUID(), ?, ?, 'solution', 'complete', 'succeeded', 'SECRET', 'SECRET', 'SECRET')`, tenantID, requestID)
			f.execAdmin(t, `INSERT INTO approvals (id, tenant_id, request_id, subject_type, subject_id, stage, status, requested_by, subject_digest, comment) VALUES (UUID(), ?, ?, 'plan', 's', 'plan', 'approved', 'u', 'd', 'SECRET')`, tenantID, requestID)
			f.execAdmin(t, `SET @pack = UUID()`)
			f.execAdmin(t, `INSERT INTO context_packs (id, tenant_id, request_id, stage, cp_version, input_digest, digest, budget_tokens, used_tokens, items, missing, body)
				VALUES (@pack, ?, ?, 'solution', '1', REPEAT('a', 64), REPEAT('b', 64), 1, 1, '["SECRET"]', '["SECRET"]', 'SECRET')`, tenantID, requestID)
			f.execAdmin(t, `INSERT INTO evidence (id, tenant_id, request_id, context_pack_id, seq, source_id, ref, retrieved_at, freshness, trust, digest, size, title, excerpt)
				VALUES (UUID(), ?, ?, @pack, 1, 's', 'r', NOW(6), 'fresh', 'high', REPEAT('c', 64), 1, 'SECRET', 'SECRET')`, tenantID, requestID)
			f.execAdmin(t, `INSERT INTO openspec_changes (id, tenant_id, request_id, change_id, status, tasks_sync_state, tasks_sync_error) VALUES (UUID(), ?, ?, 'c', 'ready', 'failed', 'SECRET')`, tenantID, requestID)
			f.execAdmin(t, `INSERT INTO request_revisions (id, tenant_id, request_id, revision, cause, snapshot, digest, actor_kind) VALUES (UUID(), ?, ?, 1, 'created', '{"title":"SECRET"}', 'd', 'user')`, tenantID, requestID)
			f.execAdmin(t, `SET @clar = UUID()`)
			f.execAdmin(t, `INSERT INTO clarifications (id, tenant_id, request_id, seq, source, status, resume_status, asked_request_revision, due_at, cancel_reason, created_by)
				VALUES (@clar, ?, ?, 1, 'manual', 'open', 'analyzing', 1, NOW(6), 'SECRET', 'u')`, tenantID, requestID)
			f.execAdmin(t, `INSERT INTO clarification_questions (id, tenant_id, clarification_id, seq, question_key, kind, prompt, reason, options, suggested_default, answer)
				VALUES (UUID(), ?, @clar, 1, 'k', 'text', 'SECRET', 'SECRET', '["SECRET"]', '"SECRET"', '"SECRET"')`, tenantID)
			f.execAdmin(t, `SET @dec = UUID()`)
			f.execAdmin(t, `INSERT INTO decisions (id, tenant_id, request_id, seq, subject_kind, subject_id, question, options, recommendation_reason, rationale, status)
				VALUES (@dec, ?, ?, 1, 'other', UUID(), 'SECRET', '["SECRET"]', 'SECRET', 'SECRET', 'open')`, tenantID, requestID)
			f.execAdmin(t, `INSERT INTO decision_history (id, tenant_id, decision_id, action, rationale) VALUES (UUID(), ?, @dec, 'chosen', 'SECRET')`, tenantID)
			f.execAdmin(t, `INSERT INTO task_run_outcomes (id, tenant_id, request_id, task_id, outcome, error_message, event_id, occurred_at) VALUES (UUID(), ?, ?, UUID(), 'failed', 'SECRET', UUID(), NOW(6))`, tenantID, requestID)
			f.execAdmin(t, `INSERT INTO request_checks (id, tenant_id, request_id, kind, status, metrics, summary, source) VALUES (UUID(), ?, ?, 'tests_after', 'passed', '{"k":"SECRET"}', 'SECRET', 'manual')`, tenantID, requestID)
		},
		ReadColumn: func(t *testing.T, c domain.ErasableColumn, requestID string) (string, bool) {
			where := c.KeyColumn + ` = ?`
			if c.Parent != "" {
				where = c.KeyColumn + ` IN (SELECT id FROM ` + c.Parent + ` WHERE request_id = ?)`
			}
			var val sql.NullString
			if err := f.admin.QueryRow(`SELECT CAST(`+c.Column+` AS CHAR) FROM `+c.Table+` WHERE `+where+` LIMIT 1`, requestID).Scan(&val); err != nil {
				t.Fatalf("read %s.%s: %v", c.Table, c.Column, err)
			}
			return val.String, !val.Valid
		},
	}
}

func TestMySQL_SecurityContract(t *testing.T) {
	contracttest.RunSecurityContract(t, func(t *testing.T) contracttest.SecurityEnv {
		return newMigratedMySQL(t).securityEnv()
	})
}

// A text-like column nobody classified would silently survive erasure.
func TestMySQL_EveryTextColumnDeclaredErasableOrExempt(t *testing.T) {
	f := newMigratedMySQL(t)
	list := func() []string {
		rows, err := f.admin.Query(`
			SELECT CONCAT(table_name, '.', column_name) FROM information_schema.columns
			WHERE table_schema = DATABASE() AND data_type IN ('text', 'varchar', 'longtext', 'mediumtext', 'tinytext', 'json') ORDER BY 1`)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		var out []string
		for rows.Next() {
			var s string
			_ = rows.Scan(&s)
			out = append(out, s)
		}
		return out
	}
	contracttest.CheckTextColumnsDeclared(t, list)
	contracttest.CheckExemptColumnsExist(t, list, domain.MySQLOnlyExemptTextColumns)
}

// MySQL has no RLS, so the schema contract is: every table carries tenant_id NOT NULL.
func TestMySQL_EveryTableHasTenantID(t *testing.T) {
	f := newMigratedMySQL(t)
	rows, err := f.admin.Query(`
		SELECT t.table_name FROM information_schema.tables t
		WHERE t.table_schema = DATABASE() AND t.table_type = 'BASE TABLE' AND t.table_name <> 'schema_migrations'
		  AND NOT EXISTS (SELECT 1 FROM information_schema.columns c WHERE c.table_schema = t.table_schema
		                  AND c.table_name = t.table_name AND c.column_name = 'tenant_id' AND c.is_nullable = 'NO')`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		_ = rows.Scan(&name)
		t.Errorf("table %s has no tenant_id NOT NULL column", name)
	}
}
