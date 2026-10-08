//go:build integration

package mysql

import (
	"testing"

	"github.com/stablyai/orca-go/services/request-service/internal/adapter/contracttest"
)

func (f *myFixture) solutionEnv() contracttest.SolutionEnv {
	base := New(f.db)
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
			var key any
			if idemKey != nil {
				key = *idemKey
			}
			_, err := f.admin.Exec(`INSERT INTO analysis_runs (id, tenant_id, request_id, kind, mode, status, idempotency_key)
				VALUES (?, ?, ?, ?, 'complete', ?, ?)`, id, tenantID, requestID, kind, status, key)
			return err
		},
		ExpireRun: func(t *testing.T, runID string) {
			if _, err := f.admin.Exec(`UPDATE analysis_runs SET lease_expires_at = DATE_SUB(CURRENT_TIMESTAMP(6), INTERVAL 1 MINUTE) WHERE id = ?`, runID); err != nil {
				t.Fatal(err)
			}
		},
		TamperOptions: func(t *testing.T, solutionID, optionsJSON string) {
			if _, err := f.admin.Exec(`UPDATE solutions SET options = ? WHERE id = ?`, optionsJSON, solutionID); err != nil {
				t.Fatal(err)
			}
		},
	}
}

func TestMySQL_SolutionContract(t *testing.T) {
	f := newMigratedMySQL(t)
	contracttest.RunSolutionContract(t, func(*testing.T) contracttest.SolutionEnv { return f.solutionEnv() })
}
