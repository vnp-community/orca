# TASK-219: Wire `automation.list`/`automation.update`/`automation.delete` wscompat channels

**From Solution:** SOL-033 (`wscompat` wiring section)
**Priority:** P1
**Service:** `api-gateway`
**File:** `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go`
**Depends on:** TASK-218
**Status:** `[x]` DONE — `automation.list`/`automation.update`/`automation.delete` registered in `channels_automation_task.go` (kept out of `channels.go` per the cross-worktree conflict note in that file's package doc comment). `go build`/`go vet`/`go test` clean.

---

## Context

Wires the 3 remaining `automation.*` frontend channels once TASK-218's
RPCs exist, completing the full 6-method `automation.*` catalog.

## Changes to make

**File:** `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go`

Add to `registerAutomationChannels`, after TASK-217's channels:

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
			ID             string  `json:"id"`
			Name           *string `json:"name"`
			RRule          *string `json:"rrule"`
			StepConfigJSON *string `json:"stepConfigJson"`
			StepType       *string `json:"stepType"`
			Enabled        *bool   `json:"enabled"`
			Dtstart        *string `json:"dtstart"`
			Timezone       *string `json:"timezone"`
		}
		in, err := decodeArg[updateArgs](args, 0)
		if err != nil {
			return nil, err
		}
		req := &automationv1.UpdateAutomationRequest{Id: in.ID, TenantId: id.TenantID}
		if in.Name != nil {
			req.Name = wrapperspb.String(*in.Name)
		}
		if in.RRule != nil {
			req.Rrule = wrapperspb.String(*in.RRule)
		}
		if in.StepConfigJSON != nil {
			req.StepConfigJson = wrapperspb.String(*in.StepConfigJSON)
		}
		if in.StepType != nil {
			req.StepType = parseStepType(*in.StepType)
		}
		if in.Enabled != nil {
			req.Enabled = wrapperspb.Bool(*in.Enabled)
		}
		if in.Dtstart != nil {
			req.Dtstart = wrapperspb.String(*in.Dtstart)
		}
		if in.Timezone != nil {
			req.Timezone = wrapperspb.String(*in.Timezone)
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

Add `"google.golang.org/protobuf/types/known/wrapperspb"` to this file's
import block — first use of `wrapperspb` in `wscompat`.

Update the `automation.*` channel-count doc comment (updated in TASK-217)
to reflect all 6 methods now wired:

```go
// ── automation.* (all 6 methods wired: runNow, create, runs (Part 1) plus
// list/update/delete (Part 2, real repository-backed CRUD) — see BUG-033
// and SOL-033) ────────────────────────────────────────────────────────────
```

## Verify

```bash
cd /opt/repos/orca/backend-go/services/api-gateway
go build ./... && go vet ./internal/adapter/wscompat/...
```
