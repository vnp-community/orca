# TASK-WF-004-03: Conditional version bump on `UpdateTemplate`

**From Solution:** BE-SOL-004
**Priority:** P1
**Service:** `workflow-service`
**File:** `backend-go/services/workflow-service/internal/usecase/update_template.go`, `backend-go/services/workflow-service/internal/usecase/ports.go` (`TemplateRepository.Update` signature), `backend-go/services/workflow-service/internal/adapter/postgres/repository.go`
**Depends on:** None
**Status:** `[x]` DONE

## Execution notes (2026-09-09)

Re-verified live: `update_template.go`, `ports.go`'s `Update` signature,
and the real postgres `Update` SQL all matched the task's citations
exactly (including the confirmed absence of `UpdateConditional`).
Implemented against the task's own proposed "breaking" definition (step
removed / type changed / dependsOn changed) — **product sign-off on that
definition is still a merge blocker**, exactly as the task's header
mandates; not settled by this implementation.

**Real, pre-existing bug found and fixed while touching this exact SQL
statement (confirmed unrelated to this task's own change — same failure
reproduced against the unmodified `Update` method before any edit, and
matches a failure already observed in this worktree's very first full
postgres-integration-suite run during TASK-WF-005-01, before TASK-WF-004-03
was even started):** the real `Update` SQL's `parent_template_id =
NULLIF($4, '')` had no explicit `::uuid` cast. Postgres unifies `NULLIF`'s
result type with the `''` literal as `text`, and rejects assigning `text`
to `parent_template_id`'s `uuid` column outright — `column
"parent_template_id" is of type uuid but expression is of type text`.
This meant **every UpdateTemplate call against a real database failed**,
for any template, parent or not — a live, total-breakage bug, not a
theoretical one. Fixed by adding `::uuid` to the same clause. Flagging
this prominently since it's a correctness fix, not scope creep, but was
discovered opportunistically rather than being this task's stated target.

**Changes made:**
1. `internal/usecase/ports.go`: `Update` widened with `bump bool`; new
   `HasActiveExecutionsUsingTemplate` method.
2. `internal/usecase/update_template.go`: captures `existing` (previously
   discarded), computes `isBreaking`/`hasActiveUsage`/`bump` before the
   `Update` call; added `isBreakingChange`/`diffIsBreaking`/
   `sameDependsOnSet` helpers (order-independent `dependsOn` set
   comparison — reordering entries alone is not itself breaking).
3. `internal/adapter/postgres/repository.go`: `Update`'s SET list's
   version-increment clause is now conditional on `bump` (WHERE clause's
   version-match check, the `ErrTemplateVersionConflict` trigger, is
   completely unchanged); added `HasActiveExecutionsUsingTemplate`
   (same non-terminal status set as `HasActiveExecutions`) + the
   `parent_template_id` UUID cast fix above.
4. Test fakes: `fakeTemplateRepository`'s `Update`/new
   `HasActiveExecutionsUsingTemplate` updated; all direct
   `repo.Update(...)` call sites across the package (including
   `clone_template_test.go`, written earlier this session) updated to the
   new 4-arg signature.

**Existing-test regression handling (explicit, per the task's own
"regression check" requirement):** one pre-existing test,
`TestUpdateTemplate_Succeeds_ForwardsExpectedVersionAndReturnsBumpedResult`,
asserted the OLD unconditional-bump behavior (version 1→2 on every
write) — this is now a legitimately outdated assumption, not a
regression to preserve; renamed to
`TestUpdateTemplate_NonBreakingChange_DoesNotBumpVersion` and its
assertion flipped to match the new correct behavior (version stays 1),
with `hasActiveExecutionsUsingTemplate=true` set explicitly to prove
`isBreaking`, not `hasActiveUsage`, is what gates the no-bump outcome
here. Every other pre-existing `TestUpdateTemplate_*` case
(`StaleExpectedVersion`, `CyclicParent`, `EmptyParent`,
`NoExecutionRepositoryDependency`) passes completely unmodified.

**Verify output:**
```
go build ./services/workflow-service/...   # clean
go vet   ./services/workflow-service/...   # clean
go test  ./services/workflow-service/internal/usecase/... -run TestUpdateTemplate -v
  # 9/9 PASS — non-breaking-does-not-bump, breaking+active-bumps,
  # breaking+no-active-does-not-bump (all 3 test-plan cases), a
  # table-driven breaking-change-detection test (4 sub-cases: type
  # change, dependsOn change, pure addition, config-only edit), plus
  # every pre-existing case (stale-version, cyclic-parent, empty-parent,
  # no-execution-repo-dependency) unmodified
go test -tags=integration ./services/workflow-service/internal/adapter/postgres/... \
  -run "TestRepository_Update_CorrectVersion_Succeeds|TestRepository_Update_StaleVersion_ReturnsConflict|TestRepository_Update_BumpFalse_DoesNotIncrementVersion|TestRepository_HasActiveExecutionsUsingTemplate" -v
  # 4/4 PASS against a real testcontainers Postgres — including the
  # PRE-EXISTING TestRepository_Update_CorrectVersion_Succeeds, which now
  # passes for the first time after the uuid-cast fix (confirmed broken
  # before, on the unmodified statement)
