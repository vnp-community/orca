# TASK-218: Add `automation.list`/`update`/`delete` — proto + repository + usecase

**From Solution:** SOL-033 (Part 2)
**Priority:** P1
**Service:** `automation-service`
**File:** `backend-go/proto/orca/automation/v1/automation.proto`, `internal/usecase/ports.go`, `internal/usecase/list_automations.go`, `update_automation.go`, `delete_automation.go` (new), `internal/adapter/postgres/repository.go`, `internal/adapter/grpc/server.go`, `cmd/server/main.go`
**Depends on:** none
**Status:** `[x]` DONE — proto RPCs/messages added and generated (`buf generate` clean, `wrapperspb` confirmed working), `AutomationRepository` port extended, `list_automations.go`/`update_automation.go`/`delete_automation.go` usecases added, Postgres `List`/`Update`/`Delete` implemented (fixed a real `uuid > text` type-mismatch bug in the `List` cursor query along the way), gRPC server methods + `main.go` wiring added. `go build`/`go vet` clean for the whole service (default + `-tags e2e`). `next.Validate()` doesn't exist on `domain.Automation` — adapted by reusing `domain.NewAutomation`'s invariant checks instead (see report).

---

## Context

`automation.delete`/`list`/`update` are unbuilt at every layer, including
the repository port itself (`AutomationRepository` only has
`Create`/`Get` today). This task extends the schema that's actually there
(`migrations/0001_init.up.sql` + `0002_scheduler_columns.up.sql`) — not
`automation-service.md` §5's fuller TDD field list, which the scaffold
already diverged from in a defensible direction (generic
`step_type`/`step_config_json` vs. the doc's agent-specific fields).

## Changes to make

### Step 1: Proto — `backend-go/proto/orca/automation/v1/automation.proto`

Add to the `AutomationService` service block:

```protobuf
  rpc ListAutomations(ListAutomationsRequest) returns (ListAutomationsResponse);
  rpc UpdateAutomation(UpdateAutomationRequest) returns (UpdateAutomationResponse);
  rpc DeleteAutomation(DeleteAutomationRequest) returns (google.protobuf.Empty);
```

Add the import for `Empty` and `wrappers.proto` at the top of the file
(no existing message in this proto uses either yet):

```protobuf
import "google/protobuf/empty.proto";
import "google/protobuf/wrappers.proto";
```

Append messages:

```protobuf
message ListAutomationsRequest {
  string tenant_id = 1;
  string page_token = 2;
  int32 page_size = 3;
}
message ListAutomationsResponse {
  repeated Automation automations = 1;
  string next_page_token = 2;
}

// UpdateAutomationRequest uses optional (wrapper-typed) fields rather than
// full-replace semantics — a partial edit (e.g. just toggling `enabled`) is
// the frontend's real use case (an automation's on/off switch in the UI
// list), and full-replace would force every caller to re-send fields it
// isn't changing.
message UpdateAutomationRequest {
  string id = 1;
  string tenant_id = 2;
  google.protobuf.StringValue name = 3;
  google.protobuf.StringValue rrule = 4;
  google.protobuf.StringValue step_config_json = 5;
  orca.workflow.v1.StepType step_type = 6; // 0 (unspecified) = no change
  google.protobuf.BoolValue enabled = 7;
  google.protobuf.StringValue dtstart = 8;
  google.protobuf.StringValue timezone = 9;
}
message UpdateAutomationResponse {
  Automation automation = 1;
}

message DeleteAutomationRequest {
  string id = 1;
  string tenant_id = 2;
}
```

`buf breaking` stays clean — all additive. This is the first use of
`google.protobuf.{StringValue,BoolValue}` in this codebase's protos —
confirm `buf generate` pulls in `wrapperspb` (`google.golang.org/protobuf/types/known/wrapperspb`)
without any additional buf module config; it's a well-known type bundled
with `protoc`/`buf`'s standard includes, so no new dependency is expected.

### Step 2: Repository port extension — `internal/usecase/ports.go`

Extend `AutomationRepository`:

```go
type AutomationRepository interface {
	Create(ctx context.Context, automation domain.Automation) error
	Get(ctx context.Context, tenantID, id string) (domain.Automation, error)
	// List returns every automation for tenantID, cursor-paginated —
	// automation.list is distinct from ListRuns (runs of one automation)
	// per BUG-033's finding; this is "all automations for a tenant."
	List(ctx context.Context, tenantID, pageToken string, pageSize int32) ([]domain.Automation, string, error)
	// Update persists a partial field update (name/rrule/step_config_json/
	// step_type/enabled/dtstart/timezone) — see UpdateAutomationRequest's
	// field-mask-shaped design above.
	Update(ctx context.Context, tenantID string, automation domain.Automation) error
	// Delete removes an automation and cascades to its runs
	// (automation_runs.automation_id has ON DELETE CASCADE per
	// migrations/0001_init.up.sql — no separate run-cleanup step needed).
	Delete(ctx context.Context, tenantID, id string) error
}
```

### Step 3: Usecases — `internal/usecase/`

`update_automation.go`:

```go
package usecase

import (
	"time"

	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/automation-service/internal/domain"
)

// UpdateAutomationInput uses pointer fields: nil = "not being changed"
// (matches the proto's wrapper-typed field-mask shape) — mirrors SOL-001's
// UpdateAccessPolicy pattern of not conflating "empty string" with "unset."
type UpdateAutomationInput struct {
	TenantID       string
	ID             string
	Name           *string
	RRule          *string
	StepConfigJSON *string
	StepType       *domain.StepType
	Enabled        *bool
	Dtstart        *time.Time
	Timezone       *string
}

type UpdateAutomation struct {
	repo AutomationRepository
}

func NewUpdateAutomation(repo AutomationRepository) *UpdateAutomation {
	return &UpdateAutomation{repo: repo}
}

func (uc *UpdateAutomation) Execute(ctx context.Context, in UpdateAutomationInput) (domain.Automation, error) {
	current, err := uc.repo.Get(ctx, in.TenantID, in.ID)
	if err != nil {
		return domain.Automation{}, apperrors.New(apperrors.KindNotFound, "AUTOMATION_NOT_FOUND", "automation not found", err)
	}
	next := current
	if in.Name != nil {
		next.Name = *in.Name
	}
	if in.RRule != nil {
		// Re-validate the RRULE string on every edit, same as NewAutomation
		// does at creation — a syntactically valid-at-create rule doesn't
		// stay valid-by-construction after an in-place field edit.
		next.RRule = *in.RRule
	}
	if in.StepConfigJSON != nil {
		next.StepConfigJSON = *in.StepConfigJSON
	}
	if in.StepType != nil {
		next.StepType = *in.StepType
	}
	if in.Enabled != nil {
		next.Enabled = *in.Enabled
	}
	if in.Dtstart != nil {
		next.DTStart = *in.Dtstart
	}
	if in.Timezone != nil {
		next.Timezone = *in.Timezone
	}
	if err := next.Validate(); err != nil { // reuse domain's existing invariant checks
		return domain.Automation{}, apperrors.New(apperrors.KindInvalidArgument, "AUTOMATION_INVALID", err.Error(), err)
	}
	if err := uc.repo.Update(ctx, in.TenantID, next); err != nil {
		return domain.Automation{}, apperrors.New(apperrors.KindInternal, "AUTOMATION_UPDATE_FAILED", "failed to persist update", err)
	}
	return next, nil
}
```

**Concurrency note to flag in the PR description, not solve here**: the
scheduler ticker (`adapter/scheduler/`) reads `enabled`/`next_run_at` on its
own ~1-minute cadence while `UpdateAutomation` can toggle `enabled` or
change `rrule` concurrently. A read-modify-write `Update` (as sketched) has
a narrow race with a concurrent scheduler claim (`SELECT ... FOR UPDATE
SKIP LOCKED`) — acceptable per `automation-service.md` §8's
"at-least-once, not exactly-once, by design" framing (a stale-`enabled`
window before the next tick corrects it).

`list_automations.go`:

```go
package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/automation-service/internal/domain"
)

type ListAutomationsInput struct {
	TenantID  string
	PageToken string
	PageSize  int32
}

type ListAutomationsResult struct {
	Automations   []domain.Automation
	NextPageToken string
}

type ListAutomations struct {
	repo AutomationRepository
}

func NewListAutomations(repo AutomationRepository) *ListAutomations {
	return &ListAutomations{repo: repo}
}

func (uc *ListAutomations) Execute(ctx context.Context, in ListAutomationsInput) (ListAutomationsResult, error) {
	if in.TenantID == "" {
		return ListAutomationsResult{}, apperrors.New(apperrors.KindInvalidArgument, "AUTOMATION_MISSING_TENANT_ID", "tenant_id is required", nil)
	}
	automations, nextToken, err := uc.repo.List(ctx, in.TenantID, in.PageToken, in.PageSize)
	if err != nil {
		return ListAutomationsResult{}, apperrors.New(apperrors.KindInternal, "AUTOMATION_LIST_FAILED", "failed to list automations", err)
	}
	return ListAutomationsResult{Automations: automations, NextPageToken: nextToken}, nil
}
```

`delete_automation.go`:

```go
package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
)

type DeleteAutomationInput struct {
	TenantID string
	ID       string
}

type DeleteAutomation struct {
	repo AutomationRepository
}

func NewDeleteAutomation(repo AutomationRepository) *DeleteAutomation {
	return &DeleteAutomation{repo: repo}
}

func (uc *DeleteAutomation) Execute(ctx context.Context, in DeleteAutomationInput) error {
	if in.ID == "" {
		return apperrors.New(apperrors.KindInvalidArgument, "AUTOMATION_MISSING_ID", "id is required", nil)
	}
	if err := uc.repo.Delete(ctx, in.TenantID, in.ID); err != nil {
		return apperrors.New(apperrors.KindInternal, "AUTOMATION_DELETE_FAILED", "failed to delete automation", err)
	}
	return nil
}
```

### Step 4: Repository (Postgres) — `internal/adapter/postgres/repository.go`

```go
func (r *AutomationRepository) List(ctx context.Context, tenantID, pageToken string, pageSize int32) ([]domain.Automation, string, error) {
	if pageSize <= 0 {
		pageSize = 50
	}
	rows, err := r.pool.Query(ctx, `
		SELECT id, tenant_id, name, rrule, dtstart, step_type, step_config_json, enabled, timezone, next_run_at, created_at, updated_at
		FROM automation.automations
		WHERE tenant_id = $1 AND ($2 = '' OR id > $2)
		ORDER BY id
		LIMIT $3
	`, tenantID, pageToken, pageSize)
	if err != nil {
		return nil, "", fmt.Errorf("postgres: query automations: %w", err)
	}
	defer rows.Close()

	var out []domain.Automation
	for rows.Next() {
		a, err := scanAutomation(rows)
		if err != nil {
			return nil, "", fmt.Errorf("postgres: scan automation row: %w", err)
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, "", fmt.Errorf("postgres: iterate automation rows: %w", err)
	}
	nextToken := ""
	if len(out) == int(pageSize) {
		nextToken = out[len(out)-1].ID
	}
	return out, nextToken, nil
}

func (r *AutomationRepository) Update(ctx context.Context, tenantID string, a domain.Automation) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE automation.automations
		SET name = $3, rrule = $4, step_type = $5, step_config_json = $6,
		    enabled = $7, timezone = $8, updated_at = now()
		WHERE tenant_id = $1 AND id = $2
	`, tenantID, a.ID, a.Name, a.RRule, string(a.StepType), a.StepConfigJSON, a.Enabled, a.Timezone)
	if err != nil {
		return fmt.Errorf("postgres: update automation: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("postgres: automation %s not found for tenant %s", a.ID, tenantID)
	}
	return nil
}

