# TASK-041: Add `issuetracking.proto` credential RPCs + ports/usecases/adapter (from scratch)

**From Solution:** SOL-007
**Priority:** P0
**Service:** `issue-tracking-service`
**File:** `backend-go/proto/orca/issuetracking/v1/issuetracking.proto`, `internal/usecase/{ports.go,set_integration_credential.go,get_integration_credential_status.go,list_integration_credentials.go,revoke_auth.go}` (new), `internal/adapter/credential/client.go`, `internal/adapter/grpc/server.go`, `cmd/server/main.go`
**Depends on:** TASK-037, TASK-038
**Status:** `[x]` DONE — `issuetracking.proto`'s 4 RPCs/messages and generated stubs already existed on this branch; `usecase/ports.go`'s 4 credential ports (`CredentialWriter`/`CredentialStatusReader`/`CredentialLister`/`CredentialRevoker`) already existed too. What this task added: `internal/usecase/{set_integration_credential.go,get_integration_credential_status.go,list_integration_credentials.go,revoke_auth.go}` (all 4 new), 4 new `Resolver` methods in `internal/adapter/credential/client.go` (`WriteRaw`/`GetStatus`/`ListConfiguredProviders`/`RevokeByOwner`, keyed by a new provider-name-only `credentialsOwnerID` — deliberately distinct from `Resolve`/`Write`/`ExistingCredentialID`'s existing per-user `"<userID>:<provider>"` owner_id), 4 new gRPC methods + `toProtoProvider` in `internal/adapter/grpc/server.go`, and composition-root wiring in `cmd/server/main.go`. `go build ./... && go vet ./... && go test ./...` clean; new usecase tests in `internal/usecase/{set_integration_credential_test.go,get_integration_credential_status_test.go,list_integration_credentials_test.go,revoke_auth_test.go}`. No `grpc/server_test.go` added — this codebase has no existing convention of unit-testing the translation-only grpc adapter layer directly (neither does scm-integration-service's equivalent server.go), only through usecase-level tests.

---

## Context

Mirrors TASK-039/TASK-040's `scm-integration-service` shape, but this
service starts from **zero** credential-write/status/revoke surface —
`issuetracking.proto` today only has `ListIssues`/`CreateIssue`/
`LinkIssue`, and `RevokeAuth` doesn't exist here at all (unlike
`scm-integration-service`, which already has a working `RevokeAuth` to
reuse). This is a larger diff than TASK-040's, not underestimated per
SOL-007's own flag.

