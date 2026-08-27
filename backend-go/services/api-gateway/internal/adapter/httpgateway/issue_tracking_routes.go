package httpgateway

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	gatewaygrpc "github.com/stablyai/orca-go/services/api-gateway/internal/adapter/grpc"

	issuetrackingv1 "github.com/stablyai/orca-go/proto/gen/go/orca/issuetracking/v1"
)

// mountIssueTrackingRoutes wires a real REST->gRPC reverse-proxy for
// issue-tracking-service, following the usage_routes.go reference pattern:
// hand-written translation from REST to the real gRPC contract, tenant/user
// identity always sourced from identityFromContext (never the request
// body), gRPC errors mapped via writeGRPCError.
func mountIssueTrackingRoutes(r chi.Router, client issuetrackingv1.IssueTrackingServiceClient) {
	r.Route("/v1/issues", func(sub chi.Router) {
		sub.Get("/", handleListIssues(client))
		sub.Post("/", handleCreateIssue(client))
		sub.Post("/link", handleLinkIssue(client))
	})
}

func handleListIssues(client issuetrackingv1.IssueTrackingServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := identityFromContext(r.Context())
		q := r.URL.Query()

		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
		resp, err := client.ListIssues(ctx, &issuetrackingv1.ListIssuesRequest{
			TenantId:   identity.TenantID,
			Provider:   parseIssueProvider(q.Get("provider")),
			ProjectKey: q.Get("project_key"),
		})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, resp)
	}
}

// createIssueRequestBody is the REST request shape for POST /v1/issues —
// tenant_id is deliberately absent: it comes from the validated Identity,
// never trusted from the request body, per api-gateway.md §9.
type createIssueRequestBody struct {
	Provider    string `json:"provider"`
	ProjectKey  string `json:"project_key"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

func handleCreateIssue(client issuetrackingv1.IssueTrackingServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := identityFromContext(r.Context())

		var body createIssueRequestBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSONError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "invalid JSON body: "+err.Error())
			return
		}

		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
		resp, err := client.CreateIssue(ctx, &issuetrackingv1.CreateIssueRequest{
			TenantId:    identity.TenantID,
			Provider:    parseIssueProvider(body.Provider),
			ProjectKey:  body.ProjectKey,
			Title:       body.Title,
			Description: body.Description,
		})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, resp.GetIssue())
	}
}

// linkIssueRequestBody is the REST request shape for POST /v1/issues/link.
// LinkIssueRequest carries no tenant/user field (it just references
// issue_id/task_id, both already tenant-scoped upstream), so there is
// nothing here to override from identity — still routed through
// gatewaygrpc.AttachIdentity like every other outbound call.
type linkIssueRequestBody struct {
	IssueID string `json:"issue_id"`
	TaskID  string `json:"task_id"`
}

func handleLinkIssue(client issuetrackingv1.IssueTrackingServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := identityFromContext(r.Context())

		var body linkIssueRequestBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSONError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "invalid JSON body: "+err.Error())
			return
		}
		if body.IssueID == "" || body.TaskID == "" {
			writeJSONError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "issue_id and task_id are required")
			return
		}

		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
		resp, err := client.LinkIssue(ctx, &issuetrackingv1.LinkIssueRequest{
			IssueId: body.IssueID,
			TaskId:  body.TaskID,
		})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, resp)
	}
}

func parseIssueProvider(v string) issuetrackingv1.IssueProvider {
	switch v {
	case "jira":
		return issuetrackingv1.IssueProvider_ISSUE_PROVIDER_JIRA
	case "linear":
		return issuetrackingv1.IssueProvider_ISSUE_PROVIDER_LINEAR
	default:
		return issuetrackingv1.IssueProvider_ISSUE_PROVIDER_UNSPECIFIED
	}
}
