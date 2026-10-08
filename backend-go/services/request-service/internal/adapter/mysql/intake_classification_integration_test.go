//go:build integration

package mysql

import (
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/request-service/internal/adapter/contracttest"
)

func (f *myFixture) intakeEnv() contracttest.IntakeEnv {
	base := New(f.db)
	return contracttest.IntakeEnv{
		Env:       f.contractEnv(),
		Outbox:    base,
		Returns:   NewReturnHistoryRepository(base),
		Processed: NewProcessedEventRepository(base),
		Runs:      NewClassificationRunRepository(base),
		OutboxSubjects: func(t *testing.T, tenantID string) []string {
			rows, err := f.admin.Query(`SELECT subject FROM outbox_events WHERE tenant_id = ? ORDER BY seq`, tenantID)
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
			if err := f.admin.QueryRow(`SELECT count(*) FROM `+table+` WHERE tenant_id = ?`, tenantID).Scan(&n); err != nil {
				t.Fatal(err)
			}
			return n
		},
		NextNumberPeek: func(t *testing.T, tenantID string) int64 {
			var n int64
			if err := f.admin.QueryRow(`SELECT coalesce(max(next_number), 0) FROM request_counters WHERE tenant_id = ?`, tenantID).Scan(&n); err != nil {
				t.Fatal(err)
			}
			return n
		},
		SetRunLease: func(t *testing.T, runID string, expires time.Time) {
			if _, err := f.admin.Exec(`UPDATE classification_runs SET lease_expires_at = ? WHERE id = ?`, expires.UTC(), runID); err != nil {
				t.Fatal(err)
			}
		},
	}
}

func TestMySQL_IntakeContract(t *testing.T) {
	f := newMigratedMySQL(t)
	contracttest.RunIntakeContract(t, func(*testing.T) contracttest.IntakeEnv { return f.intakeEnv() })
}

func TestMySQL_ClassificationContract(t *testing.T) {
	f := newMigratedMySQL(t)
	contracttest.RunClassificationContract(t, func(*testing.T) contracttest.IntakeEnv { return f.intakeEnv() })
}

func TestMySQL_RequestRPCContract(t *testing.T) {
	f := newMigratedMySQL(t)
	contracttest.RunRequestRPCContract(t, func(*testing.T) contracttest.IntakeEnv { return f.intakeEnv() })
}
