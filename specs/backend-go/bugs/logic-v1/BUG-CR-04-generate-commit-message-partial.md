# BUG-CR-04: AI commit-message generation ignores branch/issue context, recent-commit style, and large-diff fallback

**Business Logic:** [BL-CR-04](../../../../docs/logic/code-review/BL-CR-04-generate-commit-message.md) — Tạo Commit Message bằng AI
**Priority (per spec):** P1
**Status:** PARTIAL
**Severity:** Medium
**Symptom:** Clicking "Generate Message" produces an AI commit message from the staged diff alone. It will not match the project's recent commit style, will not auto-include the ticket/issue number from the branch name (`fix/ORCA-123-foo` → nothing gets added), and on a very large staged change sends the entire diff text to the AI relay instead of falling back to file-stats-only, unlike the spec's explicit BR-CR-15/16.

---

## Spec summary

Generating a commit message must gather the staged diff, the last ~5 commits (for style/convention matching), the current branch name, and any issue/ticket ID parsed from that branch name; build one AI prompt from all of it; stream the result into the message field for the user to edit and confirm (never auto-commit, BR-CR-14). BR-CR-15 caps prompt size at file-stats-only beyond 50 changed files; BR-CR-16 requires the branch-derived issue ID to always be included.

## What backend-go has

- A real, working `GenerateCommitMessage` usecase and RPC, relayed end-to-end:
  - Proto: `GenerateCommitMessage(GenerateCommitMessageRequest{worktree_id}) returns (GenerateCommitMessageResponse{message})` (`backend-go/proto/orca/gitgateway/v1/gitgateway.proto:24,181-187`).
  - Usecase composes the full staged diff via `gatherFullDiff` (`backend-go/services/git-gateway-service/internal/usecase/generate_commit_message.go:34-77`, `diff_composer.go:17-32`) and relays `commitMessagePromptPrefix + fullDiff` to the Dev Server Agent's `ai.complete` (`generate_commit_message.go:17,69`).
  - Reachable both ways the frontend can call it: REST `POST /v1/git/commit-message` (`backend-go/services/api-gateway/internal/adapter/httpgateway/git_routes.go:29,208`) and WS `git.generateCommitMessage` (`backend-go/services/api-gateway/internal/adapter/wscompat/channels_git.go:102-119`).
  - Correctly enforces BR-CR-14 by construction: this RPC only returns a message string, it never calls `Commit` itself — an explicit, separate `git.commit`/`Commit` call is required to actually commit.

## What's missing

- **No recent-commit-log context (flow step 2b, `git log --oneline -5`).** `GenerateCommitMessageInput` is just `{WorktreeID}` (`generate_commit_message.go:10-12`); nothing in the usecase fetches or forwards recent commit history, so the AI has no basis to match the project's existing commit-message convention/style.
- **No branch name or issue/ticket ID (BR-CR-16).** The usecase never reads the current branch name, so it cannot extract or auto-include a ticket ID (e.g. `#123`, `ORCA-123`) the way the spec's flow step 2c/2d and Output Format's `Refs: #123` line require.
- **No >50-files fallback to stats-only (BR-CR-15).** `gatherFullDiff` (`diff_composer.go:17-32`) unconditionally concatenates one `GetDiff` call's full unified diff per changed file returned by `GetStatus`, with no file-count check or truncation — a large staged change sends the complete diff text to the AI relay rather than degrading to file stats only.

## See also

- [BUG-032](../missing-v1/BUG-032-git-channels-partially-implemented.md) documents `git.generateCommitMessage`'s wiring status as of an earlier snapshot (then unwired); it is now wired (`channels_git.go:102-119`) — this bug instead covers the business-rule gaps inside the usecase itself, which BUG-032 does not address.

## References

- `backend-go/services/git-gateway-service/internal/usecase/generate_commit_message.go:1-77`
- `backend-go/services/git-gateway-service/internal/usecase/diff_composer.go:1-32`
- `backend-go/proto/orca/gitgateway/v1/gitgateway.proto:24,181-187`
- `backend-go/services/api-gateway/internal/adapter/httpgateway/git_routes.go:29,189-209`
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_git.go:102-119`
- `docs/logic/code-review/BL-CR-04-generate-commit-message.md:21-64`
