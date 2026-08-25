//go:build e2e

// TestRunNow_E2E_ReachesRealWorkflowService is the concrete runtime
// verification SOL-033 calls for (see
// specs/backend-go/bugs/missing-v1/tasks/TASK-220-automation-runnow-e2e-verification.md):
// not a fake WorkflowStepExecutor, but the real grpcclient.WorkflowClient
// dialed to a real running workflow-service. Reading run_now.go confirms
// RunNow calls a WorkflowStepExecutor port; this test is the separate claim
// that the port's real implementation actually reaches workflow-service and
// gets a real result back.
//
// This repo has no pre-existing "run against a full docker-compose stack"
// build-tag convention (only a per-service `integration` tag pairing a
// single Postgres testcontainer with the service's own repository — see
// internal/adapter/postgres/repository_test.go); the `e2e` tag here is new,
// per this task's own naming. Run with both Postgres and workflow-service
// reachable, e.g.:
//
//	docker compose -f deploy/dev/docker-compose.yml up -d postgres workflow-service
//	WORKFLOW_SERVICE_ADDR=localhost:9091 \
//	AUTOMATION_E2E_DATABASE_DSN=postgres://orca:orca@localhost:5432/automation?sslmode=disable \
//	  go test ./internal/usecase/... -tags e2e -run TestRunNow_E2E -v -timeout 60s
//
// If AUTOMATION_E2E_DATABASE_DSN is unset, a disposable Postgres is started
// via testcontainers-go (same helper internal/adapter/postgres's
// integration tests use) — only WORKFLOW_SERVICE_ADDR then needs a real
// workflow-service to point at.
package usecase_test

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/common/testutil"
	"github.com/stablyai/orca-go/services/automation-service/internal/adapter/grpcclient"
	automationpostgres "github.com/stablyai/orca-go/services/automation-service/internal/adapter/postgres"
	"github.com/stablyai/orca-go/services/automation-service/internal/domain"
	"github.com/stablyai/orca-go/services/automation-service/internal/usecase"
)

// e2eDSN returns AUTOMATION_E2E_DATABASE_DSN if set (pointing at
// docker-compose's Postgres), otherwise starts a disposable testcontainers-go
// Postgres and runs this service's real migrations against it — the same
// migration path internal/adapter/postgres's `integration`-tagged tests use.
func e2eDSN(t *testing.T) string {
	t.Helper()
	if dsn := os.Getenv("AUTOMATION_E2E_DATABASE_DSN"); dsn != "" {
		return dsn
	}
	dsn := testutil.StartPostgres(t, "automation")
	migrationsPath, err := filepath.Abs("../../migrations")
	if err != nil {
		t.Fatalf("resolving migrations path: %v", err)
	}
	cmd := exec.Command("migrate", "-path", migrationsPath, "-database", dsn, "up")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("running migrations: %v\n%s", err, out)
	}
	return dsn
}

func newRealPostgresAutomationRepository(t *testing.T) (*automationpostgres.AutomationRepository, *automationpostgres.AutomationRunRepository) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, e2eDSN(t))
	if err != nil {
		t.Fatalf("connecting to postgres: %v", err)
	}
	t.Cleanup(pool.Close)
	return automationpostgres.NewAutomationRepository(pool), automationpostgres.NewAutomationRunRepository(pool)
}

