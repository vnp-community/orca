package httpgateway

import (
	"encoding/csv"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"google.golang.org/protobuf/types/known/wrapperspb"

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
		sub.Get("/users", handleListUsers(client))   // reused from auth_admin_routes.go
		sub.Post("/users", handleCreateUser(client)) // reused from auth_admin_routes.go
		sub.Patch("/users/{id}", handleUpdateUser(client))
		sub.Delete("/users/{id}", handleDeactivateUser(client))
		sub.Get("/sessions", handleListAllSessions(client))
		sub.Delete("/sessions/{sessionId}", handleRevokeSession(client)) // reused from auth_admin_routes.go
		sub.Delete("/users/{userId}/sessions", handleForceRevokeAllSessions(client))
		sub.Get("/policies", handleListPolicies(client))
		sub.Post("/policies", handleCreatePolicy(client))
		sub.Put("/policies/{id}", handleUpdatePolicy(client))
		sub.Delete("/policies/{id}", handleDeletePolicy(client))
		sub.Get("/audit", handleQueryAuditLog(client)) // reused from auth_admin_routes.go
		sub.Get("/audit/export", handleExportAuditLog(client))
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

// handleListAllSessions backs GET /admin/api/sessions. With user_id set,
// proxies to the single-user ListSessionsForUser (unchanged, narrower
// contract). Without it, proxies to the tenant-scoped cross-user
// ListSessions — the admin dashboard's "active sessions across all users"
// view.
func handleListAllSessions(client authv1.AuthServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := identityFromContext(r.Context())
		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)

		if userID := r.URL.Query().Get("user_id"); userID != "" {
			resp, err := client.ListSessionsForUser(ctx, &authv1.ListSessionsForUserRequest{UserId: userID})
			if err != nil {
				writeGRPCError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, resp)
			return
		}

		resp, err := client.ListSessions(ctx, &authv1.ListSessionsRequest{
			PageToken: r.URL.Query().Get("page_token"),
		})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, resp)
	}
}

// updateUserRequestBody — {email?, name?, role?}. is_active is
// deliberately NOT here — deactivation stays on DELETE
// /admin/api/users/:id (handleDeactivateUser), which already matches the
// spec's actual step-by-step flow.
type updateUserRequestBody struct {
	Email *string `json:"email"`
	Name  *string `json:"name"`
	Role  *string `json:"role"`
}

func handleUpdateUser(client authv1.AuthServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := identityFromContext(r.Context())
		id := chi.URLParam(r, "id")

		var body updateUserRequestBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSONError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "invalid JSON body: "+err.Error())
			return
		}

		req := &authv1.UpdateUserRequest{UserId: id}
		if body.Email != nil {
			req.Email = wrapperspb.String(*body.Email)
		}
		if body.Name != nil {
			req.Name = wrapperspb.String(*body.Name)
		}
		if body.Role != nil {
			r := parseRole(*body.Role)
			req.Role = &r
		}

		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
		resp, err := client.UpdateUser(ctx, req)
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, resp.GetUser())
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

// handleExportAuditLog streams the full (paginated) audit log as CSV.
// Headers are sent before all pages are fetched — if a later page's RPC
// call fails, the client sees a truncated CSV rather than a clean JSON
// error, since headers can't be swapped mid-stream. Accepted for a first
// pass; a future improvement could buffer to a temp file and stream only
// once complete.
func handleExportAuditLog(client authv1.AuthServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := identityFromContext(r.Context())
		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)

		w.Header().Set("Content-Type", "text/csv")
		w.Header().Set("Content-Disposition", `attachment; filename="audit-log.csv"`)
		cw := csv.NewWriter(w)
		_ = cw.Write([]string{"id", "actor_id", "action", "target_type", "target_id", "ip_address", "occurred_at", "metadata"})

		pageToken := ""
		for {
			resp, err := client.QueryAuditLog(ctx, &authv1.QueryAuditLogRequest{
				TenantId:  identity.TenantID,
				PageToken: pageToken,
				PageSize:  200, // matches query_audit_log.go's existing pagination cap
			})
			if err != nil {
				return // best effort: stop writing rows, client sees a truncated CSV
			}
			for _, e := range resp.GetEntries() {
				_ = cw.Write([]string{
					e.GetId(), e.GetActorId(), e.GetAction(), e.GetTargetType(), e.GetTargetId(),
					e.GetIpAddress(), e.GetOccurredAt().AsTime().Format(time.RFC3339), e.GetMetadataJson(),
				})
			}
			if resp.GetNextPageToken() == "" {
				break
			}
			pageToken = resp.GetNextPageToken()
		}
		cw.Flush()
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
