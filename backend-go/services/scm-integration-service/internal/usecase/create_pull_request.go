package usecase

import (
	"context"
	"encoding/json"
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
	TenantID            string
	Provider            domain.ScmProvider
	Repo                string
	Title               string
	Body                string
	HeadBranch          string
	BaseBranch          string
	LinkedIssueProvider string
	LinkedIssueRef      string
}

// CreatePullRequest resolves this tenant's per-provider credential, resolves
// the concrete provider adapter, and delegates. On success it durably
// enqueues orca.scm.pull_request.created (SOL-PI-03/BR-PI-09) — enqueue
// failure is logged, never fails the RPC, since the provider-side mutation
// already succeeded.
type CreatePullRequest struct {
	credentials CredentialResolver
	providers   ProviderRegistry
	outbox      OutboxEnqueuer
	logger      *slog.Logger
}

func NewCreatePullRequest(credentials CredentialResolver, providers ProviderRegistry, outbox OutboxEnqueuer, logger *slog.Logger) *CreatePullRequest {
	if logger == nil {
		logger = slog.Default()
	}
	return &CreatePullRequest{credentials: credentials, providers: providers, outbox: outbox, logger: logger}
}

func (uc *CreatePullRequest) Execute(ctx context.Context, in CreatePullRequestParams) (domain.PullRequest, error) {
	if in.TenantID == "" {
		return domain.PullRequest{}, apperrors.New(apperrors.KindInvalidArgument, "SCM_NO_TENANT", "tenant_id is required", nil)
	}
	if in.Repo == "" {
		return domain.PullRequest{}, apperrors.New(apperrors.KindInvalidArgument, "SCM_EMPTY_REPO", "repo is required", nil)
	}
	if in.Title == "" {
		return domain.PullRequest{}, apperrors.New(apperrors.KindInvalidArgument, "SCM_EMPTY_TITLE", "title is required", nil)
	}

	cred, err := uc.credentials.Resolve(ctx, in.TenantID, in.Provider)
	if err != nil {
		return domain.PullRequest{}, apperrors.New(apperrors.KindInternal, "SCM_CREDENTIAL_RESOLVE_FAILED", "failed to resolve provider credential", err)
	}

	provider, err := uc.providers.Resolve(in.Provider)
	if err != nil {
		return domain.PullRequest{}, apperrors.New(apperrors.KindInvalidArgument, "SCM_PROVIDER_UNSUPPORTED", "no adapter registered for this provider", err)
	}

	pr, err := provider.CreatePullRequest(ctx, cred, in.Repo, CreatePullRequestInput{
		Title:      in.Title,
		Body:       in.Body,
		HeadBranch: in.HeadBranch,
		BaseBranch: in.BaseBranch,
	})
	if err != nil {
		return domain.PullRequest{}, apperrors.New(apperrors.KindInternal, "SCM_CREATE_PULL_REQUEST_FAILED", "failed to create pull request", err)
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
	return pr, nil
}
