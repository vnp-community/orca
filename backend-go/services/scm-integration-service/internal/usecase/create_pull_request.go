package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/scm-integration-service/internal/domain"
)

// CreatePullRequestParams mirrors CreatePullRequestRequest 1:1 — see
// ListIssuesInput's doc comment for why TenantID is an explicit field here.
// Named *Params (not *Input) to avoid colliding with the port-level
// CreatePullRequestInput in ports.go, which is the narrower shape the
// ScmProvider adapter itself receives.
//
// LinkedIssueProvider/LinkedIssueRef would come from parsing the PR body's
// closing-keyword reference (e.g. "Fixes #123") — no such parser exists in
// this codebase yet (TASK-PI-03-05's own scope note), so both are always
// empty for now; orca.scm.pull_request.created is still published with an
// empty linked_issue_ref, which issue-status-sync's consumer already
// no-ops on.
type CreatePullRequestParams struct {
	TenantID          string
	Provider          domain.ScmProvider
	Repo              string
	Title             string
	Body              string
	HeadBranch        string
	BaseBranch        string
	Draft             bool  // NEW — BR-CR-20
	LinkedIssueNumber int32 // NEW — BR-CR-19; 0 means "no linked issue"
	// LinkedIssueProvider/LinkedIssueRef feed the orca.scm.pull_request.created
	// outbox event payload (SOL-PI-03) — see this file's doc comment for why
	// both are always empty today.
	LinkedIssueProvider string
	LinkedIssueRef      string
}

// CreatePullRequestResult carries the created PR plus a non-fatal
// linked-issue-update error, if any — see the Execute doc comment below
// for why a failed issue update never turns a successful PR creation into
// a failed call.
type CreatePullRequestResult struct {
	PullRequest            domain.PullRequest
	LinkedIssueUpdateError string
}

// CreatePullRequest resolves this tenant's per-provider credential, resolves
// the concrete provider adapter, and delegates. On success it durably
// enqueues orca.scm.pull_request.created (SOL-PI-03/BR-PI-09) — enqueue
// failure is logged, never fails the RPC, since the provider-side mutation
// already succeeded.
type CreatePullRequest struct {
	credentials CredentialResolver
	providers   ProviderRegistry
	updateIssue *UpdateIssue // NEW — in-process composition, mirrors GenerateCommitMessage's pattern (SOL-CR-04)
	outbox      OutboxEnqueuer
	logger      *slog.Logger
}

func NewCreatePullRequest(credentials CredentialResolver, providers ProviderRegistry, updateIssue *UpdateIssue, outbox OutboxEnqueuer, logger *slog.Logger) *CreatePullRequest {
	if logger == nil {
		logger = slog.Default()
	}
	return &CreatePullRequest{credentials: credentials, providers: providers, updateIssue: updateIssue, outbox: outbox, logger: logger}
}

func (uc *CreatePullRequest) Execute(ctx context.Context, in CreatePullRequestParams) (CreatePullRequestResult, error) {
	if in.TenantID == "" {
		return CreatePullRequestResult{}, apperrors.New(apperrors.KindInvalidArgument, "SCM_NO_TENANT", "tenant_id is required", nil)
	}
	if in.Repo == "" {
		return CreatePullRequestResult{}, apperrors.New(apperrors.KindInvalidArgument, "SCM_EMPTY_REPO", "repo is required", nil)
	}
	if in.Title == "" {
		return CreatePullRequestResult{}, apperrors.New(apperrors.KindInvalidArgument, "SCM_EMPTY_TITLE", "title is required", nil)
	}

	cred, err := uc.credentials.Resolve(ctx, in.TenantID, in.Provider)
	if err != nil {
		return CreatePullRequestResult{}, apperrors.New(apperrors.KindInternal, "SCM_CREDENTIAL_RESOLVE_FAILED", "failed to resolve provider credential", err)
	}
	provider, err := uc.providers.Resolve(in.Provider)
	if err != nil {
		return CreatePullRequestResult{}, apperrors.New(apperrors.KindInvalidArgument, "SCM_PROVIDER_UNSUPPORTED", "no adapter registered for this provider", err)
	}

	// BR-CR-17 — reuse BranchExists (already implemented, already used by
	// CheckHostedReviewEligibility) as an explicit precondition inside
	// CreatePullRequest itself.
	exists, err := provider.BranchExists(ctx, cred, in.Repo, in.HeadBranch)
	if err != nil {
		return CreatePullRequestResult{}, apperrors.New(apperrors.KindInternal, "SCM_BRANCH_EXISTS_CHECK_FAILED", "failed to verify branch was pushed", err)
	}
	if !exists {
		return CreatePullRequestResult{}, apperrors.New(apperrors.KindFailedPrecondition, "SCM_BRANCH_NOT_PUSHED", "branch must be pushed to the remote before a pull request can be created", nil)
	}

	pr, err := provider.CreatePullRequest(ctx, cred, in.Repo, CreatePullRequestInput{
		Title: in.Title, Body: in.Body, HeadBranch: in.HeadBranch, BaseBranch: in.BaseBranch,
		Draft: in.Draft, // BR-CR-20
	})
	if err != nil {
		if in.Draft && errors.Is(err, domain.ErrCapabilityUnsupported) {
			return CreatePullRequestResult{}, apperrors.New(apperrors.KindFailedPrecondition, "SCM_DRAFT_UNSUPPORTED", "this provider does not support draft pull requests", err)
		}
		return CreatePullRequestResult{}, apperrors.New(apperrors.KindInternal, "SCM_CREATE_PULL_REQUEST_FAILED", "failed to create pull request", err)
	}

	result := CreatePullRequestResult{PullRequest: pr}
	// BR-CR-19 — best-effort: the PR is already real at this point; a
	// failed issue update must not look like a failed PR creation to the
	// caller, so this error is carried in the result, not returned as the
	// call's error.
	if in.LinkedIssueNumber != 0 {
		state := "in_review" // provider-appropriate mapping is UpdateIssue/the provider adapter's own concern, unchanged by this task
		if _, err := uc.updateIssue.Execute(ctx, UpdateIssueParams{
			TenantID: in.TenantID, Provider: in.Provider, Repo: in.Repo,
			Number: in.LinkedIssueNumber, Patch: IssuePatch{State: &state},
		}); err != nil {
			result.LinkedIssueUpdateError = err.Error()
		}
	}

	if uc.outbox != nil {
		payload, mErr := json.Marshal(prLifecycleEventPayload{
			Provider: string(in.Provider), Repo: in.Repo, PrNumber: pr.Number,
			LinkedIssueProvider: in.LinkedIssueProvider, LinkedIssueRef: in.LinkedIssueRef,
		})
		if mErr == nil {
			event := domain.OutboxEvent{
				ID: uuid.NewString(), Subject: subjectPullRequestCreated,
				OccurredAt: time.Now().UTC(), PayloadJSON: payload,
			}
			if enqErr := uc.outbox.Enqueue(ctx, in.TenantID, event); enqErr != nil {
				uc.logger.WarnContext(ctx, "failed to enqueue pr.created event", "error", enqErr, "pr", pr.ID)
			}
		} else {
			uc.logger.WarnContext(ctx, "failed to marshal pr.created event payload", "error", mErr, "pr", pr.ID)
		}
	}
	return result, nil
}
