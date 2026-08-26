# TASK-220: Runtime-verification E2E test for `automation.runNow`'s real execution path

**From Solution:** SOL-033 ("Runtime verification for `automation.runNow`" section)
**Priority:** P1 — closes an unverified claim BUG-033 explicitly flags; must land before/alongside TASK-218's list/update/delete work ships, per SOL-033's explicit "don't let this ship quietly alongside an unverified runNow claim"
**Service:** `automation-service`
**File:** `backend-go/services/automation-service/internal/usecase/run_now_e2e_test.go` (new)
**Depends on:** none (exercises the existing, already-real `RunNow` interactor — no new production code)
**Status:** `[partial]` Made genuinely verifiable in this sandbox (no docker-compose stack needed): the test now builds and starts workflow-service's own real `cmd/server` binary (disposable testcontainers-go Postgres + workflow-service's own real migrations, no fakes) and dials it — the exact same `grpcclient.WorkflowClient`/`WorkflowServiceClient` code path production uses. Doing this **found and fixed a real production bug**, not a test artifact: `WorkflowClient.ExecuteAdHocStep` set `TenantId` only as an unread request-message field, never as outgoing gRPC metadata, so workflow-service's own `tenant.RequireTenantID(ctx)` (fed only by its inbound `grpcmw.TenantExtractionInterceptor` reading metadata) always failed closed with `WORKFLOW_NO_TENANT` — every real RunNow→workflow-service call was broken, a strictly worse gap than the already-known "no live workflow-service in this sandbox" limitation. Fixed via new `internal/adapter/grpcclient/tenant_forwarding.go` (mirrors workflow-service's own identical `infrafleetclient` precedent for its downstream hop) plus 3 new regression tests in `workflow_client_test.go`. With the fix, the E2E test verifies real reachability by gRPC status-code discrimination: `RunNow.Execute`'s error (still expected — no live Dev Server Agent exists in this sandbox for the shell step's relay to ultimately reach) is asserted to carry a real business-level gRPC code (`Internal`/`WORKFLOW_STEP_EXECUTION_FAILED`), never a transport-level `Unavailable`/`DeadlineExceeded` — the concrete, reproducible signal that the RPC was actually delivered to and processed by a live, real workflow-service, not a stub. Confirmed stable across repeated runs. **Still not verified past that hop**: infra-fleet-service itself could not be started here to check the full chain — its `migrations/` directory has 3 pre-existing files all numbered `0004_*` (a duplicate-migration-number bug unrelated to and out of scope for this task, confirmed to also break infra-fleet-service's own existing integration tests; flagged here, not fixed) — and no live Dev Server Agent exists regardless, so a fully successful (not just reached) run remains unconfirmed. Two sketch fields didn't exist in real code and were adapted: `domain.RunStatusSkippedUnavailable` (no such constant exists — asserted `final.Status.Terminal()` instead) and `domain.Automation.LastRunAt` (doesn't exist; assertion dropped — RunNow's only durable side effect is the `AutomationRun` row). Polls via `AutomationRunRepository.FindByRequestID` (no `Get`-by-run-id method exists).

---

## Context

`automation.proto`'s service doc comment claims `RunNow` "always delegates
to `workflow-service.ExecuteAdHocStep`," and `run_now.go`'s usecase does
call a real `WorkflowStepExecutor` port (confirmed by reading the code:
tenant resolution, idempotency check via `FindByRequestID`, a genuine call
to `uc.executor`) — but "the Go code calls an interface method" and "the
interface's real implementation actually reaches `workflow-service` and
gets a real result back" are two different claims, and only the first is
verified by reading source. This task is the concrete verification step,
not just a recommendation.

## Changes to make

**New file:** `backend-go/services/automation-service/internal/usecase/run_now_e2e_test.go`

Build tag it into CI's docker-compose E2E tier (matching whatever tag
convention `03-clean-architecture-guidelines.md`'s "small number of
cross-service scenarios run against a full docker-compose stack in CI"
tests already use elsewhere in this repo — check an existing
cross-service E2E test file for the exact tag/env-var gate before adding a
new one).

