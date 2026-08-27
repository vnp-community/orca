# SOL-033: Wire `automation.create`/`runs`, build `list`/`update`/`delete` down to the repository layer, and verify `runNow`'s execution path end-to-end

**Resolves:** [BUG-033](../BUG-033-automation-channels-partially-implemented.md)
**Service:** `automation-service` (new RPCs + repository methods) + `api-gateway` (`wscompat` wiring)
**Affected files (proposed):**
- `backend-go/proto/orca/automation/v1/automation.proto`
- `backend-go/services/automation-service/internal/usecase/ports.go` (extend `AutomationRepository`)
- `backend-go/services/automation-service/internal/usecase/list_automations.go`,
  `update_automation.go`, `delete_automation.go` (new)
- `backend-go/services/automation-service/internal/adapter/postgres/repository.go`
- `backend-go/services/automation-service/internal/adapter/grpc/server.go`
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go`
- `backend-go/services/automation-service/internal/usecase/run_now_e2e_test.go` (new — runtime verification, see final section)
**Status:** 📋 Proposed — not yet implemented

---

## Two distinct gaps, one lighter than the other

Per BUG-033, `automation.create`/`automation.runs` are wrapper-only gaps —
`CreateAutomation`/`ListRuns` already exist end-to-end
(`automation.proto:14,16`, `server.go:39,66`, REST at
`automation_routes.go:23,25`). `automation.delete`/`list`/`update` are
unbuilt at every layer, including the repository port itself
(`AutomationRepository` only has `Create`/`Get` — `ports.go:19-21`). This
proposal treats them accordingly: Part 1 is a thin `wscompat` wrapper; Part
2 is real proto + repository + usecase work grounded in the schema that
actually exists today (`migrations/0001_init.up.sql` +
`0002_scheduler_columns.up.sql`), not the TDD's fuller
`automation-service.md` §5 field list — the scaffold's `Automation` already
diverged from that doc in a defensible direction (a generic
`step_type`/`step_config_json` pair shared with `workflow-service`'s own
`StepType` enum, rather than the doc's agent-specific `prompt`/`precheck`/
`agent_id`/`execution_target` fields) — this proposal extends the schema
that's actually there.

---

## Part 1 — Wiring-only quick wins: `automation.create`/`automation.runs`

Following `registerAutomationChannels`'s existing `automation.runNow`
pattern (`channels.go:257-275`) exactly:

```go
// Add to registerAutomationChannels (channels.go), after runNow.

r.Register("automation.create", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
	type createArgs struct {
		Name           string `json:"name"`
		RRule          string `json:"rrule"`
		StepConfigJSON string `json:"stepConfigJson"`
		StepType       string `json:"stepType"`
		Dtstart        string `json:"dtstart"`
		Timezone       string `json:"timezone"`
	}
	in, err := decodeArg[createArgs](args, 0)
	if err != nil {
		return nil, err
	}
	resp, err := client.CreateAutomation(ctx, &automationv1.CreateAutomationRequest{
		TenantId: id.TenantID, Name: in.Name, Rrule: in.RRule,
		StepConfigJson: in.StepConfigJSON, StepType: parseStepType(in.StepType),
		Dtstart: in.Dtstart, Timezone: in.Timezone,
	})
	if err != nil {
		return nil, err
	}
	return resp.GetAutomation(), nil
})

