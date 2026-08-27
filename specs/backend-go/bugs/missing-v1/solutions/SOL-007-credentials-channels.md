# SOL-007: Route `credentials.*` through the owning domain services, not a direct gateway→broker rule

**Resolves:** [BUG-007](../BUG-007-credentials-channels-not-implemented.md)
**Service:** `scm-integration-service` + `issue-tracking-service` (new RPCs, reusing their existing `credential-broker-service` adapters) + `credential-broker-service` (2 new RPCs, 1 additive field) + `api-gateway` (new `wscompat` channels — no new `registry.go` rule)
**Affected files (proposed):**
- `backend-go/proto/orca/credentialbroker/v1/credentialbroker.proto`
- `backend-go/proto/orca/scmintegration/v1/scmintegration.proto`
- `backend-go/proto/orca/issuetracking/v1/issuetracking.proto`
- `backend-go/services/scm-integration-service/internal/usecase/{set_integration_credential,get_integration_credential_status,list_integration_credentials}.go` (new)
- `backend-go/services/scm-integration-service/internal/usecase/ports.go` (extend `CredentialWriter`, new `CredentialStatusReader`/`CredentialLister` ports)
- `backend-go/services/issue-tracking-service/internal/usecase/{revoke_auth,set_integration_credential,get_integration_credential_status,list_integration_credentials}.go` (new — this service has no credential-broker adapter at all yet)
- `backend-go/services/credential-broker-service/internal/usecase/{get_credential_metadata_by_owner,list_credentials_by_category}.go` (new)
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_credentials.go` (new)
**Status:** 📋 Proposed — not yet implemented

---

## (a) Routing decision: through the owning domain services, not a new gateway→broker rule

`registry.go`'s comment says credential-broker-service is "reached only
indirectly via infra-fleet-service's credential path" — but tracing that
claim against `credential-broker-service.md` §7's own dependency table
shows it's specific to **`SSH_CREDENTIAL`** category traffic:
`infra-fleet-service` calls `ResolveCredential`/`RotateCredential` for SSH
certs targeting dev servers, which is that service's own operational
concern. It has no relationship to `credentials.*`'s 5 providers
(bitbucket/azure-devops/gitea/linear/jira), all `INTEGRATION_OAUTH_TOKEN`-
category. Routing `credentials.*` through `infra-fleet-service` "per
`registry.go`'s comment" would be following the letter of that note into
the wrong service — a category mismatch, not the design it's actually
describing.

The right owning services already exist in that same dependency table:
`credential-broker-service.md` §7 lists `scm-integration-service` and
`issue-tracking-service` as the two authorized callers for
`INTEGRATION_OAUTH_TOKEN` credentials — and `scm-integration-service`
**already implements this exact pattern**, just for the OAuth-flow half of
credential management: `GetAuthStatus`/`RevokeAuth`
(`scmintegration.proto:23,26`) already forward to
`credential-broker-service` via this service's own `CredentialResolver`/
`CredentialRevoker` ports (`ports.go:67-78,153-166`), and
`CompleteOAuthFlow` already demonstrates the write path via
`CredentialWriter` (`ports.go:141-151`, `complete_oauth_flow.go:74`) —
**`owner_id = provider name` is already an established, working convention
here**, not something this solution invents.

Adding a direct `api-gateway` → `credential-broker-service` rule to
`registry.go` would be the wrong fix even setting the category mismatch
aside: `credential-broker-service.md` §7 states plainly, "Called by
`api-gateway` (indirectly, via the above) — No direct calls — the gateway
never talks to this service for tenant secrets; it always goes through the
owning domain service." That's a deliberate security boundary (§2's "every
other service's Vault identity is scoped only to its own dynamic-DB-
credential lease" argument extends structurally to "every other *gateway
route* only reaches secrets through an owning domain service's narrower,
purpose-built RPCs," not a generic secret-CRUD surface exposed to the
edge). Breaking it for `credentials.*` alone, when the correct owning
services already exist and already implement most of the needed shape,
would be a regression, not a fix.

**Decision: no new `registry.go` rule.** `api-gateway` already dials both
`scm-integration-service` (`/v1/scm`, `RouteWired`) and
`issue-tracking-service` (`/v1/issues`, `RouteWired`) — `credentials.*`'s
`wscompat` channels reuse those existing gRPC clients, fanning out per
`service` param to whichever of the two owns it (mapping table below).

---

## (c) The mapping/adapter layer — `service` string ↔ `(category, owner_id)`

| Frontend `service` | Owning backend-go service | `CredentialCategory` | `owner_id` |
|---|---|---|---|
| `bitbucket` | `scm-integration-service` | `CREDENTIAL_CATEGORY_SCM_OAUTH` | `"bitbucket"` |
| `azure-devops` | `scm-integration-service` | `CREDENTIAL_CATEGORY_SCM_OAUTH` | `"azure-devops"` |
| `gitea` | `scm-integration-service` | `CREDENTIAL_CATEGORY_SCM_OAUTH` | `"gitea"` |
| `linear` | `issue-tracking-service` | `CREDENTIAL_CATEGORY_ISSUE_TRACKER_OAUTH` | `"linear"` |
| `jira` | `issue-tracking-service` | `CREDENTIAL_CATEGORY_ISSUE_TRACKER_OAUTH` | `"jira"` |

This reuses `CompleteOAuthFlow`'s existing `owner_id = provider name`
convention (`complete_oauth_flow.go`'s doc comment,
`scmintegration.proto:135`) verbatim — `credentials.set` and
`CompleteOAuthFlow` end up writing into the exact same
`(tenant_id, category, owner_id)` slot for a given provider, just via two
different acquisition mechanisms (a pasted PAT vs. an OAuth authorization
code). That convergence is deliberate, not incidental: a tenant should have
**one** connected Bitbucket credential regardless of whether it was set
through the OAuth consent flow or pasted as a token in Web Server mode's
settings screen, and `ListIssues`/`CreatePullRequest`'s existing
`CredentialResolver.Resolve` call (`ports.go:76-78`) should transparently
pick up whichever one is active — no branching needed anywhere else in
`scm-integration-service` for this to work.

### Explicit architecture-upgrade decision: tenant-wide, not per-user

The old TS `WebCredentialStore` was **per-user** (`.enc` files under
`<userDataPath>/users/<userId>/...`, per
`06-secrets-vault-architecture.md`'s migration notes). The
`owner_id = provider name` convention this solution reuses is **tenant-wide**
— one Bitbucket credential per tenant, shared by every user, matching what
`scm-integration-service` already committed to for its OAuth flow (there is
no `user_id` in `CredentialResolver`'s read path at all,
`ports.go:76-78`). This is a real, deliberate behavior change from the old
TS backend, flagged explicitly rather than silently ported: for a
multi-user Web Server mode deployment, one shared org-level integration
credential per provider is arguably the more sensible design (mirrors how
GitHub/GitLab's own excluded `gh`/`glab` OS-keychain credentials already
work — one connected account per deployment, not one per user) — but it is
a product-visible change (a second user in the same tenant can no longer
have their *own* Bitbucket token independent of the first), not a
transparent implementation detail. Flag to product before shipping if
per-user isolation for these 5 providers is a hard requirement; if so, the
alternative is a composite `owner_id = "{userID}:{service}"` — but that
would *diverge* from `CompleteOAuthFlow`'s existing convention on the same
category, creating two incompatible interpretations of `owner_id` for
`CREDENTIAL_CATEGORY_SCM_OAUTH` depending on which path wrote the
credential, which is worse than picking one and documenting it.

### `mode` field

No backend-go concept maps to the TS frontend's `'electron'` stub mode —
and it needs none: a request that reaches `wscompat`'s `credentials.*`
handlers at all is, by construction, not running in Electron's local IPC
path (`runtime-credentials-client.ts`'s own comment: Electron mode "has no
separate native handler, it just answers with `mode:'electron'` stubs" —
entirely client-side, never reaching this RPC surface). Every response from
these handlers hardcodes `mode: "server"` — an honest constant, not a
fabricated value, following `channels.go`'s existing convention (see
`toDevServerView`'s doc comment on filling required-but-unavailable fields
with honest placeholders rather than invented data).

---

## Design — Broker additions (`credential-broker-service`)

Two new RPCs, both additive, both metadata-only (no secret value in either
response — preserving the "database dump yields no usable credential"
invariant `credential-broker-service.md` §1 states as load-bearing):

```protobuf
// credentialbroker.proto additions

