package httpgateway

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"

	gatewaygrpc "github.com/stablyai/orca-go/services/api-gateway/internal/adapter/grpc"

	scmintegrationv1 "github.com/stablyai/orca-go/proto/gen/go/orca/scmintegration/v1"
)

// mountSCMRoutes wires a real REST->gRPC reverse-proxy for
// scm-integration-service, following the usage_routes.go reference pattern:
// hand-written translation from REST to the real gRPC contract, tenant/user
// identity always sourced from identityFromContext (never the request
// body), gRPC errors mapped via writeGRPCError. Per
// backend-go/docs/execution-plan.md §15, scm-integration-service's Phase 3
// work is done (all 5 providers make real HTTP calls, plus a real OAuth 2.0
// web flow), so this replaces the former 501 stub for /v1/scm.
func mountSCMRoutes(r chi.Router, client scmintegrationv1.ScmIntegrationServiceClient) {
	r.Route("/v1/scm", func(sub chi.Router) {
		sub.Get("/issues", handleListSCMIssues(client))
		sub.Get("/issues/{number}/comments", handleListIssueComments(client))
		sub.Post("/pull-requests", handleCreatePullRequest(client))
		sub.Get("/pull-requests", handleListPullRequests(client))
		sub.Get("/rate-limit", handleGetRateLimitStatus(client))
		sub.Get("/auth-status", handleGetAuthStatus(client))
		sub.Post("/oauth/start", handleStartOAuthFlow(client))
		sub.Post("/oauth/complete", handleCompleteOAuthFlow(client))
		sub.Post("/oauth/revoke", handleRevokeAuth(client))
	})
}

func handleListSCMIssues(client scmintegrationv1.ScmIntegrationServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := identityFromContext(r.Context())
		q := r.URL.Query()

		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
		resp, err := client.ListIssues(ctx, &scmintegrationv1.ListIssuesRequest{
			TenantId: identity.TenantID,
			Provider: parseSCMProvider(q.Get("provider")),
			Repo:     q.Get("repo"),
			Filter: &scmintegrationv1.IssueFilter{
				State:     q.Get("state"),
				Assignee:  q.Get("assignee"),
				Labels:    q["label"], // repeatable query param, e.g. ?label=bug&label=p0
				Milestone: q.Get("milestone"),
			},
			ForceRefresh: q.Get("refresh") == "true",
		})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, resp)
	}
}

// handleListIssueComments serves GET /v1/scm/issues/{number}/comments —
// repo is a query param (not a path segment) since repo slugs contain "/"
// and this route's path only reserves {number}, matching /v1/scm/issues'
// own repo-as-query-param convention above.
func handleListIssueComments(client scmintegrationv1.ScmIntegrationServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := identityFromContext(r.Context())
		repo := r.URL.Query().Get("repo")
		number := chi.URLParam(r, "number")
		slug := fmt.Sprintf("%s#%s", repo, number)

		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
		resp, err := client.ListIssueCommentsBySlug(ctx, &scmintegrationv1.ListIssueCommentsBySlugRequest{
			TenantId: identity.TenantID, ItemSlug: slug,
		})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, resp)
	}
}

// createPullRequestRequestBody is the REST request shape for POST
// /v1/scm/pull-requests — tenant_id is deliberately absent: it comes from
// the validated Identity, never trusted from the request body, per
// api-gateway.md §9.
type createPullRequestRequestBody struct {
	Provider   string `json:"provider"`
	Repo       string `json:"repo"`
	Title      string `json:"title"`
	Body       string `json:"body"`
	HeadBranch string `json:"head_branch"`
	BaseBranch string `json:"base_branch"`
	RequestID  string `json:"request_id"`
}

func handleCreatePullRequest(client scmintegrationv1.ScmIntegrationServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := identityFromContext(r.Context())

		var body createPullRequestRequestBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSONError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "invalid JSON body: "+err.Error())
			return
		}

		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
		resp, err := client.CreatePullRequest(ctx, &scmintegrationv1.CreatePullRequestRequest{
			TenantId:   identity.TenantID,
			Provider:   parseSCMProvider(body.Provider),
			Repo:       body.Repo,
			Title:      body.Title,
			Body:       body.Body,
			HeadBranch: body.HeadBranch,
			BaseBranch: body.BaseBranch,
			RequestId:  body.RequestID,
		})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, resp.GetPullRequest())
	}
}

func handleListPullRequests(client scmintegrationv1.ScmIntegrationServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := identityFromContext(r.Context())
		q := r.URL.Query()

		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
		resp, err := client.ListPullRequests(ctx, &scmintegrationv1.ListPullRequestsRequest{
			TenantId: identity.TenantID,
			Provider: parseSCMProvider(q.Get("provider")),
			Repo:     q.Get("repo"),
		})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, resp)
	}
}

