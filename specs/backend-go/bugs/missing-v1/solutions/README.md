# Missing-v1 Solutions — Proposed Fixes, Grounded in `specs/backend-go/tdd/`

This directory contains one proposed solution per bug in
[`../`](../) (35 bugs → 35 solutions, `SOL-0XX` resolves `BUG-0XX`
1:1). Every proposal was written by first reading its bug report's own
"which methods already have a backing RPC" findings, then reading the
relevant `specs/backend-go/tdd/services/*.md` deep-dive (the target
architecture for `backend-go`, written before/alongside the current
partial implementation) to design the fix against that target, not from
scratch.

> **Status: 📋 All 35 proposed — none implemented.** These are design
> documents only. No code was changed. Compare `../../api-v1/solutions/`,
> a separate, already-implemented set of fixes from a different
> investigation — this directory hasn't reached that stage yet.

## What "grounded in the TDD" turned out to mean in practice

For a striking number of these, **the target design already exists on
paper** — the TDD spec'd the exact RPC the frontend needs, `backend-go`
just hasn't built it yet. These are the lowest-risk, highest-confidence
proposals in the set:

| SOL | What the TDD already specified verbatim |
|---|---|
| SOL-001 | `auth-service.md`'s full admin-console RPC list (`DeactivateUser`, `ForceRevokeAllSessionsForUser`, `AccessPolicy` CRUD, …) |
| SOL-003 | `notification-service.md`'s `UnregisterPushSubscription` RPC |
| SOL-019 | `tenant-service.md`'s all 5 missing `profile.*` RPCs |
| SOL-020 | `project-service.md`'s 3 membership RPCs (`ListMembers`/`RemoveMember`/`UpdateMemberRole`) |
| SOL-024 | `infra-fleet-service.md`'s 3 of 4 `ssh.*` RPCs |
| SOL-028 | `tenant-service.md`'s `ListTeams`/`RemoveTeamMember` |
| SOL-035 | `08-inter-service-communication.md`'s "API Gateway responsibilities" item 5 (the gRPC-stream→`push`-frame bridge, described almost exactly as this solution designs it) |

For everything else, the TDD gave enough domain-model/schema/RPC-style
context to design the missing piece consistently with the rest of the
system, even where no exact RPC was pre-specified.

## Index

