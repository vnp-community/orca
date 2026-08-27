# TASK-AIP-01-08: Thread registration fields through gRPC server, REST, and WS

**From Solution:** SOL-AIP-01
**Priority:** P2
**Service:** `ai-provider-service` (+ `api-gateway`)
**File:** `backend-go/services/ai-provider-service/internal/adapter/grpc/server.go`
**Depends on:** TASK-AIP-01-04, TASK-AIP-01-06
**Status:** `[ ]` TODO

---

## Context

`TASK-AIP-01-04` added the wire fields and `TASK-AIP-01-06` wired the
usecase; nothing yet reads the new `CreateAccountRequest` fields into
`CreateAccountInput`, writes them into the wire `ProviderAccount`
response, or exposes them from `httpgateway`/`wscompat`.

## Changes to make

In `backend-go/services/ai-provider-service/internal/adapter/grpc/server.go`,
update `CreateAccount`:

```go
func (s *Server) CreateAccount(ctx context.Context, req *aiproviderv1.CreateAccountRequest) (*aiproviderv1.CreateAccountResponse, error) {
	account, err := s.createAccount.Execute(ctx, usecase.CreateAccountInput{
		TenantID:      req.GetTenantId(),
		ProviderType:  toDomainProviderType(req.GetType()),
		DevServerID:   req.GetDevServerId(),
		Label:         req.GetLabel(),
		ModelHint:     req.GetModelHint(),
		BaseURL:       req.GetBaseUrl(),
		QuotaLimitDay: int(req.GetQuotaLimitDay()),
		Models:        req.GetModels(),
		IsDefault:     req.GetIsDefault(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &aiproviderv1.CreateAccountResponse{Account: toProtoAccount(account)}, nil
}
```

Update `toProtoAccount`:

```go
func toProtoAccount(a domain.ProviderAccount) *aiproviderv1.ProviderAccount {
	out := &aiproviderv1.ProviderAccount{
		Id: a.ID, TenantId: a.TenantID, Type: toProtoProviderType(a.ProviderType),
		Status: string(a.Status), CredentialRef: a.CredentialRef, DevServerId: a.DevServerID,
		Label: a.Label, ModelHint: a.ModelHint, BaseUrl: a.BaseURL,
		QuotaLimitDay: int32(a.QuotaLimitDay), Models: a.Models, IsDefault: a.IsDefault,
		CreatedBy: a.CreatedBy,
	}
	if a.LastHealthCheckAt != nil {
		out.LastHealthCheckAt = a.LastHealthCheckAt.Format(time.RFC3339)
	}
	return out
}
```
Add `"time"` to imports.

In `backend-go/services/api-gateway/internal/adapter/httpgateway/ai_provider_routes.go`,
extend `createAccountRequestBody` and `handleCreateAccount`:

```go
type createAccountRequestBody struct {
	Type          string   `json:"type"`
	DevServerID   string   `json:"dev_server_id"`
	Label         string   `json:"label"`
	ModelHint     string   `json:"model_hint"`
	BaseURL       string   `json:"base_url"`
	QuotaLimitDay int32    `json:"quota_limit_day"`
	Models        []string `json:"models"`
	IsDefault     bool     `json:"is_default"`
}
```
Thread the new fields into the `client.CreateAccount(ctx,
&aiproviderv1.CreateAccountRequest{...})` call in `handleCreateAccount`.

In
`backend-go/services/api-gateway/internal/adapter/wscompat/channels_ai_provider.go`,
extend `handleAiProviderCreate`'s `createArgs` struct with the same JSON
fields (`devServerId`, `label`, `modelHint`, `baseUrl`, `quotaLimitDay`,
`models`, `isDefault`) and thread them into the
`client.CreateAccount(...)` call, following
`handleAiProviderUpdate`'s existing field-threading shape.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./...
go test ./services/ai-provider-service/internal/adapter/grpc/...
go test ./services/api-gateway/internal/adapter/httpgateway/... -run TestCreateAccount
go test ./services/api-gateway/internal/adapter/wscompat/... -run TestAiProvider
```

Expected: existing `ai_provider_routes_test.go` and
`channels_ai_provider_test.go` cases still pass; extend each with one case
asserting the new fields round-trip end-to-end (REST body → gRPC request →
usecase input, and WS args → gRPC request → usecase input).
