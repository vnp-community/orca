# Missing-v2 Bug Reports — Live RPC/HTTP verification against `172.20.2.39`

This directory catalogs bugs found by actually **calling** the deployed
`backend-go` stack — not by auditing source against `specs/frontend/api/`
(that's `../missing-v1/`'s methodology) or reading browser console errors
(`../api-v1/`'s). This pass used a real RPC/HTTP test client
([`tests/client/rpc-client.ts`](../../../../tests/client/rpc-client.ts),
[`rpc-transport.spec.ts`](../../../../tests/client/rpc-transport.spec.ts),
[`rpc-catalog.spec.ts`](../../../../tests/client/rpc-catalog.spec.ts)),
authenticated as the real bootstrap admin, against the live deployment at
`ORCA_SERVER_URL` (default `http://172.20.2.39:6769`) — so every finding
here is a **reproduced runtime failure**, not a static "channel not
registered" gap. `../missing-v1/`'s own methodology note calls this out as
a known blind spot: *"Param/response shape correctness... a channel being
'wired' doesn't mean its wire shape matches... byte-for-byte"* — this
directory is exactly that follow-up pass.

## Methodology

1. Ran `tests/client/rpc-catalog.spec.ts` against the live deployment —
   picked the parameterless-or-optional-params, read-only method from each
   namespace `wscompat/channels.go` registers (confirmed registered via
   `grep -rhoE '\.Register\("[a-zA-Z][a-zA-Z0-9]*\.[a-zA-Z0-9_.]+"'
   backend-go/services/api-gateway/internal/adapter/wscompat/*.go`, 245
   channels as of 2026-08-27 — well past `missing-v1`'s original 8/13,
   confirming most of that directory's gaps really are resolved at the
   wiring level).
2. For every failure, re-ran the exact call standalone (raw `ws`/`fetch`
   scripts, not through vitest) to get the precise error text, then
   cross-referenced that error string directly against `backend-go`
   source (`grep -rn "<ERROR_CODE>" backend-go --include="*.go"`) to find
   the real `file:line` origin — never guessed from the error text alone.
3. Where two channels produced different failure modes for the same
   authenticated session (e.g. `folderWorkspace.list` vs. `project.list`),
   used that differential directly as evidence to isolate which layer was
   broken — see BUG-001's "Confirmed" section for the clearest example of
   this technique.
4. Two findings (BUG-003, BUG-004) hit the limit of what's observable from
   outside the process — the real Go `error` wrapped by `apperrors.New`
   never crosses the gRPC→JSON boundary, only its code+message do. Both
   reports say so explicitly instead of guessing further, per this
   directory's `../missing-v1/` and `../logic-v1/` precedent of citing only
   what was actually confirmed.

## Bug Index

| ID | Title | Severity | Root cause confirmed? | Solution |
|----|-------|----------|---|---|
| [BUG-001](./BUG-001-folderworkspace-channels-missing-attach-identity.md) | `folderWorkspace.*` (5/5 channels) never attach caller identity → `PROJECT_NO_TENANT` | High | ✅ Yes — exact missing line, 5 sites | [SOL-001](./solutions/SOL-001-wscompat-identity-attach-in-dispatch.md) |
| [BUG-002](./BUG-002-bootstrap-admin-missing-tenant-company-seed.md) | Bootstrap admin has no `tenant-service` company/department row → every `profile.*` call fails | High | ✅ Yes — bootstrap only writes `auth-service`'s own `users` table | [SOL-002](./solutions/SOL-002-bootstrap-admin-provisions-tenant-company.md) |
| [BUG-003](./BUG-003-project-authorization-policy-eval-failing.md) | `project.rego` OPA evaluation fails for every project → blocks `repo.list`/`worktree.list`/every `requireProjectAccess`-gated RPC | High | ✅ Yes — OPA bundle never copied into the container image (systemic, 4 services) | [SOL-003](./solutions/SOL-003-embed-opa-bundle-in-container-images.md) |
| [BUG-004](./BUG-004-project-list-internal-repository-error.md) | `project.list` fails with an opaque internal repository error | Medium | ✅ Yes — empty `page_token` bound as an invalid UUID literal | [SOL-004](./solutions/SOL-004-project-list-empty-pagetoken-uuid.md) |
| [BUG-005](./BUG-005-wscompat-empty-lists-serialize-as-null.md) | Empty list results serialize as JSON `null` instead of `[]` (`projectGroup.list`, `ssh.listTargets`, `team.list`, `credentials.list`, likely more) | Medium | ✅ Yes — Go nil-slice-to-JSON semantics, provable without deployment state | [SOL-005](./solutions/SOL-005-normalize-nil-slices-before-json-encode.md) |
| [BUG-006](./BUG-006-wscompat-session-dialect-drops-null-params.md) | `WebSessionClient` dialect bridge drops `params` when a call has none → `"missing arg[0]"` for every no-arg method | Medium | ✅ Yes — exact line in `session_dialect.go` | [SOL-006](./solutions/SOL-006-session-dialect-always-populate-args.md) |
| [BUG-007](./BUG-007-admin-api-nginx-route-missing.md) | `/admin/api/*` implemented in `api-gateway` but nginx never proxies to it — **regression vs. `../missing-v1/BUG-001`'s "✅ Resolved" status** | High | ✅ Yes — exhaustive nginx `location` block list, zero matches | [SOL-007](./solutions/SOL-007-nginx-admin-api-location-block.md) |
| [BUG-008](./BUG-008-project-profile-resolve-missing-tenant-metadata.md) | `project.list`/create-project flow fails `PROJECT_PROFILE_RESOLVE_FAILED` for `developer`/unset-role callers — reported live on `b15.openledger.vn` | High | ✅ Yes — `TenantProfileResolver.GetResolvedProfile` never calls this package's own `withTenantMetadata` helper, unlike its two sibling call sites | ✅ Fixed — [SOL-008](./solutions/SOL-008-profile-resolver-forward-tenant-metadata.md) |
| [BUG-009](./BUG-009-infra-agent-exec-failed-generic-relay-error.md) | `INFRA_AGENT_EXEC_FAILED` — selecting "System default" Claude/Codex account on a Remote Dev Server always fails, live on `b15.openledger.vn` | High | ✅ Yes — `accountsRelayArgs.AccountID` (`string`, not `*string`) collapses the client's valid `accountId:null` into `""`, which the agent correctly rejects — 30 occurrences/3h once SOL-009's logging made it visible | ✅ Fixed & deployed — [SOL-010](./solutions/SOL-010-accounts-relay-preserve-null-accountid.md) |
| [BUG-010](./BUG-010-rebind-repo-dev-server-lookup-failed-missing-tenant-metadata.md) | `PROJECT_DEV_SERVER_LOOKUP_FAILED` — `repo.rebindDevServer`/`repo.add`/`project.create` can't validate a dev server, live on `b15.openledger.vn` | High | ✅ Yes — same shape as BUG-008 one layer down: `InfraFleetDevServerLister.Exists`/`InfraFleetHostnameResolver.Hostname` never call this package's own `withTenantMetadata`, so infra-fleet-service sees no tenant and fails closed with `INFRA_NO_TENANT` — 100% reproducible, confirmed via matching `trace_id` across both services' logs | ✅ Fixed & deployed — [SOL-011](./solutions/SOL-011-dev-server-lister-forward-tenant-metadata.md) |
| [BUG-011](./BUG-011-detected-worktrees-merge-disk-first-drops-db-rows.md) | A DB-tracked worktree silently disappears from "Project Workspace (Beta)"'s sidebar — no error shown | High | 🟡 Corrected same-day — the disk-first merge is intentional design (drives frontend's delete-purge), not a bug in itself; real question narrowed to why a successful `DetectWorktrees` scan under-reported | 🔴 Needs live repro before any fix — do not union-merge |
| [BUG-012](./BUG-012-gitgateway-status-failed-opaque-relay-error.md) | `GITGATEWAY_STATUS_FAILED` — CRITICAL, ~34 usecases (essentially every worktree git operation) | High | ✅ Yes — `ConnectionResolver.ResolveConnection`'s `!Connected` branch returns the raw worktree ID as `RepoPath` (a placeholder never meant to reach `git status`'s `cwd`); confirmed via `gitnexus impact({target: "dispatchExecutor"})` → CRITICAL, 34 direct callers, not just `GetStatus` | ✅ Fixed, deployed & live-verified — [SOL-013](./solutions/SOL-013-connection-resolver-worktree-path-fallback.md) (path-echo symptom; see BUG-015 for the deeper layer this uncovered) |
| [BUG-015](./BUG-015-dispatch-executor-always-local-never-relays-to-dev-server.md) | `dispatchExecutor`/`dispatchFilesystemExecutor` always dispatched to `local`, never relayed to a worktree's real dev server, even when one was bound and reachable — found live immediately after verifying SOL-013's own deploy | High | ✅ Yes — same "zero rows, system-wide `infra.connections`" root cause as BUG-012, this time driving executor SELECTION (relay vs local), not path value | ✅ Fixed, deployed & live-verified — [SOL-014](./solutions/SOL-014-connection-resolver-relay-via-dev-server-reachability.md); `grpcurl GetStatus` for both of BUG-012's original failing worktree ids now succeeds end-to-end via `RelayByDevServer` |