| SOL | Resolves | One-line proposal | Needs `agent/` work? |
|---|---|---|---|
| [SOL-001](./SOL-001-admin-console-rest-routes.md) | BUG-001 | New `auth-service` admin RPCs + `/admin/api/*` routes, per the TDD's existing spec | No |
| [SOL-002](./SOL-002-auth-sso-stub-route.md) | BUG-002 | One-route `501` stub, no design needed | No |
| [SOL-003](./SOL-003-web-push-endpoints.md) | BUG-003 | Move routes, drop auth gate, add `Unsubscribe` RPC | No |
| [SOL-004](./SOL-004-accounts-channels.md) | BUG-004 | Relay via `infra-fleet-service.Relay`, no new backend storage | Small (new JSON-RPC methods on the agent side) |
| [SOL-005](./SOL-005-aiprovider-channels.md) | BUG-005 | Close proto↔TDD gap; `testConnection` via `infra-fleet-service.Relay`, not a new agent adapter | No |
| [SOL-006](./SOL-006-browser-channels.md) | BUG-006 | Extend Dev Server Agent relay to a remote browser pane | **Yes — blocked**, substantial new agent capability |
| [SOL-007](./SOL-007-credentials-channels.md) | BUG-007 | Route through `scm-integration-service`/`issue-tracking-service`, not a new gateway rule | No — but makes credentials tenant-wide (product decision) |
| [SOL-008](./SOL-008-emulator-channels.md) | BUG-008 | Relay to agent-driven ADB/`simctl` (target design); honest stub shippable now | **Yes — blocked**, two-part proposal |
| [SOL-009](./SOL-009-files-channels.md) | BUG-009 | 16 new file-I/O RPCs on `git-gateway-service` | No |
| [SOL-010](./SOL-010-folderworkspace-channels.md) | BUG-010 | New `folder_workspaces` table + CRUD on `project-service` | No |
| [SOL-011](./SOL-011-host-channels.md) | BUG-011 | Honest local answer now; per-target relay once agent can answer | Small, two-part proposal |
| [SOL-012](./SOL-012-github-channels.md) | BUG-012 | ~11 new `ScmIntegrationService` RPCs + a GitHub-Projects GraphQL sub-surface | No |
| [SOL-013](./SOL-013-gitlab-channels.md) | BUG-013 | 3 new GitLab-only RPCs, same pattern as SOL-012 | No |
| [SOL-014](./SOL-014-hostedreview-channels.md) | BUG-014 | Wire 2 existing RPCs, add `CheckHostedReviewEligibility` | No |
| [SOL-015](./SOL-015-jira-channels.md) | BUG-015 | Build on `issue-tracking-service.md`'s 21-RPC sketch | No |
| [SOL-016](./SOL-016-linear-channels.md) | BUG-016 | Shares SOL-015's surface; diverges only where Linear's model genuinely differs | No |
| [SOL-017](./SOL-017-nativechat-channels.md) | BUG-017 | Relay via `infra-fleet-service.Relay`, no new service/proto | Needs a `connectionId` param (small) |
| [SOL-018](./SOL-018-orchestration-channels.md) | BUG-018 | New `GetDispatchContextForTask` read RPC (the field wasn't actually missing) | No |
| [SOL-019](./SOL-019-profile-channels.md) | BUG-019 | Implement `tenant-service.md`'s already-specified 5 RPCs | No |
| [SOL-020](./SOL-020-project-channels.md) | BUG-020 | Wire 4 ready RPCs; implement 3 already-specified membership RPCs | No |
| [SOL-021](./SOL-021-projectgroup-channels.md) | BUG-021 | Wire 4 ready RPCs; add 3 new, scans relayed via dev server not backend host | No |
| [SOL-022](./SOL-022-projecthostsetup-channels.md) | BUG-022 | Settle ownership as `project-service`; new table, ID-only FK to `infra-fleet-service` | No |
| [SOL-023](./SOL-023-repo-channels.md) | BUG-023 | Wire 4 ready RPCs; split remaining 9 across `project-service`/`git-gateway-service` | No |
| [SOL-024](./SOL-024-ssh-channels.md) | BUG-024 | 3 RPCs already named in the TDD; `connect` designed as a remote handshake act | No |
| [SOL-025](./SOL-025-status-channels.md) | BUG-025 | Local no-downstream handler — one caller doesn't even reach `wscompat` | No |
| [SOL-026](./SOL-026-workspace-channels.md) | BUG-026 | Thin wrapper over `files.*`'s directory RPC — blocked on SOL-009 landing first | No |
| [SOL-027](./SOL-027-workspaceports-channels.md) | BUG-027 | Wire existing `ScanWorkspacePorts`; add `KillWorkspacePort`, same shape | No |
| [SOL-028](./SOL-028-team-channels.md) | BUG-028 | 2 new RPCs, already specified in `tenant-service.md` | No |
| [SOL-029](./SOL-029-terminal-channels.md) | BUG-029 | New `AttachPty` streaming RPC + 8 lifecycle RPCs on `infra-fleet-service` | No — adapter over the *existing* wire protocol (Option A) |
| [SOL-030](./SOL-030-workflow-channels.md) | BUG-030 | Wire 3 ready RPCs; add `UpdateTemplate` with version-bump semantics | No |
| [SOL-031](./SOL-031-worktree-channels.md) | BUG-031 | New on-disk worktree RPCs on `git-gateway-service`, saga-coordinated with `project-service` | No |
| [SOL-032](./SOL-032-git-channels.md) | BUG-032 | Wire 4 ready RPCs; ~28 new RPCs across 5 operation families | No |
| [SOL-033](./SOL-033-automation-channels.md) | BUG-033 | Wire 2 ready RPCs; build 3 more to the repository layer; verify `runNow` end-to-end | No |
| [SOL-034](./SOL-034-task-channels.md) | BUG-034 | Keep `create`/`get`; wire `execute`; build 6 more against the real schema | No |
| [SOL-035](./SOL-035-ws-server-push-bridge.md) | BUG-035 | gRPC-stream→`push`-frame bridge, per the TDD's own already-specified design | No |
| [SOL-036](./SOL-036-expose-git-ops-on-agent-part-a.md) | BUG-036 | Re-expose `git.status`/`diff`/`commit` on the agent's Part A dispatcher, mirroring the 8-method precedent already there | **Yes — but reuses already-built Part B logic, low risk** |

## The two most consequential proposals

- **SOL-032 (`git.*`)** — the largest gap in the whole audit (28 new RPCs
  across branch/staging/history/remote/AI-assist families), and `git.*`
  underlies the product's entire commit/branch/code-review workflow. Its
  dispatch design (extending the exact mechanism `GetStatus`/`GetDiff`
  already use) is also the template SOL-009 (`files.*`) and SOL-034
  (`task.execute`'s relay) build on.
- **SOL-031 (`worktree.*`)** — `git-gateway-service` currently has **zero**
  worktree RPCs; nothing in `backend-go` can create or remove an on-disk
  worktree today. Establishes the cross-service consistency pattern (saga
  + compensation) for split ownership between `git-gateway-service`
  (on-disk truth) and `project-service` (bookkeeping) — a pattern other
  split-ownership namespaces (SOL-023's `repo.*`) also reference.

`SOL-029` (`terminal.*`) is the other major new-surface design (a PTY
streaming RPC built from nothing), but it's explicitly **not** blocked on
`agent/` changes — it adapts over the existing wire protocol per
`08-inter-service-communication.md`'s Option A recommendation, and its
output is designed to flow through SOL-035's push bridge once both land.

## A critical finding, not just a missing feature: SOL-036

Every other item in this index proposes building something that doesn't
exist yet. `SOL-036` is different — it fixes something that's currently
**broken while claiming to work**: `git.status`/`git.diff` are marked
"wired" in `../README.md`, but tracing the full data flow against the
real `agent/` source (BUG-036) found the agent process backend-go's
transport reaches never registers those methods at all, on any connection
mode. `SOL-032`'s entire ~28-RPC plan builds on the same broken
assumption. Read `SOL-036` before starting any `git.*` relay work,
including the "quick win" wiring in `TASK-206`.

## Blocked on out-of-scope `agent/` work

The TDD's own stated boundary excludes `agent/` (the Dev Server Agent)
from this rewrite's scope. Three proposals ran into that boundary directly
and say so rather than papering over it:

| SOL | What's blocked | What ships now instead |
|---|---|---|
| [SOL-006](./SOL-006-browser-channels.md) | `browser.*` needs real CDP-level remote browser control — doesn't exist in `agent/` at all | Nothing — flagged as needing a product decision before any implementation starts |
| [SOL-008](./SOL-008-emulator-channels.md) | Driving ADB/`xcrun simctl` remotely — no such `agent/` capability | Honest, permanent "not supported" stubs replacing the misleading generic error |
| [SOL-011](./SOL-011-host-channels.md) | Per-target WSL/pwsh/git-bash probing on a specific dev server | Honest local `false`/`[]` answers (meaningful anyway, since `backend-go` runs on Linux containers) |

## Product decisions surfaced, not just engineering ones

- **SOL-007 (`credentials.*`)** — routing through the owning domain
  services makes integration credentials **tenant-wide**, not per-user
  like the old TS backend's `.enc` files. A real behavior change, flagged
  for explicit sign-off rather than silently ported.
- **SOL-004 (`accounts.*`)** — needs a small frontend param addition
  (`connectionId`) that doesn't exist in the current call sites; not a
  pure backend fix.

## What to do with this directory

These are proposals, ranked by how much of the design work is already
done for you (see the "already specified verbatim" table above for the
easiest starting points). None are implemented — picking one up means
writing the actual `.proto`/`usecase`/`wscompat` code these documents
sketch, plus the tests each one's "Test plan" section describes, following
the same review/verification discipline `../../api-v1/solutions/` used
(build clean, `go vet` clean, tests passing) before marking a `BUG-0XX` as
fixed in `../README.md`.
