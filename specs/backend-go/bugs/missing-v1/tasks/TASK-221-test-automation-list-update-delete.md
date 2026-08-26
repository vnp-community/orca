# TASK-221: Tests for `automation.list`/`update`/`delete` (usecase, repository, gRPC, wscompat)

**From Solution:** SOL-033 (Test plan)
**Priority:** P1
**Service:** `automation-service` + `api-gateway`
**File:** `backend-go/services/automation-service/internal/usecase/list_automations_test.go`, `update_automation_test.go`, `delete_automation_test.go` (new), `internal/adapter/postgres/repository_test.go`, `internal/adapter/grpc/server_test.go`, `backend-go/services/api-gateway/internal/adapter/wscompat/channels_test.go`
**Depends on:** TASK-218, TASK-219
**Status:** `[x]` DONE — Usecase unit tests (`list_automations_test.go`/`update_automation_test.go`/`delete_automation_test.go`), Postgres integration tests (`repository_test.go`, `-tags=integration`, real testcontainers-go Postgres — List/Update/Delete: tenant-scoping, cascade-delete, pagination), gRPC contract tests (`server_test.go`), and Step 4's `wscompat` channel tests (`channels_automation_task_test.go` — `TestAutomationListChannel_Success`, `TestAutomationUpdateChannel_LeavesUnsetFieldsAsNilWrapperValues` regression guard, `TestAutomationDeleteChannel_Success`, now present from the concurrent wscompat-owning agent's pass) are all written and passing (one pre-existing container-startup flake in this sandbox unrelated to this work, confirmed to pass on retry).

---

## Context

Per SOL-033's test plan: unit tests against a fake `AutomationRepository`
(no real Postgres), Postgres integration tests via `testcontainers-go` for
tenant-scoping and cascade-delete, gRPC contract tests, and `wscompat`
channel tests including a regression guard on `update`'s partial-field
semantics.

## Changes to make

### Step 1: Usecase unit tests — `internal/usecase/`

`list_automations_test.go`, `update_automation_test.go`,
`delete_automation_test.go` — fakes for all ports, no real Postgres, per
`03-clean-architecture-guidelines.md`, following `run_now_test.go`'s
existing fake-repository convention. Representative for
`update_automation_test.go`:

```go
package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/services/automation-service/internal/domain"
)

type fakeAutomationRepository struct {
	automations map[string]domain.Automation
	updateErr   error
	gotUpdate   domain.Automation
}

func (f *fakeAutomationRepository) Create(ctx context.Context, a domain.Automation) error {
	f.automations[a.ID] = a
	return nil
}
func (f *fakeAutomationRepository) Get(ctx context.Context, tenantID, id string) (domain.Automation, error) {
	a, ok := f.automations[id]
	if !ok {
		return domain.Automation{}, errNotFound
	}
	return a, nil
}
func (f *fakeAutomationRepository) List(ctx context.Context, tenantID, pageToken string, pageSize int32) ([]domain.Automation, string, error) {
	return nil, "", nil
}
func (f *fakeAutomationRepository) Update(ctx context.Context, tenantID string, a domain.Automation) error {
	f.gotUpdate = a
	return f.updateErr
}
func (f *fakeAutomationRepository) Delete(ctx context.Context, tenantID, id string) error {
	return nil
}

func TestUpdateAutomation_OnlyChangesProvidedFields(t *testing.T) {
	repo := &fakeAutomationRepository{automations: map[string]domain.Automation{
		"a1": {ID: "a1", Name: "original", RRule: "FREQ=DAILY", Enabled: false},
	}}
	uc := NewUpdateAutomation(repo)

	newName := "renamed"
	_, err := uc.Execute(context.Background(), UpdateAutomationInput{ID: "a1", Name: &newName})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.gotUpdate.Name != "renamed" {
		t.Errorf("expected name updated, got %q", repo.gotUpdate.Name)
	}
	if repo.gotUpdate.RRule != "FREQ=DAILY" {
		t.Errorf("expected rrule to remain unchanged, got %q", repo.gotUpdate.RRule)
	}
}

func TestUpdateAutomation_NotFound_ReturnsError(t *testing.T) {
	repo := &fakeAutomationRepository{automations: map[string]domain.Automation{}}
	uc := NewUpdateAutomation(repo)
	_, err := uc.Execute(context.Background(), UpdateAutomationInput{ID: "missing"})
	if err == nil {
		t.Fatal("expected error for missing automation")
	}
}
```

