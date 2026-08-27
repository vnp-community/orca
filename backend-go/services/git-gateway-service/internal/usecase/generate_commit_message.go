package usecase

import (
	"context"
	"strings"

	"github.com/stablyai/orca-go/common/apperrors"
)

type GenerateCommitMessageInput struct {
	WorktreeID string
}

// GenerateCommitMessage relays diff context (fetched via the same
// resolve-and-dispatch path as GetDiff) to the Dev Server Agent's
// ai.complete, per git-gateway-service.md §3.1 — this service never calls
// an LLM API directly, it only assembles context and relays.
type GenerateCommitMessage struct {
	resolver  ConnectionResolver
	getStatus *GetStatus
	getDiff   *GetDiff
	history   *History // composed the same way getStatus/getDiff already are
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