r.Register("automation.runs", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
	type runsArgs struct {
		AutomationID string `json:"automationId"`
		PageToken    string `json:"pageToken"`
		PageSize     int32  `json:"pageSize"`
	}
	in, err := decodeArg[runsArgs](args, 0)
	if err != nil {
		return nil, err
	}
	resp, err := client.ListRuns(ctx, &automationv1.ListRunsRequest{
		AutomationId: in.AutomationID, PageToken: in.PageToken, PageSize: in.PageSize,
	})
	if err != nil {
		return nil, err
	}
	return resp, nil
})
```

`TenantId` is worth flagging: `CreateAutomationRequest.tenant_id` is a
message field (unlike git-gateway-service's tenant-free proto), so it must
come from the validated `Identity` the channel handler receives, never from
`args` — same "never trust a client-supplied tenant" posture SOL-001 applies
to admin routes. `parseStepType` maps the frontend's string step-type onto
`workflowv1.StepType` — reuse whatever helper `automation_routes.go`'s REST
handler already has for this translation (`automation_routes.go`'s
`createAutomationRequestBody` decodes the same shape), don't duplicate it.

---

## Part 2 — `automation.list`/`update`/`delete`: real proto + repository + usecase work

### Repository port extension

```go
// ports.go — extend AutomationRepository
type AutomationRepository interface {
	Create(ctx context.Context, automation domain.Automation) error
	Get(ctx context.Context, tenantID, id string) (domain.Automation, error)
	// List returns every automation for tenantID, cursor-paginated —
	// automation.list is distinct from ListRuns (runs of one automation)
	// per BUG-033's finding; this is "all automations for a tenant."
	List(ctx context.Context, tenantID, pageToken string, pageSize int32) ([]domain.Automation, string, error)
	// Update persists a partial field update (name/rrule/step_config_json/
	// step_type/enabled/dtstart/timezone) — see UpdateAutomationRequest's
	// field-mask-shaped design below.
	Update(ctx context.Context, tenantID string, automation domain.Automation) error
	// Delete removes an automation and cascades to its runs
	// (automation_runs.automation_id has ON DELETE CASCADE per
	// migrations/0001_init.up.sql — no separate run-cleanup step needed).
	Delete(ctx context.Context, tenantID, id string) error
}
```

### Proto additions

```protobuf
rpc ListAutomations(ListAutomationsRequest) returns (ListAutomationsResponse);
rpc UpdateAutomation(UpdateAutomationRequest) returns (UpdateAutomationResponse);
rpc DeleteAutomation(DeleteAutomationRequest) returns (google.protobuf.Empty);

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
// full-replace semantics — a partial edit (e.g. just toggling `enabled`)
// is the frontend's real use case (an automation's on/off switch in the
// UI list), and full-replace would force every caller to re-send fields
// it isn't changing.
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

`buf breaking` stays clean — all additive.

### Usecase layer

```go
// usecase/update_automation.go
type UpdateAutomationInput struct {
	TenantID string
	ID       string
	// Pointer fields: nil = "not being changed" (matches the proto's
	// wrapper-typed field-mask shape above) — mirrors SOL-001's
	// UpdateAccessPolicy pattern of not conflating "empty string" with
	// "unset."
	Name           *string
	RRule          *string
	StepConfigJSON *string
	StepType       *domain.StepType
	Enabled        *bool
	Dtstart        *time.Time
	Timezone       *string
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
	// ... remaining fields, same pattern ...
	if err := next.Validate(); err != nil { // reuse domain's existing invariant checks
		return domain.Automation{}, apperrors.New(apperrors.KindInvalidArgument, "AUTOMATION_INVALID", err.Error(), err)
	}
	if err := uc.repo.Update(ctx, in.TenantID, next); err != nil {
		return domain.Automation{}, apperrors.New(apperrors.KindInternal, "AUTOMATION_UPDATE_FAILED", "failed to persist update", err)
	}
	return next, nil
}
```

**Concurrency note worth flagging, not solving here**: the scheduler ticker
(`adapter/scheduler/`) reads `enabled`/`next_run_at` on its own ~1-minute
cadence (§7) while `UpdateAutomation` can toggle `enabled` or change `rrule`
concurrently. A read-modify-write `Update` (as sketched) has a narrow race
with a concurrent scheduler claim (`SELECT ... FOR UPDATE SKIP LOCKED`) —
acceptable per automation-service.md §8's "at-least-once, not exactly-once,
by design" framing (a stale-`enabled` window before the next tick corrects
it), but call this out in the PR description rather than silently accepting
it as obviously fine.

`DeleteAutomation` and `ListAutomations`' usecases are simpler
pass-throughs to the new repository methods — omitted here for space, same
shape as `Get`.

### Repository (Postgres) — grounded in the actual schema

