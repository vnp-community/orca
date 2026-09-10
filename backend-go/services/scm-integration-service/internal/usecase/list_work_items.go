package usecase

import (
	"context"
	"sort"
	"strings"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/scm-integration-service/internal/domain"
)

// defaultWorkItemLimit mirrors the legacy desktop backend's default
// (backend/src/main/github/client.ts's listWorkItems default `limit = 24`).
const defaultWorkItemLimit = 24

// ListWorkItemsInput mirrors the ListWorkItemsRequest RPC 1:1, same
// convention as ListIssuesInput. Before/NoCache are accepted for wire
// compatibility with the frontend's GitHubWorkItemsListArgs but not yet
// honored — see WorkItemFilter's doc comment for the full list of
// deliberately-deferred v1 gaps (no cursor pagination, no server-side
// cache).
type ListWorkItemsInput struct {
	TenantID string
	Provider domain.ScmProvider
	Repo     string
	Query    string
	Limit    int32
	Before   string
	NoCache  bool
}

// ListWorkItems resolves this tenant's per-provider credential, resolves
// the concrete provider adapter, and — only for providers implementing
// WorkItemProvider — delegates to it. Combined issue+PR listing (not a
// generic ScmProvider method) is why this usecase, unlike ListIssues/
// ListPullRequests, does an extra type-assertion after Resolve.
type ListWorkItems struct {
	credentials CredentialResolver
	providers   ProviderRegistry
}

func NewListWorkItems(credentials CredentialResolver, providers ProviderRegistry) *ListWorkItems {
	return &ListWorkItems{credentials: credentials, providers: providers}
}

func (uc *ListWorkItems) Execute(ctx context.Context, in ListWorkItemsInput) ([]domain.WorkItem, error) {
	if in.TenantID == "" {
		return nil, apperrors.New(apperrors.KindInvalidArgument, "SCM_NO_TENANT", "tenant_id is required", nil)
	}
	if in.Repo == "" {
		return nil, apperrors.New(apperrors.KindInvalidArgument, "SCM_EMPTY_REPO", "repo is required", nil)
	}

	cred, err := uc.credentials.Resolve(ctx, in.TenantID, in.Provider)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "SCM_CREDENTIAL_RESOLVE_FAILED", "failed to resolve provider credential", err)
	}

	provider, err := uc.providers.Resolve(in.Provider)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInvalidArgument, "SCM_PROVIDER_UNSUPPORTED", "no adapter registered for this provider", err)
	}
	workItemProvider, ok := provider.(WorkItemProvider)
	if !ok {
		return nil, apperrors.New(apperrors.KindInvalidArgument, "SCM_WORK_ITEMS_UNSUPPORTED", "this provider does not support listing work items", nil)
	}

	filter := parseWorkItemQuery(in.Query)
	if in.Limit > 0 {
		filter.Limit = int(in.Limit)
	} else {
		filter.Limit = defaultWorkItemLimit
	}

	items, err := workItemProvider.ListWorkItems(ctx, cred, in.Repo, filter)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "SCM_LIST_WORK_ITEMS_FAILED", "failed to list work items", err)
	}

	sort.SliceStable(items, func(i, j int) bool { return items[i].UpdatedAt.After(items[j].UpdatedAt) })
	if len(items) > filter.Limit {
		items = items[:filter.Limit]
	}
	return items, nil
}

// parseWorkItemQuery implements the small, deliberately-partial subset of
// the legacy backend's GitHub search-syntax grammar this v1 supports — see
// WorkItemFilter's doc comment for exactly what's excluded and why. An
// empty query returns the "recent" default (all types, open state),
// matching listRecentWorkItems' behavior in the legacy backend.
func parseWorkItemQuery(raw string) WorkItemFilter {
	f := WorkItemFilter{Scope: "all", State: "open"}
	for _, tok := range strings.Fields(raw) {
		switch {
		case tok == "is:issue":
			f.Scope = "issue"
		case tok == "is:pr" || tok == "is:pull-request":
			f.Scope = "pr"
		case tok == "is:open":
			f.State = "open"
		case tok == "is:closed":
			f.State = "closed"
		case tok == "is:merged":
			f.Scope = "pr"
			f.State = "merged"
		case strings.HasPrefix(tok, "state:"):
			f.State = strings.TrimPrefix(tok, "state:")
		case strings.HasPrefix(tok, "label:"):
			if v := strings.TrimPrefix(tok, "label:"); v != "" {
				f.Labels = append(f.Labels, v)
			}
		case strings.HasPrefix(tok, "assignee:"):
			f.Assignee = strings.TrimPrefix(tok, "assignee:")
		case strings.HasPrefix(tok, "author:"):
			f.Author = strings.TrimPrefix(tok, "author:")
		default:
			// Free text and unsupported qualifiers (review-requested:,
			// reviewed-by:, @me, anything else) — silently ignored in v1,
			// per WorkItemFilter's doc comment.
		}
	}
	return f
}