go test  ./services/workflow-service/... ./services/api-gateway/...   # full suite, all ok
```

---

## ⚠ Product decision required before merge — re-stated from BE-SOL-004

The exact definition of "breaking" (this task proposes: remove a step,
change a step's type, or change a step's dependency edges) needs team
sign-off before this ships — BE-SOL-004 is explicit that its proposed
definition is a starting point, not a final one. Implement against the
proposed definition below, but flag in the PR description that product
sign-off on the definition itself is a merge blocker, not just a nice-to-have.

## Context — real divergence from BE-SOL-004's sketch, re-verified live

BE-SOL-004 sketches `uc.repo.UpdateConditional(ctx, tenantID, in.ID,
in.ExpectedVersion, changes, bump)` — **no method named
`UpdateConditional` exists**, confirmed directly. The real repository
method, read from `TemplateRepository` (`internal/usecase/ports.go:35-38`)
and its real caller in `update_template.go:61`, is:

```go
// ports.go
Update(ctx context.Context, tmpl domain.WorkflowTemplate, expectedVersion int32) (domain.WorkflowTemplate, error)
```

called as `updated, err := uc.templates.Update(ctx, next, in.ExpectedVersion)`
— it takes a full, already-constructed `domain.WorkflowTemplate` (`next`),
not a `tenantID, id string, changes` triple. `Update`'s own doc comment
(`ports.go:35-37`) confirms it "performs the version-bump-on-write
conditional UPDATE" and returns `domain.ErrTemplateVersionConflict`
(wrapped) on a version mismatch — i.e. **today `Update` unconditionally
bumps the version on every call**, per `template.go:73-75`'s comment
("Version is bumped by UpdateTemplate on every write... SOL-030").

This task must widen the REAL `Update` signature (not invent a
`UpdateConditional` that doesn't fit this codebase's existing method) to
also accept a `bump bool`, and thread it down into the real
`adapter/postgres` implementation's conditional-UPDATE SQL. The existing
optimistic-concurrency `WHERE version = $expectedVersion` clause
(`ErrTemplateVersionConflict`'s trigger) stays completely unchanged — this
task only makes the SET version = version + 1 portion of that same SQL
statement conditional on `bump`, not the WHERE clause's version-match
check.

`UpdateTemplateInput` (`update_template.go:12-16`) and `UpdateTemplate`
(`:18-24`) were re-read directly and confirmed to have no
`HasActiveExecutionsUsingTemplate`-style check today — the existing code
comment at `update_template.go:68-70` explicitly explains why NO such
guard exists for a *different* concern ("DefinitionSnapshot freezes at
Execute time... this update can never retroactively change a running
execution's behavior"). This task's new `HasActiveExecutionsUsingTemplate`
check serves a DIFFERENT purpose (deciding whether to bump the version,
not blocking the update) and does not contradict that existing comment —
call this out explicitly in the PR description so a reviewer doesn't
mistake this task for reintroducing a guard that comment already argued
against.

`TemplateRepository` has no `HasActiveExecutionsUsingTemplate` method
today — confirmed absent from `ports.go`, this task adds it.

## Changes to make

**1. `internal/usecase/ports.go`** — widen `Update`'s signature and add
the new lookup method:

```go
type TemplateRepository interface {
	CreateTemplate(ctx context.Context, tmpl domain.WorkflowTemplate) error
	GetTemplate(ctx context.Context, tenantID, id string) (domain.WorkflowTemplate, error)
	ListTemplates(ctx context.Context, tenantID, scope, pageToken string, pageSize int32) ([]domain.WorkflowTemplate, string, error)
	ResolveChain(ctx context.Context, tenantID, templateID string, maxDepth int) ([]domain.WorkflowTemplate, error)
	// Update performs the conditional UPDATE (WHERE version = expectedVersion,
	// unchanged — see ErrTemplateVersionConflict). bump controls ONLY
	// whether this write also increments version; the version-MATCH check
	// in the WHERE clause is unconditional regardless of bump's value.
	Update(ctx context.Context, tmpl domain.WorkflowTemplate, expectedVersion int32, bump bool) (domain.WorkflowTemplate, error)
	// HasActiveExecutionsUsingTemplate reports whether any non-terminal
	// WorkflowExecution currently references templateID (directly, via
	// DefinitionSnapshot's frozen template_id — confirm the real column/
	// join this needs against domain.WorkflowExecution's actual schema at
	// implementation time). Used only to decide whether a breaking change
	// should bump the version — NOT a write-blocking guard (see this
	// task's Context for why no such guard is needed for correctness).
	HasActiveExecutionsUsingTemplate(ctx context.Context, tenantID, templateID string) (bool, error)
}
```

**2. `internal/usecase/update_template.go`** — compute `isBreaking`/
`hasActiveUsage`/`bump` before the existing `Update` call
(`:61`), and pass `bump` through:

```go
func (uc *UpdateTemplate) Execute(ctx context.Context, in UpdateTemplateInput) (domain.WorkflowTemplate, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	...
	existing, err := uc.templates.GetTemplate(ctx, tenantID, in.ID) // NOTE: existing code already calls GetTemplate at :31 for existence-check only and discards the result — this task reuses that same call's return value instead of discarding it, since isBreaking needs to diff existing vs. next
	...

	next, err := domain.NewWorkflowTemplate(in.ID, tenantID, in.Name, in.DAGJSON, in.Scope, in.ParentTemplateID)
	...

	// Cycle re-validation — UNCHANGED, existing code at :49-59.
	...

	isBreaking, err := isBreakingChange(existing, next) // NEW helper — see below
	if err != nil {
		return domain.WorkflowTemplate{}, apperrors.New(apperrors.KindInvalidArgument, "WORKFLOW_INVALID_TEMPLATE", err.Error(), err)
	}
	hasActiveUsage, err := uc.templates.HasActiveExecutionsUsingTemplate(ctx, tenantID, in.ID)
	if err != nil {
		return domain.WorkflowTemplate{}, apperrors.New(apperrors.KindInternal, "WORKFLOW_TEMPLATE_USAGE_CHECK_FAILED", "failed to check active template usage", err)
	}
	bump := isBreaking && hasActiveUsage

	updated, err := uc.templates.Update(ctx, next, in.ExpectedVersion, bump)
	if err != nil {
		if errors.Is(err, domain.ErrTemplateVersionConflict) {
			return domain.WorkflowTemplate{}, apperrors.New(apperrors.KindFailedPrecondition, "WORKFLOW_TEMPLATE_VERSION_CONFLICT", "template was modified by another request", err)
		}
		return domain.WorkflowTemplate{}, apperrors.New(apperrors.KindInternal, "WORKFLOW_UPDATE_TEMPLATE_FAILED", "failed to update template", err)
	}
	return updated, nil
}

