# TASK-BE-001: gRPC-to-wscompat wiring for BE-SOL-001

> **Status: ✅ COMPLETED** — 2026-09-06
> **Files modified:** `backend-go/services/api-gateway/internal/adapter/wscompat/channels_workflow.go`,
> `backend-go/services/api-gateway/internal/adapter/wscompat/channels_workflow_test.go`

**Solution:** BE-SOL-001 | **CR:** CR-PW-005
**Depends on:** none (all 7 target RPCs already existed in the proto/gRPC server before this task)

---

## Goal

Register the 7 `WorkflowServiceClient` RPCs that had a working gRPC server implementation but no
wscompat channel: `GetExecution`, `PauseExecution`, `ResumeExecution`, `ListTemplates`,
`ResolveTemplate`, `HasActiveExecutions`, `ExecuteAdHocStep`.

## What was done

1. Added 7 `r.Register("workflow.<name>", ...)` blocks to `registerWorkflowChannels`, each:
   decode args via `decodeArg[T]`, `gatewaygrpc.AttachIdentity` with `TenantID`/`UserID` from the
   resolved `Identity`, call the corresponding gRPC method, return its response (or an inline
   `map[string]any` for multi-field responses).
2. `workflow.executeAdHocStep` reuses `parseStepType` (already defined in
   `channels_automation_task.go`) instead of duplicating the string→enum switch.
3. `workflow.template.list` / `.template.resolve` normalize `nil` slices to `[]` before returning,
   matching the established list-channel convention.
4. Extended `fakeWorkflowServiceClient` (test double) with the 7 new method overrides.
5. Added 9 new test functions (see BE-SOL-001 §4 for the full list).
6. `gofmt -w` on both files (struct field alignment after adding longer field names).

## Acceptance Criteria

- [x] `go build ./...` clean for `api-gateway` both immediately after the change and after tests
      were added.
- [x] `go test ./internal/adapter/wscompat/... -run TestWorkflow -v` — 13/13 pass (4 pre-existing
      + 9 new).
- [x] `go test ./...` for the full `api-gateway` module — all packages pass.
- [x] `gofmt -l` clean on both changed files.
- [x] No proto file touched; no other backend-go service/module touched.
- [x] `workflow.executeAdHocStep`'s `TenantId` provably comes from `Identity`, not args (tested,
      same pattern as `workflow.template.create`'s existing guard).

## gitnexus

- `impact({target:"registerWorkflowChannels", direction:"upstream", repo:"orca"})` before editing:
  risk **LOW**, impactedCount 3, 1 execution flow noted (`run` in `cmd/server/main.go`), 0 flagged
  as broken by this additive change.
- `detect_changes({scope:"all"})` after all CR-PW-004/005/006 work: risk **low**, 42 changed
  symbols across 16 files, 0 affected processes.

## Blocking
Không có — CR-PW-006 Phase B (a genuinely new `ListExecutions`/`ListStepExecutions` RPC) is a
separate, larger change (proto + `buf generate` + new server-side implementation), not part of
this task's "wire what already exists" scope.
