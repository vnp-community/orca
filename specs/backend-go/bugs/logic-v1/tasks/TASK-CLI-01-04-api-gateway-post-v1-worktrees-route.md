# TASK-CLI-01-04: `api-gateway` — `POST /v1/worktrees` REST route

**From Solution:** SOL-CLI-01
**Priority:** P1
**Service:** `api-gateway`
**File:** `backend-go/services/api-gateway/internal/adapter/httpgateway/git_routes.go`
**Depends on:** TASK-CLI-01-01 (proto field), TASK-CLI-01-03 (usecase forwards it)
**Status:** [x] DONE — added `POST /v1/worktrees` handler + 3 new tests in `git_routes_test.go`; `go build ./services/api-gateway/...` and `go test ./services/api-gateway/internal/adapter/httpgateway/... -run TestMountGitRoutes -v` all pass.

---

## Context

`project_routes.go:37`'s `POST /{id}/worktrees` only calls `RecordWorktreeCreated` (bookkeeping). No REST route performs the actual `git worktree add` — that's only reachable over `wscompat`'s stateful WS JSON-RPC today, "not something a shell script can `curl`" per BUG-CLI-01. This adds the sibling route, following `git_routes.go`'s exact existing pattern (chi router, `gatewaygrpc.AttachIdentity`, `writeGRPCError`, `decodeJSONBody`).

## Changes to make

In `backend-go/services/api-gateway/internal/adapter/httpgateway/git_routes.go`:

```go
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
```

`decodeJSONBody` already exists in `project_routes.go:55` (same package) — reuse it rather than the inline `json.NewDecoder` pattern the other handlers in this file use, since it already writes the 400 response on decode failure.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/api-gateway/...
go test ./services/api-gateway/internal/adapter/httpgateway/... -run TestGitRoutes -v
```

Expected new cases in `git_routes_test.go`: happy path returns `201` + the RPC response body; missing `branch` returns `400 INVALID_ARGUMENT` without calling the fake `GitGatewayServiceClient` (assert zero calls); `AttachIdentity` is invoked before the call (identity never trusted from the body).
