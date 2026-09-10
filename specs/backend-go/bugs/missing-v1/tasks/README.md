# Missing-v1 Tasks — Executable Breakdown of the Solutions

186 task files (`TASK-001`–`TASK-226`, with intentional gaps — see
"Numbering" below), one execution unit each, derived from the 35 proposals
in [`../solutions/`](../solutions/). Each task follows the format
established in [`../../api-v1/tasks/`](../../api-v1/tasks/): **From
Solution** / **Priority** / **Service** / **File** / **Depends on** /
**Status**, a short Context, a "Changes to make" section with real,
copy-pasteable code (not pseudocode), and a "Verify" section with exact
shell commands.

> **Status (updated 2026-08-27): 187/188 tasks `[x]` DONE, 1 `[partial]`.**
> Every task file's own `**Status:**` line is authoritative — this banner
> just aggregates them. Only `TASK-036` (`browser.*` frontend dispatch) is
> partial: its agent + backend-go layers are done and tested end-to-end,
> but wiring the frontend to the new relay path is a separate, still-open
> design decision (see that file's "Status by layer" section). This line
> originally said "nothing has been implemented" when these tasks were
> first written — that was accurate then, is not anymore, and had drifted
> out of sync with the per-task Status lines below for some time before
> this update.
> Every task was written by extracting and formalizing the code sketches
> already in its source `SOL-0XX` — not re-designed from scratch — and
> cross-checked against the actual current `backend-go` source (proto
> files, `wscompat/channels.go`, service usecase/adapter code) so the
> "Changes to make" sections reflect real code, not paraphrase. Several
> task sets found and corrected real inaccuracies in their source
> solution's sketches during this grounding pass (noted per-solution
> below where significant).

## Numbering

Each of the 8 parallel research passes that produced these tasks got a
reserved number range wide enough to avoid collisions; not every range was
used in full, leaving intentional gaps (e.g. `TASK-017`–`020`,
`TASK-113`–`125`) — these are unused headroom, not missing work. Every
task that exists is complete, verified non-truncated, and part of a
consistent dependency chain within its own solution's task set.

## Solution → task range index

| SOL | Resolves | Tasks | Count | Notes |
|---|---|---|:---:|---|
| SOL-001 | BUG-001 (admin console) | [TASK-001](./TASK-001-add-admin-proto-rpcs.md)–[006](./TASK-006-test-admin-routes-and-usecases.md) | 6 | |
| SOL-002 | BUG-002 (SSO stub) | [TASK-007](./TASK-007-add-sso-stub-route.md)–[008](./TASK-008-test-sso-stub-route.md) | 2 | |
| SOL-003 | BUG-003 (web push) | [TASK-009](./TASK-009-add-unregister-push-subscription-rpc.md)–[011](./TASK-011-test-web-push-routes.md) | 3 | |
| SOL-035 | BUG-035 (WS push bridge) | [TASK-012](./TASK-012-add-push-bridge-primitives.md)–[016](./TASK-016-test-push-bridge.md) | 5 | `terminal.*` (SOL-029) pipes through this |
| SOL-004 | BUG-004 (`accounts.*`) | TASK-021–023 | 3 | includes 1 blocked/doc-only task |
| SOL-005 | BUG-005 (`aiProvider.*`) | TASK-024–030 | 7 | |
| SOL-006 | BUG-006 (`browser.*`) | TASK-031–036 | 6 | includes 1 blocked/doc-only task |
| SOL-007 | BUG-007 (`credentials.*`) | TASK-037–043 | 7 | spans 3 services |
| SOL-008 | BUG-008 (`emulator.*`) | TASK-046–048 | 3 | includes 1 blocked/doc-only task |
| SOL-009 | BUG-009 (`files.*`) | TASK-049–060 | 12 | |
| SOL-010 | BUG-010 (`folderWorkspace.*`) | TASK-061–067 | 7 | |
| SOL-011 | BUG-011 (`host.*`) | TASK-068–070 | 3 | includes 1 blocked/doc-only task |
| SOL-012 | BUG-012 (`github.*`) | TASK-071–082 | 12 | |
| SOL-013 | BUG-013 (`gitlab.*`) | TASK-083–086 | 4 | |
| SOL-014 | BUG-014 (`hostedReview.*`) | TASK-087–090 | 4 | |
| SOL-015 | BUG-015 (`jira.*`) | TASK-096–101 | 6 | |
| SOL-016 | BUG-016 (`linear.*`) | TASK-102–107 | 6 | |
| SOL-017 | BUG-017 (`nativeChat.*`) | TASK-108–109 | 2 | |
| SOL-018 | BUG-018 (`orchestration.*`) | TASK-110–112 | 3 | |
| SOL-019 | BUG-019 (`profile.*`) | TASK-126–130 | 5 | |
| SOL-020 | BUG-020 (`project.*`) | TASK-131–135 | 5 | |
| SOL-021 | BUG-021 (`projectGroup.*`) | TASK-136–140 | 5 | |
| SOL-022 | BUG-022 (`projectHostSetup.*`) | TASK-141–145 | 5 | |
| SOL-023 | BUG-023 (`repo.*`) | TASK-151–161 | 11 | split project-service/git-gateway-service |
| SOL-024 | BUG-024 (`ssh.*`) | TASK-162–166 | 5 | |
| SOL-025 | BUG-025 (`status.get`) | [TASK-167](./TASK-167-register-status-get-channel.md) | 1 | |
| SOL-026 | BUG-026 (`workspace.*`) | [TASK-168](./TASK-168-wire-workspace-refresh-file-tree.md) | 1 | blocked on SOL-009 landing first |
| SOL-027 | BUG-027 (`workspacePorts.*`) | TASK-169–173 | 5 | |
| SOL-028 | BUG-028 (`team.*`) | TASK-176–179 | 4 | |
| SOL-029 | BUG-029 (`terminal.*`) | TASK-180–187 | 8 | **most architecturally significant** |
| SOL-030 | BUG-030 (`workflow.*`) | TASK-188–191 | 4 | |
| SOL-031 | BUG-031 (`worktree.*`) | TASK-192–196 | 5 | **2nd most architecturally significant** |
| SOL-032 | BUG-032 (`git.*`) | TASK-206–216 | 11 | **largest task-group in the whole pass** |
| SOL-033 | BUG-033 (`automation.*`) | TASK-217–221 | 5 | |
| SOL-034 | BUG-034 (`task.*`) | TASK-222–226 | 5 | |
| SOL-036 | BUG-036 (`git.*` relay unreachable) | [TASK-227](./TASK-227-expose-git-status-diff-commit-on-agent-part-a.md)–[228](./TASK-228-fix-existing-relay-param-names-and-diff-shape.md) | 2 | **P0 — blocks TASK-206–216**; TASK-227 is the one task in this directory that changes `agent/`, not `backend-go`. TASK-227 now covers all 20 unreachable methods (expanded from an initial 3), not just `status`/`diff`/`commit` |