| [BUG-013](./BUG-013-jira-adapter-cloud-only-rejects-self-hosted-jira.md) | `ISSUETRACKING_AUTH_FAILED` connecting a self-hosted Jira (`jr.servicehub.vn`) | High | ✅ Yes — Cloud-only `/rest/api/3/` hardcoded; fix confirmed working live with the user's real PAT via Bearer auth (`200 OK`, real profile data) | ✅ Fixed & confirmed working — user must leave Email blank in the connect form (Basic vs Bearer auth choice) |

| [BUG-014](./BUG-014-issuetracking-database-never-migrated.md) | `issuetracking` Postgres database never created — every issue-tracking-service DB call fails (blocks Jira/Linear Connect independently of BUG-013) | High | ✅ Yes — `migrate.sh`'s `SERVICES` list omitted `issuetracking` despite `docker-compose.yml` having a correct `migrate-issuetracking` entry; confirmed live via `apperrors` cause log (SOL-012's pattern) | ✅ Fixed — database created + migrated live |
| [BUG-016](./BUG-016-jira-adapter-never-maps-project-issuetype-assignee-fields.md) | Jira adapter's `SearchIssues`/`GetIssue` never mapped `project`/`issueType`/`assignee`/`reporter`/`priority`/`labels` — frontend crashed reading `issue.project.key` (required field, `undefined` on the wire) | High | ✅ Yes — `jiraIssue`'s `Fields` struct only ever parsed `summary`/`status`; confirmed live via direct gRPC `SearchIssues` call returning fully populated fields after the fix | ✅ Fixed, deployed & live-verified via direct `grpcurl SearchIssues` with real Jira data |
| [BUG-017](./BUG-017-jira-list-projects-uses-cloud-only-project-search-endpoint.md) | `ListProjects` 404s on this self-hosted Jira ("No project could be found with key 'search'") — blocks the project-picker step before any issue search | High | ✅ Yes — confirmed via direct `curl` against the real site: `/project/search` (paginated, Cloud/8.4+ Server-DC only) 404s, `/project` (flat array) works | ✅ Fixed, deployed & live-verified via direct `grpcurl ListProjects` returning the real project list |
| [BUG-018](./BUG-018-jira-linear-labels-null-not-empty-array.md) | `channels_jira.go`/`channels_linear.go` send `labels: null` for an issue with no labels — frontend crashed on `issue.labels.slice()` | High | ✅ Yes — nil slice + no `omitempty` encodes as JSON `null`; same bug class already fixed once in `channels_scm.go`, never ported to Jira/Linear | ✅ Fixed & deployed — matches the already-proven `channels_scm.go` pattern |

