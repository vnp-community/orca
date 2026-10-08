//go:build integration

package postgres

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/services/request-service/internal/adapter/contracttest"
)

func (f *pgFixture) solutionEnv() contracttest.SolutionEnv {
	base := New(f.app)
	ctx := context.Background()
	return contracttest.SolutionEnv{
		IntakeEnv:     f.intakeEnv(),
		SolutionStore: NewSolutionRecordRepository(base),
		AnalysisRuns:  NewAnalysisRunRepository(base),
		Approvals:     NewApprovalRepository(base),
		Approvers:     NewApproverRepository(base),
		Policies:      NewPolicyRepository(base),
		Locker:        NewRequestLocker(base),
		TxScope:       base,
		Artifacts:     f.artifactEnv(),
		RawInsertRun: func(t *testing.T, tenantID, requestID, id, kind, status string, idemKey *string) error {
			_, err := f.admin.Exec(ctx, `INSERT INTO request.analysis_runs (id, tenant_id, request_id, kind, mode, status, idempotency_key)
				VALUES ($1, $2, $3, $4, 'complete', $5, $6)`, id, tenantID, requestID, kind, status, idemKey)
			return err
		},
		ExpireRun: func(t *testing.T, runID string) {
			if _, err := f.admin.Exec(ctx, `UPDATE request.analysis_runs SET lease_expires_at = now() - interval '1 minute' WHERE id = $1`, runID); err != nil {
				t.Fatal(err)
			}
		},
		TamperOptions: func(t *testing.T, solutionID, optionsJSON string) {
			if _, err := f.admin.Exec(ctx, `UPDATE request.solutions SET options = $2::jsonb WHERE id = $1`, solutionID, optionsJSON); err != nil {
				t.Fatal(err)
			}
		},
	}
}

func TestPostgres_SolutionContract(t *testing.T) {
	f := newMigratedPostgres(t)
	contracttest.RunSolutionContract(t, func(*testing.T) contracttest.SolutionEnv { return f.solutionEnv() })
}

// analysis_runs keeps relay policies only for the lease sweep; a plain tenant session must still be fenced in.
func TestPostgres_AnalysisRuns_TenantRLS(t *testing.T) {
	f := newMigratedPostgres(t)
	env := f.solutionEnv()
	ctx := context.Background()
	tenantA, tenantB := "11111111-1111-1111-1111-111111111111", "22222222-2222-2222-2222-222222222222"
	ctxA := contracttest.CtxForTenant(tenantA)
	reqA := contracttest.SeedRequestForRLS(t, env.IntakeEnv, ctxA)
	if err := env.RawInsertRun(t, tenantA, reqA, "33333333-3333-3333-3333-333333333333", "solution", "running", nil); err != nil {
		t.Fatal(err)
	}
	if err := env.AnalysisRuns.EnsureProjectGate(ctxA, "44444444-4444-4444-4444-444444444444"); err != nil {
		t.Fatal(err)
	}
	count := func(tenantID, table string) int {
		tx, err := f.app.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = tx.Rollback(ctx) }()
		if _, err := tx.Exec(ctx, `SELECT set_config('app.tenant_id', $1, true)`, tenantID); err != nil {
			t.Fatal(err)
		}
		var n int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM request.`+table).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	for _, table := range []string{"analysis_runs", "analysis_project_gates"} {
		if a, b := count(tenantA, table), count(tenantB, table); a != 1 || b != 0 {
			t.Fatalf("%s: owner sees %d, other tenant sees %d", table, a, b)
		}
	}
	var n int
	if err := f.app.QueryRow(ctx, `SELECT count(*) FROM request.analysis_runs`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("a session without a tenant sees %d runs (%v)", n, err)
	}
	if _, err := f.app.Exec(ctx, `INSERT INTO request.analysis_runs (id, tenant_id, request_id, kind, mode, status) VALUES (gen_random_uuid(), $1, $2, 'solution', 'complete', 'failed')`, tenantA, reqA); err == nil {
		t.Fatal("an insert without the tenant GUC must violate the policy")
	}
}
