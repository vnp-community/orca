package httpgateway

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	gatewaygrpc "github.com/stablyai/orca-go/services/api-gateway/internal/adapter/grpc"

	annotationv1 "github.com/stablyai/orca-go/proto/gen/go/orca/annotation/v1"
	scmintegrationv1 "github.com/stablyai/orca-go/proto/gen/go/orca/scmintegration/v1"
)

// mountPRReviewRoutes wires POST /v1/scm/pull-requests/{prNumber}/reviews —
// api-gateway's own composition of annotation-service (read) +
// scm-integration-service (write), per 05-data-architecture.md's
// prescription that a multi-service view is computed at the edge, never by
// one service importing the other's domain model (SOL-PI-04).
func mountPRReviewRoutes(r chi.Router, scmClient scmintegrationv1.ScmIntegrationServiceClient, annotationClient annotationv1.AnnotationServiceClient) {
	r.Post("/v1/scm/pull-requests/{prNumber}/reviews", handleSubmitPullRequestReview(scmClient, annotationClient))
}

// submitReviewRequestBody is the REST request shape for POST
// /v1/scm/pull-requests/{prNumber}/reviews — prNumber itself comes from the
// path, not the body; tenant_id is deliberately absent, sourced from
// identityFromContext like every other handler in this package.
type submitReviewRequestBody struct {
	RepoID     string `json:"repoId"`
	Provider   string `json:"provider"`
	ReviewType string `json:"reviewType"` // "" | "comment" | "approve" | "request_changes"
	Summary    string `json:"summary"`
}

// handleSubmitPullRequestReview drains every page of
// AnnotationService.ListAnnotations for the repo, maps each to a
// ReviewComment, and calls ScmIntegrationService.SubmitReview. BR-PI-10 is
// checked here first (fail fast — no network call to scm-integration-service
// for an already-invalid input) and again inside submit_review.go
// (TASK-PI-04-02), belt-and-braces.
func handleSubmitPullRequestReview(scmClient scmintegrationv1.ScmIntegrationServiceClient, annotationClient annotationv1.AnnotationServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := identityFromContext(r.Context())

		prNumberRaw, err := strconv.ParseInt(chi.URLParam(r, "prNumber"), 10, 32)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "prNumber must be a valid integer")
			return
		}
		prNumber := int32(prNumberRaw)

		var body submitReviewRequestBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSONError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "invalid JSON body: "+err.Error())
			return
		}

		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)

		// Aggregation read: annotation-service is the source of truth for
		// the comments being submitted. Fully drains pagination before
		// composing — a partial comment set must never be silently
		// submitted.
		var allAnnotations []*annotationv1.Annotation
		pageToken := ""
		for {
			resp, err := annotationClient.ListAnnotations(ctx, &annotationv1.ListAnnotationsRequest{
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
			writeJSONError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "annotate at least one line before submitting a review")
			return
		}

		comments := make([]*scmintegrationv1.ReviewComment, 0, len(allAnnotations))
		for _, a := range allAnnotations {
			comments = append(comments, &scmintegrationv1.ReviewComment{
				Path: a.GetAnchor().GetFilePath(), Line: a.GetAnchor().GetLine(), Body: a.GetContent(),
			})
		}

		resp, err := scmClient.SubmitReview(ctx, &scmintegrationv1.SubmitReviewRequest{
			TenantId: identity.TenantID, Provider: parseSCMProvider(body.Provider), Repo: body.RepoID,
			PrNumber: prNumber, ReviewType: parseReviewType(body.ReviewType), // UNSPECIFIED if omitted — BR-PI-11 resolved server-side too
			SummaryBody: body.Summary, Comments: comments,
		})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, resp)
	}
}
