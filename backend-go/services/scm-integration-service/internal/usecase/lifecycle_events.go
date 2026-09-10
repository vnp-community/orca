package usecase

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
}

const (
	subjectPullRequestCreated = "orca.scm.pull_request.created"
	subjectPullRequestMerged  = "orca.scm.pull_request.merged"
)
