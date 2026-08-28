# BUG-CR-02: Annotation data model has no old/new side, no multi-line range, and no "sent to agent" state

**Business Logic:** [BL-CR-02](../../../../docs/logic/code-review/BL-CR-02-annotate-diff.md) — Annotate Dòng Code trong Diff
**Priority (per spec):** P1
**Status:** PARTIAL
**Severity:** Medium
**Symptom:** A reviewer can create/list/update/delete an inline comment anchored to a file+line and it persists correctly, but they cannot indicate whether the comment is on the removed (old) or added (new) side of the diff, cannot comment on a multi-line selection, and the backend has no idea whether a given comment was already sent to the agent — so a client cannot enforce "confirm before deleting a comment that was already sent" (BR-CR-08) without inventing that state itself.

---

## Spec summary

BL-CR-02 lets a reviewer click any line in the diff viewer, type a markdown comment, and have it saved into a "review buffer" with an inline indicator, ready to be batched and sent to the agent (BL-CR-03). The spec's `DiffComment` interface carries `file`, `line`, `side: 'old'|'new'`, `originalCode`, `comment`, `timestamp`. Business rules require: attach to either diff side (BR-CR-05), attach to a multi-line range (BR-CR-06), survive scroll/file-switch (BR-CR-07, a pure frontend-state concern), and require confirmation to delete an already-sent comment (BR-CR-08).

## What backend-go has

- `annotation-service` is a real, working microservice with full CRUD, wired both ways the frontend can reach it:
  - REST: `POST/GET /v1/annotations`, `PUT/PATCH/DELETE /v1/annotations/{id}` (`backend-go/services/api-gateway/internal/adapter/httpgateway/annotation_routes.go:26-34`).
  - WS: `annotation.create` / `annotation.list` / `annotation.update` / `annotation.delete` (`backend-go/services/api-gateway/internal/adapter/wscompat/channels.go:87,138-215`).
- Domain model `Anchor{RepoID, FilePath, Line, Ref}` and `Annotation{ID, TenantID, AuthorID, Anchor, Content, Resolved, RequestID, CreatedAt, UpdatedAt}` (`backend-go/services/annotation-service/internal/domain/annotation.go:40-113`), with real invariant validation (`NewAnchor`, `NewAnnotation`).
- `CreateAnnotation` usecase does per-(tenant_id, request_id) idempotency (`backend-go/services/annotation-service/internal/usecase/create_annotation.go:56-80`) — solid for retry-safety on the "Add Comment" click.
- Proto contract: `backend-go/proto/orca/annotation/v1/annotation.proto:18-72`.

## What's missing

- **No `side` field anywhere** — `Anchor` (`annotation.proto:18-23`, `domain/annotation.go:40-45`) has only `repo_id/file_path/line/ref`. There is no way to record whether a comment is anchored to the diff's old or new line, so BR-CR-05 cannot be satisfied server-side; a client would have to smuggle "old"/"new" into `content` or `ref` as an ad-hoc convention.
- **No multi-line range** — `line` is a single `int32` (`annotation.proto:21`); there is no `end_line` or range concept, so BR-CR-06 (comment on a multi-line selection) has no backing field.
- **No `originalCode` / code-snapshot field** — the spec's `DiffComment.originalCode` (the line's content at comment time, needed later to disambiguate against a rebased diff) is not stored; `Annotation` only stores the reviewer's `Content`, not the code it refers to.
- **No "sent to agent" state** — `Annotation` has only a `Resolved` bool (`domain/annotation.go:72`, semantically "reviewer marked this addressed"), not a distinct "already sent to agent" flag or timestamp. BR-CR-08's "confirm before deleting an already-sent comment" has nothing to check against server-side.
- **Annotations are anchored to `repo_id`, not `worktree_id`** — BL-CR-01/03's diff review happens per-worktree (agent's uncommitted branch changes), but `Anchor.RepoID` is the only scoping key; a repo with multiple active worktrees/agent sessions has no way to keep one worktree's review comments distinct from another's at the data-model level.

## See also

None — no existing missing-v1/api-v1 bug covers annotation-service; BUG-032 (git-channels-partially-implemented) covers a different, git-gateway-specific gap.

## References

- `backend-go/proto/orca/annotation/v1/annotation.proto:18-72`
- `backend-go/services/annotation-service/internal/domain/annotation.go:40-113`
- `backend-go/services/annotation-service/internal/usecase/create_annotation.go:1-82`
- `backend-go/services/annotation-service/internal/usecase/list_annotations.go:1-50`
- `backend-go/services/api-gateway/internal/adapter/httpgateway/annotation_routes.go:26-163`
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go:87,129-222`
- `docs/logic/code-review/BL-CR-02-annotate-diff.md:36-58`
