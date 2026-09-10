# TASK-PI-04-04: GitLab `SubmitReview` adapter (composed discussions + approve/note)

**From Solution:** SOL-PI-04
**Priority:** P1
**Service:** `scm-integration-service`
**File:** `backend-go/services/scm-integration-service/internal/adapter/gitlab/client.go`
**Depends on:** TASK-PI-04-01, TASK-PI-04-02
**Status:** `[x] DONE — composed discussions+approve/note; failure on any discussion call stops before approve/note.`

---

## Context

GitLab has no single "submit a review" endpoint like GitHub's — this
solution composes per-line discussions plus a separate approve/note call,
documented as an intentional per-provider difference
(`scm-integration-service.md` §6's "no shared base class" posture). A
failure partway through (e.g. the second discussion call) must not silently
continue to approve.

## Changes to make

```go
// SubmitReview composes GitLab's discussions API (one per comment) plus a
// separate approve/note call — GitLab has no single atomic review endpoint
// like GitHub's. A failure on any discussion call stops immediately and
// does NOT proceed to approve/note — partial failure is a real outcome
// here, not swallowed.
func (c *Client) SubmitReview(ctx context.Context, cred Credential, repo string, mrIID int32, in domain.ReviewInput) (domain.Review, error) {
	for _, comment := range in.Comments {
		if err := c.createDiscussion(ctx, cred, repo, mrIID, comment); err != nil {
			return domain.Review{}, fmt.Errorf("gitlab: create discussion for %s:%d: %w", comment.Path, comment.Line, err)
		}
	}
	switch in.Type {
	case domain.ReviewTypeApprove:
		return c.approveMR(ctx, cred, repo, mrIID, in.Summary)
	case domain.ReviewTypeRequestChanges, domain.ReviewTypeComment:
		// GitLab has no native "request changes" state — recorded as a
		// summary note, a documented divergence from GitHub's semantics.
		return c.noteMR(ctx, cred, repo, mrIID, in.Summary, in.Type)
	default:
		return domain.Review{}, domain.ErrCapabilityUnsupported
	}
}

// createDiscussion — POST /projects/:id/merge_requests/:iid/discussions
// with a position hash addressing comment.Path/comment.Line.
func (c *Client) createDiscussion(ctx context.Context, cred Credential, repo string, mrIID int32, comment domain.ReviewComment) error {
	// ... build request, matching this file's existing note/discussion helpers ...
	return nil
}

// approveMR — POST /projects/:id/merge_requests/:iid/approve.
func (c *Client) approveMR(ctx context.Context, cred Credential, repo string, mrIID int32, summary string) (domain.Review, error) {
	// ... approve call, then build domain.Review with State: ReviewTypeApprove ...
	return domain.Review{}, nil
}

// noteMR — POST /projects/:id/merge_requests/:iid/notes with summary as the
// note body; State reflects the caller's requested type (Comment or
// RequestChanges) even though GitLab has no matching native state.
func (c *Client) noteMR(ctx context.Context, cred Credential, repo string, mrIID int32, summary string, reviewType domain.ReviewType) (domain.Review, error) {
	// ... note call, then build domain.Review with State: reviewType ...
	return domain.Review{}, nil
}
```

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/scm-integration-service/...
go vet ./services/scm-integration-service/...
```