// isBreakingChange compares existing vs. next's parsed DAGs: a step
// removed, a step's type changed, or a step's dependsOn edges changed —
// the proposed starting definition (product sign-off required, see this
// task's header note).
func isBreakingChange(existing, next domain.WorkflowTemplate) (bool, error) {
	oldDAG, err := domain.ParseDAG(existing.DAGJSON)
	if err != nil {
		return false, err
	}
	newDAG, err := domain.ParseDAG(next.DAGJSON)
	if err != nil {
		return false, err
	}
	// ... diff oldDAG.Steps vs. newDAG.Steps by ID: any step present in
	// oldDAG but absent from newDAG (removal), or present in both with a
	// changed Type or changed DependsOn set, marks isBreaking true.
	return diffIsBreaking(oldDAG, newDAG), nil
}
```

**3. `internal/adapter/postgres/repository.go`** — widen the real
`Update` implementation's SQL to make the version-increment conditional:

```go
// Before (current, unconditional bump):
//   UPDATE workflow.templates SET name=$1, dag_json=$2, ..., version = version + 1
//   WHERE id=$n AND tenant_id=$n+1 AND version=$expectedVersion

// After (bump parameter controls the increment only):
query := `UPDATE workflow.templates SET name=$1, dag_json=$2, ...`
if bump {
	query += `, version = version + 1`
}
query += ` WHERE id=$n AND tenant_id=$n+1 AND version=$expectedVersion RETURNING version`
```

Confirm the exact current SQL text and parameter numbering directly in
`repository.go` before editing — this task's sketch shows the shape of
the change, not the literal diff, since the real query's full column list
wasn't re-transcribed here.

Also implement `HasActiveExecutionsUsingTemplate` — confirm the real
`workflow.executions` schema's template-reference column
(`template_id`, or wherever `DefinitionSnapshot`'s frozen reference
lives) and non-terminal status set (`pending|running|paused`, matching
`ExecutionRepository.ListRunning`'s doc comment's status vocabulary,
`ports.go:53-61`) before writing the query.

## Test plan

- Non-breaking change (e.g. rename, or a step's `prompt` field edit) does
  NOT bump version, even with active executions using the template.
- Breaking change (step removed) WITH active executions using the
  template → version bumps.
- Breaking change with ZERO active executions → version does NOT bump.
- The existing optimistic-concurrency conflict test
  (`ErrTemplateVersionConflict` on a stale `ExpectedVersion`) still passes
  unchanged — this is the regression check that the WHERE-clause behavior
  wasn't touched.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/workflow-service/...
go test ./services/workflow-service/internal/usecase/... -run TestUpdateTemplate -v
go test ./services/workflow-service/internal/adapter/postgres/... -run TestRepository -v
```

Expected: clean build; all 4 test-plan cases above pass; pre-existing
`ErrTemplateVersionConflict` test still passes with zero behavior change.