// GetCredentialMetadataByOwner is GetCredentialMetadata's lookup-by-owner
// counterpart — for callers that know (tenant_id, category, owner_id) but
// were never handed an opaque credential_id, mirroring
// ResolveCredentialByOwner's existing by-owner convention but for a
// metadata-only read instead of a plaintext resolve. Closes the exact gap
// BUG-007 flags: "there is no by-owner metadata-read RPC (only
// ResolveCredentialByOwner, which returns the plaintext value — a security
// mismatch for a status check)."
rpc GetCredentialMetadataByOwner(GetCredentialMetadataByOwnerRequest) returns (GetCredentialMetadataByOwnerResponse);

message GetCredentialMetadataByOwnerRequest {
  string tenant_id = 1;
  CredentialCategory category = 2;
  string owner_id = 3;
}
message GetCredentialMetadataByOwnerResponse {
  // metadata is unset (not an error) when no credential exists yet for
  // this (tenant, category, owner) — the normal "not configured" case
  // credentials.status must distinguish from a real error.
  optional CredentialMetadata metadata = 1;
}

// ListCredentialsByCategory answers "which owner_ids have a credential in
// this category for this tenant" — the RPC credentials.list needs and
// nothing in this service's proto answers today (BUG-007: "CredentialBrokerService
// has no List/ListByOwner-shaped method anywhere").
rpc ListCredentialsByCategory(ListCredentialsByCategoryRequest) returns (ListCredentialsByCategoryResponse);

