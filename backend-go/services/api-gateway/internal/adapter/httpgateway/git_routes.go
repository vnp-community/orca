package httpgateway

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	gatewaygrpc "github.com/stablyai/orca-go/services/api-gateway/internal/adapter/grpc"

	gitgatewayv1 "github.com/stablyai/orca-go/proto/gen/go/orca/gitgateway/v1"
)

// mountGitRoutes wires REST->gRPC for git-gateway-service, following the
// same hand-written reverse-proxy pattern as mountUsageRoutes
// (usage_routes.go) — see that file's doc comment for the production
// grpc-gateway-codegen caveat. git-gateway-service's proto messages carry
// no tenant_id/user_id fields at all (it's a stateless per-worktree
// dispatcher); identity still travels out-of-band via
// gatewaygrpc.AttachIdentity's outbound metadata, same as every other real
// route in this package, so nothing in the JSON body can spoof it.
func mountGitRoutes(r chi.Router, client gitgatewayv1.GitGatewayServiceClient) {
	r.Route("/v1/git", func(sub chi.Router) {
		sub.Get("/status", handleGetGitStatus(client))
		sub.Get("/diff", handleGetGitDiff(client))
		sub.Post("/commit", handleGitCommit(client))
		sub.Post("/push", handleGitPush(client))
		sub.Post("/pull", handleGitPull(client))
		sub.Post("/commit-message", handleGenerateGitCommitMessage(client))
	})
	// New: top-level, not nested under /v1/git — the resource is a
	// worktree, not a git operation, matching project_routes.go's existing
	// /v1/projects/{id}/worktrees naming for the bookkeeping-only view.
	r.Post("/v1/worktrees", handleCreateWorktree(client))
}

// createWorktreeRequestBody is the REST request shape for POST
// /v1/worktrees — see gitCommitRequestBody's doc comment on why
// tenant_id/user_id are absent.
type createWorktreeRequestBody struct {
	ProjectID      string `json:"project_id"`
	RepoID         string `json:"repo_id"`
	Branch         string `json:"branch"`
	BaseRef        string `json:"base_ref"`
	IdempotencyKey string `json:"idempotency_key"`
}

func handleCreateWorktree(client gitgatewayv1.GitGatewayServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := identityFromContext(r.Context())

		var body createWorktreeRequestBody
		if !decodeJSONBody(w, r, &body) {
			return
		}
		if body.ProjectID == "" || body.RepoID == "" || body.Branch == "" {
			writeJSONError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "project_id, repo_id, and branch are required")
			return
		}

		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
		resp, err := client.CreateWorktree(ctx, &gitgatewayv1.CreateWorktreeRequest{
			ProjectId: body.ProjectID, RepoId: body.RepoID, Branch: body.Branch,
			BaseRef: body.BaseRef, IdempotencyKey: &body.IdempotencyKey,
		})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, resp)
	}
}

func handleGetGitStatus(client gitgatewayv1.GitGatewayServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := identityFromContext(r.Context())

		worktreeID := r.URL.Query().Get("worktree_id")
		if worktreeID == "" {
			writeJSONError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "worktree_id is required")
			return
		}

		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
		resp, err := client.GetStatus(ctx, &gitgatewayv1.GetStatusRequest{WorktreeId: worktreeID})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, resp)
	}
}

func handleGetGitDiff(client gitgatewayv1.GitGatewayServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := identityFromContext(r.Context())
		q := r.URL.Query()

		worktreeID := q.Get("worktree_id")
		if worktreeID == "" {
			writeJSONError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "worktree_id is required")
			return
		}

		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
		resp, err := client.GetDiff(ctx, &gitgatewayv1.GetDiffRequest{
			WorktreeId: worktreeID,
			Staged:     q.Get("staged") == "true",
		})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, resp)
	}
}

// gitCommitRequestBody is the REST request shape for POST /v1/git/commit —
// tenant_id/user_id are deliberately absent: identity travels via
// gatewaygrpc.AttachIdentity's outbound metadata, never trusted from the
// request body, per api-gateway.md §9.
type gitCommitRequestBody struct {
	WorktreeID string   `json:"worktree_id"`
	Message    string   `json:"message"`
	Paths      []string `json:"paths"`
}

func handleGitCommit(client gitgatewayv1.GitGatewayServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := identityFromContext(r.Context())

		var body gitCommitRequestBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSONError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "invalid JSON body: "+err.Error())
			return
		}
		if body.WorktreeID == "" {
			writeJSONError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "worktree_id is required")
			return
		}

		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
		resp, err := client.Commit(ctx, &gitgatewayv1.CommitRequest{
			WorktreeId: body.WorktreeID,
			Message:    body.Message,
			Paths:      body.Paths,
		})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, resp)
	}
}

// gitPushRequestBody is the REST request shape for POST /v1/git/push — see
// gitCommitRequestBody's doc comment on why tenant_id/user_id are absent.
type gitPushRequestBody struct {
	WorktreeID string `json:"worktree_id"`
	Remote     string `json:"remote"`
	Branch     string `json:"branch"`
}

func handleGitPush(client gitgatewayv1.GitGatewayServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := identityFromContext(r.Context())

		var body gitPushRequestBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSONError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "invalid JSON body: "+err.Error())
			return
		}
		if body.WorktreeID == "" {
			writeJSONError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "worktree_id is required")
			return
		}

		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
		resp, err := client.Push(ctx, &gitgatewayv1.PushRequest{
			WorktreeId: body.WorktreeID,
			Remote:     body.Remote,
			Branch:     body.Branch,
		})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, resp)
	}
}

// gitPullRequestBody is the REST request shape for POST /v1/git/pull — see
// gitCommitRequestBody's doc comment on why tenant_id/user_id are absent.
type gitPullRequestBody struct {
	WorktreeID string `json:"worktree_id"`
}

func handleGitPull(client gitgatewayv1.GitGatewayServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := identityFromContext(r.Context())

		var body gitPullRequestBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSONError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "invalid JSON body: "+err.Error())
			return
		}
		if body.WorktreeID == "" {
			writeJSONError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "worktree_id is required")
			return
		}

		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
		resp, err := client.Pull(ctx, &gitgatewayv1.PullRequest{WorktreeId: body.WorktreeID})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, resp)
	}
}

// gitCommitMessageRequestBody is the REST request shape for POST
// /v1/git/commit-message — see gitCommitRequestBody's doc comment on why
// tenant_id/user_id are absent.
type gitCommitMessageRequestBody struct {
	WorktreeID string `json:"worktree_id"`
}

// handleGenerateGitCommitMessage wires the REST route through to
// GenerateCommitMessage even though git-gateway-service currently returns
// codes.Unimplemented for it server-side — writeGRPCError correctly
// surfaces that as a 501 once it hits this handler; making the RPC actually
// generate a message is that service's gap, not this route's.
func handleGenerateGitCommitMessage(client gitgatewayv1.GitGatewayServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := identityFromContext(r.Context())

		var body gitCommitMessageRequestBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSONError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "invalid JSON body: "+err.Error())
			return
		}
		if body.WorktreeID == "" {
			writeJSONError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "worktree_id is required")
			return
		}

		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
		resp, err := client.GenerateCommitMessage(ctx, &gitgatewayv1.GenerateCommitMessageRequest{WorktreeId: body.WorktreeID})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, resp)
	}
}