**12 of 14 root causes are confirmed** (BUG-011 and BUG-013 fully confirmed by source read; BUG-012 hedged pending SOL-012's observability fix, same shape BUG-009 started from) (BUG-003/BUG-004 were resolved
from their original "needs server-side investigation" hedge while
designing their solutions — see [`solutions/README.md`](./solutions/README.md)'s
"Root causes found while designing" section; BUG-008 was filed with its
root cause already confirmed by source inspection, no hedge needed).

**BUG-009 is a 2-stage story worth reading in full**: originally filed
2026-09-14 with a wrong "check-then-act race" theory (2 occurrences/7 days,
judged non-recurring) and a real, **fixed** observability gap
(`apperrors.ToGRPCStatus` discarded an `AppError`'s wrapped cause even from
server-side logs — [SOL-009](./solutions/SOL-009-apperrors-optional-cause-logging.md)).
That same day, SOL-009's own logging fix — deployed to `infra-fleet-service`
— immediately exposed the **real, deterministic, 100%-reproducible** root
cause on its first real recurrence (30 occurrences/3h, not 2/7-days): a
`null`-vs-`""` data-corruption bug in `api-gateway`'s `accounts.select*`
relay, fixed via [SOL-010](./solutions/SOL-010-accounts-relay-preserve-null-accountid.md)
and **deployed** (2026-09-14).

