# SOL-BE-ANNOTATE-001: `annotation-service` CRUD already suffices for CR-ANNOTATE-001 — zero backend-go code change required

> **📐 Assessment-only — confirms no `backend-go` code change is needed for
> [CR-ANNOTATE-001](../../../../../docs/crs/v4/annotate/CR-ANNOTATE-001-persist-diff-comments-to-annotation-service.md).**
> Everything CR-ANNOTATE-001 needs from a persistence API — create, list
> (filtered by worktree + unsent), delete (with the sent-guard), mark-sent —
> already exists, already ships real behavior, and was already verified
> `go build`/`go vet`/`go test` clean by the CR-02 task series
> (`specs/backend-go/bugs/logic-v1/tasks/TASK-CR-02-01..07`, all `[x]` DONE).
> This document re-derives that conclusion from the TDD + a fresh read of
> the real proto/domain/adapter code, per CR-ANNOTATE-001's own instruction
> to keep code changes to a minimum and reuse what already exists.

**CR:** [CR-ANNOTATE-001](../../../../../docs/crs/v4/annotate/CR-ANNOTATE-001-persist-diff-comments-to-annotation-service.md)
**Depends on:** nothing — reads current code only
**Affected files:** none

---

## 1. What CR-ANNOTATE-001 needs from `backend-go`

CR-ANNOTATE-001's "Giải pháp đề xuất §A" asks the frontend's review-buffer
store to call, for a worktree-scoped `DiffComment`:

1. `annotation.create` with `{repo_id, worktree_id, file_path, line, end_line?, side, original_code, ref, content}`
2. `annotation.list` filtered by `worktree_id` (and, for the unsent-buffer
   case, `sent_to_agent=false`)
3. `annotation.delete`, guarded by a `confirmed` flag when the target
   annotation's `sent_to_agent=true` (BR-CR-08)
4. `annotation.markSent` after a successful "Send to Agent" delivery

The open question this document answers: **do these 4 operations already
exist, with this exact field set, wired end-to-end (proto → usecase →
postgres → wscompat)?**

## 2. Re-deriving the answer from the TDD, then the real code

`specs/backend-go/tdd/services/annotation-service.md` §3 describes the
*original* design intent — `ProjectID`/`ReviewID`, `ListAnnotationsByFile`/
`ListAnnotationsByReview` as two separate RPCs. Reading the **real** proto
and Go source shows the implementation intentionally diverged from that
sketch, and the divergence is itself already documented and deliberate:

- `specs/backend-go/bugs/logic-v1/solutions/SOL-CR-02-annotation-side-range-sent-state.md`'s
  own "Design rationale" section states plainly: *"The current implementation
  (`domain/annotation.go:40-45`) already diverges from the TDD's own
  `Anchor{FilePath, LineNumber, Ref}` + `Annotation.ProjectID` shape... by
  using `Anchor.RepoID` instead of a top-level `ProjectID`. That divergence
  predates this bug and is out of this solution's scope."*
- This is the direct explanation for CR-ANNOTATE-001 §2's finding that
  `frontend/src/renderer/src/components/code-review/annotation-panel.tsx`
  calls `annotation.create`/`annotation.list` with `projectId`/`reviewId` —
  that component was evidently written against the TDD's *original*
  `ProjectID`/`ReviewID` sketch, before the real schema settled on
  `RepoID` + `WorktreeID` (no `ReviewID` field at all). The TDD document is
  the stale side here, not the running code — the same class of drift this
  repo's other TDD sets already carry addenda for (e.g.
  `specs/agent/tdd/v5/00-index.md`'s "Addendum B" correcting a stale
  "Health Reporter" claim). **This solution treats the real proto/Go source
  as the contract to build against, not the TDD's §3/§4 sketch.**

## 3. Direct verification against the real proto + Go source

`backend-go/proto/orca/annotation/v1/annotation.proto` (read directly, not
inferred): `SubscribeRequest`-adjacent messages carry exactly the fields
CR-ANNOTATE-001 needs —

