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

// MergePullRequestParams — see CreatePullRequestParams' doc comment for why
// LinkedIssueProvider/LinkedIssueRef are always empty for now.
type MergePullRequestParams struct {
	TenantID            string
	Provider            domain.ScmProvider
	Repo                string
	Number              int32
	MergeMethod         string
	CommitTitle         string
	CommitMessage       string
	LinkedIssueProvider string
	LinkedIssueRef      string
}

type MergePullRequestResult struct {
	PullRequest domain.PullRequest
	Merged      bool
	SHA         string
}

// MergePullRequest — on a successful merge (Merged==true only; an
// unmerged/conflicted result publishes nothing) durably enqueues
// orca.scm.pull_request.merged, same non-blocking posture as
// CreatePullRequest's own outbox enqueue (SOL-PI-03/BR-PI-09).
type MergePullRequest struct {
	credentials CredentialResolver
	providers   ProviderRegistry
	outbox      OutboxEnqueuer
	logger      *slog.Logger
}

func NewMergePullRequest(credentials CredentialResolver, providers ProviderRegistry, outbox OutboxEnqueuer, logger *slog.Logger) *MergePullRequest {
	if logger == nil {
		logger = slog.Default()
	}
	return &MergePullRequest{credentials: credentials, providers: providers, outbox: outbox, logger: logger}
}

func (uc *MergePullRequest) Execute(ctx context.Context, in MergePullRequestParams) (MergePullRequestResult, error) {
	if in.TenantID == "" {
		return MergePullRequestResult{}, apperrors.New(apperrors.KindInvalidArgument, "SCM_NO_TENANT", "tenant_id is required", nil)
	}
	if in.Repo == "" {
		return MergePullRequestResult{}, apperrors.New(apperrors.KindInvalidArgument, "SCM_EMPTY_REPO", "repo is required", nil)
	}
	if in.Number == 0 {
		return MergePullRequestResult{}, apperrors.New(apperrors.KindInvalidArgument, "SCM_EMPTY_NUMBER", "number is required", nil)
	}

	cred, err := uc.credentials.Resolve(ctx, in.TenantID, in.Provider)
	if err != nil {
		return MergePullRequestResult{}, apperrors.New(apperrors.KindInternal, "SCM_CREDENTIAL_RESOLVE_FAILED", "failed to resolve provider credential", err)
	}
	provider, err := uc.providers.Resolve(in.Provider)
	if err != nil {
		return MergePullRequestResult{}, apperrors.New(apperrors.KindInvalidArgument, "SCM_PROVIDER_UNSUPPORTED", "no adapter registered for this provider", err)
	}

	pr, merged, sha, err := provider.MergePullRequest(ctx, cred, in.Repo, in.Number, MergePullRequestInput{
		MergeMethod:   in.MergeMethod,
		CommitTitle:   in.CommitTitle,
		CommitMessage: in.CommitMessage,
	})
	if err != nil {
		return MergePullRequestResult{}, apperrors.New(apperrors.KindInternal, "SCM_MERGE_PULL_REQUEST_FAILED", "failed to merge pull request", err)
	}

	if merged && uc.outbox != nil {
		payload, mErr := json.Marshal(prLifecycleEventPayload{
			Provider: string(in.Provider), Repo: in.Repo, PrNumber: pr.Number,
			LinkedIssueProvider: in.LinkedIssueProvider, LinkedIssueRef: in.LinkedIssueRef,
		})
		if mErr == nil {
			event := domain.OutboxEvent{
				ID: uuid.NewString(), Subject: subjectPullRequestMerged,
				OccurredAt: time.Now().UTC(), PayloadJSON: payload,
			}
			if enqErr := uc.outbox.Enqueue(ctx, in.TenantID, event); enqErr != nil {
				uc.logger.WarnContext(ctx, "failed to enqueue pr.merged event", "error", enqErr, "pr", pr.ID)
			}
		} else {
			uc.logger.WarnContext(ctx, "failed to marshal pr.merged event payload", "error", mErr, "pr", pr.ID)
		}
	}
	return MergePullRequestResult{PullRequest: pr, Merged: merged, SHA: sha}, nil
}