func handleGetRateLimitStatus(client scmintegrationv1.ScmIntegrationServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := identityFromContext(r.Context())
		q := r.URL.Query()

		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
		resp, err := client.GetRateLimitStatus(ctx, &scmintegrationv1.GetRateLimitStatusRequest{
			TenantId: identity.TenantID,
			Provider: parseSCMProvider(q.Get("provider")),
		})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, resp)
	}
}

func handleGetAuthStatus(client scmintegrationv1.ScmIntegrationServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := identityFromContext(r.Context())
		q := r.URL.Query()

		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
		resp, err := client.GetAuthStatus(ctx, &scmintegrationv1.GetAuthStatusRequest{
			TenantId: identity.TenantID,
			Provider: parseSCMProvider(q.Get("provider")),
		})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, resp)
	}
}

// startOAuthFlowRequestBody is the REST request shape for POST
// /v1/scm/oauth/start — tenant_id/user_id are deliberately absent: they
// come from the validated Identity, never trusted from the request body.
type startOAuthFlowRequestBody struct {
	Provider    string `json:"provider"`
	RedirectURI string `json:"redirect_uri"`
}

func handleStartOAuthFlow(client scmintegrationv1.ScmIntegrationServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := identityFromContext(r.Context())

		var body startOAuthFlowRequestBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSONError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "invalid JSON body: "+err.Error())
			return
		}

		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
		resp, err := client.StartOAuthFlow(ctx, &scmintegrationv1.StartOAuthFlowRequest{
			TenantId:    identity.TenantID,
			UserId:      identity.UserID,
			Provider:    parseSCMProvider(body.Provider),
			RedirectUri: body.RedirectURI,
		})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, resp)
	}
}

// completeOAuthFlowRequestBody is the REST request shape for POST
// /v1/scm/oauth/complete — tenant_id/user_id are deliberately absent: they
// come from the validated Identity, never trusted from the request body.
type completeOAuthFlowRequestBody struct {
	Provider    string `json:"provider"`
	Code        string `json:"code"`
	State       string `json:"state"`
	RedirectURI string `json:"redirect_uri"`
}

func handleCompleteOAuthFlow(client scmintegrationv1.ScmIntegrationServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := identityFromContext(r.Context())

		var body completeOAuthFlowRequestBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSONError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "invalid JSON body: "+err.Error())
			return
		}

		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
		resp, err := client.CompleteOAuthFlow(ctx, &scmintegrationv1.CompleteOAuthFlowRequest{
			TenantId:    identity.TenantID,
			UserId:      identity.UserID,
			Provider:    parseSCMProvider(body.Provider),
			Code:        body.Code,
			State:       body.State,
			RedirectUri: body.RedirectURI,
		})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, resp)
	}
}

// revokeAuthRequestBody is the REST request shape for POST
// /v1/scm/oauth/revoke — tenant_id is deliberately absent: it comes from
// the validated Identity, never trusted from the request body.
type revokeAuthRequestBody struct {
	Provider string `json:"provider"`
}

func handleRevokeAuth(client scmintegrationv1.ScmIntegrationServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := identityFromContext(r.Context())

		var body revokeAuthRequestBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSONError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "invalid JSON body: "+err.Error())
			return
		}

		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
		resp, err := client.RevokeAuth(ctx, &scmintegrationv1.RevokeAuthRequest{
			TenantId: identity.TenantID,
			Provider: parseSCMProvider(body.Provider),
		})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, resp)
	}
}

// parseReviewType maps a REST/WS-facing review-type string to
// scmintegrationv1.ReviewType — "" (omitted) resolves to
// REVIEW_TYPE_UNSPECIFIED, which SubmitReview's usecase then defaults to
// REQUEST_CHANGES server-side (BR-PI-11), so an unrecognized string here
// degrades the same safe way an omitted one does, rather than erroring.
func parseReviewType(v string) scmintegrationv1.ReviewType {
	switch v {
	case "comment":
		return scmintegrationv1.ReviewType_REVIEW_TYPE_COMMENT
	case "approve":
		return scmintegrationv1.ReviewType_REVIEW_TYPE_APPROVE
	case "request_changes":
		return scmintegrationv1.ReviewType_REVIEW_TYPE_REQUEST_CHANGES
	default:
		return scmintegrationv1.ReviewType_REVIEW_TYPE_UNSPECIFIED
	}
}

func parseSCMProvider(v string) scmintegrationv1.ScmProvider {
	switch v {
	case "github":
		return scmintegrationv1.ScmProvider_SCM_PROVIDER_GITHUB
	case "gitlab":
		return scmintegrationv1.ScmProvider_SCM_PROVIDER_GITLAB
	case "bitbucket":
		return scmintegrationv1.ScmProvider_SCM_PROVIDER_BITBUCKET
	case "azure_devops":
		return scmintegrationv1.ScmProvider_SCM_PROVIDER_AZURE_DEVOPS
	case "gitea":
		return scmintegrationv1.ScmProvider_SCM_PROVIDER_GITEA
	default:
		return scmintegrationv1.ScmProvider_SCM_PROVIDER_UNSPECIFIED
	}
}
