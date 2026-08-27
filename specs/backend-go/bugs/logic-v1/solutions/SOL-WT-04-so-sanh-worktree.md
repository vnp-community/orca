# SOL-WT-04: Multi-worktree comparison aggregation; test-results/agent-summary flagged as a spec gap, not a code fix

**Resolves:** [BUG-WT-04](../BUG-WT-04-so-sanh-worktree-partial.md)
**Service:** `git-gateway-service` (new RPC) + `project-service` (schema fix: persist `base_ref`, already-specified but unimplemented)
**Affected files (proposed):**
- `backend-go/proto/orca/project/v1/project.proto` — `Worktree.base_ref`, `RecordWorktreeCreatedRequest.base_ref`, new `GetWorktree` RPC
- `backend-go/services/project-service/migrations/000X_worktree_base_ref.up.sql`/`.down.sql` (new) — add `base_ref` column (already in `project-service.md` §5's schema sketch, never migrated)
- `backend-go/services/project-service/internal/domain/worktree.go` — `BaseRef *string` field
- `backend-go/services/project-service/internal/adapter/postgres/worktree_repository.go` — thread `base_ref` through insert/scan
- `backend-go/services/project-service/internal/usecase/ports.go`, `record_worktree_created.go` — accept `baseRef`
- `backend-go/services/git-gateway-service/internal/usecase/create_worktree.go` — pass `in.BaseRef` through to `RecordWorktreeCreated` (currently dropped)
- `backend-go/proto/orca/gitgateway/v1/gitgateway.proto` — new `CompareWorktrees` RPC
- `backend-go/services/git-gateway-service/internal/usecase/compare_worktrees.go` (new)
- `backend-go/services/git-gateway-service/internal/usecase/ports.go` — `ProjectClient.GetWorktree` (new method)
**Status:** 📋 Proposed — partially a code fix (file-count/lines/aggregation), partially a flagged architecture gap (test results, agent summary — see final section)

---

## Design rationale (grounded in TDD)

### Finding: lines-added/removed already exist — the bug report is stale on this point

The bug states "no diff-related message anywhere in `gitgateway.proto`
carries an added/removed line count." Re-checking the current proto against
that claim: `GitChangeEntry` (used by both `BranchCompareResponse.entries`
and `CommitCompareResponse.entries`) already has `int32 added = 4; int32
removed = 5;` per file (`gitgateway.proto:268-274`), and
`localgit.Executor.BranchCompare`/`CommitCompare` already populate them from
real `git diff --numstat`-equivalent parsing (`executor.go:919-935`,
`stats[entry.Path]` lookup). `BranchCompare` is also already wired
end-to-end per `BUG-032`'s "already wired" table, cited by this bug itself.
**This means 3 of the spec's 4 comparison columns (file count, lines
added/removed, and per-file status) are already fully available from a
single existing RPC call per worktree** — the remaining code gap is
narrower than the bug report describes: aggregating N of these calls as one
validated set, not computing the metrics themselves.

### The `base_ref` gap this aggregation actually depends on

Enforcing BR-WT-13 (same base branch) and BR-WT-14 (same base SHA) requires
knowing each worktree's *own* base branch. `project-service.md` §5's schema
sketch already specifies this column: `worktrees ( ... base_ref TEXT, ...
)` (`:234`). The actual implementation does not have it — `domain.Worktree`
(`project-service/internal/domain/worktree.go:28-50`) has no `BaseRef`
field, `worktreeColumns` in `worktree_repository.go:14-16` doesn't select
it, and `CreateWorktree.Execute` in `git-gateway-service` receives
`in.BaseRef` (used to run `git worktree add`) but never forwards it to
`RecordWorktreeCreated` (`create_worktree.go:60`: only `path, branch,
lineage` are passed). **This is the TDD's own already-approved schema
catching up with an implementation that dropped a field** — not a new
architecture decision, just completing what §5 already specifies. Fixing it
is a prerequisite for BR-WT-13/14, folded into this solution rather than
filed separately since `CompareWorktrees` cannot enforce either rule
without it.

