package httpgateway

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	gatewaygrpc "github.com/stablyai/orca-go/services/api-gateway/internal/adapter/grpc"

	taskv1 "github.com/stablyai/orca-go/proto/gen/go/orca/task/v1"
)

// mountTaskRoutes wires REST->gRPC reverse-proxy routes for task-service,
// following the same hand-written translation pattern as
// mountUsageRoutes (usage_routes.go) — see that function's doc comment for
// why this isn't grpc-gateway-generated.
func mountTaskRoutes(r chi.Router, client taskv1.TaskServiceClient) {
	r.Route("/v1/tasks", func(sub chi.Router) {
		sub.Post("/", handleCreateTask(client))
		sub.Get("/{id}", handleGetTask(client))
		sub.Post("/{id}/edges", handleAddEdge(client))
		sub.Post("/{id}/grants", handleGrant(client))
		sub.Get("/{id}/permission", handleResolvePermission(client))
		sub.Post("/{id}/execute", handleExecuteTask(client))
		sub.Get("/{id}/active-executions", handleHasActiveExecutions(client))
		sub.Get("/{id}/subtree", handleGetSubtree(client))
		sub.Post("/{id}/progress:recalculate", handleRecalculateProgress(client))
		sub.Post("/{id}/comments", handleAddComment(client))
		sub.Get("/{id}/comments", handleListComments(client))
	})
}

// createTaskRequestBody is the REST request shape for POST /v1/tasks —
// tenant_id is deliberately absent: it comes from the validated Identity,
// never trusted from the request body, matching every existing handler in
// usage_routes.go and auth_routes.go.
type createTaskRequestBody struct {
	Title     string `json:"title"`
	ParentID  string `json:"parent_id"`
	ProjectID string `json:"project_id"`
}

func handleCreateTask(client taskv1.TaskServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := identityFromContext(r.Context())

		var body createTaskRequestBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSONError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "invalid JSON body: "+err.Error())
			return
		}

		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
		resp, err := client.CreateTask(ctx, &taskv1.CreateTaskRequest{
			TenantId:  identity.TenantID,
			Title:     body.Title,
			ParentId:  body.ParentID,
			ProjectId: body.ProjectID,
		})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, resp.GetTask())
	}
}

func handleGetTask(client taskv1.TaskServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := identityFromContext(r.Context())
		id := chi.URLParam(r, "id")

		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
		resp, err := client.GetTask(ctx, &taskv1.GetTaskRequest{Id: id})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, resp.GetTask())
	}
}

// addEdgeRequestBody is the REST request shape for POST
// /v1/tasks/{id}/edges. The path's {id} is the edge's from_task_id.
type addEdgeRequestBody struct {
	ToTaskID string `json:"to_task_id"`
	Type     string `json:"type"`
}

func handleAddEdge(client taskv1.TaskServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := identityFromContext(r.Context())
		id := chi.URLParam(r, "id")

		var body addEdgeRequestBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSONError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "invalid JSON body: "+err.Error())
			return
		}

		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
		_, err := client.AddEdge(ctx, &taskv1.AddEdgeRequest{
			FromTaskId: id,
			ToTaskId:   body.ToTaskID,
			Type:       parseEdgeType(body.Type),
		})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		writeJSON(w, http.StatusNoContent, nil)
	}
}

func parseEdgeType(v string) taskv1.EdgeType {
	switch v {
	case "parent_child":
		return taskv1.EdgeType_EDGE_TYPE_PARENT_CHILD
	case "depends_on":
		return taskv1.EdgeType_EDGE_TYPE_DEPENDS_ON
	default:
		return taskv1.EdgeType_EDGE_TYPE_UNSPECIFIED
	}
}

// grantRequestBody is the REST request shape for POST
// /v1/tasks/{id}/grants.
type grantRequestBody struct {
	SubjectID string `json:"subject_id"`
	Level     string `json:"level"`
	ApplyTree bool   `json:"apply_tree"`
}

func handleGrant(client taskv1.TaskServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := identityFromContext(r.Context())
		id := chi.URLParam(r, "id")

		var body grantRequestBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSONError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "invalid JSON body: "+err.Error())
			return
		}

		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
		_, err := client.Grant(ctx, &taskv1.GrantRequest{
			TaskId:    id,
			SubjectId: body.SubjectID,
			Level:     parseGrantLevel(body.Level),
			ApplyTree: body.ApplyTree,
		})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		writeJSON(w, http.StatusNoContent, nil)
	}
}

func parseGrantLevel(v string) taskv1.GrantLevel {
	switch v {
	case "owner":
		return taskv1.GrantLevel_GRANT_LEVEL_OWNER
	case "admin":
		return taskv1.GrantLevel_GRANT_LEVEL_ADMIN
	case "user":
		return taskv1.GrantLevel_GRANT_LEVEL_USER
	case "team":
		return taskv1.GrantLevel_GRANT_LEVEL_TEAM
	case "company":
		return taskv1.GrantLevel_GRANT_LEVEL_COMPANY
	default:
		return taskv1.GrantLevel_GRANT_LEVEL_UNSPECIFIED
	}
}

