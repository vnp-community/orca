# TASK-CR-04-03: Wire recent-commit style, branch/issue context, and the size fallback into `GenerateCommitMessage.Execute`

**From Solution:** SOL-CR-04
**Priority:** P1
**Service:** `git-gateway-service`
**File:** `backend-go/services/git-gateway-service/internal/usecase/generate_commit_message.go`, `backend-go/services/git-gateway-service/cmd/server/main.go`, `backend-go/services/git-gateway-service/internal/adapter/grpc/server.go`
**Depends on:** TASK-CR-04-01, TASK-CR-04-02
**Status:** `[x]` DONE — GenerateCommitMessage rewritten with history composition, BR-CR-15 threshold branch, BR-CR-16 issue-ref enforcement; main.go/server_test.go/dispatch_test.go updated; go build+vet clean, existing GenerateCommitMessage tests pass

---

## Context

`GenerateCommitMessage` already composes `getStatus`/`getDiff` — this task
adds a third composed usecase, `history` (already implemented, per
`ports.go:76-78`/`history.go`), and threads the pure helpers from
TASK-CR-04-01 through `Execute`, following the exact "compose an existing
usecase by field" pattern already in this struct.

## Changes to make

### 1. `generate_commit_message.go`

Remove the file-local `commitMessagePromptPrefix` const (now owned by
`commit_message_prompt.go`, TASK-CR-04-01) and replace the struct/
constructor/`Execute`:

```go
type GenerateCommitMessage struct {
	resolver  ConnectionResolver
	getStatus *GetStatus
	getDiff   *GetDiff
	history   *History // NEW — composed the same way getStatus/getDiff already are
	completer AICompleter
}

func NewGenerateCommitMessage(resolver ConnectionResolver, getStatus *GetStatus, getDiff *GetDiff, history *History, completer AICompleter) *GenerateCommitMessage {
	return &GenerateCommitMessage{resolver: resolver, getStatus: getStatus, getDiff: getDiff, history: history, completer: completer}
}

func (uc *GenerateCommitMessage) Execute(ctx context.Context, in GenerateCommitMessageInput) (string, error) {
	if in.WorktreeID == "" {
		return "", apperrors.New(apperrors.KindInvalidArgument, "GITGATEWAY_MISSING_WORKTREE_ID", "worktree_id is required", nil)
	}

	conn, err := uc.resolver.ResolveConnection(ctx, in.WorktreeID)
	if err != nil {
		return "", apperrors.New(apperrors.KindInternal, "GITGATEWAY_RESOLVE_FAILED", "failed to resolve worktree's owning host", err)
	}
	if !conn.Connected {
		return "", apperrors.New(apperrors.KindFailedPrecondition, "GITGATEWAY_NO_AI_RELAY_CONNECTION", "AI-assisted commit message generation requires a connected dev server for this worktree", nil)
	}

	status, err := uc.getStatus.Execute(ctx, GetStatusInput{WorktreeID: in.WorktreeID})
	if err != nil {
		return "", err
	}

	var diffOrStats string
	if len(status.Files) > maxFullDiffFiles {
		// BR-CR-15 — no per-file GetDiff calls at all above the threshold:
		// avoids the N extra dispatch round trips gatherFullDiffFromStatus
		// would otherwise make for content that gets discarded anyway.
		diffOrStats = statsOnlySummary(status.Files)
	} else {
		diffOrStats, err = gatherFullDiffFromStatus(ctx, uc.getDiff, in.WorktreeID, status, true)
		if err != nil {
			return "", err
		}
	}

	recent, err := uc.history.Execute(ctx, HistoryInput{WorktreeID: in.WorktreeID, Limit: 5})
	if err != nil {
		// Recent-commit context is a quality improvement, not a hard
		// requirement — a shallow/fresh repo with no history yet must still
		// produce a commit message. Degrade to no style context rather than
		// failing the whole generation.
		recent = HistoryResult{}
	}

	issueRef := extractIssueRef(status.Branch)
	prompt := buildCommitMessagePrompt(status.Branch, recent.Commits, diffOrStats, issueRef)

	message, err := uc.completer.Complete(ctx, conn.ConnectionID, prompt)
	if err != nil {
		return "", apperrors.New(apperrors.KindInternal, "GITGATEWAY_AI_COMPLETE_FAILED", "failed to generate commit message via AI relay", err)
	}
	if strings.TrimSpace(message) == "" {
		return "", apperrors.New(apperrors.KindInternal, "GITGATEWAY_AI_COMPLETE_EMPTY", "AI relay returned an empty commit message", nil)
	}

	// BR-CR-16 — "must always be included" is enforced structurally, not
	// left to the model's compliance with the prompt's instruction.
	if issueRef != "" && !strings.Contains(message, issueRef) {
		message = strings.TrimRight(message, "\n") + "\n\nRefs: " + issueRef
	}
	return message, nil
}
```

### 2. `cmd/server/main.go`

Update the existing call site to pass `historyUC` (already constructed one
line below, at `main.go:136`, just needs reordering above this call or a
forward reference):

```go
generateCommitMessageUC := usecase.NewGenerateCommitMessage(resolver, getStatusUC, getDiffUC, historyUC, relay)
```

Ensure `historyUC := usecase.NewHistory(resolver, local, relay)` is
constructed before this line (move it up if needed).

### 3. `internal/adapter/grpc/server.go`

Update the `NewGenerateCommitMessage(...)` call in the server's test
harness / composition (if constructed there directly rather than only via
`main.go`) to pass the new `history` argument — check
`server_test.go:317` and any non-test construction site.

## Verify

```bash
cd /opt/repos/orca/backend-go/services/git-gateway-service
go build ./...
go vet ./...
go test ./internal/usecase/... -run TestGenerateCommitMessage -v
```
