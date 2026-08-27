# TASK-PI-04-05: `POST /v1/scm/pull-requests/{prNumber}/reviews` — annotation aggregation route

**From Solution:** SOL-PI-04
**Priority:** P0
**Service:** `api-gateway`
**File:** `backend-go/services/api-gateway/internal/adapter/httpgateway/pr_review_routes.go` (new)
**Depends on:** TASK-PI-04-02
**Status:** `[x] DONE — pr_review_routes.go new: POST /v1/scm/pull-requests/{prNumber}/reviews drains all ListAnnotations pages, BR-PI-10 fail-fast before any scm-integration-service call, maps to ReviewComment, calls SubmitReview. Mounted in router.go guarded on both SCMClient and AnnotationClient.`

---

## Context

Neither `AnnotationService` nor `ScmIntegrationService` should import the
other's domain model (`02-microservices-decomposition.md`'s "logical FKs"
principle). `api-gateway` composes: read `AnnotationService.ListAnnotations`,
map to `ReviewComment`, call `ScmIntegrationService.SubmitReview` — the
write-side mirror of the read-aggregation pattern this codebase already uses
for every other multi-service view (`05-data-architecture.md:116-119`).
BR-PI-10 is checked here first (fail fast, no network call to
`scm-integration-service` for an already-invalid input) and again inside
`submit_review.go` (TASK-PI-04-02).

## Changes to make

```go
// pr_review_routes.go — new
package httpgateway

type submitReviewRequestBody struct {
	RepoID     string `json:"repoId"`
	Provider   string `json:"provider"`
	PRNumber   int32  `json:"prNumber"`
	ReviewType string `json:"reviewType"` // "" | "comment" | "approve" | "request_changes"
	Summary    string `json:"summary"`
}

// SubmitPullRequestReview — POST /v1/scm/pull-requests/{prNumber}/reviews.
func (h *Handler) SubmitPullRequestReview(w http.ResponseWriter, r *http.Request) {
	identity := identityFromContext(r.Context())
	var body submitReviewRequestBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Aggregation read: annotation-service is the source of truth for the
	// comments being submitted. Fully drains pagination before composing —
	// a partial comment set must never be silently submitted.
	var allAnnotations []*annotationv1.Annotation
	pageToken := ""
	for {
		resp, err := h.annotationClient.ListAnnotations(r.Context(), &annotationv1.ListAnnotationsRequest{
			RepoId: body.RepoID, PageToken: pageToken,
		})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		allAnnotations = append(allAnnotations, resp.GetAnnotations()...)
		if pageToken = resp.GetNextPageToken(); pageToken == "" {
			break
		}
	}
	if len(allAnnotations) == 0 { // BR-PI-10, checked here first
		http.Error(w, "annotate at least one line before submitting a review", http.StatusBadRequest)
		return
	}

	comments := make([]*scmintegrationv1.ReviewComment, 0, len(allAnnotations))
	for _, a := range allAnnotations {
		comments = append(comments, &scmintegrationv1.ReviewComment{
			Path: a.GetAnchor().GetFilePath(), Line: a.GetAnchor().GetLine(), Body: a.GetContent(),
		})
	}

	resp, err := h.scmClient.SubmitReview(r.Context(), &scmintegrationv1.SubmitReviewRequest{
		TenantId: identity.TenantID, Provider: parseSCMProvider(body.Provider), Repo: body.RepoID,
		PrNumber: body.PRNumber, ReviewType: parseReviewType(body.ReviewType), // UNSPECIFIED if omitted — BR-PI-11 resolved server-side too
		SummaryBody: body.Summary, Comments: comments,
	})
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}
```

Mount this route in this package's router-registration function next to
`/v1/scm/pull-requests`.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/api-gateway/...
go vet ./services/api-gateway/...
```