### Where `CompareWorktrees` belongs: `git-gateway-service`, not an edge aggregation

Unlike `worktree.detectedList` (which genuinely merges *two* services' data
— on-disk paths from `git-gateway-service`, bookkeeping from
`project-service` — and per `05-data-architecture.md`'s explicit
prescription belongs at the edge), a worktree *comparison* only needs data
`git-gateway-service` already computes (`BranchCompare` per worktree) plus
one small metadata lookup (`base_ref`, from `project-service`, which
`git-gateway-service` already calls for other reads per §7). This is a
single-service aggregation with one upstream read, matching the shape
`CreateWorktree` itself already has (`GetRepo` call, then dispatch) — it
belongs in `git-gateway-service`'s own `usecase/` layer, not `api-gateway`'s
edge, per `git-gateway-service.md` §2's "resolve → dispatch → translate"
description extended the same way SOL-WT-01 extends it with a validation
step: this is a fan-in-and-validate step, still bounded to this service's
own dependencies (`project-service`, no new edge).

---

## Design — schema/proto: `base_ref` persistence (prerequisite)

```sql
-- project-service/migrations/000X_worktree_base_ref.up.sql
ALTER TABLE project.worktrees ADD COLUMN base_ref TEXT;
```

```protobuf
// project.proto
message RecordWorktreeCreatedRequest {
  // ...existing fields...
  optional string base_ref = 6; // NEW
}
message Worktree {
  // ...existing fields...
  optional string base_ref = 12; // NEW
}
// NEW — single-worktree lookup, the same class of gap SOL-031 already
// flagged for GetRepo ("project.proto has no single-repo-by-id lookup RPC")
rpc GetWorktree(GetWorktreeRequest) returns (Worktree);
message GetWorktreeRequest { string worktree_id = 1; }
```

`git-gateway-service.CreateWorktree.Execute`'s existing call becomes:

```go
worktree, err := uc.projects.RecordWorktreeCreated(ctx, in.ProjectID, in.RepoID, result.Path, in.Branch, in.BaseRef, in.Lineage)
```

(`ProjectClient.RecordWorktreeCreated` port signature gains a `baseRef
string` parameter — mechanical, one call site.)

---

## Design — `CompareWorktrees` RPC (`gitgateway.proto`)

```protobuf
message CompareWorktreesRequest {
  repeated string worktree_ids = 1; // 2..N worktrees being compared
}
message CompareWorktreesResponse {
  string base_ref = 1;              // the shared base branch (BR-WT-13)
  repeated WorktreeComparison worktrees = 2;
}
message WorktreeComparison {
  string worktree_id = 1;
  int32 changed_files = 2;
  int32 added_lines = 3;
  int32 removed_lines = 4;
  string merge_base = 5;   // for BR-WT-14 cross-checking, see below
  string status = 6;       // BranchCompareResponse.status passthrough
  string error_message = 7;
}
```