// resolvePermissionResponseBody is the REST response shape for GET
// /v1/tasks/{id}/permission.
type resolvePermissionResponseBody struct {
	EffectiveLevel string `json:"effective_level"`
}

func handleResolvePermission(client taskv1.TaskServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := identityFromContext(r.Context())
		id := chi.URLParam(r, "id")

		// user_id is deliberately taken from the validated Identity, not a
		// query param — ResolvePermission answers "what can the caller do",
		// never "what can whoever they name in the query do".
		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
		resp, err := client.ResolvePermission(ctx, &taskv1.ResolvePermissionRequest{
			TaskId: id,
			UserId: identity.UserID,
		})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, resolvePermissionResponseBody{
			EffectiveLevel: resp.GetEffectiveLevel().String(),
		})
	}
}

// executeTaskRequestBody is the REST request shape for POST
// /v1/tasks/{id}/execute.
type executeTaskRequestBody struct {
	RequestID string `json:"request_id"`
}

func handleExecuteTask(client taskv1.TaskServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := identityFromContext(r.Context())
		id := chi.URLParam(r, "id")

		var body executeTaskRequestBody
		if r.ContentLength != 0 {
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				writeJSONError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "invalid JSON body: "+err.Error())
				return
			}
		}

		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
		resp, err := client.Execute(ctx, &taskv1.TaskServiceExecuteRequest{
			TaskId:    id,
			RequestId: body.RequestID,
		})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, resp)
	}
}

// hasActiveExecutionsResponseBody is the REST response shape for GET
// /v1/tasks/{id}/active-executions. Despite the path being nested under a
// task id, HasActiveExecutions is answered per-project (see task.proto's
// doc comment on the RPC) — the {id} here is treated as a project id.
type hasActiveExecutionsResponseBody struct {
	HasActive bool `json:"has_active"`
}

func handleHasActiveExecutions(client taskv1.TaskServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := identityFromContext(r.Context())
		id := chi.URLParam(r, "id")

		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
		resp, err := client.HasActiveExecutions(ctx, &taskv1.HasActiveExecutionsRequest{ProjectId: id})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, hasActiveExecutionsResponseBody{HasActive: resp.GetHasActive()})
	}
}

// handleGetSubtree serves GET /v1/tasks/{id}/subtree — {id} is the subtree
// root, per GetSubtree's usecase doc comment (per-node access filter, not a
// whole-branch cut).
func handleGetSubtree(client taskv1.TaskServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := identityFromContext(r.Context())
		id := chi.URLParam(r, "id")

		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
		resp, err := client.GetSubtree(ctx, &taskv1.GetSubtreeRequest{RootId: id})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, resp)
	}
}

// recalculateProgressResponseBody is the REST response shape for POST
// /v1/tasks/{id}/progress:recalculate.
type recalculateProgressResponseBody struct {
	ProgressPercent int32 `json:"progress_percent"`
}

func handleRecalculateProgress(client taskv1.TaskServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := identityFromContext(r.Context())
		id := chi.URLParam(r, "id")

		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
		resp, err := client.RecalculateProgress(ctx, &taskv1.RecalculateProgressRequest{RootId: id})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, recalculateProgressResponseBody{ProgressPercent: resp.GetProgressPercent()})
	}
}

// addCommentRequestBody is the REST request shape for POST
// /v1/tasks/{id}/comments.
type addCommentRequestBody struct {
	Content string `json:"content"`
}

func handleAddComment(client taskv1.TaskServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := identityFromContext(r.Context())
		id := chi.URLParam(r, "id")

		var body addCommentRequestBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSONError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "invalid JSON body: "+err.Error())
			return
		}

		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
		resp, err := client.AddComment(ctx, &taskv1.AddCommentRequest{TaskId: id, Content: body.Content})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, resp)
	}
}

// handleListComments serves GET /v1/tasks/{id}/comments, paginated via the
// same page_token/page_size query params every other list endpoint in this
// gateway uses.
func handleListComments(client taskv1.TaskServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := identityFromContext(r.Context())
		id := chi.URLParam(r, "id")

		var pageSize int32
		if v := r.URL.Query().Get("page_size"); v != "" {
			if n, err := strconv.ParseInt(v, 10, 32); err == nil {
				pageSize = int32(n)
			}
		}

		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
		resp, err := client.ListComments(ctx, &taskv1.ListCommentsRequest{
			TaskId:    id,
			PageToken: r.URL.Query().Get("page_token"),
			PageSize:  pageSize,
		})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, resp)
	}
}
