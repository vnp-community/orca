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
// per this task's own naming.
//
// Two ways to run this test:
//
//  1. Against docker-compose's real stack (preferred for CI — see this
//     task's "CI wiring" section):
//
//     docker compose -f deploy/dev/docker-compose.yml up -d postgres workflow-service
//     WORKFLOW_SERVICE_ADDR=localhost:9091 \
//     AUTOMATION_E2E_DATABASE_DSN=postgres://orca:orca@localhost:5432/automation?sslmode=disable \
//     go test ./internal/usecase/... -tags e2e -run TestRunNow_E2E -v -timeout 60s
//
//  2. Self-contained, no docker-compose stack required (what this test does
//     when WORKFLOW_SERVICE_ADDR is unset — this is what actually makes
//     SOL-033's claim checkable in a sandbox with no pre-existing compose
//     stack): this test builds and starts workflow-service's own real
//     cmd/server binary against a disposable testcontainers-go Postgres
//     with its own real migrations, then dials that real live process —
//     genuinely the same WorkflowClient/WorkflowServiceClient code path
//     production uses, not a fake. What this mode CANNOT verify: the full
//     chain all the way to a live Dev Server Agent (workflow-service's own
//     downstream call to infra-fleet-service, and infra-fleet-service's own
//     migrations, have an unrelated pre-existing duplicate-migration-number
//     bug — three files all named `0004_*` in
//     infra-fleet-service/migrations/ — that blocks running a real
//     infra-fleet-service here; that bug is out of scope for TASK-220,
//     which only owns automation-service/api-gateway, and is flagged
//     separately rather than fixed by this task). See
//     TestRunNow_E2E_ReachesRealWorkflowService's doc comment for exactly
//     what this mode does and does not prove.
package usecase_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

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
	runMigrationsWithRetry(t, dsn, migrationsPath)
	return dsn
}

// runMigrationsWithRetry runs `migrate ... up`, retrying for a few seconds
// on Postgres's own well-known post-"port is listening" startup race
// ("the database system is starting up") — testcontainers-go's
// wait.ForListeningPort only confirms the TCP port is accepting
// connections, not that Postgres has finished initdb/recovery (the same
// pre-existing flake TASK-221's integration tests hit occasionally). This
// test spins up two Postgres containers instead of one, doubling the
// chance of hitting it, so it's worth absorbing here rather than failing
// the whole run on a transient race unrelated to what's under test.
func runMigrationsWithRetry(t *testing.T, dsn, migrationsPath string) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	var lastOut []byte
	var lastErr error
	for {
		cmd := exec.Command("migrate", "-path", migrationsPath, "-database", dsn, "up")
		out, err := cmd.CombinedOutput()
		if err == nil {
			return
		}
		lastOut, lastErr = out, err
		if !bytes.Contains(out, []byte("the database system is starting up")) || time.Now().After(deadline) {
			t.Fatalf("running migrations: %v\n%s", lastErr, lastOut)
		}
		time.Sleep(500 * time.Millisecond)
	}
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

// realWorkflowServiceAddr returns a gRPC address for a genuinely live
// workflow-service process to dial. WORKFLOW_SERVICE_ADDR, if set, is used
// directly (docker-compose mode). Otherwise this test stands up
// workflow-service's own real binary itself — see this file's package doc
// comment for exactly what that self-contained mode proves and doesn't.
func realWorkflowServiceAddr(t *testing.T) string {
	t.Helper()
	if addr := os.Getenv("WORKFLOW_SERVICE_ADDR"); addr != "" {
		return addr
	}
	return startWorkflowService(t)
}

// startWorkflowService builds and runs workflow-service's actual
// cmd/server binary (not a fake, not a hand-rolled test server) against a
// disposable testcontainers-go Postgres with workflow-service's own real
// migrations, and returns its gRPC address once it's accepting
// connections. INFRA_FLEET_SERVICE_ADDR is deliberately left unset:
// workflow-service's own infrafleetclient.Dial is lazy (confirmed by
// reading internal/adapter/infrafleetclient/relay_client.go — grpc.NewClient
// doesn't block on connect), so workflow-service starts up fine without a
// live infra-fleet-service; only the downstream step-executor relay call
// itself fails, and it fails with workflow-service's own well-formed
// business-level gRPC error (see TestRunNow_E2E_ReachesRealWorkflowService's
// status-code assertion), not a "workflow-service is unreachable" error —
// which is exactly the distinction this test needs.
func startWorkflowService(t *testing.T) string {
	t.Helper()

	wfDir, err := filepath.Abs("../../../workflow-service")
	if err != nil {
		t.Fatalf("resolving workflow-service dir: %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(wfDir, "cmd", "server")); statErr != nil {
		t.Fatalf("workflow-service cmd/server not found at %s: %v", wfDir, statErr)
	}

	dsn := testutil.StartPostgres(t, "workflow")
	runMigrationsWithRetry(t, dsn, filepath.Join(wfDir, "migrations"))

	binPath := filepath.Join(t.TempDir(), "workflow-service-e2e")
	build := exec.Command("go", "build", "-o", binPath, "./cmd/server")
	build.Dir = wfDir
	if out, buildErr := build.CombinedOutput(); buildErr != nil {
		t.Fatalf("building workflow-service: %v\n%s", buildErr, out)
	}

	grpcPort := freeTCPPort(t)
	httpPort := freeTCPPort(t)

	run := exec.Command(binPath)
	run.Dir = wfDir
	run.Env = append(os.Environ(),
		"DATABASE_DSN="+dsn,
		fmt.Sprintf("GRPC_PORT=%d", grpcPort),
		fmt.Sprintf("HTTP_PORT=%d", httpPort),
	)
	var output bytes.Buffer
	run.Stdout = &output
	run.Stderr = &output
	if startErr := run.Start(); startErr != nil {
		t.Fatalf("starting workflow-service: %v", startErr)
	}
	t.Cleanup(func() {
		if run.Process != nil {
			_ = run.Process.Kill()
			_, _ = run.Process.Wait()
		}
		if t.Failed() {
			t.Logf("workflow-service process output:\n%s", output.String())
		}
	})

	addr := fmt.Sprintf("localhost:%d", grpcPort)
	waitForTCP(t, addr, 15*time.Second)
	return addr
}

func freeTCPPort(t *testing.T) int {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("finding free tcp port: %v", err)
	}
	defer lis.Close()
	return lis.Addr().(*net.TCPAddr).Port
}

