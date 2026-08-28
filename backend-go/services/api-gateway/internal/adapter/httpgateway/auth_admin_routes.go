package httpgateway

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"google.golang.org/protobuf/types/known/timestamppb"

	gatewaygrpc "github.com/stablyai/orca-go/services/api-gateway/internal/adapter/grpc"

	authv1 "github.com/stablyai/orca-go/proto/gen/go/orca/auth/v1"
)

// mountAuthAdminRoutes wires the REST->gRPC reverse-proxy routes for
// auth-service's admin-console RPCs, following the same hand-written
// translation pattern as mountUsageRoutes (usage_routes.go) — see that
// function's doc comment for why this isn't grpc-gateway-generated.
//
// This is distinct from auth_routes.go's /auth/* routes: those are the
// unauthenticated login/session flow, mounted outside authMiddleware; these
// are authenticated admin-console operations mounted under /v1/auth,
// enforced admin-only server-side by auth-service's requireAdminActor OPA
// policy check — a non-admin caller's gRPC PermissionDenied is mapped to
// HTTP 403 by writeGRPCError, so no admin check is duplicated here.
func mountAuthAdminRoutes(r chi.Router, client authv1.AuthServiceClient) {
	r.Route("/v1/auth", func(sub chi.Router) {
		sub.Post("/users", handleCreateUser(client))
		sub.Get("/users", handleListUsers(client))
		sub.Put("/users/{id}/role", handleUpdateUserRole(client))
		sub.Post("/sessions/{id}/revoke", handleRevokeSession(client))
		sub.Get("/audit-log", handleQueryAuditLog(client))
	})
}

// createUserRequestBody is the REST request shape for POST /v1/auth/users.
// tenant_id is deliberately absent from the body — it comes from the
// validated Identity of the acting admin, never trusted from the request
// body, matching every existing handler in usage_routes.go/task_routes.go.
type createUserRequestBody struct {
	Email    string `json:"email"`
	Name     string `json:"name"`
	Role     string `json:"role"`
	Password string `json:"password"`
}

func handleCreateUser(client authv1.AuthServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := identityFromContext(r.Context())

		var body createUserRequestBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSONError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "invalid JSON body: "+err.Error())
			return
		}

		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
		resp, err := client.CreateUser(ctx, &authv1.CreateUserRequest{
			Email:    body.Email,
			Name:     body.Name,
			TenantId: identity.TenantID,
			Role:     parseRole(body.Role),
			Password: body.Password,
		})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, resp.GetUser())
	}
}

func handleListUsers(client authv1.AuthServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := identityFromContext(r.Context())
		q := r.URL.Query()

		var pageSize int32
		if v := q.Get("page_size"); v != "" {
			n, err := strconv.ParseInt(v, 10, 32)
			if err != nil {
				writeJSONError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "page_size must be an integer")
				return
			}
			pageSize = int32(n)
		}

		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
		resp, err := client.ListUsers(ctx, &authv1.ListUsersRequest{
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

// updateUserRoleRequestBody is the REST request shape for PUT
// /v1/auth/users/{id}/role.
type updateUserRoleRequestBody struct {
	Role string `json:"role"`
}

func handleUpdateUserRole(client authv1.AuthServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := identityFromContext(r.Context())
		id := chi.URLParam(r, "id")

		var body updateUserRoleRequestBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSONError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "invalid JSON body: "+err.Error())
			return
		}

		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
		resp, err := client.UpdateUserRole(ctx, &authv1.UpdateUserRoleRequest{
			UserId: id,
			Role:   parseRole(body.Role),
		})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, resp.GetUser())
	}
}

func handleRevokeSession(client authv1.AuthServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := identityFromContext(r.Context())
		id := chi.URLParam(r, "id")

		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
		_, err := client.RevokeSession(ctx, &authv1.RevokeSessionRequest{SessionToken: id})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		writeJSON(w, http.StatusNoContent, nil)
	}
}

func handleQueryAuditLog(client authv1.AuthServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := identityFromContext(r.Context())
		q := r.URL.Query()

		var pageSize int32
		if v := q.Get("page_size"); v != "" {
			n, err := strconv.ParseInt(v, 10, 32)
			if err != nil {
				writeJSONError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "page_size must be an integer")
				return
			}
			pageSize = int32(n)
		}

		req := &authv1.QueryAuditLogRequest{
			TenantId:  identity.TenantID,
			PageToken: q.Get("page_token"),
			PageSize:  pageSize,
			Action:    q.Get("action"),
			ActorId:   q.Get("userId"), // "userId" matches the spec's literal query param name
		}
		if v := q.Get("since"); v != "" {
			since, err := time.Parse(time.RFC3339, v)
			if err != nil {
				writeJSONError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "since must be an RFC3339 timestamp")
				return
			}
			req.Since = timestamppb.New(since)
		}
		if v := q.Get("to"); v != "" {
			to, err := time.Parse(time.RFC3339, v)
			if err != nil {
				writeJSONError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "to must be an RFC3339 timestamp")
				return
			}
			req.To = timestamppb.New(to)
		}

		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
		resp, err := client.QueryAuditLog(ctx, req)
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, resp)
	}
}

func parseRole(v string) authv1.Role {
	switch v {
	case "admin":
		return authv1.Role_ROLE_ADMIN
	case "user":
		return authv1.Role_ROLE_USER
	default:
		return authv1.Role_ROLE_UNSPECIFIED
	}
}
