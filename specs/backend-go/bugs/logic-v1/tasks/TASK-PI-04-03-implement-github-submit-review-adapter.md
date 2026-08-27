# TASK-PI-04-03: GitHub `SubmitReview` adapter

**From Solution:** SOL-PI-04
**Priority:** P1
**Service:** `scm-integration-service`
**File:** `backend-go/services/scm-integration-service/internal/adapter/github/client.go`
**Depends on:** TASK-PI-04-01, TASK-PI-04-02
**Status:** `[ ]` TODO

---

## Context

BUG-PI-04 confirms `POST /repos/:repo/pulls/:pr/reviews` is never called
anywhere in this adapter today. This is a direct, single-endpoint mapping —
GitHub's Reviews API natively supports Comment/Approve/Request-Changes with
per-line comments in one call.

## Changes to make

```go
type githubReviewComment struct {
	Path string `json:"path"`
	Line int32  `json:"line"`
	Body string `json:"body"`
}
type githubReviewPayload struct {
	Body     string                 `json:"body"`
	Event    string                 `json:"event"` // COMMENT | APPROVE | REQUEST_CHANGES
	Comments []githubReviewComment  `json:"comments,omitempty"`
}

func reviewTypeToGitHubEvent(t domain.ReviewType) string {
	switch t {
	case domain.ReviewTypeApprove:
		return "APPROVE"
	case domain.ReviewTypeRequestChanges:
		return "REQUEST_CHANGES"
	default:
		return "COMMENT"
	}
}

// SubmitReview — POST /repos/{repo}/pulls/{prNumber}/reviews.
func (c *Client) SubmitReview(ctx context.Context, cred Credential, repo string, prNumber int32, in domain.ReviewInput) (domain.Review, error) {
	comments := make([]githubReviewComment, 0, len(in.Comments))
	for _, cm := range in.Comments {
		comments = append(comments, githubReviewComment{Path: cm.Path, Line: cm.Line, Body: cm.Body})
	}
	payload := githubReviewPayload{Body: in.Summary, Event: reviewTypeToGitHubEvent(in.Type), Comments: comments}

	// ... build request to /repos/{repo}/pulls/{prNumber}/reviews with cred's
	// OAuth Bearer auth, matching every other write in this file (see
	// MergePullRequest's request-building code for the exact pattern) ...

	var resp struct {
		ID        int64  `json:"id"`
		User      struct{ Login string `json:"login"` } `json:"user"`
		State     string `json:"state"`
		SubmittedAt string `json:"submitted_at"`
		HTMLURL   string `json:"html_url"`
	}
	// ... decode response ...

	return domain.Review{
		ID: fmt.Sprintf("%d", resp.ID), ReviewerID: resp.User.Login,
		State: in.Type, SubmittedAt: resp.SubmittedAt, Comments: in.Comments, URL: resp.HTMLURL,
	}, nil
}
```

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/scm-integration-service/...
go vet ./services/scm-integration-service/...
```
