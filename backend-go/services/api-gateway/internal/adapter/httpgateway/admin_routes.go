package httpgateway

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	gatewaygrpc "github.com/stablyai/orca-go/services/api-gateway/internal/adapter/grpc"

	authv1 "github.com/stablyai/orca-go/proto/gen/go/orca/auth/v1"
)

// mountAdminRoutes wires the REST->gRPC reverse-proxy routes for
// auth-service's admin-console RPCs at the literal paths
// specs/frontend/api/http-endpoints.md documents under `/admin/api/*` —
// distinct from auth_admin_routes.go's `/v1/auth/*` mount of the same
// underlying RPCs (kept for backward compat / API-versioning purposes; see
// TestAdminRoutes_AuditMatchesV1AuthAuditLog in admin_routes_test.go for the
// contract-parity guard between the two surfaces). Every handler here
// follows handleCreateUser's exact shape in auth_admin_routes.go: identity
// from context, gatewaygrpc.AttachIdentity, writeGRPCError on failure.
// Admin-only enforcement happens server-side in auth-service's
// requireAdminActor OPA check, not duplicated here — a non-admin caller's
// gRPC PermissionDenied is mapped to HTTP 403 by writeGRPCError.
func mountAdminRoutes(r chi.Router, client authv1.AuthServiceClient) {
	r.Route("/admin/api", func(sub chi.Router) {
		sub.Get("/stats", handleAdminStats(client))
		sub.Get("/users", handleListUsers(client))             // reused from auth_admin_routes.go
		sub.Post("/users", handleCreateUser(client))           // reused from auth_admin_routes.go
		sub.Patch("/users/{id}", handleUpdateUserRole(client)) // role-only for now — see doc comment below
		sub.Delete("/users/{id}", handleDeactivateUser(client))
		sub.Get("/sessions", handleListAllSessions(client))
		sub.Delete("/sessions/{sessionId}", handleRevokeSession(client)) // reused from auth_admin_routes.go
		sub.Delete("/users/{userId}/sessions", handleForceRevokeAllSessions(client))
		sub.Get("/policies", handleListPolicies(client))
		sub.Post("/policies", handleCreatePolicy(client))
		sub.Put("/policies/{id}", handleUpdatePolicy(client))
		sub.Delete("/policies/{id}", handleDeletePolicy(client))
		sub.Get("/audit", handleQueryAuditLog(client)) // reused from auth_admin_routes.go
	})
}

func handleAdminStats(client authv1.AuthServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := identityFromContext(r.Context())
		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
		resp, err := client.GetAdminStats(ctx, &authv1.GetAdminStatsRequest{})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, resp)
	}
}

// handleDeactivateUser backs DELETE /admin/api/users/:id — per
// http-endpoints.md, this is a soft-delete (is_active = false), never a
// hard row delete; see docs/ui/pages/admin-users.md for why there's no
// matching "reactivate" UI action despite ReactivateUser existing as an RPC.
func handleDeactivateUser(client authv1.AuthServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := identityFromContext(r.Context())
		id := chi.URLParam(r, "id")

		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
		resp, err := client.DeactivateUser(ctx, &authv1.DeactivateUserRequest{UserId: id})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, resp.GetUser())
	}
}

// handleListAllSessions backs GET /admin/api/sessions. http-endpoints.md
// documents this as "list active sessions across all users", but
// auth-service's admin RPC surface (TASK-001/002) only exposes
// ListSessionsForUser, scoped to one user_id — there is no
// ListAllSessions/cross-user RPC. Rather than silently 200-ing with an
// empty/wrong list, this requires ?user_id= and proxies straight to
// ListSessionsForUser; a request with no user_id gets a 400 explaining the
// gap. Follow up with a real cross-user RPC (SOL-001) if the admin console
// actually needs the all-users view live.
func handleListAllSessions(client authv1.AuthServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := identityFromContext(r.Context())
		userID := r.URL.Query().Get("user_id")
		if userID == "" {
			writeJSONError(w, http.StatusBadRequest, "INVALID_ARGUMENT",
				"user_id query param is required — auth-service has no cross-user ListAllSessions RPC yet, only ListSessionsForUser")
			return
		}

		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
		resp, err := client.ListSessionsForUser(ctx, &authv1.ListSessionsForUserRequest{UserId: userID})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, resp)
	}
}

func handleForceRevokeAllSessions(client authv1.AuthServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := identityFromContext(r.Context())
		userID := chi.URLParam(r, "userId")

		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
		resp, err := client.ForceRevokeAllSessionsForUser(ctx, &authv1.ForceRevokeAllSessionsForUserRequest{UserId: userID})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, resp)
	}
}

func handleListPolicies(client authv1.AuthServiceClient) http.HandlerFunc {
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
		resp, err := client.ListAccessPolicies(ctx, &authv1.ListAccessPoliciesRequest{
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

type createPolicyRequestBody struct {
	Name         string `json:"name"`
	Kind         string `json:"kind"`
	DocumentJSON string `json:"document_json"`
}

func handleCreatePolicy(client authv1.AuthServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := identityFromContext(r.Context())

		var body createPolicyRequestBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSONError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "invalid JSON body: "+err.Error())
			return
		}

		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
		resp, err := client.CreateAccessPolicy(ctx, &authv1.CreateAccessPolicyRequest{
			Name:         body.Name,
			Kind:         body.Kind,
			DocumentJson: body.DocumentJSON,
		})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, resp)
	}
}

type updatePolicyRequestBody struct {
	DocumentJSON    string `json:"document_json"`
	ExpectedVersion int32  `json:"expected_version"`
}

func handleUpdatePolicy(client authv1.AuthServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := identityFromContext(r.Context())
		id := chi.URLParam(r, "id")

		var body updatePolicyRequestBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSONError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "invalid JSON body: "+err.Error())
			return
		}

		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
		resp, err := client.UpdateAccessPolicy(ctx, &authv1.UpdateAccessPolicyRequest{
			Id:              id,
			DocumentJson:    body.DocumentJSON,
			ExpectedVersion: body.ExpectedVersion,
		})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, resp)
	}
}

func handleDeletePolicy(client authv1.AuthServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := identityFromContext(r.Context())
		id := chi.URLParam(r, "id")

		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
		if _, err := client.DeleteAccessPolicy(ctx, &authv1.DeleteAccessPolicyRequest{Id: id}); err != nil {
			writeGRPCError(w, err)
			return
		}
		writeJSON(w, http.StatusNoContent, nil)
	}
}
