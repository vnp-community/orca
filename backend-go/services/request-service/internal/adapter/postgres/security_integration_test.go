//go:build integration

package postgres

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/services/request-service/internal/adapter/contracttest"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

func (f *pgFixture) execAdmin(t *testing.T, sql string, args ...any) {
	t.Helper()
	if _, err := f.admin.Exec(context.Background(), sql, args...); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
}

// The superuser connection seeds and reads rows directly; the repositories under test use the NOBYPASSRLS role.
func (f *pgFixture) securityEnv() contracttest.SecurityEnv {
	base := New(f.app)
	return contracttest.SecurityEnv{
		Base:      f.contractEnv(),
		Audit:     NewAuditOutboxRepository(base),
		Nonces:    NewWebhookNonceRepository(base),
		Flags:     NewSecurityFlagRepository(base),
		Counts:    NewConcurrencyCountRepository(base),
		Retention: NewRetentionRepository(base),
		SeedRun: func(t *testing.T, tenantID, requestID, status string) {
			f.execAdmin(t, `INSERT INTO request.analysis_runs (id, tenant_id, request_id, kind, mode, status) VALUES (gen_random_uuid(), $1, $2, 'solution', 'complete', $3)`, tenantID, requestID, status)
		},
		SeedSettings: func(t *testing.T, tenantID string, days int) {
			f.execAdmin(t, `INSERT INTO request.tenant_security_settings (tenant_id, request_retention_days) VALUES ($1, $2)`, tenantID, days)
		},
		SeedContent: func(t *testing.T, tenantID, requestID string) {
			f.execAdmin(t, `UPDATE request.requests SET title = 'SECRET title', body = 'SECRET body', classification_reason = 'SECRET', return_reason = 'SECRET',
				source_url = 'https://secret.example/1', source_hints = '{"labels":["SECRET"]}',
				acceptance_criteria = '["SECRET"]', type_fields = '{"k":"SECRET"}' WHERE tenant_id = $1 AND id = $2`, tenantID, requestID)
			f.execAdmin(t, `INSERT INTO request.request_type_history (tenant_id, request_id, to_type, actor_id, reason) VALUES ($1, $2, 'bug', gen_random_uuid(), 'SECRET')`, tenantID, requestID)
			f.execAdmin(t, `INSERT INTO request.request_return_history (id, tenant_id, request_id, action, actor_kind, reason) VALUES (gen_random_uuid(), $1, $2, 'returned', 'user', 'SECRET')`, tenantID, requestID)
			f.execAdmin(t, `INSERT INTO request.solutions (id, tenant_id, request_id, options, seq) VALUES (gen_random_uuid(), $1, $2, '{"o":"SECRET"}', 1)`, tenantID, requestID)
			f.execAdmin(t, `INSERT INTO request.analysis_runs (id, tenant_id, request_id, kind, mode, status, raw_output, error_message, feedback) VALUES (gen_random_uuid(), $1, $2, 'solution', 'complete', 'succeeded', 'SECRET', 'SECRET', 'SECRET')`, tenantID, requestID)
			f.execAdmin(t, `INSERT INTO request.approvals (id, tenant_id, request_id, subject_type, subject_id, stage, status, requested_by, subject_digest, comment) VALUES (gen_random_uuid(), $1, $2, 'plan', 's', 'plan', 'approved', 'u', 'd', 'SECRET')`, tenantID, requestID)
			f.execAdmin(t, `WITH p AS (
					INSERT INTO request.context_packs (id, tenant_id, request_id, stage, cp_version, input_digest, digest, budget_tokens, used_tokens, items, missing, body)
					VALUES (gen_random_uuid(), $1, $2, 'solution', '1', repeat('a', 64), repeat('b', 64), 1, 1, '["SECRET"]', '["SECRET"]', 'SECRET') RETURNING id)
				INSERT INTO request.evidence (id, tenant_id, request_id, context_pack_id, seq, source_id, ref, retrieved_at, freshness, trust, digest, size, title, excerpt)
				SELECT gen_random_uuid(), $1, $2, p.id, 1, 's', 'r', now(), 'fresh', 'high', repeat('c', 64), 1, 'SECRET', 'SECRET' FROM p`, tenantID, requestID)
			f.execAdmin(t, `INSERT INTO request.openspec_changes (id, tenant_id, request_id, change_id, status, tasks_sync_state, tasks_sync_error) VALUES (gen_random_uuid(), $1, $2, 'c', 'ready', 'failed', 'SECRET')`, tenantID, requestID)
			f.execAdmin(t, `INSERT INTO request.request_revisions (id, tenant_id, request_id, revision, cause, snapshot, digest, actor_kind) VALUES (gen_random_uuid(), $1, $2, 1, 'created', '{"title":"SECRET"}', 'd', 'user')`, tenantID, requestID)
			f.execAdmin(t, `WITH c AS (
					INSERT INTO request.clarifications (id, tenant_id, request_id, seq, source, status, resume_status, asked_request_revision, due_at, cancel_reason, created_by)
					VALUES (gen_random_uuid(), $1, $2, 1, 'manual', 'open', 'analyzing', 1, now(), 'SECRET', 'u') RETURNING id)
				INSERT INTO request.clarification_questions (id, tenant_id, clarification_id, seq, question_key, kind, prompt, reason, options, suggested_default, answer)
				SELECT gen_random_uuid(), $1, c.id, 1, 'k', 'text', 'SECRET', 'SECRET', '["SECRET"]', '"SECRET"', '"SECRET"' FROM c`, tenantID, requestID)
			f.execAdmin(t, `WITH d AS (
					INSERT INTO request.decisions (id, tenant_id, request_id, seq, subject_kind, subject_id, question, options, recommendation_reason, rationale, status)
					VALUES (gen_random_uuid(), $1, $2, 1, 'other', gen_random_uuid()::text, 'SECRET', '["SECRET"]', 'SECRET', 'SECRET', 'open') RETURNING id)
				INSERT INTO request.decision_history (id, tenant_id, decision_id, action, rationale)
				SELECT gen_random_uuid(), $1, d.id, 'chosen', 'SECRET' FROM d`, tenantID, requestID)
			f.execAdmin(t, `INSERT INTO request.task_run_outcomes (id, tenant_id, request_id, task_id, outcome, error_message, event_id, occurred_at) VALUES (gen_random_uuid(), $1, $2, gen_random_uuid(), 'failed', 'SECRET', gen_random_uuid(), now())`, tenantID, requestID)
			f.execAdmin(t, `INSERT INTO request.request_checks (id, tenant_id, request_id, kind, status, metrics, summary, source) VALUES (gen_random_uuid(), $1, $2, 'tests_after', 'passed', '{"k":"SECRET"}', 'SECRET', 'manual')`, tenantID, requestID)
		},
		ReadColumn: func(t *testing.T, c domain.ErasableColumn, requestID string) (string, bool) {
			where := c.KeyColumn + ` = $1::uuid`
			if c.Parent != "" {
				where = c.KeyColumn + ` IN (SELECT id FROM request.` + c.Parent + ` WHERE request_id = $1::uuid)`
			}
			var val *string
			err := f.admin.QueryRow(context.Background(), `SELECT `+c.Column+`::text FROM request.`+c.Table+` WHERE `+where+` LIMIT 1`, requestID).Scan(&val)
			if err != nil {
				t.Fatalf("read %s.%s: %v", c.Table, c.Column, err)
			}
			if val == nil {
				return "", true
			}
			return *val, false
		},
	}
}

func TestPostgres_SecurityContract(t *testing.T) {
	contracttest.RunSecurityContract(t, func(t *testing.T) contracttest.SecurityEnv {
		return newMigratedPostgres(t).securityEnv()
	})
}

// A text-like column nobody classified would silently survive erasure.
func TestPostgres_EveryTextColumnDeclaredErasableOrExempt(t *testing.T) {
	f := newMigratedPostgres(t)
	list := func() []string {
		rows, err := f.admin.Query(context.Background(), `
			SELECT table_name || '.' || column_name FROM information_schema.columns
			WHERE table_schema = 'request' AND data_type IN ('text', 'character varying', 'json', 'jsonb') ORDER BY 1`)
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
	contracttest.CheckExemptColumnsExist(t, list, domain.ExemptTextColumns)
}