```go
// internal/usecase/compare_worktrees.go
func (uc *CompareWorktrees) Execute(ctx context.Context, worktreeIDs []string) (domain.CompareWorktreesResult, error) {
	if len(worktreeIDs) < 2 {
		return domain.CompareWorktreesResult{}, apperrors.New(apperrors.KindInvalidArgument, "COMPARE_NEEDS_AT_LEAST_TWO", "at least 2 worktrees required", nil)
	}

	// BR-WT-13 — every worktree must share the same base_ref.
	var sharedBaseRef string
	metas := make([]domain.WorktreeInfo, len(worktreeIDs))
	for i, id := range worktreeIDs {
		wt, err := uc.projects.GetWorktree(ctx, id)
		if err != nil {
			return domain.CompareWorktreesResult{}, apperrors.New(apperrors.KindNotFound, "WORKTREE_NOT_FOUND", "worktree not found", err)
		}
		if wt.BaseRef == "" {
			return domain.CompareWorktreesResult{}, apperrors.New(apperrors.KindFailedPrecondition, "WORKTREE_BASE_REF_UNKNOWN",
				"this worktree has no recorded base_ref (created before the base_ref backfill) — cannot validate BR-WT-13", nil)
		}
		if i == 0 {
			sharedBaseRef = wt.BaseRef
		} else if wt.BaseRef != sharedBaseRef {
			return domain.CompareWorktreesResult{}, apperrors.New(apperrors.KindFailedPrecondition, "WORKTREE_COMPARE_BASE_MISMATCH",
				fmt.Sprintf("worktree %s has base_ref %q, expected %q", id, wt.BaseRef, sharedBaseRef), nil)
		}
		metas[i] = wt
	}

	// Fan-in read: fail-fast IS correct here (unlike SOL-WT-02's fan-out) —
	// this is a pure read aggregation, a partial comparison is not useful.
	results := make([]domain.WorktreeComparison, len(worktreeIDs))
	g, gctx := errgroup.WithContext(ctx)
	for i, wt := range metas {
		i, wt := i, wt
		g.Go(func() error {
			executor, repoPath, err := dispatchExecutor(gctx, uc.resolver, uc.local, uc.relay, wt.ID)
			if err != nil {
				return err
			}
			cmp, err := executor.BranchCompare(gctx, repoPath, sharedBaseRef)
			if err != nil {
				return err
			}
			addedLines, removedLines := 0, 0
			for _, e := range cmp.Entries {
				addedLines += e.Added
				removedLines += e.Removed
			}
			results[i] = domain.WorktreeComparison{
				WorktreeID: wt.ID, ChangedFiles: cmp.ChangedFiles,
				AddedLines: addedLines, RemovedLines: removedLines,
				MergeBase: cmp.MergeBase, Status: cmp.Status, ErrorMessage: cmp.ErrorMessage,
			}
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return domain.CompareWorktreesResult{}, apperrors.New(apperrors.KindInternal, "COMPARE_WORKTREES_FAILED", "failed to compare one or more worktrees", err)
	}

	// BR-WT-14 (soft check): if merge_base differs across entries, at least
	// one worktree has a stale/unfetched view of sharedBaseRef — surfaced as
	// a per-entry warning, not a hard failure, since this is a data-quality
	// signal for the UI, not something the backend can silently fix without
	// running an implicit fetch the user didn't ask for.
	return domain.CompareWorktreesResult{BaseRef: sharedBaseRef, Worktrees: results}, nil
}
```

`ProjectClient.GetWorktree` (new port method, `ports.go`) wraps the new
`project-service.GetWorktree` RPC.

---

## What's genuinely missing and NOT fixed by this solution: test results, agent summary

Per this task's own instruction to say plainly when a gap is an
architecture decision rather than an implementation task: **these two
comparison columns cannot be added as a mechanical code change against the
current TDD**, because no service in the 17-service catalog is designated
system-of-record for either concept:

- **Test results**: no proto message, table, or RPC anywhere in
  `specs/backend-go/tdd/services/*.md` represents "the outcome of a test
  run tied to a worktree." The nearest candidates —
  `workflow-service` (step executions) and `orchestration-service`
  (coordinator runs) — model *task/step* execution, not *test* execution
  specifically, and neither's TDD doc (not in this batch's reading list,
  but referenced by `02-microservices-decomposition.md`'s catalog) attaches
  a worktree_id to a structured pass/fail/count result shape.
- **Agent summary output**: same absence. `orchestration-service` owns
  "messages, dispatch contexts, decision gates, coordinator runs"
  (`02-microservices-decomposition.md:51`) but no TDD doc reviewed here
  defines a "final summary text" field persisted anywhere, nor a
  worktree_id ↔ summary association.

