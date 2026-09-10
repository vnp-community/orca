# TASK-TG-002-02: Structured JSON `SubtaskProposal` + `AIApply` dependency edges (corrected to the real, already-transactional `AIApply`)

**From Solution:** BE-SOL-002
**Priority:** P1
**Service:** `task-service`
**File:** `backend-go/services/task-service/internal/domain/subtask_proposal.go`, `backend-go/services/task-service/internal/usecase/ai_decompose.go` (`parseSubtaskProposals`), `backend-go/services/task-service/internal/usecase/ai_apply.go`, `backend-go/proto/orca/task/v1/task.proto` (`SubtaskProposal` message widened)
**Depends on:** TASK-TG-001-04 (this task reuses `AddEdge`'s new `tasks TaskRepository`-aware constructor and its auto-block/cycle-check logic for `depends_on` edges created here)
**Status:** `[x]` DONE

---

## Context

**Grounding correction versus BE-SOL-002's own sketch.** BE-SOL-002's
"Design — dependency edges + `AIApply`" section sketches a hand-rolled
`u.tx.RunInTx(ctx, func(tx ports.Tx) error { ... u.repo.CreateTx(...);
u.repo.InsertEdgeTx(...) ... })` — the same non-existent `ports.Tx`/
`*Tx`-suffixed-method shape TASK-TG-001-04 already corrects for `AddEdge`.
Worse, this sketch also assumes `AIApply` is unwritten — it is not.

Verified directly (`internal/usecase/ai_apply.go:1-78`, read in full):
`AIApply` is **already fully implemented, already transactional**, and
already closes the exact "partial subtree on mid-loop failure" gap
BE-SOL-002 seems to assume is still open — its own doc comment (lines 23-44)
documents this in detail: it uses the REAL `usecase.TxRunner`
(`ports.go:167-184`) via `uc.txRunner.RunInTx(ctx, func(ctx, tasks, edges)
error { createTask := NewCreateTask(tasks); addEdge := NewAddEdge(edges); for
_, p := range in.Proposals { ... } })` (lines 53-73). Today it only creates
`parent_child` edges (line 67:
`addEdge.Execute(ctx, AddEdgeInput{FromTaskID: in.TaskID, ToTaskID: task.ID,
Kind: domain.EdgeKindParentChild})`) — **`depends_on` edges from
`SubtaskProposal.DependsOnIndex` genuinely don't exist yet**, that part of
BE-SOL-002's gap is real. This task widens the ALREADY-REAL `AIApply.Execute`
to also add those edges inside the SAME existing transaction, reusing
`NewAddEdge(tasks, edges)` (TASK-TG-001-04's corrected 2-arg constructor) a
second time per proposal rather than inventing `InsertEdgeTx`/`CreateTx`.

## Changes to make

**1. `internal/domain/subtask_proposal.go`** — widen (current file is 12
lines, `{Title, Description}` only):

```go
package domain

type SubtaskProposal struct {
	Title          string
	Description    string
	Type           string
	EstimatedHours float64
	DependsOnIndex []int // indices into the SAME batch, resolved to real edges in AIApply
	PromptTemplate string
}
```

**2. `internal/usecase/ai_decompose.go`** — replace `parseSubtaskProposals`
(currently the free-text numbered-list parser at lines 98-118) with a JSON
parser:

```go
func parseSubtaskProposals(raw string) ([]domain.SubtaskProposal, error) {
	var proposals []domain.SubtaskProposal
	if err := json.Unmarshal([]byte(raw), &proposals); err != nil {
		return nil, fmt.Errorf("ai_decompose: AI response is not valid JSON: %w", err)
	}
	return proposals, nil
}
```

This is an explicit-error contract change (the old parser silently degraded
to partial/empty results on malformed input; the new one does not) — update
`AIDecompose.Execute`'s call site (currently `return
parseSubtaskProposals(content), nil` at line 72) to propagate the error:

```go
proposals, err := parseSubtaskProposals(content)
if err != nil {
	return nil, apperrors.New(apperrors.KindInternal, "TASK_AI_DECOMPOSE_PARSE_FAILED", "AI response was not valid JSON", err)
}
return proposals, nil
```

`buildDecomposePrompt` (widened by TASK-TG-002-01) also needs an explicit
JSON schema example appended to its prompt body — the `AICompleter.Complete`
relay call itself needs no `format` parameter change (its interface,
`ports.go:163-165`, takes a plain `prompt string`; whatever JSON-mode
signal the underlying `ai.complete` relay call needs, if any, is carried
inside the prompt text itself today, not a separate structured param — this
scaffold has no `format` field on `AICompleter.Complete` to set).

**3. `internal/usecase/ai_apply.go`** — extend `Execute`'s existing
`RunInTx` closure (lines 53-73) to also insert `depends_on` edges after all
subtasks in the batch have been created (so index-based `DependsOnIndex`
references resolve to real IDs):

```go
func (uc *AIApply) Execute(ctx context.Context, in AIApplyInput) ([]domain.Task, error) {
	created := make([]domain.Task, 0, len(in.Proposals))
	err := uc.txRunner.RunInTx(ctx, func(ctx context.Context, tasks TaskRepository, edges EdgeRepository) error {
		createTask := NewCreateTask(tasks)
		addEdge := NewAddEdge(tasks, edges) // TASK-TG-001-04's 2-arg constructor
		createdIDs := make([]string, len(in.Proposals))
		for i, p := range in.Proposals {
			task, err := createTask.Execute(ctx, CreateTaskInput{
				Title: p.Title, ParentID: in.TaskID, // Description/Type/EstimatedHours/PromptTemplate:
				// CreateTaskInput has no such fields today (ports.go:21-26) — widen
				// CreateTaskInput alongside this change, or set them via a
				// follow-up Update call; do not silently drop them.
			})
			if err != nil {
				return apperrors.New(apperrors.KindInternal, "TASK_AI_APPLY_FAILED", "failed to create subtask from AI proposal", err)
			}
			createdIDs[i] = task.ID
			if _, err := addEdge.Execute(ctx, AddEdgeInput{FromTaskID: in.TaskID, ToTaskID: task.ID, Kind: domain.EdgeKindParentChild}); err != nil {
				return apperrors.New(apperrors.KindInternal, "TASK_AI_APPLY_FAILED", "failed to link subtask to parent", err)
			}
			created = append(created, task)
		}
		for i, p := range in.Proposals {
			for _, depIdx := range p.DependsOnIndex {
				if depIdx < 0 || depIdx >= len(createdIDs) {
					return apperrors.New(apperrors.KindInvalidArgument, "TASK_AI_APPLY_INVALID_DEPENDENCY_INDEX", "depends_on_index out of range for this batch", nil)
				}
				if _, err := addEdge.Execute(ctx, AddEdgeInput{FromTaskID: createdIDs[depIdx], ToTaskID: createdIDs[i], Kind: domain.EdgeKindDependsOn}); err != nil {
					return apperrors.New(apperrors.KindInternal, "TASK_AI_APPLY_FAILED", "failed to link AI-proposed dependency edge", err)
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return created, nil
}
```

Note the edge direction: `DependsOnIndex` on proposal `i` names tasks `i`
depends ON — `AddEdgeInput{FromTaskID: createdIDs[depIdx], ToTaskID:
createdIDs[i], Kind: EdgeKindDependsOn}` matches `AddEdge`'s existing
semantics (`fromID` is the dependency, `toID` is blocked until `fromID` is
done — see TASK-TG-001-04's auto-block direction). Because this reuses
`AddEdge.Execute` unchanged, the cycle check (`DetectCycle`) AND the
auto-block write both run per edge automatically — a proposal batch whose
`DependsOnIndex` values form a same-batch cycle is rejected the same way a
manual `AddEdge` call would be, and the whole `RunInTx` closure rolling back
means zero tasks/edges persist from a rejected batch.

**4. `task.proto`** — widen `SubtaskProposal` message (find its current
field list before editing — grep `message SubtaskProposal`) with `type`,
`estimated_hours`, `depends_on_index` (repeated int32),
`prompt_template`, matching the new domain fields.

## Test plan

- Malformed AI JSON response → `AIDecompose` returns error, zero subtasks
  created (no partial-apply) — regression test for the parser's new
  explicit-error contract.
- `AIApply` with `DependsOnIndex` forming a cycle within the same batch →
  entire transaction rolled back, zero tasks created (assert via a real DB
  test, reusing the existing
  `TestAIApply_MidLoopFailure_RollsBackEntireSubtree` fixture pattern).
- `AIApply` with valid `DependsOnIndex` values → the created tasks' edges
  match expectations exactly (parent_child to the origin task, depends_on
  among siblings per the indices).

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/task-service/...
go test ./services/task-service/internal/usecase/... -run "TestAIDecompose|TestAIApply" -v
go test ./services/task-service/internal/adapter/postgres/... -run TestRepository -v
```

Expected: clean build; the existing
`TestAIApply_MidLoopFailure_RollsBackEntireSubtree` test still passes
unchanged; new dependency-edge tests pass; malformed-JSON test asserts a
non-nil error and zero `domain.Task` rows created.

## Execution notes (2026-09-09)

Implemented exactly per the task's corrected shape: `domain.SubtaskProposal`
widened with `Type`, `EstimatedHours *float64` (pointer, not the sketch's
bare `float64` — preserves "no estimate given" vs. "estimate is 0"
distinguishability through to `CreateTaskInput`, consistent with
`domain.Task`'s own `*float64` fields from TASK-TG-001-02), `DependsOnIndex
[]int`, `PromptTemplate`. `parseSubtaskProposals` rewritten to unmarshal a
JSON array via a usecase-local `subtaskProposalJSON` wire-shape struct
(kept out of `domain/` — no existing `domain/*.go` file uses `json:"..."`
tags, so wire-format mapping stays in the usecase layer, consistent with
that convention) and now returns `(proposals, error)`; `AIDecompose.Execute`
propagates the error as `TASK_AI_DECOMPOSE_PARSE_FAILED` instead of
returning an empty slice. `buildDecomposePrompt` (TASK-TG-002-01's widened
version) gained an explicit JSON-array schema example so the AI relay
response actually parses.

`AIApply.Execute`'s existing transactional loop (unchanged: still
`uc.txRunner.RunInTx`) gained a second pass over `in.Proposals` after every
subtask has a real ID, resolving `DependsOnIndex` to real `depends_on`
edges via `NewAddEdge(tasks, edges).Execute` (TASK-TG-001-04's 2-arg
constructor) — reusing it unchanged means the cycle check and auto-block
write both run per edge automatically, exactly as the task's own note
describes. Also widened `CreateTaskInput` (not explicitly listed in the
task's own "Changes to make" numbered steps, but flagged inline in its
code sample: *"Description/Type/EstimatedHours/PromptTemplate: ... widen
CreateTaskInput alongside this change ... do not silently drop them"*) with
those 4 fields, and `CreateTask.Execute` now sets them on the constructed
`domain.Task` before persisting — closes the gap the task's own sketch
flagged rather than leaving AI-proposed `Description`/`Type`/etc. silently
dropped.

`task.proto`'s `SubtaskProposal` gained `type`, `estimated_hours` +
`has_estimated_hours` (a has-bit pair rather than a
`google.protobuf.DoubleValue`, to keep proto3 field access via plain
getters — matches this file's existing convention of plain scalars, not
wrapper types, for `Task`'s own optional numeric-shaped fields),
`depends_on_index` (repeated int32), `prompt_template`; regenerated via
`buf generate`. `server.go`'s `toProtoSubtaskProposals`/
`toDomainSubtaskProposals` widened to round-trip all 4 new fields including
the has-bit.

Fixed test fallout: 4 existing fixtures in `ai_decompose_test.go`/
`server_test.go` used the OLD free-text numbered-list format ("1. Do X")
which is no longer valid input to the new JSON parser — converted each to
a minimal JSON-array literal. Added exactly the 3 cases the task's own Test
plan names: `TestAIDecompose_MalformedJSON_ReturnsErrorNotEmptyResult`
(malformed JSON → error, nil proposals);
`TestAIApply_DependsOnIndex_SameBatchCycle_RollsBackWholeBatch` (2-cycle
within one batch → whole transaction rolled back, reusing the existing
`fakeTxRunner`'s snapshot/restore semantics the same way
`TestAIApply_MidLoopFailure_RollsBackEntireSubtree` already does); and
`TestAIApply_DependsOnIndex_CreatesDependsOnEdgesAmongSiblings` (valid
index → correct depends_on edge direction between the created siblings,
plus asserting TASK-TG-001-04's auto-block fires as a consequence). Also
added `TestAIApply_DependsOnIndex_OutOfRange_RollsBackWholeBatch` for the
explicit range-check branch, beyond what the task's Test plan named.

Verify: `go build`/`go vet ./services/task-service/...` both clean; `go
test ./services/task-service/... -run "TestAIDecompose|TestAIApply"` — all
16 cases pass, including the unchanged
`TestAIApply_MidLoopFailure_RollsBackEntireSubtree`; full `go test
./services/task-service/...` passes with no regressions. `go test
-tags=integration .../postgres/... -run TestRepository` — 3 unrelated tests
hit the same pre-existing testcontainers flake documented in
TASK-TG-001-02's execution notes on the full-suite run, all 3 passed
cleanly when re-run in isolation together.