```go
// adapter/postgres/repository.go

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
	// ... scan loop mirroring scanAutomation, return next cursor = last row's id ...
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

### `wscompat` wiring for `list`/`update`/`delete`

```go
r.Register("automation.list", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
	type listArgs struct {
		PageToken string `json:"pageToken"`
		PageSize  int32  `json:"pageSize"`
	}
	in, err := decodeArg[listArgs](args, 0)
	if err != nil {
		return nil, err
	}
	resp, err := client.ListAutomations(ctx, &automationv1.ListAutomationsRequest{
		TenantId: id.TenantID, PageToken: in.PageToken, PageSize: in.PageSize,
	})
	if err != nil {
		return nil, err
	}
	return resp, nil
})

r.Register("automation.update", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
	type updateArgs struct {
		ID      string `json:"id"`
		Name    *string `json:"name"`
		Enabled *bool   `json:"enabled"`
		// ... remaining optional fields, mirrored 1:1 onto the wrapper-typed proto request
	}
	in, err := decodeArg[updateArgs](args, 0)
	if err != nil {
		return nil, err
	}
	req := &automationv1.UpdateAutomationRequest{Id: in.ID, TenantId: id.TenantID}
	if in.Name != nil {
		req.Name = wrapperspb.String(*in.Name)
	}
	if in.Enabled != nil {
		req.Enabled = wrapperspb.Bool(*in.Enabled)
	}
	resp, err := client.UpdateAutomation(ctx, req)
	if err != nil {
		return nil, err
	}
	return resp.GetAutomation(), nil
})

