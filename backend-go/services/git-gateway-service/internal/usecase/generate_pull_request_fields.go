package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
)

type GeneratePullRequestFieldsInput struct {
	WorktreeID string
	BaseBranch string
}

type PRFields struct {
	Title       string
	Description string
}

const prFieldsPromptPrefix = "Write a pull request title and description for the following diff against the base branch. Reply with the title on the first line and the description on subsequent lines.\n\n"

// GeneratePullRequestFields follows GenerateCommitMessage's already
// established pattern exactly: gather diff context via the same dispatch
// path, relay to the Dev Server Agent's ai.complete. Per SOL-032, this
// group needed no contract correction — see TASK-211.
type GeneratePullRequestFields struct {
	resolver  ConnectionResolver
	getStatus *GetStatus
	getDiff   *GetDiff
	completer AICompleter
}

func NewGeneratePullRequestFields(resolver ConnectionResolver, getStatus *GetStatus, getDiff *GetDiff, completer AICompleter) *GeneratePullRequestFields {
	return &GeneratePullRequestFields{resolver: resolver, getStatus: getStatus, getDiff: getDiff, completer: completer}
}

func (uc *GeneratePullRequestFields) Execute(ctx context.Context, in GeneratePullRequestFieldsInput) (PRFields, error) {
	if in.WorktreeID == "" {
		return PRFields{}, apperrors.New(apperrors.KindInvalidArgument, "GITGATEWAY_MISSING_WORKTREE_ID", "worktree_id is required", nil)
	}
	conn, err := uc.resolver.ResolveConnection(ctx, in.WorktreeID)
	if err != nil {
		return PRFields{}, apperrors.New(apperrors.KindInternal, "GITGATEWAY_RESOLVE_FAILED", "failed to resolve worktree's owning host", err)
	}
	// Same posture as GenerateCommitMessage: no host-local AI inference
	// fallback exists, so a disconnected worktree is a clear
	// FailedPrecondition.
	if !conn.Connected {
		return PRFields{}, apperrors.New(apperrors.KindFailedPrecondition, "GITGATEWAY_NO_AI_RELAY_CONNECTION", "AI-assisted PR field generation requires a connected dev server for this worktree", nil)
	}
	fullDiff, err := gatherFullDiff(ctx, uc.getStatus, uc.getDiff, in.WorktreeID, false)
	if err != nil {
		return PRFields{}, err
	}
	content, err := uc.completer.Complete(ctx, conn.ConnectionID, prFieldsPromptPrefix+fullDiff)
	if err != nil {
		return PRFields{}, apperrors.New(apperrors.KindInternal, "GITGATEWAY_AI_COMPLETE_FAILED", "failed to generate PR fields via AI relay", err)
	}
	return parsePRFields(content), nil
}

// parsePRFields splits ai.complete's response on the first newline: title,
// then description. A response with no newline becomes {title, ""} rather
// than an error — a model that ignores the "title on first line" prompt
// instruction still yields a usable (if imperfect) title.
func parsePRFields(content string) PRFields {
	for i, r := range content {
		if r == '\n' {
			return PRFields{Title: content[:i], Description: content[i+1:]}
		}
	}
	return PRFields{Title: content}
}