| CR-ANNOTATE-001 needs | Real proto field | Status |
|---|---|---|
| `repo_id`, `file_path`, `line` | `Anchor.repo_id`/`file_path`/`line` (base fields, pre-existing) | ✅ |
| `worktree_id` (scope the buffer to one worktree) | `Anchor.worktree_id = 5` — *"NEW — optional; scope addition, see SOL-CR-02 rationale"* | ✅ (SOL-CR-02, `[x]` DONE) |
| `side` (BR-CR-05) | `Anchor.side = 7`, `enum Side` | ✅ (SOL-CR-02) |
| `end_line` (BR-CR-06, multi-line range) | `Anchor.end_line` | ✅ (SOL-CR-02) |
| `original_code` (spec's "Original code (context)") | `Annotation.original_code` | ✅ (SOL-CR-02) |
| `sent_to_agent`/`sent_at` (BR-CR-08) | `Annotation.sent_to_agent = 10`, `sent_at = 11` | ✅ (SOL-CR-02) |
| filter unsent-only on list | `ListAnnotationsRequest.sent_to_agent` (optional bool) `= 6` — *"lets a caller ask for only-unsent"* | ✅ (SOL-CR-02) |
| bulk mark-sent | `rpc MarkAnnotationsSent(MarkAnnotationsSentRequest) returns (MarkAnnotationsSentResponse)` | ✅ (SOL-CR-02/03) |
| delete with confirm-guard | `DeleteAnnotationInput.Confirmed` (`internal/usecase`) — TASK-CR-02-05, *"3 new test cases (unconfirmed-rejects, confirmed-proceeds, not-yet-sent-succeeds-unconfirmed) passing"* | ✅ |

Wiring into `api-gateway`'s wscompat layer (`annotation.create`/`list`/
`delete`/`markSent` WS channels, plus the REST mirror) is TASK-CR-02-07,
`[x]` DONE: *"annotationAnchorArg/create/list/delete extended,
annotation.markSent + POST /v1/annotations/mark-sent wired; go build/vet/test
pass."*

## 4. Conclusion

**`backend-go` requires zero code changes for CR-ANNOTATE-001.** Every field
and every RPC the frontend-side solution needs already exists, already
passes its own test suite, and is already reachable via the WS channel names
CR-ANNOTATE-001's frontend solution calls by name
(`annotation.create`/`annotation.list`/`annotation.delete`/
`annotation.markSent`). The only work CR-ANNOTATE-001 requires is on the
`frontend/` side — see
[SOL-FE-ANNOTATE-001](../../../../frontend/crs/v4/annotate/solutions/SOL-FE-ANNOTATE-001-persist-diff-comments-to-annotation-service.md).

The one adjacent, pre-existing bug this assessment reconfirms (not new,
already the finding of CR-ANNOTATE-001 §2, cited here only for completeness):
`annotation-panel.tsx`'s calls will keep failing against this real, correct
backend contract until the frontend side is fixed — this is a `frontend/`
problem to solve, not evidence of a `backend-go` gap.

## Not in scope

- Any design work — this is a confirmation, not a solution to implement.
- Fixing `annotation-panel.tsx` — frontend concern, see CR-ANNOTATE-001 §B
  and [SOL-FE-ANNOTATE-001](../../../../frontend/crs/v4/annotate/solutions/SOL-FE-ANNOTATE-001-persist-diff-comments-to-annotation-service.md).
- CR-ANNOTATE-002's compose/deliver split — that DOES need a small
  `backend-go` change, see
  [SOL-BE-ANNOTATE-002](./SOL-BE-ANNOTATE-002-extract-compose-only-channel.md).

## References

- [CR-ANNOTATE-001](../../../../../docs/crs/v4/annotate/CR-ANNOTATE-001-persist-diff-comments-to-annotation-service.md)
- `specs/backend-go/tdd/services/annotation-service.md` §3/§4 (the stale sketch — flagged, not followed)
- `specs/backend-go/bugs/logic-v1/solutions/SOL-CR-02-annotation-side-range-sent-state.md` (the real, shipped design this solution verifies)
- `specs/backend-go/bugs/logic-v1/tasks/TASK-CR-02-01.md` through `TASK-CR-02-07.md` (all `[x]` DONE — the actual implementation work)
- `backend-go/proto/orca/annotation/v1/annotation.proto`
