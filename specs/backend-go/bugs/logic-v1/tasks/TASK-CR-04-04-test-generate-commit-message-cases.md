# TASK-CR-04-04: Add regression tests for the size fallback, style context, and issue-ref enforcement

**From Solution:** SOL-CR-04
**Priority:** P1
**Service:** `git-gateway-service`
**File:** `backend-go/services/git-gateway-service/internal/usecase/generate_commit_message_test.go`
**Depends on:** TASK-CR-04-03
**Status:** `[ ]` TODO

---

## Context

BUG-CR-04's three gaps each need a specific regression guard: the >50-file
fallback must skip `GetDiff` entirely (not just truncate), a `History`
failure must degrade rather than fail generation, and a missing issue ref
in the model's response must be appended structurally.

## Changes to make

Update every existing `NewGenerateCommitMessage(resolver, getStatus,
getDiff, completer)` call site in this file to
`NewGenerateCommitMessage(resolver, getStatus, getDiff, history, completer)`,
passing a fake `*History` (e.g. `NewHistory(resolver, &fakeGitExecutor{},
&fakeGitExecutor{})` with a fake that returns a small fixed commit list, or
one returning an error for the degrade-path case).

Add these cases:

```go
func TestGenerateCommitMessage_OverFileThreshold_UsesStatsOnlyAndSkipsGetDiff(t *testing.T) {
	// status.Files at 51 entries → AICompleter.Complete receives a prompt
	// containing statsOnlySummary's "Staged changes are large" marker text,
	// and the fake GetDiff records zero calls — regression guard against
	// BR-CR-15 silently degrading to "truncate the full diff" instead of
	// "no full diff at all."
}

func TestGenerateCommitMessage_AtFileThreshold_UsesFullDiff(t *testing.T) {
	// status.Files at exactly 50 (boundary) → full diff path taken, not
	// stats-only.
}

func TestGenerateCommitMessage_AppendsMissingIssueRef(t *testing.T) {
	// Branch "fix/ORCA-123-foo", model response missing "ORCA-123" → the
	// returned message has a trailing "Refs: ORCA-123" line appended.
}

func TestGenerateCommitMessage_DoesNotDuplicateIssueRef(t *testing.T) {
	// Model response that already includes the issue ref → no duplicate
	// "Refs:" line.
}

func TestGenerateCommitMessage_HistoryFailure_DegradesToNoStyleContext(t *testing.T) {
	// History.Execute returning an error (e.g. fake GitExecutor.History
	// returns an error) → generation still succeeds with no recent-commit
	// section in the prompt sent to AICompleter.
}
```

Confirm the existing not-connected / empty-message / RPC-failure test
cases in this file still pass unchanged after the constructor signature
change — this is the regression guard that BUG-CR-04's fix didn't break
anything the bug report didn't flag.

## Verify

```bash
cd /opt/repos/orca/backend-go/services/git-gateway-service
go build ./...
go test ./internal/usecase/... -run TestGenerateCommitMessage -v
go test ./internal/usecase/... -v
```

Expected: all `TestGenerateCommitMessage_*` cases pass, including the
pre-existing ones, with no other package test broken by the constructor
signature change.
