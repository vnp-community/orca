# TASK-040: Implement `SetIntegrationCredential`/`GetIntegrationCredentialStatus`/`ListIntegrationCredentials` usecases for `scm-integration-service`

**From Solution:** SOL-007
**Priority:** P0
**Service:** `scm-integration-service`
**File:** `internal/usecase/{ports.go,set_integration_credential.go,get_integration_credential_status.go,list_integration_credentials.go}`, `internal/adapter/credentialbroker/client.go`, `internal/adapter/grpc/server.go`
**Depends on:** TASK-037, TASK-038, TASK-039
**Status:** `[x]` DONE — `set_integration_credential.go`/`get_integration_credential_status.go`/`list_integration_credentials.go` confirmed present in `scm-integration-service/internal/usecase/`, wired into `internal/adapter/grpc/server.go` and `internal/adapter/credentialbroker/client.go`'s `Resolver` (Write/WriteRaw/GetStatus/ListConfiguredProviders/RevokeByOwner all implemented).

---

## Context

Reuses `CompleteOAuthFlow`'s existing `owner_id = provider name` convention
verbatim — `credentials.set` and `CompleteOAuthFlow` write into the exact
same `(tenant_id, category, owner_id)` slot for a given provider, just via
two different acquisition mechanisms (pasted PAT vs. OAuth code).

**Architecture-upgrade decision, carried forward from SOL-007 (flag, not a
code change in this task):** the resulting credential is **tenant-wide**,
not per-user — matching what this service already committed to for its
OAuth flow (`CredentialResolver.Resolve` has no `user_id` in its read
path). This is a deliberate, product-visible behavior change from the old
TS backend's per-user `WebCredentialStore`; flag to product before shipping
if per-user isolation for these 5 providers is a hard requirement.

---

## Changes to make

### Step 1 — `usecase/ports.go`: extend `CredentialWriter`, add 2 read ports

```go
type CredentialWriter interface {
	Write(ctx context.Context, tenantID string, provider domain.ScmProvider, token OAuthToken) error
	// WriteRaw is SetIntegrationCredential's entry point — a manually
	// pasted token, never exchanged from an authorization code, so it
	// carries no OAuthToken.Scope. Reuses the same
	// CREDENTIAL_CATEGORY_SCM_OAUTH / owner_id=provider-name slot Write
	// already writes to.
	WriteRaw(ctx context.Context, tenantID string, provider domain.ScmProvider, token, configJSON string) error
}

// CredentialStatusReader backs GetIntegrationCredentialStatus — metadata
// only, via credential-broker-service's GetCredentialMetadataByOwner RPC
// (TASK-038), never ResolveCredentialByOwner (which would leak plaintext
// for a status check).
type CredentialStatusReader interface {
	GetStatus(ctx context.Context, tenantID string, provider domain.ScmProvider) (configured bool, configJSON string, err error)
}

// CredentialLister backs ListIntegrationCredentials via
// credential-broker-service's ListCredentialsByCategory RPC (TASK-038).
type CredentialLister interface {
	ListConfiguredProviders(ctx context.Context, tenantID string) ([]domain.ScmProvider, error)
}
```

### Step 2 — `usecase/set_integration_credential.go` (new)

```go
package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/scm-integration-service/internal/domain"
)

type SetIntegrationCredentialInput struct {
	TenantID   string
	Provider   domain.ScmProvider
	Token      string
	ConfigJSON string
}

type SetIntegrationCredential struct {
	writer CredentialWriter
}

func NewSetIntegrationCredential(writer CredentialWriter) *SetIntegrationCredential {
	return &SetIntegrationCredential{writer: writer}
}

func (uc *SetIntegrationCredential) Execute(ctx context.Context, in SetIntegrationCredentialInput) error {
	if in.TenantID == "" {
		return apperrors.New(apperrors.KindInvalidArgument, "SCM_NO_TENANT", "tenant_id is required", nil)
	}
	if in.Token == "" {
		return apperrors.New(apperrors.KindInvalidArgument, "SCM_NO_TOKEN", "token is required", nil)
	}
	if err := uc.writer.WriteRaw(ctx, in.TenantID, in.Provider, in.Token, in.ConfigJSON); err != nil {
		return apperrors.New(apperrors.KindInternal, "SCM_CREDENTIAL_WRITE_FAILED", "failed to write credential via credential-broker-service", err)
	}
	return nil
}
```

### Step 3 — `usecase/get_integration_credential_status.go` (new)

```go
package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/scm-integration-service/internal/domain"
)

type GetIntegrationCredentialStatusInput struct {
	TenantID string
	Provider domain.ScmProvider
}

type GetIntegrationCredentialStatusResult struct {
	Configured bool
	ConfigJSON string
}

type GetIntegrationCredentialStatus struct {
	reader CredentialStatusReader
}

func NewGetIntegrationCredentialStatus(reader CredentialStatusReader) *GetIntegrationCredentialStatus {
	return &GetIntegrationCredentialStatus{reader: reader}
}

func (uc *GetIntegrationCredentialStatus) Execute(ctx context.Context, in GetIntegrationCredentialStatusInput) (GetIntegrationCredentialStatusResult, error) {
	if in.TenantID == "" {
		return GetIntegrationCredentialStatusResult{}, apperrors.New(apperrors.KindInvalidArgument, "SCM_NO_TENANT", "tenant_id is required", nil)
	}
	configured, configJSON, err := uc.reader.GetStatus(ctx, in.TenantID, in.Provider)
	if err != nil {
		return GetIntegrationCredentialStatusResult{}, apperrors.New(apperrors.KindInternal, "SCM_STATUS_FETCH_FAILED", "failed to fetch credential status", err)
	}
	return GetIntegrationCredentialStatusResult{Configured: configured, ConfigJSON: configJSON}, nil
}
```

