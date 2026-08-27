package usecase

import (
	"context"
	"strings"

	"github.com/stablyai/orca-go/common/apperrors"
)

type GenerateCommitMessageInput struct {
	WorktreeID string
}

// commitMessagePromptPrefix frames the staged diff for ai.complete — kept
// minimal since prompt engineering belongs to the Dev Server Agent's
// handler, not this relay point (git-gateway-service.md §3.1).
const commitMessagePromptPrefix = "Write a concise, conventional-commits-style commit message for the following staged diff. Reply with only the commit message text.\n\n"

// GenerateCommitMessage relays diff context (fetched via the same
// resolve-and-dispatch path as GetDiff) to the Dev Server Agent's
// ai.complete, per git-gateway-service.md §3.1 — this service never calls
// an LLM API directly, it only assembles context and relays.
type GenerateCommitMessage struct {
	resolver  ConnectionResolver
	getStatus *GetStatus
	getDiff   *GetDiff
	completer AICompleter
}

func NewGenerateCommitMessage(resolver ConnectionResolver, getStatus *GetStatus, getDiff *GetDiff, completer AICompleter) *GenerateCommitMessage {
	return &GenerateCommitMessage{resolver: resolver, getStatus: getStatus, getDiff: getDiff, completer: completer}
}

func (uc *GenerateCommitMessage) Execute(ctx context.Context, in GenerateCommitMessageInput) (string, error) {
	if in.WorktreeID == "" {
		return "", apperrors.New(apperrors.KindInvalidArgument, "GITGATEWAY_MISSING_WORKTREE_ID", "worktree_id is required", nil)
	}

	conn, err := uc.resolver.ResolveConnection(ctx, in.WorktreeID)
	if err != nil {
		return "", apperrors.New(apperrors.KindInternal, "GITGATEWAY_RESOLVE_FAILED", "failed to resolve worktree's owning host", err)
	}

	// ai.complete only exists on the Dev Server Agent's execution plane
	// (specs/agent/api/agent-rpc-catalog-runtime.md's ai.complete entry;
	// git-gateway-service.md §3.1's "this service never calls an LLM API
	// directly"). Connected=false means this worktree has no dev server —
	// there is no host-local AI inference fallback to degrade to, so this
	// is a clear FailedPrecondition rather than a silently empty message.
	if !conn.Connected {
		return "", apperrors.New(apperrors.KindFailedPrecondition, "GITGATEWAY_NO_AI_RELAY_CONNECTION", "AI-assisted commit message generation requires a connected dev server for this worktree", nil)
	}

	// GetDiff is per-file only (TASK-228: the real git.diff has no
	// whole-repo mode) — gatherFullDiff composes the full staged diff via
	// GetStatus (enumerate changed files) + one GetDiff call per file,
	// reusing GetStatus/GetDiff's own resolve-and-dispatch rather than
	// calling a GitExecutor directly here, so diff-fetching logic (and its
	// error wrapping) stays owned in exactly one place. This does mean
	// ResolveConnection is called extra times per request (once above to
	// decide the AI-relay path, again inside GetStatus/GetDiff.Execute) —
	// acceptable duplication for a stateless lookup, traded for not
	// re-deriving dispatchExecutor's routing here.
	fullDiff, err := gatherFullDiff(ctx, uc.getStatus, uc.getDiff, in.WorktreeID, true)
	if err != nil {
		return "", err
	}

	message, err := uc.completer.Complete(ctx, conn.ConnectionID, commitMessagePromptPrefix+fullDiff)
	if err != nil {
		return "", apperrors.New(apperrors.KindInternal, "GITGATEWAY_AI_COMPLETE_FAILED", "failed to generate commit message via AI relay", err)
	}
	if strings.TrimSpace(message) == "" {
		return "", apperrors.New(apperrors.KindInternal, "GITGATEWAY_AI_COMPLETE_EMPTY", "AI relay returned an empty commit message", nil)
	}
	return message, nil
}