message ListCredentialsByCategoryRequest {
  string tenant_id = 1;
  CredentialCategory category = 2;
}
message ListCredentialsByCategoryResponse {
  repeated CredentialMetadata credentials = 1;
}
```

One additive field, non-secret configuration only (e.g. a self-hosted
Gitea/Jira instance base URL — part of `credentials.set`'s existing
`{ service, token, config }` param shape that has nowhere to live today):

```protobuf
message WriteCredentialRequest {
  string tenant_id = 1;
  string owner_id = 2;
  CredentialCategory category = 3;
  bytes encrypted_envelope = 4;
  string config_json = 5; // NEW — non-secret sidecar config only (e.g. a
                           // self-hosted instance base URL). NEVER put
                           // anything sensitive here: unlike
                           // encrypted_envelope, this field is returned
                           // verbatim by metadata-only reads
                           // (GetCredentialMetadataByOwner, GetCredentialMetadata).
}

message CredentialMetadata {
  string id = 1;
  string tenant_id = 2;
  string owner_id = 3;
  CredentialCategory category = 4;
  string status = 5;
  string vault_path = 6;
  string config_json = 7; // NEW — mirrors WriteCredentialRequest.config_json
}
```

---

## Design — `scm-integration-service` additions (representative; `issue-tracking-service` mirrors this)

```protobuf
// scmintegration.proto additions
rpc SetIntegrationCredential(SetIntegrationCredentialRequest) returns (SetIntegrationCredentialResponse);
rpc GetIntegrationCredentialStatus(GetIntegrationCredentialStatusRequest) returns (GetIntegrationCredentialStatusResponse);
rpc ListIntegrationCredentials(ListIntegrationCredentialsRequest) returns (ListIntegrationCredentialsResponse);
// RevokeAuth (scmintegration.proto:26) is REUSED as-is for credentials.revoke — no new RPC needed.

message SetIntegrationCredentialRequest {
  string tenant_id = 1;
  ScmProvider provider = 2;
  string token = 3;       // raw PAT/token, transport-encrypted at the edge same as every other credential write
  string config_json = 4; // optional, non-secret
}
message SetIntegrationCredentialResponse {}

