# BUG-WT-02: Fan-out (N-worktree, N-agent prompt broadcast) has no backend-go implementation

**Business Logic:** [BL-WT-02](../../../../docs/logic/worktree-management/BL-WT-02-fan-out-worktree.md) — Fan-out Prompt tới Nhiều Worktree
**Priority (per spec):** P0
**Status:** NOT_IMPLEMENTED
**Severity:** Critical
**Symptom:** There is no way to send one prompt to N parallel worktree+agent pairs and get N independent results in one action. A caller would have to manually issue N separate `worktree.create` calls and N separate agent-start/prompt-inject calls itself, with zero backend coordination, no partial-failure isolation guarantee, no shared-base-branch enforcement, and no N-limit enforcement — every business rule in the spec is entirely client-side folklore, not a backend contract.

---

## Spec summary

`BL-WT-02` describes a single user action — one prompt, N parallel worktrees (1–10, default 3), each running its own agent with the same prompt injected only after that agent has fully started — as one coordinated flow. It requires per-worktree failure isolation (one failing worktree doesn't block or affect the others, and can be retried individually) and defines business rules for the N-cap (BR-WT-05, max 10), shared base branch across all N (BR-WT-06), post-start prompt injection ordering (BR-WT-07), and failure isolation (BR-WT-08).

## What backend-go has

- `CreateWorktree` (single-worktree saga, `backend-go/services/git-gateway-service/internal/usecase/create_worktree.go:41-71`) — the only per-worktree building block that exists, callable once per worktree.
- Nothing else. There is no orchestration-service, task-service, or api-gateway usecase that:
  - accepts a prompt + N + base branch + agent type as one request,
  - creates N worktrees,
  - starts N agents,
  - injects the prompt into each after startup,
  - or reports N independent statuses back to the caller.

Confirmed absent by direct search:
- `grep -rli "fan.?out\|fanout" backend-go/ specs/backend-go/` — the only hit anywhere in the Go tree is an unrelated string in a notification-service broadcaster test; no `fan-out`/`fanout` concept exists in backend-go proto, usecase, or adapter code.
- `backend-go/services/orchestration-service/internal/usecase/` contains only `create_dispatch_context.go`, `create_gate.go`, `get_dispatch_context_for_task.go`, `keyed_serializer.go`, `resolve_gate.go`, `update_task_status_and_promote.go` — none reference worktrees at all (`grep -rln "worktree" backend-go/services/orchestration-service/internal/**/*.go` returns no matches).
- No proto in `backend-go/proto/orca/*/v1/*.proto` defines a batch/fan-out RPC (`CreateWorktree` in `gitgateway.proto` is strictly single-worktree, one `branch`/`base_ref` pair per call).

## What's missing

- The entire coordinating flow: no RPC/usecase accepts N and fans out N `CreateWorktree` + N agent-starts + N prompt-injections as one unit of work.
- **BR-WT-05** (N ≤ 10 per fan-out): no cap enforced anywhere — since no fan-out entry point exists, there is nothing to cap.
- **BR-WT-06** (all N worktrees share the same base branch): unenforceable — no fan-out call site to validate it at.
- **BR-WT-07** (prompt injected only after the agent is fully started): unenforceable — no ordering guarantee exists because no orchestration exists.
- **BR-WT-08** (one agent failing doesn't affect the others): unenforceable — no partial-failure isolation logic exists; a caller looping over N `worktree.create` calls today gets no isolation guarantee beyond whatever each independent gRPC call naturally provides, and no retry-single-worktree affordance.
- **[A1]/[A2] alternate flows** (one worktree fails → continue + warn + allow retry; N exceeds resource limits → warn + suggest lower N): no such logic exists anywhere.

## See also

- None — no missing-v1/api-v1 bug currently documents this gap; it was not previously reported as a "channel wiring" issue because no `fanOut.*`/`worktree.fanOut` channel is referenced in the frontend RPC catalog audits at all (this is a genuinely un-scoped capability, not a wiring gap on an existing channel).

## References

- `backend-go/services/git-gateway-service/internal/usecase/create_worktree.go:11-71` — the only reusable per-worktree building block
- `backend-go/services/orchestration-service/internal/usecase/` — full directory listing, no worktree awareness
- `backend-go/proto/orca/gitgateway/v1/gitgateway.proto` — `CreateWorktree` RPC, single-worktree only
- `docs/logic/worktree-management/BL-WT-02-fan-out-worktree.md`
