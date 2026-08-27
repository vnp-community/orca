# TASK-AIP-02-07: Thread new `ResolveProvider` fields through gRPC server and REST route

**From Solution:** SOL-AIP-02
**Priority:** P0 — without this, the filtering fix in `TASK-AIP-02-05` is unreachable from any real caller
**Service:** `ai-provider-service` (+ `api-gateway`)
**File:** `backend-go/services/ai-provider-service/internal/adapter/grpc/server.go`
**Depends on:** TASK-AIP-02-01, TASK-AIP-02-05, TASK-AIP-02-06
**Status:** `[ ]` TODO

---

## Context

`TASK-AIP-02-05`/`-06` extended `ResolveProviderInput`, but
`grpc/server.go`'s `ResolveProvider` handler (`server.go:71-80`) still
only threads `UserId`/`ProjectId`. `httpgateway/ai_provider_routes.go`'s
`handleResolveProvider` (`ai_provider_routes.go:66-93`) is the same gap on
the REST side. Neither the correctness fix nor the extensions do anything
for a real caller until this wiring lands.

## Changes to make

In
`backend-go/services/ai-provider-service/internal/adapter/grpc/server.go`:

```go
func (s *Server) ResolveProvider(ctx context.Context, req *aiproviderv1.ResolveProviderRequest) (*aiproviderv1.ResolveProviderResponse, error) {
	account, err := s.resolveProvider.Resolve(ctx, usecase.ResolveProviderInput{
		UserID:      req.GetUserId(),
		ProjectID:   req.GetProjectId(),
		DevServerID: req.GetDevServerId(),
		ModelHint:   req.GetModelHint(),
		AccountID:   req.GetAccountId(),
		ScopedRef:   req.GetScopedRef(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(mapDomainError(err))
	}
	return &aiproviderv1.ResolveProviderResponse{Account: toProtoAccount(account)}, nil
}
```

In
`backend-go/services/api-gateway/internal/adapter/httpgateway/ai_provider_routes.go`'s
`handleResolveProvider`:

```go
func handleResolveProvider(client aiproviderv1.AiProviderServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := identityFromContext(r.Context())
		q := r.URL.Query()

		userID := q.Get("user_id")
		if userID == "" {
			userID = identity.UserID
		}

		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
		resp, err := client.ResolveProvider(ctx, &aiproviderv1.ResolveProviderRequest{
			TenantId:    identity.TenantID,
			UserId:      userID,
			ProjectId:   q.Get("project_id"),
			DevServerId: q.Get("dev_server_id"),
			ModelHint:   q.Get("model_hint"),
			AccountId:   q.Get("account_id"),
			ScopedRef:   q.Get("scoped_ref"),
		})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, resp.GetAccount())
	}
}
```

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./...
go test ./services/ai-provider-service/internal/adapter/grpc/...
go test ./services/api-gateway/internal/adapter/httpgateway/... -run TestResolveProvider
```

Extend the gRPC server test and the httpgateway test with one case each
asserting `dev_server_id`/`model_hint` reach `ResolveProviderInput`
unmodified.
