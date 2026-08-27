// Package domain holds scm-integration-service's entities and value
// objects. Per specs/backend-go/architecture/03-clean-architecture-guidelines.md,
// this package has zero imports outside stdlib + other domain/ packages —
// no HTTP client, no gRPC, no provider SDK. See
// specs/backend-go/services/scm-integration-service.md §4: these types are
// provider-agnostic by design — each adapter in internal/adapter/{github,
// gitlab,...}/ converts to/from a provider's own wire shape on every call,
// nothing here knows GitHub's or GitLab's JSON shape.
package domain

import (
	"errors"
	"time"
)

// ScmProvider identifies which source-control host a request/adapter call
// targets — mirrors scmintegrationv1.ScmProvider (see internal/adapter/grpc
// for the proto<->domain mapping) but kept independent of the generated
// proto package so this package stays framework-free.
type ScmProvider string

const (
	ScmProviderGitHub      ScmProvider = "github"
	ScmProviderGitLab      ScmProvider = "gitlab"
	ScmProviderBitbucket   ScmProvider = "bitbucket"
	ScmProviderAzureDevOps ScmProvider = "azure_devops"
	ScmProviderGitea       ScmProvider = "gitea"
)

// Valid reports whether p is one of the five providers this service knows
// about (§1: GitHub, GitLab, Bitbucket, Azure DevOps, Gitea).
func (p ScmProvider) Valid() bool {
	switch p {
	case ScmProviderGitHub, ScmProviderGitLab, ScmProviderBitbucket, ScmProviderAzureDevOps, ScmProviderGitea:
		return true
	default:
		return false
	}
}

var (
	// ErrInvalidProvider is returned when Provider isn't one of the known
	// enum values.
	ErrInvalidProvider = errors.New("domain: invalid scm provider")
	// ErrEmptyRepo guards against an issue/PR with no owning repository —
	// never a meaningful domain state.
	ErrEmptyRepo = errors.New("domain: repo is required")
	// ErrEmptyTitle guards against a titleless issue/PR.
	ErrEmptyTitle = errors.New("domain: title is required")
	// ErrCapabilityUnsupported is a shared sentinel a provider adapter wraps
	// its own package-level "not supported" error with (via %w), so
	// usecase/ code can detect the condition via errors.Is without
	// importing the adapter package directly — see
	// scm-integration-service.md §4's ErrCapabilityUnsupported degrade
	// pattern and this file's package doc comment on why usecase/ never
	// imports adapter packages.
	ErrCapabilityUnsupported = errors.New("domain: capability not supported by this provider")
)

// Issue is a provider-agnostic issue — see scm-integration-service.md §4.
// One shape covers GitHub Issues, GitLab Issues, Bitbucket Issues, etc.
type Issue struct {
	ID       string
	Provider ScmProvider
	Repo     string
	Title    string
	State    string
	URL      string
	Number   int32
}

// NewIssue constructs an Issue, enforcing the invariants a record must
// satisfy to be meaningful — mirrors the constructor-enforced-invariant
// pattern from usage-service's domain package.
func NewIssue(id string, provider ScmProvider, repo, title, state, url string) (Issue, error) {
	if !provider.Valid() {
		return Issue{}, ErrInvalidProvider
	}
	if repo == "" {
		return Issue{}, ErrEmptyRepo
	}
	if title == "" {
		return Issue{}, ErrEmptyTitle
	}
	return Issue{ID: id, Provider: provider, Repo: repo, Title: title, State: state, URL: url}, nil
}

// PullRequest covers both "pull request" (GitHub) and "merge request"
// (GitLab/Bitbucket) — same concept, provider-specific name only (§4).
type PullRequest struct {
	ID         string
	Provider   ScmProvider
	Repo       string
	Title      string
	State      string
	URL        string
	HeadBranch string
	BaseBranch string
	Number     int32
	Draft      bool // NEW — BR-CR-20
}

// NewPullRequest constructs a PullRequest, enforcing the same non-empty
// repo/title invariants as NewIssue.
func NewPullRequest(id string, provider ScmProvider, repo, title, state, url, headBranch, baseBranch string) (PullRequest, error) {
	if !provider.Valid() {
		return PullRequest{}, ErrInvalidProvider
	}
	if repo == "" {
		return PullRequest{}, ErrEmptyRepo
	}
	if title == "" {
		return PullRequest{}, ErrEmptyTitle
	}
	return PullRequest{
		ID: id, Provider: provider, Repo: repo, Title: title, State: state, URL: url,
		HeadBranch: headBranch, BaseBranch: baseBranch,
	}, nil
}

// RateLimitStatus is a snapshot of one provider's rate-limit bucket for the
// resolved credential — see §8: usecase/ code checks this before a burst of
// calls instead of reacting to 429s after the fact. GitHub exposes separate
// REST/GraphQL/search buckets; this type represents one bucket at a time.
type RateLimitStatus struct {
	Provider  ScmProvider
	Remaining int
	Limit     int
	ResetAt   time.Time
}

// MergeRequest is GitLab's pull-request-equivalent concept — kept as its
// own domain type (not folded into PullRequest) because it carries fields
// PullRequest doesn't (source/target branch by GitLab's own names,
// discussion counts, GitLab's own merge_status vocabulary), matching
// SOL-013's proto-level MergeRequest message 1:1.
type MergeRequest struct {
	ID                        string
	Repo                      string
	State                     string
	IID                       int32
	Title                     string
	URL                       string
	SourceBranch              string
	TargetBranch              string
	Draft                     bool
	DiscussionCount           int32
	UnresolvedDiscussionCount int32
	MergeStatus               string
}

// MergeRequestDiscussion mirrors scmintegrationv1.MergeRequestDiscussion.
type MergeRequestDiscussion struct {
	ID         string
	Resolved   bool
	ResolvedBy string
}

// WorkItemDetailsGitLab mirrors scmintegrationv1.WorkItemDetailsGitLab —
// named with the GitLab suffix to avoid colliding with SOL-012's
// provider-agnostic WorkItemDetails (usecase package, GitHub Projects v2).
type WorkItemDetailsGitLab struct {
	ID       string
	IID      int32
	ItemType string
	Title    string
	Body     string
	State    string
	URL      string
	Labels   []string
}

// ── SubmitReview (BUG-PI-04/SOL-PI-04) ──────────────────────────────────

type ReviewType string

const (
	ReviewTypeUnspecified    ReviewType = ""
	ReviewTypeComment        ReviewType = "comment"
	ReviewTypeApprove        ReviewType = "approve"
	ReviewTypeRequestChanges ReviewType = "request_changes"
)

type ReviewComment struct {
	Path string
	Line int32
	Body string
}

type ReviewInput struct {
	Type     ReviewType
	Summary  string
	Comments []ReviewComment
}

type Review struct {
	ID          string
	ReviewerID  string
	State       ReviewType
	SubmittedAt string
	Comments    []ReviewComment
	URL         string
}
