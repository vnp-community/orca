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

**All 7 root causes are now confirmed** (BUG-003/BUG-004 were resolved
from their original "needs server-side investigation" hedge while
designing their solutions — see [`solutions/README.md`](./solutions/README.md)'s
"Root causes found while designing" section). **Solutions are proposed,
not yet implemented** — see [`solutions/`](./solutions/) for all 7
designs, each grounded in [`specs/backend-go/tdd/`](../../tdd/), and
[`tasks/`](./tasks/) for their 15-task executable breakdown (also not yet
implemented — every task is `[ ]` TODO).

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
