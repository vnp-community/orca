package httpgateway

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"

	"github.com/go-chi/chi/v5"

	gatewaygrpc "github.com/stablyai/orca-go/services/api-gateway/internal/adapter/grpc"

	projectv1 "github.com/stablyai/orca-go/proto/gen/go/orca/project/v1"
)

// mountProjectRoutes wires REST->gRPC for project-service's full RPC
// surface, following the hand-written translation pattern established by
// mountUsageRoutes (usage_routes.go) — no grpc-gateway codegen, see that
// file's doc comment. tenant_id/user_id are always taken from the resolved
// Identity, never the request body (api-gateway.md §9).
func mountProjectRoutes(r chi.Router, client projectv1.ProjectServiceClient) {
	r.Route("/v1/projects", func(sub chi.Router) {
		sub.Post("/", handleCreateProject(client))
		sub.Get("/", handleListProjects(client))
		sub.Get("/{id}", handleGetProject(client))
		sub.Put("/{id}", handleUpdateProject(client))
		sub.Delete("/{id}", handleDeleteProject(client))

		sub.Post("/{id}/members", handleAddMember(client))
		sub.Put("/{id}/dev-server", handleRebindDevServer(client))

		sub.Post("/{id}/repos", handleAddRepo(client))
		sub.Get("/{id}/repos", handleListRepos(client))
		sub.Put("/{id}/repos/reorder", handleReorderRepos(client))
		sub.Delete("/{id}/repos/{repoId}", handleRemoveRepo(client))

		sub.Post("/{id}/worktrees", handleRecordWorktreeCreated(client))
		sub.Delete("/{id}/worktrees/{worktreeId}", handleRecordWorktreeRemoved(client))
		sub.Get("/{id}/worktrees", handleListWorktrees(client))
		sub.Put("/{id}/worktrees/{worktreeId}/activation", handleSetWorktreeActivation(client))
		sub.Put("/{id}/worktrees/{worktreeId}/rename", handleRenameWorktree(client))
	})

	r.Route("/v1/project-groups", func(sub chi.Router) {
		sub.Post("/", handleCreateProjectGroup(client))
		sub.Get("/", handleListProjectGroups(client))
		sub.Put("/{id}", handleUpdateProjectGroup(client))
		sub.Delete("/{id}", handleDeleteProjectGroup(client))
	})
}

// decodeJSONBody decodes r's JSON body into dst, writing a 400 response and
// returning false on failure — the shared shape every handler below uses so
// each stays a mechanical decode+call+respond.
func decodeJSONBody(w http.ResponseWriter, r *http.Request, dst any) bool {
	if r.Body == nil || r.ContentLength == 0 {
		return true
	}
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "invalid JSON body: "+err.Error())
		return false
	}
	return true
}

// --- Project ---

type createProjectRequestBody struct {
	Name          string `json:"name"`
	Description   string `json:"description"`
	DefaultBranch string `json:"default_branch"`
	Visibility    string `json:"visibility"`
}

func handleCreateProject(client projectv1.ProjectServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := identityFromContext(r.Context())

		var body createProjectRequestBody
		if !decodeJSONBody(w, r, &body) {
			return
		}

		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
		resp, err := client.CreateProject(ctx, &projectv1.CreateProjectRequest{
			TenantId:      identity.TenantID,
			Name:          body.Name,
			Description:   body.Description,
			DefaultBranch: body.DefaultBranch,
			Visibility:    body.Visibility,
		})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, resp.GetProject())
	}
}

func handleGetProject(client projectv1.ProjectServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := identityFromContext(r.Context())
		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)

		resp, err := client.GetProject(ctx, &projectv1.GetProjectRequest{Id: chi.URLParam(r, "id")})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, resp.GetProject())
	}
}

func handleListProjects(client projectv1.ProjectServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := identityFromContext(r.Context())
		q := r.URL.Query()

		pageSize, ok := parsePageSize(w, q)
		if !ok {
			return
		}

		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
		resp, err := client.ListProjects(ctx, &projectv1.ListProjectsRequest{
			TenantId:  identity.TenantID,
			PageToken: q.Get("page_token"),
			PageSize:  pageSize,
		})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, resp)
	}
}

