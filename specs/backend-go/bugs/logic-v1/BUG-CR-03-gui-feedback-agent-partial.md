# BUG-CR-03: No backend composition of the structured review-feedback prompt sent to the agent

**Business Logic:** [BL-CR-03](../../../../docs/logic/code-review/BL-CR-03-gui-feedback-agent.md) — Gửi Feedback Review về Agent
**Priority (per spec):** P1
**Status:** PARTIAL
**Severity:** High
**Symptom:** There is no single backend-go call a client can make to turn "all my review comments on this worktree" into the agent-parseable prompt described in the spec and have it delivered. A caller has the raw primitives (list annotations, write raw text into a PTY) but must reinvent the entire prompt-formatting business rule (BR-CR-09/10/11) itself, and the backend has no record afterward of which comments were actually sent (so "annotation count badge reset" / "review buffer cleared" — flow steps 6-7 — has no server-side state to reset).

---

## Spec summary

After annotating a diff (BL-CR-02), the reviewer clicks "Send to Agent". The system must collect all `DiffComment`s, format them into one consistent, agent-parseable prompt block (`File: ..., Line ... (side), Code: ..., Feedback: ...`, per worktree), inject that text into the agent's PTY, then reset the annotation badge/buffer. Business rules require relative repo-root paths (BR-CR-10), 2 lines of code context before/after each comment for disambiguation (BR-CR-11), a consistent parseable format (BR-CR-09), and a visual send-confirmation (BR-CR-12, a frontend concern).

## What backend-go has

- **Storage/collection primitive**: `annotation.list` / `GET /v1/annotations` (`backend-go/services/api-gateway/internal/adapter/wscompat/channels.go:165-186`, `annotation_routes.go:86-114`) returns all annotations for a `repo_id`/`file_path` — usable to gather the comments to send.
- **Transport primitive**: `terminal.send` (`backend-go/services/api-gateway/internal/adapter/wscompat/channels_terminal.go:276-304`) writes arbitrary `data` into a live PTY session via `PtyClientFrame_Input` — this is the only mechanism in backend-go that could deliver a composed prompt into the agent's terminal.

## What's missing

- **No prompt-composition logic anywhere in backend-go.** Grep across the whole tree for review-feedback formatting (`ReviewFeedback`, `SendFeedback`, `formatReviewPrompt`, etc.) returns nothing. BR-CR-09's "prompt format must be consistent so the agent can parse it" has no single owned implementation — if this is composed only in the Electron renderer, backend-go itself implements none of steps 2-3 of the flow (collect all comments; format into the structured block).
- **No code-context assembly.** BR-CR-11 requires 2 lines before/after each commented line to disambiguate — since `Annotation` doesn't store `originalCode` (see BUG-CR-02) and no usecase fetches surrounding diff lines at send-time, nothing in backend-go can produce this context server-side; a caller would need to separately call `git.diff`/`git.commitDiff` and manually splice it in.
- **No relative-path normalization guarantee.** BR-CR-10 requires repo-root-relative paths in the prompt; `Annotation.Anchor.FilePath` is stored as whatever the client sent with no server-side normalization/validation against the repo root.
- **No "sent to agent" bookkeeping.** Nothing marks annotations as delivered once `terminal.send` succeeds — flow steps 6 ("annotation count badge reset") and 7 ("review buffer cleared") have no corresponding server-side state transition (see BUG-CR-02's related gap on the missing "sent" flag). A page reload or second client has no way to know which comments were already delivered.
- **No worktree-scoped correlation.** The spec's prompt header is "Review feedback for {worktree-name}"; annotations are keyed by `repo_id` (see BUG-CR-02), not `worktree_id`, so there is no backend-native way to select "only this worktree's review comments" when composing the batch.

## See also

- [BUG-CR-02](./BUG-CR-02-annotate-diff-partial.md) — the annotation data-model gaps (no side, no code snapshot, no sent-state) that this bug's missing composition logic would need to consume.

## References

- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_terminal.go:276-304` — `terminal.send` (generic PTY input, no prompt formatting)
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go:165-186` — `annotation.list`
- `backend-go/services/annotation-service/internal/domain/annotation.go:63-76` — `Annotation` struct (no side/context/sent fields)
- `docs/logic/code-review/BL-CR-03-gui-feedback-agent.md:21-53`
