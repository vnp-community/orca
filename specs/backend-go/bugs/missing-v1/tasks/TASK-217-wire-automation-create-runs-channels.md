# TASK-217: Wire `automation.create`/`automation.runs` wscompat channels

**From Solution:** SOL-033 (Part 1)
**Priority:** P0 — zero new backend risk, ship first
**Service:** `api-gateway`
**File:** `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go`
**Depends on:** none
**Status:** `[x]` DONE — `automation.create`/`automation.runs` registered in the new `channels_automation_task.go` (not `channels.go` directly, to avoid cross-worktree merge conflicts — see that file's package doc comment). `go build`/`go vet`/`go test` clean for `api-gateway`. Still needs the integration pass to add `registerTaskCRUDChannels`/`registerAutomationChannels`-equivalent calls into `RegisterRealChannels` and `main.go`.

---

## Context

Per BUG-033, `CreateAutomation`/`ListRuns` already exist end-to-end
(`automation.proto:14,16`, `server.go`, REST at `automation_routes.go:23,25`)
— only the `wscompat` channel registration is missing. This is a pure
wrapper addition, following `registerAutomationChannels`'s existing
`automation.runNow` pattern (`channels.go:257-275`) exactly.

## Changes to make

**File:** `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go`

### Step 1: Add the 2 channel registrations to `registerAutomationChannels`

Add inside `registerAutomationChannels`, alongside the existing
`automation.runNow` registration:

```go
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

`id.TenantID` (from the validated `Identity` the channel handler receives)
is used for `CreateAutomationRequest.TenantId` — **never** read a tenant id
out of `args`, per the same "never trust a client-supplied tenant" posture
`SOL-001`'s admin routes apply and `httpgateway`'s
`createAutomationRequestBody` already follows (`automation_routes.go:29-32`).

`parseStepType` maps the frontend's string step-type onto
`workflowv1.StepType` — this helper already exists in the `httpgateway`
package (used by `automation_routes.go`'s REST handler). Since
`wscompat` and `httpgateway` are separate packages, either:

- **(a)** duplicate a minimal `parseStepType` into `wscompat` (simplest,
  matches this file's existing self-containment — it does not import
  `httpgateway`), or
- **(b)** hoist `parseStepType` into a shared location both packages import
  (e.g. `automationv1` helper package or a small `internal/stepconfig`
  package) if one doesn't already exist for cross-package reuse.

Default to **(a)** for this task — a single small string-to-enum switch is
cheap to duplicate and keeps `wscompat` free of a new cross-package
dependency; add a doc comment on the duplicated function noting the
`httpgateway` twin so a future consolidation isn't a surprise:

```go
// parseStepType mirrors httpgateway's parseStepType (automation_routes.go)
// — duplicated rather than imported since wscompat has no existing
// dependency on httpgateway and this is a single small switch. Keep the
// two in sync if workflowv1.StepType grows a new value.
func parseStepType(s string) workflowv1.StepType {
	switch s {
	case "shell":
		return workflowv1.StepType_STEP_TYPE_SHELL
	case "agent":
		return workflowv1.StepType_STEP_TYPE_AGENT
	default:
		return workflowv1.StepType_STEP_TYPE_UNSPECIFIED
	}
}
```

Confirm the exact `workflowv1.StepType` enum values against
`backend-go/proto/orca/workflow/v1/workflow.proto` and
`httpgateway`'s real `parseStepType` before finalizing — the switch above
is illustrative, not verified against the generated enum.

### Step 2: Update the file's channel-count doc comment

Update the comment above `registerAutomationChannels` from "the one real
cross-service call in this whole scaffold" framing to reflect the 3 now
real channels (`runNow`, `create`, `runs`):

```go
// ── automation.* (3 of 6 methods: runNow — the one real cross-service call
// in this whole scaffold — plus create/runs, backed by real usecases per
// BUG-033 Part 1. list/update/delete are tracked in SOL-033 Part 2 /
// TASK-218 through TASK-221) ─────────────────────────────────────────────
```

## Verify

```bash
cd /opt/repos/orca/backend-go/services/api-gateway
go build ./... && go vet ./internal/adapter/wscompat/...
go test ./internal/adapter/wscompat/... -run TestAutomation -v
```