type updateProjectRequestBody struct {
	Name          string `json:"name"`
	Description   string `json:"description"`
	DefaultBranch string `json:"default_branch"`
	Visibility    string `json:"visibility"`
}

func handleUpdateProject(client projectv1.ProjectServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := identityFromContext(r.Context())

		var body updateProjectRequestBody
		if !decodeJSONBody(w, r, &body) {
			return
		}

		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
		resp, err := client.UpdateProject(ctx, &projectv1.UpdateProjectRequest{
			ProjectId:     chi.URLParam(r, "id"),
			Name:          body.Name,
			Description:   body.Description,
			DefaultBranch: body.DefaultBranch,
			Visibility:    body.Visibility,
		})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, resp.GetProject())
	}
}

func handleDeleteProject(client projectv1.ProjectServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := identityFromContext(r.Context())
		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)

		_, err := client.DeleteProject(ctx, &projectv1.DeleteProjectRequest{ProjectId: chi.URLParam(r, "id")})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// --- Members / dev-server ---

type addMemberRequestBody struct {
	UserID string `json:"user_id"`
	Role   string `json:"role"`
}

func handleAddMember(client projectv1.ProjectServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := identityFromContext(r.Context())

		var body addMemberRequestBody
		if !decodeJSONBody(w, r, &body) {
			return
		}

		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
		_, err := client.AddMember(ctx, &projectv1.AddMemberRequest{
			ProjectId: chi.URLParam(r, "id"),
			UserId:    body.UserID,
			Role:      parseProjectRole(body.Role),
		})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

type rebindDevServerRequestBody struct {
	NewDevServerID string `json:"new_dev_server_id"`
}

func handleRebindDevServer(client projectv1.ProjectServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := identityFromContext(r.Context())

		var body rebindDevServerRequestBody
		if !decodeJSONBody(w, r, &body) {
			return
		}

		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
		resp, err := client.RebindDevServer(ctx, &projectv1.RebindDevServerRequest{
			ProjectId:      chi.URLParam(r, "id"),
			NewDevServerId: body.NewDevServerID,
		})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, resp.GetProject())
	}
}

// --- Repos ---

type addRepoRequestBody struct {
	URL         string `json:"url"`
	DisplayName string `json:"display_name"`
}

func handleAddRepo(client projectv1.ProjectServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := identityFromContext(r.Context())

		var body addRepoRequestBody
		if !decodeJSONBody(w, r, &body) {
			return
		}

		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
		resp, err := client.AddRepo(ctx, &projectv1.AddRepoRequest{
			ProjectId:   chi.URLParam(r, "id"),
			Url:         body.URL,
			DisplayName: body.DisplayName,
		})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, resp.GetRepo())
	}
}

func handleListRepos(client projectv1.ProjectServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := identityFromContext(r.Context())
		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)

		resp, err := client.ListRepos(ctx, &projectv1.ListReposRequest{ProjectId: chi.URLParam(r, "id")})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, resp)
	}
}

type reorderReposRequestBody struct {
	RepoIDsInOrder []string `json:"repo_ids_in_order"`
}

func handleReorderRepos(client projectv1.ProjectServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := identityFromContext(r.Context())

		var body reorderReposRequestBody
		if !decodeJSONBody(w, r, &body) {
			return
		}

		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
		_, err := client.ReorderRepos(ctx, &projectv1.ReorderReposRequest{
			ProjectId:      chi.URLParam(r, "id"),
			RepoIdsInOrder: body.RepoIDsInOrder,
		})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func handleRemoveRepo(client projectv1.ProjectServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := identityFromContext(r.Context())
		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)

		_, err := client.RemoveRepo(ctx, &projectv1.RemoveRepoRequest{RepoId: chi.URLParam(r, "repoId")})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// --- Worktrees ---

type recordWorktreeCreatedRequestBody struct {
	RepoID string `json:"repo_id"`
	Path   string `json:"path"`
	Branch string `json:"branch"`
}

func handleRecordWorktreeCreated(client projectv1.ProjectServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := identityFromContext(r.Context())

		var body recordWorktreeCreatedRequestBody
		if !decodeJSONBody(w, r, &body) {
			return
		}

		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
		resp, err := client.RecordWorktreeCreated(ctx, &projectv1.RecordWorktreeCreatedRequest{
			ProjectId: chi.URLParam(r, "id"),
			RepoId:    body.RepoID,
			Path:      body.Path,
			Branch:    body.Branch,
		})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, resp.GetWorktree())
	}
}

