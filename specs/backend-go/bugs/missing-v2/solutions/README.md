# Solutions — Missing-v2 Bug Fixes

See [`../tasks/`](../tasks/) for the executable breakdown of every
solution below into 15 `TASK-XXX` units, each with real code grounded in
the current `backend-go` source and its own Verify steps.

Design-level solutions for every bug in [`../`](../), grounded in
[`specs/backend-go/tdd/`](../../../tdd/)'s target architecture (and, where
`tdd/` itself doesn't cover a doc the README's own map references,
[`specs/backend-go/crs/v0/`](../../../crs/v0/) — `migration/`/`standards/`,
which the `tdd/README.md` doc-map treats as part of the same corpus). Each
`SOL-XXX` cites the specific architecture principle or design doc section
its fix follows, not just "seemed reasonable" — per this bug-report
family's own established discipline of citing real sources over guessing.

> **Status: 🟡 Proposed — none of these are implemented yet.** Every root
> cause was confirmed by reading the actual `backend-go` source (not
> guessed), several during the process of designing the fix itself (see
> "Root causes found while designing," below) — but no code has been
> changed. This differs from `../../api-v1/solutions/` and
> `../../missing-v1/solutions/`, both of which document fixes that were
> actually applied and verified; treat those as the pattern for what
> "done" looks like for these once implemented, not as evidence these
> already match that state.

## Solution Index

| Solution | Bug | Fix | Priority | Scope |
|----------|-----|-----|----------|-------|
| [SOL-001](./SOL-001-wscompat-identity-attach-in-dispatch.md) | BUG-001 | Attach caller identity once in `Registry.Dispatch`, not per-handler — closes `folderWorkspace.*`'s gap and prevents the next one | High | `api-gateway` |
| [SOL-002](./SOL-002-bootstrap-admin-provisions-tenant-company.md) | BUG-002 | `auth-service` bootstrap provisions a `tenant-service` company as a synchronous saga step, using `tenant-service`'s own originated `tenant_id` (not an operator-supplied one) | High | `auth-service` ↔ `tenant-service` |
| [SOL-003](./SOL-003-embed-opa-bundle-in-container-images.md) | BUG-003 | Package the `orca-authz` Rego bundle into all 4 embedded-OPA services' container images; fail startup readiness if it doesn't load | High | `api-gateway` Docker builds (4 services) |
| [SOL-004](./SOL-004-project-list-empty-pagetoken-uuid.md) | BUG-004 | Implement AIP-158's "empty `page_token` = first page" correctly instead of binding `""` as a UUID cursor | Medium | `project-service` |
| [SOL-005](./SOL-005-normalize-nil-slices-before-json-encode.md) | BUG-005 | Normalize `nil` slices to `[]` once, in `Registry.Dispatch`'s return path, covering every channel not just the 4 confirmed | Medium | `api-gateway` |
| [SOL-006](./SOL-006-session-dialect-always-populate-args.md) | BUG-006 | `normalizeInboundMessage` always populates `Args[0]` for the session-client dialect, even with no `params` | Medium | `api-gateway` |
| [SOL-007](./SOL-007-nginx-admin-api-location-block.md) | BUG-007 | Add the missing `/admin/api/` nginx `location` block, proxying to `api-gateway` like `/v1/` already does | High | deploy config |

## Suggested implementation order

Unlike `api-v1/solutions`' strict `SOL-001 → 002 → 003 → 004` chain (a
true root-cause dependency, each masking the next), these 7 are mostly
**independent** — different services, different failure classes. Two soft
groupings worth doing together rather than strictly ordering:

1. **`api-gateway`-internal, same file family** (SOL-001, SOL-005, SOL-006)
   — all three touch `internal/adapter/wscompat/registry.go`/
   `session_dialect.go`'s shared dispatch path. Landing them as one PR
   avoids three separate reviewers each independently re-deriving "this
   belongs in `Dispatch`, not per-handler" for the same file.
2. **Everything else** (SOL-002, SOL-003, SOL-004, SOL-007) has no
   ordering dependency on group 1 or on each other — pick by whichever
   team owns `auth-service`/`tenant-service` (SOL-002),
   `project-service` (SOL-004), the shared `common/policy` +
   4 Dockerfiles (SOL-003), or deploy config (SOL-007) has capacity first.

One real dependency worth calling out: **SOL-006 (args normalization)
makes SOL-004's exact failure mode (`page_token` defaulting to `""`) the
common case for every `WebSessionClient`-dialect caller that doesn't
explicitly paginate** — landing SOL-004 without SOL-006 still fixes the
bug (the empty-string branch handles it either way), but landing SOL-006
*first* would make BUG-004 harder to reproduce for whoever picks up
SOL-004 next (the “missing arg[0]” failure SOL-006 fixes currently makes
the empty-pageToken case trivially easy to trigger in a live test — see
BUG-004's report, which found this exact interaction). Not a blocker
either direction, just worth the implementer's awareness.

## Root causes found while designing, not just filing

Two bugs (BUG-003, BUG-004) were filed with an explicit "needs
server-side investigation" hedge — no log access was available at filing
time. Designing their solutions required reading the real implementation
anyway (you can't design a fix without knowing what's actually broken),
which resolved both definitively from source alone:

- **BUG-003** turned out to be a **Docker packaging gap** (the OPA Rego
  bundle is never copied into any of the 4 embedded-OPA services' final
  container images), not an OPA sidecar/config issue as originally
  hypothesized — and it's **systemic**, affecting `auth-service`,
  `task-service`, and `annotation-service` identically, not just
  `project-service`. Both `BUG-003.md` and this solution have been updated
  to reflect that; the original bug report's hedge is now resolved, not
  still open.
- **BUG-004** turned out to be a straightforward `page_token`/UUID-typing
  bug in a raw SQL query, unrelated to BUG-002's tenant-provisioning gap
  (the weaker of the two hypotheses the original report offered, and
  correctly flagged there as the weaker one).

Both `BUG-003.md`/`BUG-004.md` were updated in place once these were
confirmed, per this repo's established convention (`api-v1`/`missing-v1`'s
bug files carry `✅ Fixed`/status updates the same way) — the "Not
Confirmed" sections that originally appeared there are gone, replaced with
"Root Cause — CONFIRMED."

## Cross-cutting design theme

Three of the seven fixes (SOL-001, SOL-005, SOL-006) are the same shape:
**a cross-cutting concern implemented per-handler instead of once at the
shared dispatch boundary.** This mirrors `api-v1/BUG-005`'s own root
cause (session-dialect handling itself started as something that would
have needed per-handler awareness, and was correctly built as a boundary
fix instead) and matches `08-inter-service-communication.md`'s explicit
design position (*"Server-side interceptors... handle [cross-cutting
concerns]... No service hand-rolls this per-RPC"*) applied to
`wscompat`'s client-side dispatch. Worth treating as a standing lesson for
`wscompat` specifically: any future "N channels need X" requirement should
default to "add X to `Dispatch`," not "remember to add X to N handlers."