**188 tasks total** (186 + TASK-227's expansion note + TASK-228), matching
the file count. Some of TASK-207/209's original scope may have split
further during the correction pass below — check each file's own header
for its final task count if precision matters.

## Start here if you're about to touch `git.*`

`TASK-227`/`TASK-228` are not optional preamble — they're **P0 blockers**,
found by tracing the real frontend→backend→agent→backend→frontend data
flow (not inferred from docs): the agent process backend-go's transport
reaches never registers 20 of the 34 `git.*` methods this whole task set
plans to use, on any connection mode (`relay-ssh` included — it now runs
the same agent binary as the WebSocket modes, not a separate SSH Relay
Daemon), and even the 8 methods that ARE reachable were designed with the
wrong request/response shape in several cases (per-file vs. whole-repo
diffs, two-sided vs. single-ref comparisons, a detector mistaken for an
executor). `TASK-206`'s "quick win" wiring and every relay-calling task in
`TASK-207`–`213` were corrected in place to flag which fixes are
mechanical (apply directly) vs. which are blocked on a real design
decision (4 open questions, none fabricated — see `SOL-032`'s §0). See
[BUG-036](../BUG-036-git-relay-methods-unreachable-on-agent.md) for the
full trace and `solutions/SOL-032-git-channels.md`'s §0 for the
authoritative per-method correction table.

## Blocked-on-`agent/` tasks — do not start without a decision first

Three solutions hit the TDD's stated `agent/`-out-of-scope boundary.
Each has an explicit, separately-numbered "blocked/doc-only" task marked
**DO NOT START** in its own header — the rest of that solution's tasks
(the honest-stub/local-answer half) ARE safe to start:

| Task | Solution | What's blocked |
|---|---|---|
| `TASK-023` | SOL-004 (`accounts.*`) | small — new agent-side JSON-RPC methods + a frontend `connectionId` param |
| `TASK-036` | SOL-006 (`browser.*`) | large — real CDP-level remote browser control doesn't exist in `agent/` at all |
| `TASK-048` | SOL-008 (`emulator.*`) | large — driving ADB/`xcrun simctl` remotely, no such `agent/` capability |
| `TASK-070` | SOL-011 (`host.*`) | small — per-target WSL/pwsh/git-bash probing on a specific dev server |

## The two biggest builds

- **`git.*` (SOL-032, TASK-206–216, 11 tasks)** — 4 wiring-only wins, then
  5 new-RPC tasks (one per operation family: branch/ref, staging,
  history/compare, remote, AI-assist), then wiring + 3 test tasks. The
  single largest and most consequential build in this entire pass.
- **`terminal.*` (SOL-029, TASK-180–187, 8 tasks)** — builds an entire PTY
  streaming RPC (`AttachPty`) from nothing, adapting over the *existing*
  Dev Server Agent wire protocol (no `agent/` changes needed). Its output
  is designed to pipe through `TASK-012`–`016`'s push bridge.
- **`worktree.*` (SOL-031, TASK-192–196, 5 tasks)** close behind — gives
  `git-gateway-service` its first-ever on-disk worktree RPCs, coordinated
  with `project-service`'s existing bookkeeping via an explicit saga with
  compensation on failure.

## Suggested execution order

Not a strict global sequence (35 independent solutions can run in
parallel workstreams) — but within any one solution, follow its own
`Depends on:` chain, and across solutions, this is the rough priority by
how much of the app's core loop each one blocks:

1. **Bootstrap-path fixes first**: `TASK-001`–`016` (admin console, SSO
   stub, web push, push bridge) — these touch what every session needs at
   connect time.
2. **Core workspace loop**: `git.*` (206–216), `worktree.*` (192–196),
   `files.*` (049–060), `repo.*` (151–161), `task.*` (222–226),
   `terminal.*` (180–187) — the largest and most load-bearing gaps.
3. **Collaboration/identity**: `profile.*`/`project.*`/`projectGroup.*`/
   `projectHostSetup.*`/`team.*` (126–145, 176–179) — several are mostly
   wiring-only, cheap wins.
4. **Integrations**: `github.*`/`gitlab.*`/`hostedReview.*` (071–090),
   `jira.*`/`linear.*` (096–107), `ssh.*` (162–166), `aiProvider.*`
   (024–030), `credentials.*` (037–043).
5. **Everything else**: the remaining, smaller/niche namespaces.

Within each task, run its own `## Verify` block before moving to whatever
depends on it — the dependency chains in this pass were built and
cross-checked specifically so that a task's prerequisites always compile
cleanly before the task itself is attempted.
