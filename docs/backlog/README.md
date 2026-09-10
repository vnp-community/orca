# Backlog

Work that's genuinely **not ready to implement yet** — each blocked on a
missing dependency (unbuilt agent capability, unbuilt upstream feature) or a
decision only a human/product owner can make, not on more engineering time.
Most of this was surfaced by the `specs/backend-go/bugs/missing-v3/`
audit-and-fix pass (2026-09-07/08); this directory exists so the remaining
work is discoverable without wading back into that bug/solution/task
directory tree, which is now otherwise fully resolved.

Each entry restates its own context standalone — you shouldn't need to open
`missing-v3/` to understand what it is — but links back to the original
`BUG-XXX`/`TASK-XXX` file for full investigation detail and evidence.

Resolved items are removed from this directory once confirmed closed, not
kept as a historical record — see each service/CR's own tracking docs
(`specs/backend-go/bugs/missing-v3/`, `specs/{agent,backend-go,frontend}/crs/`)
for closed-item history.

This directory was moved here from `specs/backlog/` (2026-09-08) so every
backlog item lives in one place — see [`TODO.md`](./TODO.md), a separate,
manually-maintained list of larger initiatives (surveyed/planned but not
yet built, or reverted) that predates this directory and uses its own
format; the two are complementary, not duplicates.

## Index