message GetIntegrationCredentialStatusRequest {
  string tenant_id = 1;
  ScmProvider provider = 2;
}
message GetIntegrationCredentialStatusResponse {
  bool configured = 1;
  string config_json = 2; // present only when configured
}

message ListIntegrationCredentialsRequest {
  string tenant_id = 1;
}
message ListIntegrationCredentialsResponse {
  repeated ScmProvider configured_providers = 1;
}
```

```go
// usecase/ports.go — extend CredentialWriter, add 2 new read ports
type CredentialWriter interface {
	Write(ctx context.Context, tenantID string, provider domain.ScmProvider, token OAuthToken) error
	// WriteRaw is SetIntegrationCredential's entry point — a manually
	// pasted token, never exchanged from an authorization code, so it
	// carries no OAuthToken.Scope. Reuses the same
	// CREDENTIAL_CATEGORY_SCM_OAUTH / owner_id=provider-name slot Write
	// already writes to (see this file's "explicit architecture-upgrade
	// decision" section in SOL-007).
	WriteRaw(ctx context.Context, tenantID string, provider domain.ScmProvider, token, configJSON string) error
}

// CredentialStatusReader backs GetIntegrationCredentialStatus — metadata
// only, via the new GetCredentialMetadataByOwner broker RPC, never
// ResolveCredentialByOwner (which would leak plaintext for a status check —
// exactly the mismatch BUG-007 flags).
type CredentialStatusReader interface {
	GetStatus(ctx context.Context, tenantID string, provider domain.ScmProvider) (configured bool, configJSON string, err error)
}

// CredentialLister backs ListIntegrationCredentials via the new
// ListCredentialsByCategory broker RPC.
type CredentialLister interface {
	ListConfiguredProviders(ctx context.Context, tenantID string) ([]domain.ScmProvider, error)
}
```

```go
// usecase/set_integration_credential.go
type SetIntegrationCredential struct {
	writer CredentialWriter
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

The `internal/adapter/credentialbroker` implementation of `WriteRaw` is a
thin wrapper: build `WriteCredentialRequest{TenantID, OwnerID: provider.String(),
Category: CREDENTIAL_CATEGORY_SCM_OAUTH, EncryptedEnvelope: []byte(token), ConfigJson: configJSON}`,
call `CredentialBrokerServiceClient.WriteCredential` — the same client this
package's existing `Write`/`RevokeByOwner` methods already use (per
`revoke_auth.go`'s and `complete_oauth_flow.go`'s reliance on it).

### `issue-tracking-service`'s parallel additions

| New method | Notes |
|---|---|
| `SetIntegrationCredential` | Identical shape to scm's, `CREDENTIAL_CATEGORY_ISSUE_TRACKER_OAUTH`, `owner_id = IssueProvider.String()` |
| `GetIntegrationCredentialStatus` | Identical shape |
| `ListIntegrationCredentials` | Identical shape |
| `RevokeAuth` | **Also new** — unlike `scm-integration-service`, `issue-tracking-service` has no OAuth-flow surface at all yet (`issuetracking.proto:9-13` is only `ListIssues`/`CreateIssue`/`LinkIssue`), so this service needs its own `CredentialRevoker` port + `RevokeCredentialByOwner`-backed usecase from scratch, mirroring `scm-integration-service`'s `revoke_auth.go` verbatim |

`issue-tracking-service` currently has **no `internal/adapter/credentialbroker`
package at all** (confirmed: no OAuth/credential RPCs exist on its proto
today) — this solution's `issue-tracking-service` changes are a larger diff
than `scm-integration-service`'s (which only adds 3 RPCs onto an existing
credential-broker adapter), flagged here so scope isn't underestimated.

---

## Design — `wscompat` channel wiring

