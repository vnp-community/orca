package usecase

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/services/scm-integration-service/internal/domain"
)

// parsedMergeEvent is the minimal shape ReceiveWebhook needs to publish
// orca.scm.pull_request.merged.
//
// TenantID is deliberately best-effort empty for now: this service has no
// mapping from an inbound webhook's repo to a tenant_id yet (no per-tenant
// webhook URL/secret registration surface exists — see WebhookVerifier's
// own doc comment in ports.go for the same gap on the verification side).
// The published event still carries a useful provider/repo/pr_number;
// issue-status-sync's own consumer already no-ops on an empty
// linked_issue_ref (see create_pull_request.go's CreatePullRequestParams
// doc comment for this codebase's established precedent of documenting an
// empty field rather than fabricating one).
type parsedMergeEvent struct {
	TenantID string
	Provider string
	Repo     string
	PRNumber int32
}

// githubPullRequestWebhookPayload is the subset of GitHub's "pull_request"
// webhook event payload this receiver needs
// (https://docs.github.com/en/webhooks/webhook-events-and-payloads#pull_request).
type githubPullRequestWebhookPayload struct {
	Action      string `json:"action"`
	Number      int32  `json:"number"`
	PullRequest struct {
		Number int32 `json:"number"`
		Merged bool  `json:"merged"`
		Base   struct {
			Repo struct {
				FullName string `json:"full_name"`
			} `json:"repo"`
		} `json:"base"`
	} `json:"pull_request"`
}

// gitlabMergeRequestWebhookPayload is the subset of GitLab's "Merge Request
// Hook" event payload this receiver needs
// (https://docs.gitlab.com/user/project/integrations/webhook_events/#merge-request-events).
type gitlabMergeRequestWebhookPayload struct {
	ObjectKind string `json:"object_kind"`
	Project    struct {
		PathWithNamespace string `json:"path_with_namespace"`
	} `json:"project"`
	ObjectAttributes struct {
		Action string `json:"action"`
		IID    int32  `json:"iid"`
	} `json:"object_attributes"`
}

// parseMergeEvent reports whether rawBody represents a "PR/MR merged" event
// for provider — the only event type ReceiveWebhook acts on (SOL-PI-03).
// GitHub: a "pull_request" event with action=="closed" && merged==true.
// GitLab: a "merge_request" event with object_attributes.action=="merge".
// Every other event type (or a body that fails to parse) reports
// isMerge=false — the caller still records the delivery for idempotency,
// just never enqueues a lifecycle event for it.
func parseMergeEvent(provider domain.ScmProvider, rawBody []byte) (parsedMergeEvent, bool) {
	switch provider {
	case domain.ScmProviderGitHub:
		var payload githubPullRequestWebhookPayload
		if err := json.Unmarshal(rawBody, &payload); err != nil {
			return parsedMergeEvent{}, false
		}
		if payload.Action != "closed" || !payload.PullRequest.Merged {
			return parsedMergeEvent{}, false
		}
		number := payload.PullRequest.Number
		if number == 0 {
			number = payload.Number
		}
		return parsedMergeEvent{
			Provider: string(domain.ScmProviderGitHub),
			Repo:     payload.PullRequest.Base.Repo.FullName,
			PRNumber: number,
		}, true

	case domain.ScmProviderGitLab:
		var payload gitlabMergeRequestWebhookPayload
		if err := json.Unmarshal(rawBody, &payload); err != nil {
			return parsedMergeEvent{}, false
		}
		if payload.ObjectKind != "merge_request" || payload.ObjectAttributes.Action != "merge" {
			return parsedMergeEvent{}, false
		}
		return parsedMergeEvent{
			Provider: string(domain.ScmProviderGitLab),
			Repo:     payload.Project.PathWithNamespace,
			PRNumber: payload.ObjectAttributes.IID,
		}, true

	default:
		return parsedMergeEvent{}, false
	}
}

// prMergedEventFromWebhook builds the orca.scm.pull_request.merged outbox
// event — same payload shape (prLifecycleEventPayload, lifecycle_events.go)
// CreatePullRequest/MergePullRequest already publish from their own
// direct-mutation paths.
func prMergedEventFromWebhook(parsed parsedMergeEvent) domain.OutboxEvent {
	payload, err := json.Marshal(prLifecycleEventPayload{Provider: parsed.Provider, Repo: parsed.Repo, PrNumber: parsed.PRNumber})
	if err != nil {
		payload = []byte("{}")
	}
	return domain.OutboxEvent{
		ID: uuid.NewString(), Subject: subjectPullRequestMerged,
		OccurredAt: time.Now().UTC(), PayloadJSON: payload,
	}
}
