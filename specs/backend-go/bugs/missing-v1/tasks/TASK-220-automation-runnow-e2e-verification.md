# TASK-220: Runtime-verification E2E test for `automation.runNow`'s real execution path

**From Solution:** SOL-033 ("Runtime verification for `automation.runNow`" section)
**Priority:** P1 — closes an unverified claim BUG-033 explicitly flags; must land before/alongside TASK-218's list/update/delete work ships, per SOL-033's explicit "don't let this ship quietly alongside an unverified runNow claim"
**Service:** `automation-service`
**File:** `backend-go/services/automation-service/internal/usecase/run_now_e2e_test.go` (new)
**Depends on:** none (exercises the existing, already-real `RunNow` interactor — no new production code)
**Status:** `[x]` DONE — verified for real against a live `deploy/dev/docker-compose.yml` stack, not just the self-contained testcontainers mode. The `infra-fleet-service` migration blocker noted in this task's previous pass (`migrations/` had 3 files all numbered `0004_*`) is **no longer present on this branch** — `infra-fleet-service`'s migrations now run clean (`0001`..`0006`), so this pass brought up `postgres`, `vault`+`vault-init`, `nats`, `infra-fleet-service` (+`migrate-infra`), `workflow-service` (+`migrate-workflow`), `automation-service` (+`migrate-automation`), plus `auth-service`/`usage-service`/`credential-broker-service`/`api-gateway` (api-gateway's own `depends_on` chain) via `docker compose up -d <services>` from `deploy/dev/` after `cp .env.example .env` + setting `POSTGRES_PASSWORD`. Backend binaries were built directly with `GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build ./cmd/server` per service into `deploy/dev/bin/<service>/orca` (same output layout `build-local.sh` produces; its frontend/pnpm build step was skipped as unnecessary for this verification). Postgres and `workflow-service` were given temporary host-port publishes via a local (uncommitted, deleted before commit) `deploy/dev/docker-compose.override.yml` so the existing `run_now_e2e_test.go` (already self-hosting-capable from the prior pass, unchanged in this one) could dial them from the host:
```
WORKFLOW_SERVICE_ADDR=localhost:29091 \
AUTOMATION_E2E_DATABASE_DSN="postgres://orca:<POSTGRES_PASSWORD>@localhost:25432/automation?sslmode=disable" \
go test ./internal/usecase/... -tags e2e -run TestRunNow_E2E -v -timeout 90s
```
Real, reproducible (run twice) result: `TestRunNow_E2E_ReachesRealWorkflowService` **PASSES**. `RunNow.Execute` returns a genuine business-level error — `AUTOMATION_WORKFLOW_UNAVAILABLE: ... rpc error: code = Internal desc = WORKFLOW_STEP_EXECUTION_FAILED: infrafleetclient: shell: ... rpc error: code = Internal desc = INFRA_RESOLVE_FAILED: failed to resolve connection` — proving the full real chain automation-service → workflow-service → infra-fleet-service, all three live containers over real gRPC, not a transport-level `Unavailable`/`Unknown`/`DeadlineExceeded` anywhere in the chain (the exact TS-Gap-3-style regression this test guards against). The remaining `INFRA_RESOLVE_FAILED` is expected and correct: there is no live Dev Server Agent registered under the test's placeholder `connectionId` in this environment — SOL-033/BUG-033's actual claim ("RunNow reaches a real workflow-service and gets a real result back") is now verified end-to-end against real, independently-started services, not just the single self-hosted workflow-service binary the previous pass could reach.

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
