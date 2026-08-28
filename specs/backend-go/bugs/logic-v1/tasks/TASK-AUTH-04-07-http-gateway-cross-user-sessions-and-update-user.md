# TASK-AUTH-04-07: `GET /admin/api/sessions` uses `ListSessions`; `PATCH /admin/api/users/{id}` becomes a full `UpdateUser`

**From Solution:** SOL-AUTH-04
**Priority:** P1
**Service:** `api-gateway` (httpgateway)
**File:** `backend-go/services/api-gateway/internal/adapter/httpgateway/admin_routes.go`
**Depends on:** TASK-AUTH-04-01, TASK-AUTH-04-03, TASK-AUTH-04-04, TASK-AUTH-04-05
**Status:** `[ ]` TODO

---

## Context

`handleListAllSessions` currently 400s whenever `user_id` is absent from the query string, because `auth-service` had no cross-user RPC — it does now (TASK-AUTH-04-03/-05). Separately, `PATCH /admin/api/users/{id}` is wired to the role-only `handleUpdateUserRole`; the spec's admin-console user-edit form needs email/name too, backed by the new `UpdateUser` RPC (TASK-AUTH-04-04/-05).

## Changes to make

In `backend-go/services/api-gateway/internal/adapter/httpgateway/admin_routes.go`, replace `handleListAllSessions`:

```go
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
```

Change the route table's `Patch` mount from `handleUpdateUserRole` to a new `handleUpdateUser`, and add the handler:

```go
// mountAdminRoutes — change:
//   sub.Patch("/users/{id}", handleUpdateUserRole(client))
// to:
//   sub.Patch("/users/{id}", handleUpdateUser(client))

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
```

Add `"google.golang.org/protobuf/types/known/wrapperspb"` to imports. `/v1/auth/users/{id}/role` (`auth_admin_routes.go`, backed by `handleUpdateUserRole`) is left mounted as-is for backward compatibility — only `admin_routes.go`'s `PATCH /admin/api/users/{id}` mount switches handlers.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/api-gateway/...
go test ./services/api-gateway/internal/adapter/httpgateway/... -run TestAdminRoutes -v
```

Expected: `GET /admin/api/sessions` with no `user_id` → `200` with cross-user results instead of the prior `400`; `GET /admin/api/sessions?user_id=X` unchanged behavior; `PATCH /admin/api/users/{id}` with `{"email": "new@x.com"}` → `200`, role unchanged (fake `UpdateUser` receives `Email` set, `Name`/`Role` nil).
