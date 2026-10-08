//go:build integration

package mysql

import (
	"context"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/request-service/internal/adapter/contracttest"
)

func (f *myFixture) rolloutEnv() contracttest.RolloutEnv {
	base := New(f.db)
	return contracttest.RolloutEnv{
		ApprovalEnv: f.approvalEnv(),
		Flow:        NewFlowSettingsRepository(base),
		Finder:      NewSourceLookup(base),
		Samples:     NewMetricsSamples(base),
		AgeRequest: func(t *testing.T, id string, age time.Duration) {
			t.Helper()
			if _, err := f.admin.ExecContext(context.Background(), `UPDATE requests SET updated_at = TIMESTAMPADD(SECOND, -?, CURRENT_TIMESTAMP(6)) WHERE id = ?`, int(age.Seconds()), id); err != nil {
				t.Fatal(err)
			}
		},
	}
}

func TestMySQL_RolloutContract(t *testing.T) {
	f := newMigratedMySQL(t)
	contracttest.RunRolloutContract(t, func(*testing.T) contracttest.RolloutEnv { return f.rolloutEnv() })
}
