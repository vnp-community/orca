package grpc

import (
	"github.com/stablyai/orca-go/common/apperrors"
)

// linkedIssueProviders are the providers issue-status-sync knows how to
// drive; anything else would record a link that can never be synced.
var linkedIssueProviders = map[string]bool{"jira": true, "linear": true, "github": true, "gitlab": true}

// validateLinkedIssue checks CreateWorktreeRequest's linked_issue_* pair:
// both set (with a known provider) or both empty.
func validateLinkedIssue(provider, ref string) error {
	if provider == "" && ref == "" {
		return nil
	}
	if provider == "" || ref == "" {
		return apperrors.New(apperrors.KindInvalidArgument, "WORKTREE_LINKED_ISSUE_INCOMPLETE", "linked_issue_provider and linked_issue_ref must be set together", nil)
	}
	if !linkedIssueProviders[provider] {
		return apperrors.New(apperrors.KindInvalidArgument, "WORKTREE_LINKED_ISSUE_UNKNOWN_PROVIDER", "linked_issue_provider must be one of jira/linear/github/gitlab", nil)
	}
	return nil
}
