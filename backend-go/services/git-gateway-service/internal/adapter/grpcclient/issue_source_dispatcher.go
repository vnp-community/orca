// issue_source_dispatcher.go composes ScmSourceClient/IssueTrackingSourceClient
// into the single usecase.IssueSourceClient CreateWorktreeFromIssue depends
// on — the gRPC handler's oneof (ScmIssueRef vs TrackerIssueRef) already
// resolved into domain.IssueRef.Provider by the time this is called
// (toDomainIssueRef in internal/adapter/grpc/server.go), so dispatch here
// is a plain provider-string switch.
package grpcclient

import (
	"context"
	"fmt"

	"github.com/stablyai/orca-go/services/git-gateway-service/internal/domain"
)

type issueSourceClient interface {
	GetIssue(ctx context.Context, ref domain.IssueRef) (domain.Issue, error)
}

// IssueSourceDispatcher implements usecase.IssueSourceClient by routing to
// ScmSourceClient (github/gitlab) or IssueTrackingSourceClient (jira/linear)
// based on ref.Provider.
type IssueSourceDispatcher struct {
	scm     issueSourceClient
	tracker issueSourceClient
}

func NewIssueSourceDispatcher(scm *ScmSourceClient, tracker *IssueTrackingSourceClient) *IssueSourceDispatcher {
	return &IssueSourceDispatcher{scm: scm, tracker: tracker}
}

func (d *IssueSourceDispatcher) GetIssue(ctx context.Context, ref domain.IssueRef) (domain.Issue, error) {
	switch ref.Provider {
	case "github", "gitlab":
		return d.scm.GetIssue(ctx, ref)
	case "jira", "linear":
		return d.tracker.GetIssue(ctx, ref)
	default:
		return domain.Issue{}, fmt.Errorf("grpcclient: unknown issue provider %q", ref.Provider)
	}
}
