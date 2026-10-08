//go:build integration

package mysql

import (
	"database/sql"
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/services/request-service/internal/adapter/contracttest"
)

func (f *myFixture) lifecycleEnv() contracttest.LifecycleEnv {
	base := New(f.db)
	return contracttest.LifecycleEnv{
		Env:     f.contractEnv(),
		TxScope: base,
		Outbox:  base,
		Returns: NewReturnHistoryRepository(base),
		OutboxRows: func(t *testing.T, tenantID string) []contracttest.OutboxRow {
			t.Helper()
			rows, err := f.admin.Query(`SELECT subject, CAST(payload AS CHAR) FROM outbox_events WHERE tenant_id = ? ORDER BY seq`, tenantID)
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
			cols := []string{"id", "tenant_id", "number", "title", "body", "source_provider", "source_url", "classification_reason", "return_reason", "status", "urgency", "reporter_id"}
			vals := map[string]any{
				"id": uuid.NewString(), "tenant_id": tenantID, "number": int64(1), "title": "t", "body": "", "source_provider": "manual",
				"source_url": "", "classification_reason": "", "return_reason": "", "status": "new", "urgency": "normal", "reporter_id": uuid.NewString(),
			}
			for k, v := range overrides {
				if _, ok := vals[k]; !ok {
					cols = append(cols, k)
				}
				vals[k] = v
			}
			args := make([]any, len(cols))
			ph := make([]string, len(cols))
			for i, c := range cols {
				args[i], ph[i] = vals[c], "?"
			}
			_, err := f.admin.Exec(fmt.Sprintf("INSERT INTO requests (%s) VALUES (%s)", strings.Join(cols, ", "), strings.Join(ph, ", ")), args...)
			return err
		},
	}
}

func TestMySQL_TransitionContract(t *testing.T) {
	f := newMigratedMySQL(t)
	contracttest.RunTransitionContract(t, func(*testing.T) contracttest.LifecycleEnv { return f.lifecycleEnv() })
}

func TestMySQL_LifecycleExitContract(t *testing.T) {
	f := newMigratedMySQL(t)
	contracttest.RunLifecycleExitContract(t, func(*testing.T) contracttest.LifecycleEnv { return f.lifecycleEnv() })
}

// A backlog row that exists before 0020 has no category; the migration must backfill 'other' before adding the pairing CHECK.
func TestMySQL_Migration_ReturnBackfill(t *testing.T) {
	_, admin := startMySQL(t)
	before, target := contracttest.MigrationUpScriptsSplit(t, "mysql", "0020")
	applyScripts(t, admin, before)
	env := (&myFixture{db: admin, admin: admin}).lifecycleEnv()
	if err := env.InsertRawRequest(uuid.NewString(), map[string]any{"status": "request_backlog", "returned_from_stage": "plan"}); err != nil {
		t.Fatal(err)
	}
	applyScripts(t, admin, []string{target})
	var category sql.NullString
	if err := admin.QueryRow(`SELECT returned_category FROM requests WHERE status = 'request_backlog'`).Scan(&category); err != nil {
		t.Fatal(err)
	}
	if category.String != "other" {
		t.Fatalf("backfilled category = %q, want other", category.String)
	}
}