### Step 4 — `usecase/list_integration_credentials.go` (new)

```go
package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/scm-integration-service/internal/domain"
)

type ListIntegrationCredentials struct {
	lister CredentialLister
}

func NewListIntegrationCredentials(lister CredentialLister) *ListIntegrationCredentials {
	return &ListIntegrationCredentials{lister: lister}
}

func (uc *ListIntegrationCredentials) Execute(ctx context.Context, tenantID string) ([]domain.ScmProvider, error) {
	if tenantID == "" {
		return nil, apperrors.New(apperrors.KindInvalidArgument, "SCM_NO_TENANT", "tenant_id is required", nil)
	}
	providers, err := uc.lister.ListConfiguredProviders(ctx, tenantID)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "SCM_LIST_FAILED", "failed to list configured providers", err)
	}
	return providers, nil
}
```

### Step 5 — `adapter/credentialbroker/client.go`: implement the 3 new port methods

Add to the existing `Resolver` type (same file `scm-integration-service`'s
`credentialbroker` package already has — `Resolver` already implements
`CredentialResolver`/`CredentialWriter`/`CredentialRevoker`; extend it
rather than creating a new type):

```go
var _ usecase.CredentialWriter = (*Resolver)(nil) // already true — WriteRaw below extends it

// WriteRaw implements usecase.CredentialWriter.WriteRaw — same owner_id
// convention as Write, but a manually pasted token instead of an
// OAuthToken (no Scope), plus the new ConfigJson passthrough (TASK-037/038).
func (r *Resolver) WriteRaw(ctx context.Context, tenantID string, provider domain.ScmProvider, token, configJSON string) error {
	_, err := r.client.WriteCredential(ctx, &credentialbrokerv1.WriteCredentialRequest{
		TenantId:          tenantID,
		OwnerId:           string(provider),
		Category:          credentialbrokerv1.CredentialCategory_CREDENTIAL_CATEGORY_SCM_OAUTH,
		EncryptedEnvelope: []byte(token),
		ConfigJson:        configJSON,
	})
	if err != nil {
		return fmt.Errorf("credentialbroker: writing raw %s credential: %w", provider, err)
	}
	return nil
}

var _ usecase.CredentialStatusReader = (*Resolver)(nil)

// GetStatus implements usecase.CredentialStatusReader via
// GetCredentialMetadataByOwner (TASK-038) — metadata only, never plaintext.
func (r *Resolver) GetStatus(ctx context.Context, tenantID string, provider domain.ScmProvider) (bool, string, error) {
	resp, err := r.client.GetCredentialMetadataByOwner(ctx, &credentialbrokerv1.GetCredentialMetadataByOwnerRequest{
		TenantId: tenantID,
		Category: credentialbrokerv1.CredentialCategory_CREDENTIAL_CATEGORY_SCM_OAUTH,
		OwnerId:  string(provider),
	})
	if err != nil {
		return false, "", fmt.Errorf("credentialbroker: fetching %s credential status: %w", provider, err)
	}
	metadata := resp.GetMetadata()
	if metadata == nil {
		return false, "", nil
	}
	return true, metadata.GetConfigJson(), nil
}

var _ usecase.CredentialLister = (*Resolver)(nil)

// ListConfiguredProviders implements usecase.CredentialLister via
// ListCredentialsByCategory (TASK-038).
func (r *Resolver) ListConfiguredProviders(ctx context.Context, tenantID string) ([]domain.ScmProvider, error) {
	resp, err := r.client.ListCredentialsByCategory(ctx, &credentialbrokerv1.ListCredentialsByCategoryRequest{
		TenantId: tenantID,
		Category: credentialbrokerv1.CredentialCategory_CREDENTIAL_CATEGORY_SCM_OAUTH,
	})
	if err != nil {
		return nil, fmt.Errorf("credentialbroker: listing scm credentials: %w", err)
	}
	providers := make([]domain.ScmProvider, 0, len(resp.GetCredentials()))
	for _, m := range resp.GetCredentials() {
		providers = append(providers, domain.ScmProvider(m.GetOwnerId()))
	}
	return providers, nil
}
```

### Step 6 — `adapter/grpc/server.go`: wire the 3 new RPCs

Add `setIntegrationCredential`/`getIntegrationCredentialStatus`/
`listIntegrationCredentials` usecase fields, thread through `New(...)`, add
3 gRPC methods translating request/response — follow `GetAuthStatus`'s
handler in the same file for the general shape.

### Step 7 — `cmd/server/main.go`: construct and wire the 3 new usecases

Same `credentialbroker.Resolver` instance already satisfies all 3 new
ports (Step 5) — no new dial needed, just 3 new `usecase.New...(resolver)`
constructions passed into `grpc.New(...)`'s extended parameter list.

---

## Verify

```bash
cd /opt/repos/orca/backend-go/services/scm-integration-service
go build ./... && go vet ./...
```

Expected: clean build. Usecase-level tests are added in TASK-043.