```go
// channels_credentials.go
package wscompat

import (
	"context"
	"encoding/json"
	"fmt"

	issuetrackingv1 "github.com/stablyai/orca-go/proto/gen/go/orca/issuetracking/v1"
	scmintegrationv1 "github.com/stablyai/orca-go/proto/gen/go/orca/scmintegration/v1"

	gatewaygrpc "github.com/stablyai/orca-go/services/api-gateway/internal/adapter/grpc"
	"github.com/stablyai/orca-go/services/api-gateway/internal/usecase"
)

// credentialServiceRoute is the (c) mapping table above, expressed as code.
var scmProviders = map[string]scmintegrationv1.ScmProvider{
	"bitbucket":     scmintegrationv1.ScmProvider_SCM_PROVIDER_BITBUCKET,
	"azure-devops":  scmintegrationv1.ScmProvider_SCM_PROVIDER_AZURE_DEVOPS,
	"gitea":         scmintegrationv1.ScmProvider_SCM_PROVIDER_GITEA,
}
var issueProviders = map[string]issuetrackingv1.IssueProvider{
	"linear": issuetrackingv1.IssueProvider_ISSUE_PROVIDER_LINEAR,
	"jira":   issuetrackingv1.IssueProvider_ISSUE_PROVIDER_JIRA,
}

func registerCredentialsChannels(r *Registry, scm scmintegrationv1.ScmIntegrationServiceClient, issue issuetrackingv1.IssueTrackingServiceClient) {
	r.Register("credentials.set", handleCredentialsSet(scm, issue))
	r.Register("credentials.revoke", handleCredentialsRevoke(scm, issue))
	r.Register("credentials.status", handleCredentialsStatus(scm, issue))
	r.Register("credentials.list", handleCredentialsList(scm, issue))
}

type credentialsSetArgs struct {
	Service string `json:"service"`
	Token   string `json:"token"`
	Config  any    `json:"config"`
}

// handleCredentialsSet is representative — status/revoke follow the same
// "look up which of the 2 services owns `service`, call the matching RPC"
// shape, just with a different RPC/response per verb.
func handleCredentialsSet(scm scmintegrationv1.ScmIntegrationServiceClient, issue issuetrackingv1.IssueTrackingServiceClient) ChannelHandler {
	return func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		in, err := decodeArg[credentialsSetArgs](args, 0)
		if err != nil {
			return nil, err
		}
		configJSON, err := json.Marshal(in.Config)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()

		if provider, ok := scmProviders[in.Service]; ok {
			_, err := scm.SetIntegrationCredential(rpcCtx, &scmintegrationv1.SetIntegrationCredentialRequest{
				TenantId: id.TenantID, Provider: provider, Token: in.Token, ConfigJson: string(configJSON),
			})
			return map[string]bool{"ok": err == nil}, err
		}
		if provider, ok := issueProviders[in.Service]; ok {
			_, err := issue.SetIntegrationCredential(rpcCtx, &issuetrackingv1.SetIntegrationCredentialRequest{
				TenantId: id.TenantID, Provider: provider, Token: in.Token, ConfigJson: string(configJSON),
			})
			return map[string]bool{"ok": err == nil}, err
		}
		return nil, fmt.Errorf("CREDENTIALS_UNKNOWN_SERVICE: %q is not a recognized credentials.* service", in.Service)
	}
}

// handleCredentialsList fans out to BOTH services and merges — the
// frontend's { services: string[], mode } spans all 5 providers in one call.
func handleCredentialsList(scm scmintegrationv1.ScmIntegrationServiceClient, issue issuetrackingv1.IssueTrackingServiceClient) ChannelHandler {
	return func(ctx context.Context, id Identity, _ []json.RawMessage) (any, error) {
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()

		var services []string
		scmResp, err := scm.ListIntegrationCredentials(rpcCtx, &scmintegrationv1.ListIntegrationCredentialsRequest{TenantId: id.TenantID})
		if err != nil {
			return nil, err
		}
		for _, p := range scmResp.GetConfiguredProviders() {
			services = append(services, scmProviderName(p))
		}
		issueResp, err := issue.ListIntegrationCredentials(rpcCtx, &issuetrackingv1.ListIntegrationCredentialsRequest{TenantId: id.TenantID})
		if err != nil {
			return nil, err
		}
		for _, p := range issueResp.GetConfiguredProviders() {
			services = append(services, issueProviderName(p))
		}
		return map[string]any{"services": services, "mode": "server"}, nil
	}
}
```

