//go:build integration

package postgres

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/services/request-service/internal/adapter/contracttest"
)

func (f *pgFixture) lifecycleEnv() contracttest.LifecycleEnv {
	base := New(f.app)
	return contracttest.LifecycleEnv{
		Env:     f.contractEnv(),
		TxScope: base,
		Outbox:  base,
		Returns: NewReturnHistoryRepository(base),
		OutboxRows: func(t *testing.T, tenantID string) []contracttest.OutboxRow {
			t.Helper()
			rows, err := f.admin.Query(context.Background(),
				`SELECT subject, payload::text FROM request.outbox_events WHERE tenant_id = $1 ORDER BY seq`, tenantID)
			if err != nil {
				t.Fatal(err)
			}
			defer rows.Close()
			var out []contracttest.OutboxRow
			for rows.Next() {
				var r contracttest.OutboxRow
				var payload string
				if err := rows.Scan(&r.Subject, &payload); err != nil {
					t.Fatal(err)
				}
				r.Payload = []byte(payload)
				out = append(out, r)
			}
			return out
		},
		InsertRawRequest: func(tenantID string, overrides map[string]any) error {
			ov := map[string]any{"tenant_id": tenantID}
			for k, v := range overrides {
				ov[k] = v
			}
			return insertRequestRow(f.admin, ov)
		},
	}
}

func TestPostgres_TransitionContract(t *testing.T) {
	f := newMigratedPostgres(t)
	contracttest.RunTransitionContract(t, func(*testing.T) contracttest.LifecycleEnv { return f.lifecycleEnv() })
}

func TestPostgres_LifecycleExitContract(t *testing.T) {
	f := newMigratedPostgres(t)
	contracttest.RunLifecycleExitContract(t, func(*testing.T) contracttest.LifecycleEnv { return f.lifecycleEnv() })
}

// TransitionUnderRLS: the app role is NOSUPERUSER NOBYPASSRLS, so this fails if any statement of the
// transition path forgets set_config('app.tenant_id').
func TestPostgres_TransitionUnderRLS(t *testing.T) {
	f := newMigratedPostgres(t)
	var super, bypass bool
	if err := f.admin.QueryRow(context.Background(), `SELECT rolsuper, rolbypassrls FROM pg_roles WHERE rolname = $1`, appRole).Scan(&super, &bypass); err != nil || super || bypass {
		t.Fatalf("app role must be NOSUPERUSER NOBYPASSRLS: super=%v bypass=%v err=%v", super, bypass, err)
	}
	contracttest.RunTransitionContract(t, func(*testing.T) contracttest.LifecycleEnv { return f.lifecycleEnv() })
}

// A backlog row that exists before 0020 has no category; the migration must backfill 'other' before adding the pairing CHECK.
func TestPostgres_Migration_ReturnBackfill(t *testing.T) {
	_, admin := startPostgres(t)
	before, target := contracttest.MigrationUpScriptsSplit(t, "postgres", "0020")
	applyScripts(t, admin, before)
	if err := insertRequestRow(admin, map[string]any{"status": "request_backlog", "returned_from_stage": "plan"}); err != nil {
		t.Fatal(err)
	}
	applyScripts(t, admin, []string{target})
	var category *string
	if err := admin.QueryRow(context.Background(), `SELECT returned_category FROM request.requests WHERE status = 'request_backlog'`).Scan(&category); err != nil {
		t.Fatal(err)
	}
	if category == nil || *category != "other" {
		t.Fatalf("backfilled category = %v, want other", category)
	}
	assertAllTablesForceRLS(t, admin)
}