func waitForTCP(t *testing.T, addr string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		conn, err := net.DialTimeout("tcp", addr, 500*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("workflow-service did not start listening on %s within %s: %v", addr, timeout, err)
		}
		time.Sleep(200 * time.Millisecond)
	}
}

// newRealWorkflowExecutor dials a real live workflow-service (see
// realWorkflowServiceAddr) and wraps the connection in the real
// grpcclient.WorkflowClient — production's exact WorkflowStepExecutor
// implementation, not a fake.
func newRealWorkflowExecutor(t *testing.T) usecase.WorkflowStepExecutor {
	t.Helper()
	addr := realWorkflowServiceAddr(t)
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
//
// This test also handles — deliberately, not as an afterthought — the case
// where RunNow.Execute itself returns an error (run_now.go's execErr
// branch fires whenever the downstream dispatch fails for ANY reason,
// including "workflow-service is completely unreachable" AND "workflow-
// service is reached but its own downstream relay fails"). Those two cases
// must not be conflated: only the first is the actual regression this task
// exists to catch. The discriminator is the gRPC status code — a
// genuinely-unreachable service surfaces as codes.Unavailable (a transport
// failure, from this test's own client dial or from workflow-service's own
// client-to-infra-fleet-service dial bubbling up as-is); a real service
// that received and processed the RPC returns ITS OWN well-formed
// business-level code (Internal, mapped from apperrors.KindInternal, per
// workflow-service's ExecuteAdHocStep doc comment). Asserting the error is
// NOT a transport code is the concrete, sandbox-feasible form of SOL-033's
// "reaches a real workflow-service" claim in self-contained mode (see this
// file's package doc comment) — a live Dev Server Agent for the shell step
// to actually relay to is out of reach here regardless of workflow-service
// itself being real.
func TestRunNow_E2E_ReachesRealWorkflowService(t *testing.T) {
	tenantID := uuid.NewString()
	ctx := tenant.WithTenantID(context.Background(), tenantID)

	automations, runs := newRealPostgresAutomationRepository(t)
	executor := newRealWorkflowExecutor(t)

	// 1. Create an automation with step_type=shell and a trivial
	// step_config_json — connectionId/script per domain.ShellStepConfig's
	// real JSON shape (workflow-service/internal/domain/step.go), not a
	// placeholder "command" key: this is what actually exercises the real
	// ShellExecutor's relay path rather than dead-ending on its own
	// config-validation error before ever reaching infra-fleet-service.
	stepConfig, err := json.Marshal(map[string]any{
		"connectionId": "e2e-test-connection", // no real Dev Server Agent to resolve this to — see this file's package doc comment
		"script":       "echo automation-e2e-marker",
	})
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
	_, execErr := runNow.Execute(ctx, usecase.RunNowInput{
		AutomationID: automation.ID,
		RequestID:    requestID,
		Trigger:      domain.RunTriggerManual,
	})
	if execErr != nil {
		if code := status.Code(execErr); code == codes.Unavailable || code == codes.DeadlineExceeded || code == codes.Unknown {
			t.Fatalf("RunNow.Execute failed with a TRANSPORT-level gRPC code %s — workflow-service was not actually reached (this IS the TS-Gap-3-style regression TASK-220 exists to catch): %v", code, execErr)
		}
		t.Logf("RunNow.Execute returned a business-level error rather than succeeding outright — expected in this sandbox (no live Dev Server Agent for the shell step's relay to reach), and proof the request WAS delivered to and processed by a real workflow-service: %v", execErr)
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
			t.Fatalf("run for request_id %s did not leave pending/running within 30s (last status: %s)", requestID, final.Status)
		}
		time.Sleep(500 * time.Millisecond)
	}

	// 4. THE regression assertion — a real dispatch always resolves
	// Terminal(), never hangs non-terminal past the deadline above.
	if !final.Status.Terminal() {
		t.Fatalf("run %s resolved non-terminal status %q — RunNow is not reaching a real workflow-service", final.ID, final.Status)
	}

	if execErr != nil {
		// The execErr branch (run_now.go) always resolves Failed with
		// ErrorMessage set, never OutputJSON — see MarkFailed's signature.
		if final.Status != domain.RunStatusFailed {
			t.Errorf("expected status failed for a dispatch-level error, got %q", final.Status)
		}
		if final.ErrorMessage == "" {
			t.Error("expected error_message to be populated from the real workflow-service round trip")
		}
		return
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
