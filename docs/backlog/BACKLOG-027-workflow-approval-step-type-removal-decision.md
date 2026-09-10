# BACKLOG-027: `WorkflowStepType`'s `'approval'` — pending a product decision on removal, not a technical blocker

**Origin:** `specs/frontend/crs/v4/workflow/tasks/FE-TASK-004-step-type-vocabulary-fix.md` (CR-WF-006)
**Priority:** Low — purely a naming/scope cleanup question, nothing is broken by leaving it as-is
**Blocked on:** **Product confirmation** on whether `'approval'` should be removed from `WorkflowStepType`
**Owner:** whoever owns the workflow feature's product scope

---

## What this is

FE-TASK-004 renamed `WorkflowStepType`'s `'notify'` to `'notification'` to
match the backend's real `StepType` enum (`step.go:16-23`: `agent | shell |
notification | webhook | condition`) — done, safe, shipped 2026-09-09.

`'approval'` is the one value in the frontend's `WorkflowStepType` that
**has no backend equivalent at all**. It was deliberately left in place
rather than removed, per this task's explicit scope: "GIỮ LẠI chờ xác nhận
sản phẩm trước khi xoá" (keep it pending product sign-off).

## Why it wasn't just deleted

Removing a step type a user might already have selected in an existing
workflow template is a product-visible behavior change (templates using
`'approval'` would need a migration story), not a pure type cleanup. That
call belongs to whoever owns the feature's scope, not to whoever happens to
touch this enum next.

## What unblocks this

Product confirms one of:
1. Remove `'approval'` from `WorkflowStepType` (small, mechanical change —
   the type is otherwise clean after FE-TASK-004's rename).
2. Keep it and give it a real backend equivalent (a bigger scope — would
   need its own CR, since `step.go`'s `StepType` enum and executor
   dispatch would need a new case).
