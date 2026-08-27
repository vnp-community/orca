# Missing-v2 Tasks — Executable Breakdown of the Solutions

15 task files (`TASK-001`–`TASK-015`), one execution unit each, derived
from the 7 proposals in [`../solutions/`](../solutions/). Each follows the
format established in [`../../missing-v1/tasks/`](../../missing-v1/tasks/)
(itself following [`../../api-v1/tasks/`](../../api-v1/tasks/)):
**From Solution** / **Priority** / **Service** / **File** / **Depends
on** / **Status**, a Context section, a "Changes to make" section with
real, copy-pasteable code grounded in the actual current `backend-go`
source (not the more abstract sketches `solutions/*.md` themselves
sometimes used — every task here was cross-checked against the real file
at the cited line, including several places where doing so surfaced a
detail the source solution's sketch got only approximately right, noted
per-task below), and a "Verify" section with exact commands.

> **Status (updated 2026-08-27): all 15 tasks `[x]` DONE — 15/15.** Every
> task file's own `**Status:**` line is authoritative — this banner
> aggregates it. All Go code changes across 6 modules (`common/policy`,
> `api-gateway`, `auth-service`, `task-service`, `annotation-service`,
> `project-service`) build, vet, and pass their full test suites cleanly.
>
> **The 4 tasks originally left `[partial]` (TASK-006, TASK-007, TASK-014,
> TASK-015) were finished by live-verifying against `deploy/dev` — the
> repo's real Docker Compose deployment set, brought up locally end to
> end**: `postgres`/`vault`/`nats`, all migrations, all 17 backend-go
> services, and the frontend nginx container, all real, all healthy. This
> surfaced that the original TASK-006/007 fix (editing
> `services/*/deploy/Dockerfile`) was aimed at a path `deploy/dev` doesn't
> actually use — it never builds a custom image for any service (see
> `deploy/dev/README.md`: every container runs a stock
> `gcr.io/distroless/static-debian12:nonroot` image with the binary
> bind-mounted read-only). The Dockerfile edits are kept (real, correct,
> may matter for a future image-build path) but the actual fix was a new
> `policy/orca-authz` bind mount added to `deploy/dev/docker-compose.yml`
> for the 4 OPA-embedding services. **Two more real, pre-existing bugs in
> `deploy/dev/docker-compose.yml` were found and fixed along the way** (not
> caused by this pass): a stale `BOOTSTRAP_TENANT_ID` env var left over
> from before TASK-003 removed that config field (renamed to
> `BOOTSTRAP_COMPANY_NAME`), and the checked-in CI script's hardcoded port
> `8080`/unset `SERVER_BIND_IP` (fixed to read `FRONTEND_HTTP_PORT` and
> override to `127.0.0.1` for a same-host check).
>
> **Live proof, not just "it builds":** `repo.list`/`worktree.list` against
> a syntactically-valid nonexistent `projectId`, which previously returned
> `PROJECT_POLICY_EVAL_FAILED` (BUG-003 — OPA bundle never loading), now
> returns a clean `PROJECT_NOT_AUTHORIZED` policy decision. `GET
> /admin/api/stats` and `POST /admin/api/users` through the real nginx
> container, which previously returned `200 text/html`/`405` (BUG-007),
> now return real `application/json` (`200`/`201`). `tests/client/rpc-catalog.spec.ts`
> re-run against this live stack went from 12/22 to 20/22 passing (2 of the
> fixed 8 were bugs in the test's own shape assertions, not the backend —
> corrected in that file); the CI routing-check script now runs unattended
> end-to-end (`docker compose up` → login → curl → assert → `docker
> compose down`), exit 0.
>
> **3 findings surfaced by the `deploy/dev` live pass — all 3 now fixed
> (updated 2026-08-27, second pass):**
> 1. ✅ **Dockerfile go-version/build-context bug, all 17 services.** All 17
>    `deploy/Dockerfile`s bumped `golang:1.23-bookworm` → `golang:1.25-bookworm`
>    (`automation-service` already had this; the other 16 didn't) and
>    `COPY services/<name> ./services/<name>` → `COPY services ./services`
>    (the whole workspace, matching what every Dockerfile's own doc comment
>    already claimed happens: `go.work` lists all 17 modules, so `go build`
>    in workspace mode needs every one of them present, not just the target
>    service's). **Live-verified**: real `docker build` for `auth-service`
>    (has the OPA-bundle `COPY policy` step) and `project-service` (has a
>    cross-service `depends_on`) both completed, exit 0, producing real
>    ~53MB distroless images — not just "the Dockerfile parses."
> 2. ✅ **`profile.getUserProfile`/`profile.listDepts` still failing** — root
>    cause was **not** a missing bootstrap `UserProfile` row (the original
>    hypothesis, wrong): `channels_tenant_project.go`'s handlers sent the
>    caller's raw (usually omitted) `userId`/`companyId` param straight
>    through — an empty string bound into tenant-service's UUID columns,
>    erroring `TENANT_PROFILE_LOOKUP_FAILED`/`TENANT_LIST_DEPARTMENTS_FAILED`
>    — instead of defaulting to the caller's own `id.UserID`/`id.TenantID`
>    like `profile.getResolved` (right above both, in the same file)
>    already correctly does. Same bug class as BUG-001/BUG-004. Fixed with
>    `decodeOptionalArg` + `cmp.Or(param, id.X)`.
> 3. ✅ **`/admin/api/*` response shape** (snake_case, numeric `role` enum,
>    `/admin/api/users` structurally fine as `{users:[...]}` — that part
>    was never wrong, confirmed against the old TS backend's own
>    `res.json({ users, total: users.length })`). An initial fix made
>    `writeJSON` protojson-aware globally — **reverted** after the real
>    test suite showed it broke ~10 unrelated, already-passing tests in
>    `ai_provider_routes.go`/`infra_routes.go`/`notification_routes.go`/
>    `orchestration_routes.go` (this package's existing tests assert
>    today's raw-proto-passthrough shape for those routes; changing the
>    shared helper changed them all at once). Fixed instead with
>    route-local shaping structs (`userJSON`/`usersListJSON`/
>    `adminStatsJSON` in `auth_admin_routes.go`/`admin_routes.go`) scoped
>    to only the routes named in the finding. Backend-go's `Role` enum is
>    2-valued (`admin`/`user`), not the old TS backend's 3-valued
>    (`admin`/`lead`/`developer`) — documented as a deliberate,
>    non-lossless simplification, not silently pretended away.
>
> Both real deviations from each task's original code sketch (TASK-001's
> timeout, TASK-010's proto-safety narrowing, TASK-003's `TenantID` field
> removal) and these 3 findings (plus the protojson-blast-radius correction
> above) were caught only by actually building/running things, not
> guessed — see each affected task file / this section for the full detail.
> `go build`/`go vet`/`go test` clean across all 6 touched Go modules after
> every fix in this pass, re-verified directly.

## Solution → task index

| SOL | Resolves | Tasks | Count | Notes |
|---|---|---|:---:|---|
| SOL-001 | BUG-001 (`folderWorkspace.*` missing identity) | [TASK-001](./TASK-001-attach-identity-and-timeout-in-dispatch.md)–[002](./TASK-002-test-dispatch-identity-attach.md) | 2 | Edits `registry.go`'s `Dispatch` — do first, land alongside TASK-010/012 |
| SOL-002 | BUG-002 (bootstrap missing tenant company) | [TASK-003](./TASK-003-bootstrap-provisions-tenant-company.md)–[004](./TASK-004-test-bootstrap-tenant-provisioning.md) | 2 | Removes `BootstrapConfig.TenantID`; check other call sites before merging |
| SOL-003 | BUG-003 (OPA bundle missing from images) | [TASK-005](./TASK-005-opa-evaluator-warm-and-failfast.md)–[007](./TASK-007-test-opa-bundle-packaging.md) | 3 | Systemic — touches 4 services + `common/policy` |
| SOL-004 | BUG-004 (`project.list` empty-pageToken UUID error) | [TASK-008](./TASK-008-project-list-empty-pagetoken-fix.md)–[009](./TASK-009-test-project-list-pagetoken.md) | 2 | No existing test file for this usecase — TASK-009 creates one |
| SOL-005 | BUG-005 (empty lists → `null`) | [TASK-010](./TASK-010-normalize-nil-slices-in-dispatch.md)–[011](./TASK-011-test-nil-slice-normalization.md) | 2 | Also edits `Dispatch` — depends on TASK-001 |
| SOL-006 | BUG-006 (session dialect drops `params`) | [TASK-012](./TASK-012-session-dialect-populate-empty-args.md)–[013](./TASK-013-test-session-dialect-empty-params.md) | 2 | Same file family as TASK-001/010, different function |
| SOL-007 | BUG-007 (nginx `/admin/api/*` unrouted) | [TASK-014](./TASK-014-nginx-admin-api-location-block.md)–[015](./TASK-015-test-nginx-admin-api-routing.md) | 2 | Deploy config only, no Go code |

## Dependency graph

```
TASK-001 (identity+timeout in Dispatch)
  ├─→ TASK-002 (test)
  └─→ TASK-010 (nil-slice normalize, same function) ─→ TASK-011 (test)

TASK-003 (bootstrap saga)      ─→ TASK-004 (test)
TASK-005 (OPA Warm + fail-fast) ─┐
TASK-006 (Dockerfiles + config) ─┴─→ TASK-007 (test: unit + CI image check)
TASK-008 (project.list fix)    ─→ TASK-009 (test)
TASK-012 (session dialect fix) ─→ TASK-013 (test)
TASK-014 (nginx location block) ─→ TASK-015 (test: CI routing check)
```

Only **TASK-001 → TASK-010** is a true same-function edit dependency
(both modify `Registry.Dispatch`'s body) — land those two (plus TASK-012,
same file but a different function with no direct overlap) as one PR per
`solutions/README.md`'s own grouping recommendation, to avoid two
reviewers independently re-deriving "this belongs in `Dispatch`" for the
same file. Every other task set (SOL-002, SOL-003, SOL-004, SOL-007) is
fully independent — parallelizable across whichever team owns each
service.

## Corrections found while grounding tasks in real source

Writing these tasks required reading the actual current code at every
cited line (not just the `SOL-XXX.md` sketches) — three real discrepancies
between the solution's sketch and the true current shape were found and
corrected here, not silently carried forward:

- **TASK-003 (SOL-002):** `SOL-002.md` itself already documents this
  correction (see its "Correction found while designing this fix" note) —
  `tenant-service.CreateCompany` generates the tenant ID, it doesn't accept
  one, so `BootstrapConfig.TenantID` must be removed entirely, not just
  reinterpreted. TASK-003's code is grounded directly in that corrected
  design.
- **TASK-005/006/007 (SOL-003):** confirmed via direct `diff` that all 4
  services' `deploy/Dockerfile`s are byte-identical except for the service
  name, and that no `/readyz` handler exists in any of them yet (SOL-003's
  sketch left "wire into `/readyz`" as an open option; TASK-005 resolves
  that concretely to a hard startup failure, the only option actually
  achievable against the current codebase, and notes `/readyz` wiring as
  a future improvement once that endpoint exists).
- **TASK-008/009 (SOL-004):** confirmed `list_projects_test.go` doesn't
  exist at all yet (SOL-004's Testing Plan implied extending existing
  coverage) — TASK-009 creates it fresh, and confirmed the exact fake
  helpers (`newFakeProjectRepository`, `withTenantAndUser`) already
  established by sibling test files in the same package to reuse instead
  of inventing new ones.

## Numbering

Sequential, no gaps — unlike `missing-v1/tasks/`'s reserved-range scheme
(that directory's scale, 35 solutions/226 tasks, needed parallel
research-pass ranges; this one's 7 solutions/15 tasks were sized and
numbered in one pass).