**BUG-001–007's solutions are proposed, not yet implemented** — see
[`solutions/`](./solutions/) for all 10 designs, each grounded in
[`specs/backend-go/tdd/`](../../tdd/), and [`tasks/`](./tasks/) for their
21-task executable breakdown. **BUG-008, BUG-009 (observability half), and
BUG-009 (root-cause half) are the exceptions: SOL-008/TASK-016-017,
SOL-009/TASK-018-019, and SOL-010/TASK-020-021 are all implemented,
verified, and deployed** (2026-09-14) — `go build`/`go vet`/`go test` clean
across every touched module, and each fix's regression test was confirmed
to fail against the pre-fix code before landing.

## Cross-cutting observations

- **Every session-dialect error is flattened to `code: "internal"`**
  (`session_dialect.go`'s `writeDialectError`, an explicitly-documented
  Phase 1 simplification from `../api-v1/BUG-005`'s fix — not a new bug,
  but it means the error **message string**, not the code, is the only
  signal available for diagnosing a `WebSessionClient`-dialect failure.
  Every report above relies on message-text matching for this reason.
- **BUG-001 and BUG-006 are both "one shared normalization/wiring step
  got skipped or is incomplete" bugs** — same shape as `../api-v1/BUG-005`
  itself. `wscompat`'s pattern of "N handlers must each remember to call a
  helper" (`AttachIdentity`, in BUG-001's case) is proving failure-prone;
  worth considering a structural fix (e.g. `Registry.Register` wrapping
  every handler to attach identity automatically) rather than continuing
  to patch call sites one at a time as new gaps like this surface.
- **The live deployment's backend-go build is materially ahead of
  `../missing-v1/`'s snapshot.** That directory's headline number (8/13
  channels wired) is stale — 245 channels are registered as of this pass.
  Re-running `../missing-v1/`'s own methodology (name-existence check
  against `rpc-catalog.md`) would likely close most of its remaining
  "missing" line items; what's actually broken now is runtime correctness
  of already-wired channels, which is this directory's whole point.
- **This was a point-in-time snapshot against a live, actively-redeployed
  environment.** Re-running `tests/client/rpc-catalog.spec.ts` produced
  measurably different results across two runs minutes apart during this
  investigation (e.g. a channel that returned "missing arg[0]" in one run
  behaved differently after `rpc-client.ts` started sending `params: {}`
  instead of omitting it) — confirm each bug is still live before starting
  a fix, the same caveat `../missing-v1/README.md`'s own status line
  carries forward from its resolved items.

## What this doesn't cover

- **Exhaustive sweep.** This pass tested a representative, mostly
  read-only slice of registered channels (per `rpc-catalog.spec.ts`'s own
  file header) — not all 245. BUG-005 in particular is explicit that its
  4 confirmed instances are likely not the full set.
- **Non-bootstrap users / real project data.** Every finding here was
  reproduced against the ONE user that exists on this deployment (the
  bootstrap admin) with zero pre-existing projects/repos/worktrees. Some
  findings (BUG-002, arguably BUG-004) may be specific to that empty-state
  scenario; BUG-001, BUG-005, BUG-006, BUG-007 are not (their root causes
  are visible directly in code, independent of data state).
- **Server-side logs.** BUG-003 and BUG-004 are flagged as needing them —
  not available to this client-only investigation.
