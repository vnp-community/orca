package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/tenant"
)

// prLifecycleEventPayload is the JSON payload shape for
// orca.scm.pull_request.created/orca.scm.pull_request.merged — mirrors
// scmintegrationv1.PullRequestLifecycleEvent's field names (SOL-PI-03).
// event_id/tenant_id/occurred_at/schema_version live on the outer
// eventbus.Event envelope (common/eventbus.Event), not duplicated here.
type prLifecycleEventPayload struct {
	Provider            string `json:"provider"`
	Repo                string `json:"repo"`
	PrNumber            int32  `json:"pr_number"`
	LinkedIssueProvider string `json:"linked_issue_provider,omitempty"`
	LinkedIssueRef      string `json:"linked_issue_ref,omitempty"`
	// ActorUserID lets issue-status-sync act with that person's own tracker
	// credential; absent when the caller carried no user.
	ActorUserID string `json:"actor_user_id,omitempty"`
}

// newPRLifecyclePayload fills the linked issue from the PR text only when the
// caller supplied none (caller-provided values win).
func newPRLifecyclePayload(ctx context.Context, provider, repo string, prNumber int32, linkedProvider, linkedRef, headBranch, title, body string) prLifecycleEventPayload {
	if linkedProvider == "" || linkedRef == "" {
		linkedProvider, linkedRef = ParseLinkedJiraIssue(headBranch, title, body)
	}
	actor, _ := tenant.UserID(ctx)
	return prLifecycleEventPayload{
		Provider: provider, Repo: repo, PrNumber: prNumber,
		LinkedIssueProvider: linkedProvider, LinkedIssueRef: linkedRef, ActorUserID: actor,
	}
}

const (
	subjectPullRequestCreated = "orca.scm.pull_request.created"
	subjectPullRequestMerged  = "orca.scm.pull_request.merged"
)