r.Register("automation.delete", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
	type deleteArgs struct {
		ID string `json:"id"`
	}
	in, err := decodeArg[deleteArgs](args, 0)
	if err != nil {
		return nil, err
	}
	if _, err := client.DeleteAutomation(ctx, &automationv1.DeleteAutomationRequest{Id: in.ID, TenantId: id.TenantID}); err != nil {
		return nil, err
	}
	return map[string]bool{"success": true}, nil
})
```

---

## Runtime verification for `automation.runNow` (do not trust the doc comment)

BUG-033 flags that `automation.proto`'s service doc comment claims
`RunNow` "always delegates to `workflow-service.ExecuteAdHocStep`" and that
`run_now.go`'s usecase does call a real `WorkflowStepExecutor` port — but
this was confirmed only by reading the wiring, not by running it. Reading
`run_now.go` in full confirms the interactor shape is real (tenant
resolution, idempotency check via `FindByRequestID`, a genuine call to
`uc.executor` — a `WorkflowStepExecutor`, not a no-op) — but "the Go code
calls an interface method" and "the interface's real implementation
actually reaches `workflow-service` and gets a real result back" are two
different claims, and only the first is verified by reading source.

**Concrete verification step, not just "recommend a test"**:

1. **Integration test**, `services/automation-service/internal/usecase/run_now_e2e_test.go`,
   run against `docker-compose`'s real stack (per
   `03-clean-architecture-guidelines.md`'s "small number of cross-service
   scenarios run against a full docker-compose stack in CI" policy) — not
   a fake `WorkflowStepExecutor`, the actual
   `grpcclient.WorkflowExecutor` dialed to a real running
   `workflow-service`:
   - Create an automation with `step_type = shell` and a trivial
     `step_config_json` (e.g. `echo automation-e2e-marker`).
   - Call `RunNow` with a fresh `request_id`.
   - Poll `GetRun`/`ListRuns` (or the returned `workflow_execution_id`,
     once that field is threaded through per automation-service.md §4)
     until status leaves `pending`/`running`.
   - **Assert the terminal status is `succeeded` or `failed` — never
     `skipped_unavailable`** — this is the exact TS-Gap-3 regression this
     migration claims to close (automation-service.md §10), so a test that
     merely asserts "no error was returned" would miss a silent
     reintroduction of the old no-op behavior.
   - Assert the automation's `last_run_at` was updated and the run's
     `output_json`/`error` reflects the shell command's real output, not a
     placeholder — proof the call reached an actual executor, not a stub
     that returns a canned success.
2. **CI gate**: run this test in the same `docker-compose` E2E tier as
   other cross-service scenarios, not skipped/marked `-short` — a flaky or
   skipped verification here is equivalent to not having it, given this is
   exactly the path BUG-033 says was never confirmed live.
3. If step 1 reveals `RunNow` does NOT reach a real `workflow-service`
   instance (e.g. `WorkflowExecutor`'s `grpcclient` implementation is
   itself still a stub, mirroring `git-gateway-service`'s
   `ConnectionResolver` stub noted in that service's own `ports.go`), file
   that as its own bug immediately — don't let this proposal's `list`/
   `update`/`delete` work quietly ship alongside an unverified `runNow`
   claim.

## Test plan

- `services/automation-service/internal/usecase/list_automations_test.go`,
  `update_automation_test.go`, `delete_automation_test.go` — unit tests
  against a fake `AutomationRepository`, per the existing `run_now_test.go`
  convention (fakes for all ports, no real Postgres).
- `adapter/postgres/repository_test.go` — extend with `testcontainers-go`
  Postgres integration tests for `List`/`Update`/`Delete`, verifying
  tenant-scoping (`WHERE tenant_id = $1`) actually excludes another
  tenant's rows, and that `Delete` cascades to `automation_runs` (insert a
  run, delete the automation, assert the run row is gone via the `ON DELETE
  CASCADE` FK).
- `adapter/grpc/server_test.go` — contract tests for the 3 new RPCs.
- `wscompat/channels_test.go` — one test per new channel
  (`TestAutomationListChannel_Success`, etc.), following
  `TestDevServerListChannel_Success`'s shape; one test asserting
  `automation.update`'s handler leaves unset fields as `nil` wrapper values
  on the wire request (regression guard against accidentally sending
  zero-value overwrites for fields the caller didn't touch).
- `run_now_e2e_test.go` — the runtime-verification integration test above,
  gated in CI's docker-compose E2E tier.

## References

- `specs/backend-go/tdd/services/automation-service.md` — full service
  design; §2/§6 (Gap 3 fix — `RunNow` must call `workflow-service`, never a
  second execution engine), §3 (API surface including
  `ListAutomations`/`UpdateAutomation`/`DeleteAutomation`), §5 (schema —
  compare against the scaffold's actual migrations, which already diverged
  on field shape), §7 (scheduler ticker concurrency), §8 (idempotency,
  at-least-once semantics), §10 (Gap 3 closure framing, data-migration note)
- `specs/backend-go/bugs/missing-v1/BUG-033-automation-channels-partially-implemented.md` —
  full gap table and the runtime-verification flag this section responds to
  directly
- `backend-go/proto/orca/automation/v1/automation.proto` — current 4-RPC
  surface; note its `Automation`/`CreateAutomationRequest` field shape
  (`step_type`/`step_config_json`, not the TDD's `prompt`/`agent_id`/
  `execution_target`) — this proposal's `ListAutomations`/`UpdateAutomation`
  follow the proto's actual shape, not the TDD's
- `backend-go/services/automation-service/internal/usecase/ports.go:14-40` —
  `AutomationRepository`/`AutomationRunRepository`/`WorkflowStepExecutor`
  ports this proposal extends
- `backend-go/services/automation-service/internal/usecase/run_now.go` —
  the real (not stubbed) `RunNow` interactor whose downstream call this
  proposal's runtime-verification step actually exercises
- `backend-go/services/automation-service/internal/adapter/postgres/repository.go:1-60` —
  `Create`/`Get` pattern `List`/`Update`/`Delete` follow
- `backend-go/services/automation-service/migrations/0001_init.up.sql`,
  `0002_scheduler_columns.up.sql` — the actual current schema this
  proposal's SQL is grounded in
- `backend-go/services/api-gateway/internal/adapter/httpgateway/automation_routes.go:1-40`
  — REST equivalents; reuse its `parseStepType`-equivalent translation
  helper rather than duplicating it in `wscompat`
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go:257-275`
  — `registerAutomationChannels`, the wiring pattern this proposal follows
- `specs/frontend/api/rpc-catalog.md:98-107` — full `automation.*` frontend
  call-site table (6 methods)