Recommendation: this needs a product/architecture decision — which service
becomes system-of-record for "an agent run's outcome, keyed by worktree,"
before either column can be built. Candidates worth evaluating in a
follow-up design pass: extend `orchestration-service`'s coordinator-run
concept with a terminal summary field + worktree_id logical FK, or treat
this as a new, narrow read model per `05-data-architecture.md`'s "if
aggregation becomes a performance problem... a purpose-built read-model
service" escape hatch (though the driver here is missing data, not
performance). **Not attempted as code in this solution** — BR-WT-13/14/15
and the file/line-count columns are the only parts of BL-WT-04 this
solution actually closes.

BR-WT-15 (no auto-selected winner) requires no enforcement code: `CompareWorktrees`
returns comparison data only, with no ranking/scoring field and no "winner"
concept anywhere in its response — satisfied by omission, same as today,
but now a deliberate response-shape invariant to preserve rather than an
accident of nothing existing yet.

---

## Test plan

- `project-service/internal/domain/worktree_test.go` — `NewWorktree` accepts/round-trips `BaseRef`.
- `project-service/internal/adapter/postgres/worktree_repository_test.go` — `RecordWorktreeCreated`/`scanWorktree` persist and return `base_ref`; migration up/down test (per `05-data-architecture.md`'s CI requirement).
- `git-gateway-service/internal/usecase/create_worktree_test.go` — `in.BaseRef` is forwarded to `RecordWorktreeCreated` (regression guard against the current silent drop).
- `git-gateway-service/internal/usecase/compare_worktrees_test.go`:
  - `_LessThanTwoWorktrees_Rejects`
  - `_BaseRefMismatch_RejectsBeforeAnyBranchCompareCall` (BR-WT-13)
  - `_MissingBaseRef_RejectsWithClearCode` (pre-backfill worktree)
  - `_AggregatesAddedRemovedFromEntries` (fake `BranchCompare` returning multiple `GitChangeEntry`, assert summed correctly)
  - `_OneWorktreeCompareFails_WholeCallFails` (contrast explicitly documented against SOL-WT-02's per-item isolation — this is deliberately fail-fast)
- `adapter/grpc` — contract test for the new `CompareWorktrees`/`GetWorktree` RPCs.

## References

- `specs/backend-go/bugs/logic-v1/BUG-WT-04-so-sanh-worktree-partial.md` — full gap list (see "Finding" note above re: its lines-added/removed claim)
- `specs/backend-go/tdd/services/project-service.md:176-180,228-239` (§4/§5 `Worktree` domain model + schema sketch already specifying `base_ref`)
- `specs/backend-go/tdd/services/git-gateway-service.md:48-73` (§2, resolve→dispatch→translate, extended here for a fan-in-and-validate read)
- `specs/backend-go/tdd/architecture/02-microservices-decomposition.md:49-51` (`orchestration-service` scope — cited as the nearest, still-insufficient candidate for test-results/agent-summary ownership)
- `specs/backend-go/tdd/architecture/05-data-architecture.md:114-124` (edge-aggregation prescription this solution deliberately does NOT use, contrasted against `worktree.detectedList`)
- `backend-go/proto/orca/gitgateway/v1/gitgateway.proto:268-335` (`GitChangeEntry.added/removed`, `BranchCompareResponse`/`CommitCompareResponse` — already real, contra the bug's claim)
- `backend-go/services/git-gateway-service/internal/adapter/localgit/executor.go:919-935,1019-1029` (`BranchCompare`'s real per-file added/removed population)
- `backend-go/services/git-gateway-service/internal/usecase/create_worktree.go:60` (the dropped `in.BaseRef` forward)
- `backend-go/services/project-service/internal/domain/worktree.go:28-50`, `internal/adapter/postgres/worktree_repository.go:14-16` (missing `base_ref` field/column)
- `specs/backend-go/bugs/missing-v1/BUG-032-git-channels-partially-implemented.md` — confirms `BranchCompare` is already wired end-to-end
- `docs/logic/worktree-management/BL-WT-04-so-sanh-worktree.md`
