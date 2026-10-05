package grpc

import (
	"strings"

	"github.com/stablyai/orca-go/common/apperrors"
)

// linkedIssueProviders are the providers issue-status-sync knows how to
// drive; anything else would record a link that can never be synced.
var linkedIssueProviders = map[string]bool{"jira": true, "linear": true, "github": true, "gitlab": true}

// maxLinkedIssueSiteLen bounds linked_issue_site (a site base URL or id).
const maxLinkedIssueSiteLen = 512

// validateLinkedIssue checks CreateWorktreeRequest's linked_issue_* fields:
// provider and ref both set (with a known provider) or both empty. The
// optional site is only meaningful with a link; it returns the trimmed site.
func validateLinkedIssue(provider, ref, site string) (string, error) {
	site = strings.TrimSpace(site)
	if provider == "" && ref == "" {
		if site != "" {
			return "", apperrors.New(apperrors.KindInvalidArgument, "WORKTREE_LINKED_ISSUE_SITE_WITHOUT_LINK", "linked_issue_site requires linked_issue_provider and linked_issue_ref", nil)
		}
		return "", nil
	}
	if provider == "" || ref == "" {
		return "", apperrors.New(apperrors.KindInvalidArgument, "WORKTREE_LINKED_ISSUE_INCOMPLETE", "linked_issue_provider and linked_issue_ref must be set together", nil)
	}
	if !linkedIssueProviders[provider] {
		return "", apperrors.New(apperrors.KindInvalidArgument, "WORKTREE_LINKED_ISSUE_UNKNOWN_PROVIDER", "linked_issue_provider must be one of jira/linear/github/gitlab", nil)
	}
	if len(site) > maxLinkedIssueSiteLen {
		return "", apperrors.New(apperrors.KindInvalidArgument, "WORKTREE_LINKED_ISSUE_SITE_TOO_LONG", "linked_issue_site must be at most 512 characters", nil)
	}
	return site, nil
}
