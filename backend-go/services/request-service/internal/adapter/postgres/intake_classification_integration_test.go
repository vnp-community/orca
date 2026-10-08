//go:build integration

package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/request-service/internal/adapter/contracttest"
)

func (f *pgFixture) intakeEnv() contracttest.IntakeEnv {
	base := New(f.app)
	ctx := context.Background()
	return contracttest.IntakeEnv{
		Env:       f.contractEnv(),
		Outbox:    base,
		Returns:   NewReturnHistoryRepository(base),
		Processed: NewProcessedEventRepository(base),
		Runs:      NewClassificationRunRepository(base),
		OutboxSubjects: func(t *testing.T, tenantID string) []string {
			rows, err := f.admin.Query(ctx, `SELECT subject FROM request.outbox_events WHERE tenant_id = $1 ORDER BY seq`, tenantID)
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
		},
		CountRows: func(t *testing.T, table, tenantID string) int {
			var n int
			if err := f.admin.QueryRow(ctx, `SELECT count(*) FROM request.`+table+` WHERE tenant_id = $1`, tenantID).Scan(&n); err != nil {
				t.Fatal(err)
			}
			return n
		},
		NextNumberPeek: func(t *testing.T, tenantID string) int64 {
			var n int64
			if err := f.admin.QueryRow(ctx, `SELECT coalesce(max(next_number), 0) FROM request.request_counters WHERE tenant_id = $1`, tenantID).Scan(&n); err != nil {
				t.Fatal(err)
			}
			return n
		},
		SetRunLease: func(t *testing.T, runID string, expires time.Time) {
			if _, err := f.admin.Exec(ctx, `UPDATE request.classification_runs SET lease_expires_at = $2 WHERE id = $1`, runID, expires); err != nil {
				t.Fatal(err)
			}
		},
	}
}

func TestPostgres_IntakeContract(t *testing.T) {
	f := newMigratedPostgres(t)
	contracttest.RunIntakeContract(t, func(*testing.T) contracttest.IntakeEnv { return f.intakeEnv() })
}

func TestPostgres_ClassificationContract(t *testing.T) {
	f := newMigratedPostgres(t)
	contracttest.RunClassificationContract(t, func(*testing.T) contracttest.IntakeEnv { return f.intakeEnv() })
}

// classification_runs is covered by the generic RLS sweep, but the relay policies are the
// deliberate hole in it: prove a plain tenant session still cannot read another tenant's runs.
func TestPostgres_ClassificationRuns_TenantRLS(t *testing.T) {
	f := newMigratedPostgres(t)
	env := f.intakeEnv()
	ctxA := contracttest.CtxForTenant("11111111-1111-1111-1111-111111111111")
	runID := contracttest.SeedClassifyingRunForRLS(t, env, ctxA)
	var visible int
	if err := f.admin.QueryRow(context.Background(), `SELECT count(*) FROM request.classification_runs WHERE id = $1`, runID).Scan(&visible); err != nil || visible != 1 {
		t.Fatalf("run not stored: %d %v", visible, err)
	}
	var n int
	// No tenant GUC and no relay GUC: FORCE RLS must hide every row from the application role.
	if err := f.app.QueryRow(context.Background(), `SELECT count(*) FROM request.classification_runs`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("app role saw %d rows without a tenant", n)
	}
}

func TestPostgres_RequestRPCContract(t *testing.T) {
	f := newMigratedPostgres(t)
	contracttest.RunRequestRPCContract(t, func(*testing.T) contracttest.IntakeEnv { return f.intakeEnv() })
}
