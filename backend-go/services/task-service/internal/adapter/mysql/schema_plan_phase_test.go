//go:build integration

package mysql

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/common/testutil"
)

func insertRawTask(t *testing.T, repo *Repository, tenantID, taskType, status string, requestID any) error {
	t.Helper()
	_, err := repo.pool.ExecContext(context.Background(), `
		INSERT INTO tasks (id, tenant_id, title, status, task_type, request_id)
		VALUES (?, ?, 'raw', ?, ?, ?)
	`, uuid.NewString(), tenantID, status, taskType, requestID)
	return err
}

func TestMigration0015_AcceptsPlanPhase_RejectsUnknown(t *testing.T) {
	repo := setupRepository(t)
	tenantID := uuid.NewString()
	for _, typ := range []string{"task", "bug", "feature", "epic", "plan", "phase"} {
		if err := insertRawTask(t, repo, tenantID, typ, "open", nil); err != nil {
			t.Errorf("task_type %q must be accepted: %v", typ, err)
		}
	}
	if err := insertRawTask(t, repo, tenantID, "xyz", "open", nil); err == nil {
		t.Error("task_type 'xyz' must be rejected by the CHECK constraint")
	}
}

func TestMigration0016_UniqueActivePlanPerRequest(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	tenantID, otherTenant, reqID := uuid.NewString(), uuid.NewString(), uuid.NewString()

	if err := insertRawTask(t, repo, tenantID, "plan", "open", reqID); err != nil {
		t.Fatalf("first plan: %v", err)
	}
	if err := insertRawTask(t, repo, tenantID, "plan", "open", reqID); err == nil {
		t.Fatal("a second active plan for the same (tenant, request) must violate uq_tasks_active_plan_per_request")
	}
	// Same request id in another tenant is a different key.
	if err := insertRawTask(t, repo, otherTenant, "plan", "open", reqID); err != nil {
		t.Errorf("other tenant: %v", err)
	}
	// Non-plan tasks sharing a request id are fine.
	for i := 0; i < 2; i++ {
		if err := insertRawTask(t, repo, tenantID, "task", "open", reqID); err != nil {
			t.Errorf("task rows may share request_id: %v", err)
		}
	}
	// After the active plan is cancelled a replacement is allowed.
	if _, err := repo.pool.ExecContext(ctx, `UPDATE tasks SET status='cancelled' WHERE tenant_id=? AND task_type='plan'`, tenantID); err != nil {
		t.Fatal(err)
	}
	if err := insertRawTask(t, repo, tenantID, "plan", "open", reqID); err != nil {
		t.Errorf("replacement after cancel: %v", err)
	}
}

// TestMigrations_UpDownUpFull runs the whole chain up, down to 0014, then up again on a fresh database.
func TestMigrations_0015_0016_UpDownUp(t *testing.T) {
	dsn := testutil.StartMySQL(t, "task")
	migrationsPath, err := filepath.Abs("../../../migrations/mysql")
	if err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) {
		t.Helper()
		full := append([]string{"-path", migrationsPath, "-database", dsn}, args...)
		if out, err := exec.Command("migrate", full...).CombinedOutput(); err != nil {
			t.Fatalf("migrate %v: %v\n%s", args, err, out)
		}
	}
	run("up")
	run("down", "2")
	run("up")
}