`list_automations_test.go` / `delete_automation_test.go` follow the same
fake-repository shape, asserting `List`/`Delete` are called with the
expected tenant-scoped arguments and errors propagate as typed
`apperrors.AppError`s.

### Step 2: Postgres integration tests — `internal/adapter/postgres/repository_test.go`

Extend with `testcontainers-go` Postgres integration tests for
`List`/`Update`/`Delete`:

- **Tenant-scoping**: insert automations for 2 different tenants, assert
  `List(tenantA, ...)` never returns tenant B's rows; assert `Update`/
  `Delete` against tenant A's id with tenant B's `tenantID` argument fail
  (0 rows affected → the "not found for tenant" error path).
- **Cascade-delete**: insert an automation, insert an `automation_runs` row
  referencing it, `Delete` the automation, assert the run row is gone via
  the `ON DELETE CASCADE` FK (a direct `SELECT` against
  `automation.automation_runs` in the test, not through the repository
  API).
- **Pagination**: insert more rows than one page size, assert `List`'s
  `next_page_token` chains correctly across 2 calls and the full set is
  covered with no duplicates/gaps.

### Step 3: gRPC contract tests — `internal/adapter/grpc/server_test.go`

Contract tests for `ListAutomations`/`UpdateAutomation`/`DeleteAutomation`:
request → usecase call → response translation, following this file's
existing per-RPC test shape (fake usecase, assert the proto response
matches the usecase result 1:1). Include one case asserting
`UpdateAutomationRequest` with all wrapper fields unset (only `id`/
`tenant_id` populated) reaches `UpdateAutomationInput` with every pointer
field `nil` — the translation-layer half of the "don't overwrite unset
fields" guarantee.

### Step 4: `wscompat` channel tests — `channels_test.go`

`TestAutomationListChannel_Success`, `TestAutomationDeleteChannel_Success`
following `TestDevServerListChannel_Success`'s shape. The key regression
guard, per SOL-033:

```go
func TestAutomationUpdateChannel_LeavesUnsetFieldsAsNilWrapperValues(t *testing.T) {
	var gotReq *automationv1.UpdateAutomationRequest
	fake := &fakeAutomationServiceClient{
		updateAutomationFunc: func(ctx context.Context, in *automationv1.UpdateAutomationRequest) (*automationv1.UpdateAutomationResponse, error) {
			gotReq = in
			return &automationv1.UpdateAutomationResponse{Automation: &automationv1.Automation{Id: in.GetId()}}, nil
		},
	}
	r := NewRegistry()
	registerAutomationChannels(r, fake)

	// Only "enabled" is set — regression guard against accidentally
	// sending zero-value overwrites (empty string, false) for fields the
	// caller didn't touch.
	if _, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1"}, "automation.update",
		argsJSON(t, map[string]any{"id": "a1", "enabled": true})); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotReq.GetName() != nil {
		t.Errorf("expected Name to remain a nil wrapper value, got %v", gotReq.GetName())
	}
	if gotReq.GetRrule() != nil {
		t.Errorf("expected Rrule to remain a nil wrapper value, got %v", gotReq.GetRrule())
	}
	if gotReq.GetEnabled() == nil || !gotReq.GetEnabled().GetValue() {
		t.Errorf("expected Enabled=true wrapper value, got %v", gotReq.GetEnabled())
	}
}
```

`fakeAutomationServiceClient` needs a `updateAutomationFunc`/
`listAutomationsFunc`/`deleteAutomationFunc` field added if this file's
existing `automation.*` tests (added when TASK-217's `automation.create`/
`automation.runs` tests were written) don't already define one —
follow `fakeGitGatewayServiceClient`'s embed-and-override pattern from
TASK-216 if a fake `AutomationServiceClient` doesn't already exist in this
file.

## Verify

```bash
cd /opt/repos/orca/backend-go/services/automation-service
go test ./internal/usecase/... -count=1 -v
go test ./internal/adapter/postgres/... -count=1 -v   # requires Docker for testcontainers-go
go test ./internal/adapter/grpc/... -count=1 -v
cd /opt/repos/orca/backend-go/services/api-gateway
go test ./internal/adapter/wscompat/... -run TestAutomation -count=1 -v
```