func (r *AutomationRepository) Delete(ctx context.Context, tenantID, id string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM automation.automations WHERE tenant_id = $1 AND id = $2`, tenantID, id)
	if err != nil {
		return fmt.Errorf("postgres: delete automation: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("postgres: automation %s not found for tenant %s", id, tenantID)
	}
	return nil
}
```

`scanAutomation` already exists (used by `Get`) — reuse it as-is for
`List`'s row scan (it must already accept a `rowScanner`-shaped interface
satisfied by both `pgx.Row` and `pgx.Rows`, per `ai-provider-service`'s
`Repository`'s identical `rowScanner` convention — if `automation-service`'s
`scanAutomation` is currently typed only for `pgx.Row`, widen its
parameter type to the same `rowScanner` interface as part of this step).

### Step 5: gRPC adapter — `internal/adapter/grpc/server.go`

Add 3 fields to `Server` (`listAutomations *usecase.ListAutomations`,
`updateAutomation *usecase.UpdateAutomation`,
`deleteAutomation *usecase.DeleteAutomation`), extend `New`'s params, and
add 3 translation methods:

```go
func (s *Server) ListAutomations(ctx context.Context, req *automationv1.ListAutomationsRequest) (*automationv1.ListAutomationsResponse, error) {
	result, err := s.listAutomations.Execute(ctx, usecase.ListAutomationsInput{
		TenantID: req.GetTenantId(), PageToken: req.GetPageToken(), PageSize: req.GetPageSize(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	out := make([]*automationv1.Automation, 0, len(result.Automations))
	for _, a := range result.Automations {
		out = append(out, toProtoAutomation(a))
	}
	return &automationv1.ListAutomationsResponse{Automations: out, NextPageToken: result.NextPageToken}, nil
}

func (s *Server) UpdateAutomation(ctx context.Context, req *automationv1.UpdateAutomationRequest) (*automationv1.UpdateAutomationResponse, error) {
	in := usecase.UpdateAutomationInput{TenantID: req.GetTenantId(), ID: req.GetId()}
	if req.GetName() != nil {
		v := req.GetName().GetValue()
		in.Name = &v
	}
	if req.GetRrule() != nil {
		v := req.GetRrule().GetValue()
		in.RRule = &v
	}
	if req.GetStepConfigJson() != nil {
		v := req.GetStepConfigJson().GetValue()
		in.StepConfigJSON = &v
	}
	if req.GetStepType() != 0 {
		v := domain.StepType(req.GetStepType().String())
		in.StepType = &v
	}
	if req.GetEnabled() != nil {
		v := req.GetEnabled().GetValue()
		in.Enabled = &v
	}
	// Dtstart/Timezone follow the same nil-check-then-pointer pattern —
	// Dtstart needs an RFC3339 parse (time.Parse(time.RFC3339, v)) before
	// assignment; propagate a parse failure as KindInvalidArgument.
	automation, err := s.updateAutomation.Execute(ctx, in)
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &automationv1.UpdateAutomationResponse{Automation: toProtoAutomation(automation)}, nil
}

func (s *Server) DeleteAutomation(ctx context.Context, req *automationv1.DeleteAutomationRequest) (*emptypb.Empty, error) {
	if err := s.deleteAutomation.Execute(ctx, usecase.DeleteAutomationInput{TenantID: req.GetTenantId(), ID: req.GetId()}); err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &emptypb.Empty{}, nil
}
```

`toProtoAutomation` should already exist in this file (used by
`CreateAutomation`/`GetAutomation` translation) — reuse it as-is.

### Step 6: Composition root — `cmd/server/main.go`

```go
	listAutomationsUC := usecase.NewListAutomations(automationRepo)
	updateAutomationUC := usecase.NewUpdateAutomation(automationRepo)
	deleteAutomationUC := usecase.NewDeleteAutomation(automationRepo)
```

Extend the `automationgrpc.New(...)` call with all 3 (variable names
illustrative — match whatever this service's actual composition root
already calls its repository/usecase variables).

## Verify

```bash
cd /opt/repos/orca/backend-go
buf generate proto
buf breaking proto --against '.git#branch=main'
cd services/automation-service
go build ./... && go vet ./...
```

Expected: clean build, `buf breaking` reports only additions. Full test
coverage for this work is TASK-221, not this task.
