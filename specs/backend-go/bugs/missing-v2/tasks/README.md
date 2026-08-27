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

> **Status: all 15 tasks `[ ]` TODO.** Nothing in this directory has been
> implemented yet — these are ready-to-execute units, not a record of
> completed work (contrast with `../../missing-v1/tasks/`'s banner, which
> tracks 187/188 DONE).

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