**Wire-shape wrinkle, not present in TASK-040:** this service's existing
`internal/adapter/credential/client.go` (`Resolver.Resolve`) already
establishes a convention that `ResolveCredentialByOwner`'s plaintext value
JSON-decodes into `{"baseUrl":"...","email":"...","token":"..."}` — Jira
needs all three fields, Linear only `token`. `SetIntegrationCredential`'s
write path must produce an envelope in that same shape (not a bare token
string the way `scm-integration-service`'s `WriteRaw` does), or
`Resolve`'s existing decode breaks for every credential written through
the new path. `credentials.set`'s `{ service, token, config }` params
supply `token` directly and `baseUrl`/`email` (Jira only) via `config`.

---

## Changes to make

### Step 1 — `issuetracking.proto`: add 4 RPCs (including `RevokeAuth`, new here)

```protobuf
service IssueTrackingService {
  rpc ListIssues(ListIssuesRequest) returns (ListIssuesResponse);
  rpc CreateIssue(CreateIssueRequest) returns (CreateIssueResponse);
  rpc LinkIssue(LinkIssueRequest) returns (LinkIssueResponse);

  // SetIntegrationCredential/GetIntegrationCredentialStatus/
  // ListIntegrationCredentials/RevokeAuth back api-gateway's
  // credentials.set/status/list/revoke channels for jira/linear
  // (SOL-007). Unlike scm-integration-service, this service has no prior
  // OAuth-flow surface — RevokeAuth is new here too, not reused.
  rpc SetIntegrationCredential(SetIntegrationCredentialRequest) returns (SetIntegrationCredentialResponse);
  rpc GetIntegrationCredentialStatus(GetIntegrationCredentialStatusRequest) returns (GetIntegrationCredentialStatusResponse);
  rpc ListIntegrationCredentials(ListIntegrationCredentialsRequest) returns (ListIntegrationCredentialsResponse);
  rpc RevokeAuth(RevokeAuthRequest) returns (RevokeAuthResponse);
}
```

Append messages (identical shape to `scmintegration.proto`'s, `ScmProvider`
swapped for `IssueProvider`):

```protobuf
message SetIntegrationCredentialRequest {
  string tenant_id = 1;
  IssueProvider provider = 2;
  string token = 3;
  string config_json = 4; // optional, non-secret — e.g. Jira's baseUrl/email
}
message SetIntegrationCredentialResponse {}

message GetIntegrationCredentialStatusRequest {
  string tenant_id = 1;
  IssueProvider provider = 2;
}
message GetIntegrationCredentialStatusResponse {
  bool configured = 1;
  string config_json = 2;
}

message ListIntegrationCredentialsRequest {
  string tenant_id = 1;
}
message ListIntegrationCredentialsResponse {
  repeated IssueProvider configured_providers = 1;
}

message RevokeAuthRequest {
  string tenant_id = 1;
  IssueProvider provider = 2;
}
message RevokeAuthResponse {}
```

Regenerate stubs: `cd /opt/repos/orca/backend-go && buf generate proto`.

### Step 2 — `usecase/ports.go`: add `CredentialWriter`/`CredentialStatusReader`/`CredentialLister`/`CredentialRevoker`

None of these 4 ports exist on this service yet (contrast TASK-040, which
only extended an existing `CredentialWriter`). Add all 4 fresh, same
shapes as `scm-integration-service`'s (TASK-040 Step 1), substituting
`domain.Provider` for `domain.ScmProvider`:

```go
type CredentialWriter interface {
	// WriteRaw writes a manually pasted token (+ optional non-secret
	// config) as this tenant's credential for provider. See this file's
	// package doc comment (TASK-041) for the JSON envelope shape this must
	// produce to match Resolve's existing decode convention.
	WriteRaw(ctx context.Context, tenantID string, provider domain.Provider, token, configJSON string) error
}

type CredentialStatusReader interface {
	GetStatus(ctx context.Context, tenantID string, provider domain.Provider) (configured bool, configJSON string, err error)
}

type CredentialLister interface {
	ListConfiguredProviders(ctx context.Context, tenantID string) ([]domain.Provider, error)
}

type CredentialRevoker interface {
	RevokeByOwner(ctx context.Context, tenantID string, provider domain.Provider) error
}
```

### Step 3 — 4 new usecase files

`set_integration_credential.go`, `get_integration_credential_status.go`,
`list_integration_credentials.go` — identical structure to TASK-040 Steps
2–4, substituting `domain.Provider`, error code prefix `ISSUETRACKING_`
instead of `SCM_`.

`revoke_auth.go` (new — mirrors `scm-integration-service`'s existing
`revoke_auth.go` verbatim, see that file for the exact shape):

```go
package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/issue-tracking-service/internal/domain"
)

type RevokeAuthInput struct {
	TenantID string
	Provider domain.Provider
}

type RevokeAuth struct {
	revoker CredentialRevoker
}

func NewRevokeAuth(revoker CredentialRevoker) *RevokeAuth {
	return &RevokeAuth{revoker: revoker}
}

func (uc *RevokeAuth) Execute(ctx context.Context, in RevokeAuthInput) error {
	if in.TenantID == "" {
		return apperrors.New(apperrors.KindInvalidArgument, "ISSUETRACKING_NO_TENANT", "tenant_id is required", nil)
	}
	if err := uc.revoker.RevokeByOwner(ctx, in.TenantID, in.Provider); err != nil {
		return apperrors.New(apperrors.KindInternal, "ISSUETRACKING_REVOKE_FAILED", "failed to revoke credential", err)
	}
	return nil
}
```

### Step 4 — `internal/adapter/credential/client.go`: extend `Resolver` with the 4 new port methods

```go
var _ usecase.CredentialWriter = (*Resolver)(nil)

// WriteRaw implements usecase.CredentialWriter.WriteRaw. Builds the same
// {"baseUrl","email","token"} envelope shape Resolve already expects to
// decode (see this file's package doc comment) — configJSON is expected to
// be a JSON object with optional baseUrl/email keys (Jira only; Linear
// callers pass "" or "{}").
func (r *Resolver) WriteRaw(ctx context.Context, tenantID string, provider domain.Provider, token, configJSON string) error {
	envelope := credentialEnvelope{Token: token}
	if configJSON != "" {
		var cfg struct {
			BaseURL string `json:"baseUrl"`
			Email   string `json:"email"`
		}
		if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
			return fmt.Errorf("credential: decoding config_json for %s: %w", provider, err)
		}
		envelope.BaseURL, envelope.Email = cfg.BaseURL, cfg.Email
	}
	blob, err := json.Marshal(envelope)
	if err != nil {
		return fmt.Errorf("credential: encoding %s credential envelope: %w", provider, err)
	}
	_, err = r.client.WriteCredential(ctx, &credentialbrokerv1.WriteCredentialRequest{
		TenantId: tenantID, OwnerId: string(provider),
		Category:          credentialbrokerv1.CredentialCategory_CREDENTIAL_CATEGORY_ISSUE_TRACKER_OAUTH,
		EncryptedEnvelope: blob, ConfigJson: configJSON,
	})
	if err != nil {
		return fmt.Errorf("credential: writing %s credential: %w", provider, err)
	}
	return nil
}

var _ usecase.CredentialStatusReader = (*Resolver)(nil)

func (r *Resolver) GetStatus(ctx context.Context, tenantID string, provider domain.Provider) (bool, string, error) {
	resp, err := r.client.GetCredentialMetadataByOwner(ctx, &credentialbrokerv1.GetCredentialMetadataByOwnerRequest{
		TenantId: tenantID, Category: credentialbrokerv1.CredentialCategory_CREDENTIAL_CATEGORY_ISSUE_TRACKER_OAUTH, OwnerId: string(provider),
	})
	if err != nil {
		return false, "", fmt.Errorf("credential: fetching %s credential status: %w", provider, err)
	}
	metadata := resp.GetMetadata()
	if metadata == nil {
		return false, "", nil
	}
	return true, metadata.GetConfigJson(), nil
}

var _ usecase.CredentialLister = (*Resolver)(nil)

func (r *Resolver) ListConfiguredProviders(ctx context.Context, tenantID string) ([]domain.Provider, error) {
	resp, err := r.client.ListCredentialsByCategory(ctx, &credentialbrokerv1.ListCredentialsByCategoryRequest{
		TenantId: tenantID, Category: credentialbrokerv1.CredentialCategory_CREDENTIAL_CATEGORY_ISSUE_TRACKER_OAUTH,
	})
	if err != nil {
		return nil, fmt.Errorf("credential: listing issue-tracker credentials: %w", err)
	}
	providers := make([]domain.Provider, 0, len(resp.GetCredentials()))
	for _, m := range resp.GetCredentials() {
		providers = append(providers, domain.Provider(m.GetOwnerId()))
	}
	return providers, nil
}

var _ usecase.CredentialRevoker = (*Resolver)(nil)

func (r *Resolver) RevokeByOwner(ctx context.Context, tenantID string, provider domain.Provider) error {
	_, err := r.client.RevokeCredentialByOwner(ctx, &credentialbrokerv1.RevokeCredentialByOwnerRequest{
		TenantId: tenantID, Category: credentialbrokerv1.CredentialCategory_CREDENTIAL_CATEGORY_ISSUE_TRACKER_OAUTH, OwnerId: string(provider),
	})
	if err != nil {
		return fmt.Errorf("credential: revoking %s credential: %w", provider, err)
	}
	return nil
}
```

### Step 5 — `adapter/grpc/server.go`: wire the 4 new RPCs

Add 4 usecase fields to `Server`, thread through `New(...)`, add 4 gRPC
methods — same translation shape as `ListIssues`'s existing handler in the
same file.

### Step 6 — `cmd/server/main.go`: construct and wire the 4 new usecases

The existing `credentialResolver := credential.New(brokerConn)` (already
dialed) now also satisfies `CredentialWriter`/`CredentialStatusReader`/
`CredentialLister`/`CredentialRevoker` (Step 4) — no new dial. Add
`usecase.NewSetIntegrationCredential(credentialResolver)`,
`usecase.NewGetIntegrationCredentialStatus(credentialResolver)`,
`usecase.NewListIntegrationCredentials(credentialResolver)`,
`usecase.NewRevokeAuth(credentialResolver)`, pass into
`grpc.New(...)`'s extended parameter list.

---

## Verify

```bash
cd /opt/repos/orca/backend-go/services/issue-tracking-service
go build ./... && go vet ./...
```

Expected: clean build. Usecase-level tests are added in TASK-043.