func handleRecordWorktreeRemoved(client projectv1.ProjectServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := identityFromContext(r.Context())
		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)

		_, err := client.RecordWorktreeRemoved(ctx, &projectv1.RecordWorktreeRemovedRequest{
			WorktreeId: chi.URLParam(r, "worktreeId"),
		})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func handleListWorktrees(client projectv1.ProjectServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := identityFromContext(r.Context())
		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)

		resp, err := client.ListWorktrees(ctx, &projectv1.ListWorktreesRequest{ProjectId: chi.URLParam(r, "id")})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, resp)
	}
}

type setWorktreeActivationRequestBody struct {
	Active bool `json:"active"`
}

func handleSetWorktreeActivation(client projectv1.ProjectServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := identityFromContext(r.Context())

		var body setWorktreeActivationRequestBody
		if !decodeJSONBody(w, r, &body) {
			return
		}

		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
		resp, err := client.SetWorktreeActivation(ctx, &projectv1.SetWorktreeActivationRequest{
			WorktreeId: chi.URLParam(r, "worktreeId"),
			Active:     body.Active,
		})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, resp.GetWorktree())
	}
}

type renameWorktreeRequestBody struct {
	Branch string `json:"branch"`
}

func handleRenameWorktree(client projectv1.ProjectServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := identityFromContext(r.Context())

		var body renameWorktreeRequestBody
		if !decodeJSONBody(w, r, &body) {
			return
		}

		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
		resp, err := client.RenameWorktree(ctx, &projectv1.RenameWorktreeRequest{
			WorktreeId: chi.URLParam(r, "worktreeId"),
			Branch:     body.Branch,
		})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, resp.GetWorktree())
	}
}

// --- Project groups ---

type createProjectGroupRequestBody struct {
	Name          string `json:"name"`
	ParentGroupID string `json:"parent_group_id"`
}

func handleCreateProjectGroup(client projectv1.ProjectServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := identityFromContext(r.Context())

		var body createProjectGroupRequestBody
		if !decodeJSONBody(w, r, &body) {
			return
		}

		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
		resp, err := client.CreateProjectGroup(ctx, &projectv1.CreateProjectGroupRequest{
			Name:          body.Name,
			ParentGroupId: body.ParentGroupID,
		})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, resp.GetGroup())
	}
}

type updateProjectGroupRequestBody struct {
	Name string `json:"name"`
}

func handleUpdateProjectGroup(client projectv1.ProjectServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := identityFromContext(r.Context())

		var body updateProjectGroupRequestBody
		if !decodeJSONBody(w, r, &body) {
			return
		}

		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
		resp, err := client.UpdateProjectGroup(ctx, &projectv1.UpdateProjectGroupRequest{
			GroupId: chi.URLParam(r, "id"),
			Name:    body.Name,
		})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, resp.GetGroup())
	}
}

func handleDeleteProjectGroup(client projectv1.ProjectServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := identityFromContext(r.Context())
		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)

		_, err := client.DeleteProjectGroup(ctx, &projectv1.DeleteProjectGroupRequest{GroupId: chi.URLParam(r, "id")})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func handleListProjectGroups(client projectv1.ProjectServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := identityFromContext(r.Context())
		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)

		resp, err := client.ListProjectGroups(ctx, &projectv1.ListProjectGroupsRequest{})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, resp)
	}
}

// --- shared parsing helpers ---

func parseProjectRole(v string) projectv1.ProjectRole {
	switch v {
	case "member":
		return projectv1.ProjectRole_PROJECT_ROLE_MEMBER
	case "owner":
		return projectv1.ProjectRole_PROJECT_ROLE_OWNER
	default:
		return projectv1.ProjectRole_PROJECT_ROLE_UNSPECIFIED
	}
}

func parsePageSize(w http.ResponseWriter, q url.Values) (int32, bool) {
	v := q.Get("page_size")
	if v == "" {
		return 0, true
	}
	n, err := strconv.ParseInt(v, 10, 32)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "page_size must be an integer")
		return 0, false
	}
	return int32(n), true
}