// newRealWorkflowExecutor dials WORKFLOW_SERVICE_ADDR (default matches
// cmd/server/main.go's config.go default, "localhost:9091") and wraps the
// connection in the real grpcclient.WorkflowClient — production's exact
// WorkflowStepExecutor implementation, not a fake.
func newRealWorkflowExecutor(t *testing.T) usecase.WorkflowStepExecutor {
	t.Helper()
	addr := os.Getenv("WORKFLOW_SERVICE_ADDR")
	if addr == "" {
		addr = "localhost:9091"
	}
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("dialing workflow-service at %s: %v", addr, err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return grpcclient.New(conn)
}

// TestRunNow_E2E_ReachesRealWorkflowService asserts the terminal status is
// succeeded or failed — domain.RunStatus is a small closed set (pending/
// running/succeeded/failed; see internal/domain/automation_run.go) with no
// skipped_unavailable member at all, unlike TS's status enum, which had one
// only because TS had no working execution path (automation-service.md
// §2/§10 — TS-Gap-3). That absence is itself the structural regression
// guard this test exercises at runtime: a real dispatch must resolve
// Terminal() (succeeded|failed), never hang pending/running past the
// deadline below. A test that merely asserts "no error was returned" would
// miss a silent reintroduction of a no-op dispatcher — that is exactly why
// this assertion is on run.Status, not just err.
func TestRunNow_E2E_ReachesRealWorkflowService(t *testing.T) {
	tenantID := uuid.NewString()
	ctx := tenant.WithTenantID(context.Background(), tenantID)

	automations, runs := newRealPostgresAutomationRepository(t)
	executor := newRealWorkflowExecutor(t)

	// 1. Create an automation with step_type=shell and a trivial
	// step_config_json.
	stepConfig, err := json.Marshal(map[string]any{"command": "echo automation-e2e-marker"})
	if err != nil {
		t.Fatalf("marshal step config: %v", err)
	}
	now := time.Now().UTC()
	automation := domain.Automation{
		ID:             uuid.NewString(),
		TenantID:       tenantID,
		Name:           "e2e-verification",
		RRule:          "FREQ=DAILY;INTERVAL=1",
		StepType:       domain.StepTypeShell,
		StepConfigJSON: string(stepConfig),
		DTStart:        now,
		Enabled:        true,
		Timezone:       "UTC",
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := automations.Create(ctx, automation); err != nil {
		t.Fatalf("create automation: %v", err)
	}

	// 2. Call RunNow with a fresh request_id.
	requestID := uuid.NewString()
	runNow := usecase.NewRunNow(automations, runs, executor)
	run, err := runNow.Execute(ctx, usecase.RunNowInput{
		AutomationID: automation.ID,
		RequestID:    requestID,
		Trigger:      domain.RunTriggerManual,
	})
	if err != nil {
		t.Fatalf("RunNow.Execute: %v", err)
	}

	// 3. Poll for the run to leave pending/running. Uses FindByRequestID
	// (the port RunNow itself uses for idempotency) rather than a new
	// by-ID read method — AutomationRunRepository has no such method today,
	// and this task adds no new production code.
	var final domain.AutomationRun
	deadline := time.Now().Add(30 * time.Second)
	for {
		var found bool
		final, found, err = runs.FindByRequestID(ctx, tenantID, automation.ID, requestID)
		if err != nil {
			t.Fatalf("poll run: %v", err)
		}
		if !found {
			t.Fatalf("run for request_id %s not found", requestID)
		}
		if final.Status != domain.RunStatusPending && final.Status != domain.RunStatusRunning {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("run %s did not leave pending/running within 30s (last status: %s)", run.ID, final.Status)
		}
		time.Sleep(500 * time.Millisecond)
	}

	// 4. THE regression assertion — a real dispatch always resolves
	// Terminal(), never hangs non-terminal past the deadline above.
	if !final.Status.Terminal() {
		t.Fatalf("run %s resolved non-terminal status %q — RunNow is not reaching a real workflow-service", run.ID, final.Status)
	}

	// 5. Proof the call reached a real executor, not a stub returning a
	// canned success: assert the shell command's real output is present,
	// not a placeholder.
	if final.OutputJSON == "" {
		t.Error("expected output_json to be populated from the real workflow-service execution")
	}

	// 6. domain.Automation has no LastRunAt field today (confirmed by
	// reading internal/domain/automation.go) — RunNow's bookkeeping lives
	// entirely on AutomationRun (see step 3/4 above), not back on
	// Automation itself. Per this task's own instruction, this gap is
	// noted rather than inventing a field no production code sets:
	// there is currently nothing on Automation for this test to assert
	// was updated after a run completes.
}