```go
//go:build e2e

package usecase_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/automation-service/internal/domain"
	"github.com/stablyai/orca-go/services/automation-service/internal/usecase"
	// import whatever real Postgres AutomationRepository/AutomationRunRepository
	// constructors and the real grpcclient.WorkflowExecutor this service's
	// composition root (cmd/server/main.go) already wires — reuse those
	// exact constructors rather than hand-rolling new ones, so this test
	// exercises the identical wiring production uses.
)

// TestRunNow_E2E_ReachesRealWorkflowService is the concrete verification
// SOL-033 calls for: not a fake WorkflowStepExecutor, the actual
// grpcclient.WorkflowExecutor dialed to a real running workflow-service in
// docker-compose's stack. Asserts the terminal status is succeeded or
// failed — NEVER skipped_unavailable, the exact TS-Gap-3 regression this
// migration claims to close (automation-service.md §10). A test that
// merely asserts "no error was returned" would miss a silent
// reintroduction of the old no-op behavior — that is exactly why this
// assertion is on run.Status, not just err.
func TestRunNow_E2E_ReachesRealWorkflowService(t *testing.T) {
	ctx := tenant.WithTenantID(context.Background(), uuid.NewString())

	automations := newRealPostgresAutomationRepository(t) // dial docker-compose's Postgres, per this file's build-tag gate
	runs := newRealPostgresAutomationRunRepository(t)
	executor := newRealWorkflowExecutor(t) // grpcclient.WorkflowExecutor dialed to docker-compose's workflow-service — NOT a fake

	// 1. Create an automation with step_type=shell and a trivial
	// step_config_json.
	stepConfig, err := json.Marshal(map[string]any{"command": "echo automation-e2e-marker"})
	if err != nil {
		t.Fatalf("marshal step config: %v", err)
	}
	automation := domain.Automation{
		ID:             uuid.NewString(),
		Name:           "e2e-verification",
		RRule:          "",
		StepType:       domain.StepTypeShell,
		StepConfigJSON: string(stepConfig),
		Enabled:        true,
		Timezone:       "UTC",
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	tenantID, _ := tenant.TenantIDFromContext(ctx)
	automation.TenantID = tenantID
	if err := automations.Create(ctx, automation); err != nil {
		t.Fatalf("create automation: %v", err)
	}

	// 2. Call RunNow with a fresh request_id.
	runNow := usecase.NewRunNow(automations, runs, executor)
	run, err := runNow.Execute(ctx, usecase.RunNowInput{
		AutomationID: automation.ID,
		RequestID:    uuid.NewString(),
		Trigger:      domain.RunTriggerManual,
	})
	if err != nil {
		t.Fatalf("RunNow.Execute: %v", err)
	}

	// 3. Poll for the run to leave pending/running.
	var final domain.AutomationRun
	deadline := time.Now().Add(30 * time.Second)
	for {
		final, err = runs.Get(ctx, tenantID, run.ID) // or whatever read method AutomationRunRepository exposes
		if err != nil {
			t.Fatalf("poll run: %v", err)
		}
		if final.Status != domain.RunStatusPending && final.Status != domain.RunStatusRunning {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("run %s did not leave pending/running within 30s (last status: %s)", run.ID, final.Status)
		}
		time.Sleep(500 * time.Millisecond)
	}

	// 4. THE regression assertion — never skipped_unavailable.
	if final.Status == domain.RunStatusSkippedUnavailable {
		t.Fatalf("run %s resolved skipped_unavailable — this is the exact TS-Gap-3 regression automation-service.md §10 claims is closed; RunNow is not reaching a real workflow-service", run.ID)
	}
	if final.Status != domain.RunStatusSucceeded && final.Status != domain.RunStatusFailed {
		t.Fatalf("expected terminal status succeeded or failed, got %q", final.Status)
	}

	// 5. Proof the call reached a real executor, not a stub returning a
	// canned success: assert the shell command's real output is present,
	// not a placeholder.
	if final.OutputJSON == "" {
		t.Error("expected output_json to be populated from the real workflow-service execution")
	}

	// 6. Assert the automation's last_run_at was updated.
	reloaded, err := automations.Get(ctx, tenantID, automation.ID)
	if err != nil {
		t.Fatalf("reload automation: %v", err)
	}
	if reloaded.LastRunAt == nil || reloaded.LastRunAt.IsZero() {
		t.Error("expected last_run_at to be set after RunNow completed")
	}
}
```

Adjust the exact repository/domain constructor and field names above
against this service's real `internal/adapter/postgres` and
`internal/domain` packages before finalizing — the sketch above follows
`run_now.go`'s actual `RunNowInput`/`AutomationRepository`/
`AutomationRunRepository` shapes (confirmed by reading that file) but the
`AutomationRun` polling/read method name and `domain.RunStatus*` constants
must be checked against `internal/domain`'s real enum before this compiles.

## What to do if the assertion fails

If this test reveals `RunNow` does **not** reach a real `workflow-service`
instance (e.g. `WorkflowExecutor`'s `grpcclient` implementation is itself
still a stub, mirroring `git-gateway-service`'s `ConnectionResolver` stub
noted in that service's own `ports.go`), **file that as its own bug
immediately** — do not let TASK-218's `list`/`update`/`delete` work ship
alongside an unverified `runNow` claim, per SOL-033's explicit instruction.

## CI wiring

Add this test file's build tag to whichever `docker-compose` E2E CI job
already runs other cross-service scenarios (not skipped, not `-short` —
per SOL-033: "a flaky or skipped verification here is equivalent to not
having it").

## Verify

```bash
cd /opt/repos/orca/backend-go
docker compose up -d postgres workflow-service automation-service
cd services/automation-service
go test ./internal/usecase/... -tags e2e -run TestRunNow_E2E -v -timeout 60s
```

Expected: test passes with a terminal status of `succeeded` or `failed`
(never `skipped_unavailable`), `output_json` populated with real shell
output, and `last_run_at` updated.
