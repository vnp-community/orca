package httpgateway

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	gatewaygrpc "github.com/stablyai/orca-go/services/api-gateway/internal/adapter/grpc"

	tenantv1 "github.com/stablyai/orca-go/proto/gen/go/orca/tenant/v1"
)

// mountTenantRoutes wires the real REST->gRPC reverse-proxy path for
// tenant-service, following the same hand-written translation pattern as
// mountUsageRoutes (usage_routes.go) — see that function's doc comment for
// why this isn't grpc-gateway-generated.
//
// tenant-service's RPCs don't accept a tenant_id on the wire at all (this
// is the service that MINTS tenant identity — Company/Department/Team all
// key off company_id, not tenant_id) — every outbound call still carries
// gatewaygrpc.AttachIdentity(ctx, identity) so the server's own interceptor
// can scope/audit the call, but there's no tenant_id field to scrub from
// any of these request bodies. Where a message DOES carry a user_id tied
// to the caller (GetResolvedProfile's default target, ResolvePermission-
// style "who am I" semantics), that value comes from identityFromContext,
// never the JSON body — see handleGetResolvedProfile.
func mountTenantRoutes(r chi.Router, client tenantv1.TenantServiceClient) {
	r.Route("/v1/tenants", func(sub chi.Router) {
		sub.Post("/companies", handleCreateCompany(client))
		sub.Get("/validate", handleValidateTenant(client))
		sub.Post("/departments", handleCreateDepartment(client))
		sub.Put("/users/{id}/department", handleSetUserDepartment(client))
		sub.Get("/profile", handleGetResolvedProfile(client))
		sub.Post("/teams", handleCreateTeam(client))
		sub.Post("/teams/{id}/members", handleAddTeamMember(client))
		sub.Get("/teams/{id}/members", handleListTeamMembers(client))
	})
}

// createCompanyRequestBody is the REST request shape for POST
// /v1/tenants/companies.
type createCompanyRequestBody struct {
	Name string `json:"name"`
}

func handleCreateCompany(client tenantv1.TenantServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := identityFromContext(r.Context())

		var body createCompanyRequestBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSONError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "invalid JSON body: "+err.Error())
			return
		}

		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
		resp, err := client.CreateCompany(ctx, &tenantv1.CreateCompanyRequest{Name: body.Name})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, resp.GetCompany())
	}
}

func handleValidateTenant(client tenantv1.TenantServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := identityFromContext(r.Context())
		tenantID := r.URL.Query().Get("tenant_id")
		if tenantID == "" {
			writeJSONError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "tenant_id query param is required")
			return
		}

		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
		resp, err := client.ValidateTenant(ctx, &tenantv1.ValidateTenantRequest{TenantId: tenantID})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, resp)
	}
}

// createDepartmentRequestBody is the REST request shape for POST
// /v1/tenants/departments.
type createDepartmentRequestBody struct {
	CompanyID string `json:"company_id"`
	Name      string `json:"name"`
}

func handleCreateDepartment(client tenantv1.TenantServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := identityFromContext(r.Context())

		var body createDepartmentRequestBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSONError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "invalid JSON body: "+err.Error())
			return
		}

		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
		resp, err := client.CreateDepartment(ctx, &tenantv1.CreateDepartmentRequest{
			CompanyId: body.CompanyID,
			Name:      body.Name,
		})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, resp.GetDepartment())
	}
}

// setUserDepartmentRequestBody is the REST request shape for PUT
// /v1/tenants/users/{id}/department — the path's {id} is the target user,
// not the JSON body, matching handleGetTask/handleAddEdge's convention in
// task_routes.go.
type setUserDepartmentRequestBody struct {
	DepartmentID string `json:"department_id"`
}

func handleSetUserDepartment(client tenantv1.TenantServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := identityFromContext(r.Context())
		userID := chi.URLParam(r, "id")

		var body setUserDepartmentRequestBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSONError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "invalid JSON body: "+err.Error())
			return
		}

		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
		_, err := client.SetUserDepartment(ctx, &tenantv1.SetUserDepartmentRequest{
			UserId:       userID,
			DepartmentId: body.DepartmentID,
		})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		writeJSON(w, http.StatusNoContent, nil)
	}
}

// getResolvedProfileResponseBody is the REST response shape for GET
// /v1/tenants/profile.
type getResolvedProfileResponseBody struct {
	ResolvedSettingsJSON string `json:"resolved_settings_json"`
}

func handleGetResolvedProfile(client tenantv1.TenantServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := identityFromContext(r.Context())

		// user_id defaults to the calling identity's own user, but an
		// explicit ?user_id= query param may target another user (e.g. an
		// admin inspecting someone else's resolved profile) — never taken
		// from a JSON body since GET requests carry none.
		userID := r.URL.Query().Get("user_id")
		if userID == "" {
			userID = identity.UserID
		}

		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
		resp, err := client.GetResolvedProfile(ctx, &tenantv1.GetResolvedProfileRequest{UserId: userID})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, getResolvedProfileResponseBody{
			ResolvedSettingsJSON: resp.GetResolvedSettingsJson(),
		})
	}
}

// createTeamRequestBody is the REST request shape for POST
// /v1/tenants/teams. SettingsJSON is optional — an empty/absent value
// means no team-layer override, per tenant.proto's CreateTeamRequest doc
// comment.
type createTeamRequestBody struct {
	CompanyID    string `json:"company_id"`
	Name         string `json:"name"`
	SettingsJSON string `json:"settings_json"`
}

func handleCreateTeam(client tenantv1.TenantServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := identityFromContext(r.Context())

		var body createTeamRequestBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSONError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "invalid JSON body: "+err.Error())
			return
		}

		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
		resp, err := client.CreateTeam(ctx, &tenantv1.CreateTeamRequest{
			CompanyId:    body.CompanyID,
			Name:         body.Name,
			SettingsJson: body.SettingsJSON,
		})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, resp.GetTeam())
	}
}

// addTeamMemberRequestBody is the REST request shape for POST
// /v1/tenants/teams/{id}/members — the path's {id} is the team.
type addTeamMemberRequestBody struct {
	UserID   string `json:"user_id"`
	Priority int32  `json:"priority"`
}

func handleAddTeamMember(client tenantv1.TenantServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := identityFromContext(r.Context())
		teamID := chi.URLParam(r, "id")

		var body addTeamMemberRequestBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSONError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "invalid JSON body: "+err.Error())
			return
		}

		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
		_, err := client.AddTeamMember(ctx, &tenantv1.AddTeamMemberRequest{
			TeamId:   teamID,
			UserId:   body.UserID,
			Priority: body.Priority,
		})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		writeJSON(w, http.StatusNoContent, nil)
	}
}

func handleListTeamMembers(client tenantv1.TenantServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := identityFromContext(r.Context())
		teamID := chi.URLParam(r, "id")

		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
		resp, err := client.ListTeamMembers(ctx, &tenantv1.ListTeamMembersRequest{TeamId: teamID})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, resp)
	}
}