| ID | Title | Blocked on | Priority |
|----|-------|-----------|----------|
| [BACKLOG-001](./BACKLOG-001-ephemeralvm-ssh-outbound-client.md) | Ephemeral VM `ssh`-connection-type lifecycle | New `agent/` outbound SSH-client subsystem | Low |
| [BACKLOG-002](./BACKLOG-002-environment-devserver-resolution.md) | Resolve a bare runtime `environmentId` to a dev server (unblocks `terminal.create` and `files.browseServerDir` for connectionless environments) | `ephemeralVm.provision` (doesn't exist yet) | Medium |
| [BACKLOG-004](./BACKLOG-004-browser-profile-import-live-wiring.md) | Wire up `browser.profileImportFromBrowser`'s real cookie-import orchestration | Needs real-browser/live-OS-keychain testing before shipping | Low |
| [BACKLOG-005](./BACKLOG-005-telemetry-consent-identity-decision.md) | `telemetry.track` — pick a consent/identity model for backend-go, then implement it | **Human product/privacy decision**, not engineering | Low (until decided) |
| [BACKLOG-009](./BACKLOG-009-orchestration-service-fail-dispatch-missing.md) | `FailDispatch` RPC built and tested (circuit-breaker for real); its intended caller (`TASK-TASKV1-005-10`'s tick loop) is fully speced but, per 2026-09-08 re-check, **`TASK-TASKV1-005-01..09` it depends on is unimplemented** — same root cause as BACKLOG-017 | `TASK-TASKV1-005-01..09` (CoordinatorRun domain/repo/usecases, 2 new adapters, 6 new proto RPCs) — a real implementation pass, not a design decision | Low-Medium |
| [BACKLOG-012](./BACKLOG-012-worktree-missing-infra-connections-row-local-exec-fallback.md) | Dev-server-backed worktree with no `infra.connections` row falls back to running `git status` LOCALLY inside git-gateway-service's own (distroless, repo-less) container — `GITGATEWAY_STATUS_FAILED` | Root cause not yet diagnosed | Medium-High |
| [BACKLOG-013](./BACKLOG-013-dispatch-context-handle-worktree-linkage.md) | Backend RESOLVED 2026-09-08: `dispatch_contexts.worktree_id` added, threaded end-to-end. Frontend `mapDispatchContextsToSessions` NOT implemented — traced: `CreateDispatchContext` has zero real callers anywhere, same root cause as BACKLOG-009/017 | Same as BACKLOG-009/017: the coordinator/dispatch stack (`TASK-TASKV1-005-*`) is unimplemented | Low |
| [BACKLOG-014](./BACKLOG-014-legacy-electron-backend-bug-audit-unverified.md) | Legacy Electron/Node backend (still live in prod for Task/Workflow + all of desktop) has ~70 unverified bug findings from a 5-6-week-old audit (consolidates 4 loose top-level index files, now removed). 2026-09-08: the 2 highest-severity findings (`new Function()` RCE, missing relay `agent.exec`/`pty.*` handlers) checked — both were already fixed days after the audit, no action needed; ~68 findings still unverified | A re-verification pass against current code for the remaining ~68 findings | Medium |
| [BACKLOG-015](./BACKLOG-015-task-graph-rpcs-not-wired-to-wscompat.md) | `AddEdge`/`Grant`/`ResolvePermission` now wired to `wscompat`; `AddComment`/`ListComments` still don't exist at any layer — `TaskComments` UI sits on fallback | Needs new usecase+proto+migration design | Medium |
| [BACKLOG-017](./BACKLOG-017-orchestration-service-new-rpcs-not-exposed-to-frontend.md) | 6 `orchestration-service` coordinator-lifecycle RPCs this item assumed existed (`StartCoordinatorRun`, `GetCoordinatorRun`, `CompleteCoordinatorRun`, `FailCoordinatorRun`, `RecordHeartbeat`, `ListPendingDecisionGates`) — per 2026-09-08 re-check, **none exist at any layer** (proto/usecase/gRPC), not just unwired from `wscompat` | Same as BACKLOG-009: `TASK-TASKV1-005-01..09` is unimplemented | Medium |
| [BACKLOG-018](./BACKLOG-018-mobile-push-subscribe-auth-mechanism.md) | `POST /api/mobile/push-subscribe` route (F03 Mobile Companion's last remaining backend-go piece) — mobile has no cookie session to authenticate the caller | Option B (mobile SSO) chosen, but its prerequisite CR (token issuance/refresh/revocation) doesn't exist yet | Medium |
| [BACKLOG-019](./BACKLOG-019-fleet-cloud-credential-storage-security-review.md) | Cloud provider credential storage design for fleet Terraform provisioning (F31) — first time backend-go would hold a raw, long-lived 3rd-party credential | **Human security review sign-off** — design proposal ready, reviewer role not yet assigned | Medium |
| [BACKLOG-020](./BACKLOG-020-automations-pr-merged-trigger-detection-mechanism.md) | Automations "PR merged" trigger — `scm-integration-service` has zero PR-merge-detection mechanism (no webhook, no polling), confirmed by 2 independent surveys | Product/architecture decision (webhook vs. polling, real scope) + an open `trigger_depth` propagation design | Low-Medium |
| [BACKLOG-021](./BACKLOG-021-workflow-execution-live-streaming-blocked.md) | Workflow execution live streaming (FE-TASK-005) — polling fallback already works; live-event replacement blocked 2 backend layers deep | `CR-FLOW-TASK-003` → `BE-SOL-006`, both 📋 Proposed | Medium |
| [BACKLOG-022](./BACKLOG-022-automation-agent-completion-signal-decision.md) | Automation/VM-lifecycle "agent finished" trigger — `AgentDetector`'s signal has a confirmed false-positive (agent waiting for user input looks identical to "done") | Product decision: extend `AgentDetector` with a filter, or trigger off PTY-exit/RPC-response instead | Medium |
| [BACKLOG-023](./BACKLOG-023-worktree-cleanup-service-not-wired.md) | `WorktreeCleanupService` built + tested (7/7 pass), deliberately NOT wired into `desktop/src/main`'s composition root — would start auto-deleting real user worktrees on next build | Explicit user/product authorization to enable, not an engineering gap | Medium-High |
| [BACKLOG-024](./BACKLOG-024-automation-action-dispatch-connectionid-gap.md) | Every automation action except `create_pr` (`run_agent`/`run_script`/`send_notification`/`commit_push`) has the right shape but FAILS to dispatch — `workflow-service` requires `connectionId`, nothing resolves it from automation's `runContext` | New CR/task to resolve automation's `runContext`/`executionTargetId` → `infra-fleet-service` `connectionId` — not yet scoped | High |
| [BACKLOG-025](./BACKLOG-025-task-grant-list-revoke-share-link-blocked.md) | `TaskGrantModal`'s list/revoke/share-link — Add Grant + permission badge work today; viewing/managing existing grants does not | `BE-SOL-003` (task-access-control-team-scope-and-sharing), 📋 Proposed | Low-Medium |
| [BACKLOG-026](./BACKLOG-026-workflow-template-library-search-share-blocked.md) | `WorkflowLibrary`'s search/sort/share — Browse/Use work today against the real `workflow.template.list` RPC; enhancement features don't exist yet | `BE-SOL-005` (template-sharing-library-and-list-executions), 📋 Proposed | Low |
| [BACKLOG-027](./BACKLOG-027-workflow-approval-step-type-removal-decision.md) | `WorkflowStepType`'s `'approval'` has no backend equivalent — kept intentionally pending a product call on removal vs. giving it a real backend implementation | Product confirmation | Low |
| [BACKLOG-028](./BACKLOG-028-ephemeral-vm-worktree-mount-copy-out-decision.md) | Ephemeral VM worktree mount/copy-out — `docs/features/F18-ephemeral-vm.md` describes a mechanism that was never built; real shipped behavior points the workspace at the recipe's own in-VM project root instead | Product decision: fix the spec, or build the real feature (SSH-only, needs new agent write capability + BACKLOG-022's decision) | Low |
| [BACKLOG-029](./BACKLOG-029-hidden-target-id-repo-scope-mismatch.md) | `hidden_target_id` never populated at git-gateway-service's repo-scoped dispatch path (before a worktree exists) — likely because the concept is worktree-scoped, not repo-scoped, so there may be nothing to populate | Product/architecture decision on whether repo-scoped hidden-target routing is even a real use case | Low |

## Status of everything else

Every other finding from the `missing-v3` pass (16 bugs, 10 solutions, 42
tasks, plus the follow-on `onboarding.openGhAuthTerminal` implementation and
the direct `BUG-015`/`BUG-016` fixes) is done and verified — see
`specs/backend-go/bugs/missing-v3/README.md` and
`specs/backend-go/bugs/missing-v3/tasks/README.md` for the full record.

The `docs/crs/v3/storage/` storage-consolidation effort (a separate, later
effort) is **in progress, not backlog** — most of its ~37 tasks across
`specs/{agent,backend-go,frontend}/crs/v3/storage/tasks/` are done or
actively being worked; see those directories' own READMEs for live status.