`handleCredentialsRevoke`/`handleCredentialsStatus` follow
`handleCredentialsSet`'s exact dispatch shape (lookup in `scmProviders`/
`issueProviders`, call the matching RPC — `RevokeAuth`/
`GetIntegrationCredentialStatus`), omitted here per this task's "one
representative sketch" guidance.

---

## Test plan

- `credential-broker-service/internal/usecase/get_credential_metadata_by_owner_test.go` —
  no credential exists → `metadata` unset, not an error; existing credential
  → metadata returned, `config_json` round-trips, no secret field anywhere
  in the response type (a compile-time guarantee, per
  `credential-broker-service.md` §9's "no field capable of holding a secret"
  discipline — assert via `reflect` that `GetCredentialMetadataByOwnerResponse`
  has no `[]byte`/`bytes`-typed field beyond what's already vetted).
- `list_credentials_by_category_test.go` — 3 credentials across 2 tenants,
  filter returns only the requesting tenant's rows.
- `scm-integration-service/internal/usecase/set_integration_credential_test.go` —
  fake `CredentialWriter`, asserts `WriteRaw` is called with
  `owner_id = provider.String()`; empty token → `SCM_NO_TOKEN`.
- `get_integration_credential_status_test.go` — not-configured vs. configured
  cases, `config_json` presence matches `configured`.
- `issue-tracking-service`: same 4 tests, plus a new `revoke_auth_test.go`
  mirroring `scm-integration-service`'s (this service has none today).
- `channels_credentials_test.go` — table-driven over all 5 `service` values
  × 4 channels (20 cases), fake `scm`/`issue` clients, asserting correct
  service dispatch; a 6th unknown `service` value → `CREDENTIALS_UNKNOWN_SERVICE`
  without calling either client.
- `credentials.list` — fake both clients returning 2 configured providers
  each, assert the merged `services` array has all 4, `mode: "server"`
  always present.
- Cross-service consistency test (`scm-integration-service` integration
  suite): `SetIntegrationCredential(bitbucket, token)` then
  `CredentialResolver.Resolve` (the path `ListIssues`/`CreatePullRequest`
  already use) returns that same token — proves the tenant-wide convergence
  this solution's mapping section claims, not just an assertion in prose.

## References

- `specs/backend-go/tdd/services/credential-broker-service.md` §2,§3,§7,§9 — broker-not-duplicate-vault rationale, existing RPC surface, per-category dependency table this solution's routing decision is grounded in
- `specs/backend-go/tdd/architecture/06-secrets-vault-architecture.md` — Vault-backed design vs. the old TS `WebCredentialStore` per-user `.enc` files; migration notes confirming this is a real architecture upgrade, not a straight port
- `backend-go/services/api-gateway/internal/domain/registry.go:79-81` — the comment this solution's routing decision directly addresses and corrects (infra-fleet-service's credential path is SSH-specific, not applicable here)
- `backend-go/proto/orca/credentialbroker/v1/credentialbroker.proto:41,49,127-133` — `ResolveCredentialByOwner`/`RevokeCredentialByOwner`/`GetCredentialMetadata`, the existing by-owner conventions this solution's 2 new RPCs extend
- `backend-go/proto/orca/scmintegration/v1/scmintegration.proto:16-27,97-159` — `GetAuthStatus`/`RevokeAuth`/`CompleteOAuthFlow`, the already-working `owner_id = provider name` convention this solution reuses
- `backend-go/services/scm-integration-service/internal/usecase/ports.go:67-166` — `CredentialResolver`/`CredentialWriter`/`CredentialRevoker`, extended by this solution
- `backend-go/services/scm-integration-service/internal/usecase/{revoke_auth,complete_oauth_flow}.go` — the exact usecase pattern `SetIntegrationCredential`/`GetIntegrationCredentialStatus` mirror
- `backend-go/proto/orca/issuetracking/v1/issuetracking.proto` — confirms no credential/auth RPCs exist on this service today (larger diff than `scm-integration-service`'s)
- `specs/backend-go/bugs/missing-v1/BUG-007-credentials-channels-not-implemented.md` — full method-by-method gap breakdown, the 2-option architecture-decision framing this solution resolves in favor of option 1
