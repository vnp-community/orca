# TASK-043: Tests for `credentials.*` — broker, `scm-integration-service`, `issue-tracking-service`, `wscompat`

**From Solution:** SOL-007 (Test plan section)
**Priority:** P2
**Service:** `credential-broker-service`, `scm-integration-service`, `issue-tracking-service`, `api-gateway`
**File:** `services/credential-broker-service/internal/usecase/{get_credential_metadata_by_owner_test.go,list_credentials_by_category_test.go}` (new), `services/scm-integration-service/internal/usecase/{set_integration_credential_test.go,get_integration_credential_status_test.go}` (new), `services/issue-tracking-service/internal/usecase/{set_integration_credential_test.go,get_integration_credential_status_test.go,revoke_auth_test.go}` (new), `services/api-gateway/internal/adapter/wscompat/channels_credentials_test.go` (new)
**Depends on:** TASK-038, TASK-040, TASK-041, TASK-042
**Status:** `[partial]` — the `issue-tracking-service` and `api-gateway` slices of this task's scope are done as part of this run (paired with TASK-041/042): `services/issue-tracking-service/internal/usecase/{set_integration_credential_test.go,get_integration_credential_status_test.go,revoke_auth_test.go,list_integration_credentials_test.go}` (new) and `services/api-gateway/internal/adapter/wscompat/channels_credentials_test.go` (new, jira/linear scope only — see TASK-042's status note). The `credential-broker-service` (`get_credential_metadata_by_owner_test.go`, `list_credentials_by_category_test.go`) and `scm-integration-service` (`set_integration_credential_test.go`, `get_integration_credential_status_test.go`) files this task's doc also names already existed on this branch before this run (from the TASK-037/038/039/040 work) — verified present and passing, not re-touched. The doc's "Cross-service consistency test (optional, integration-tier)" for `scm-integration-service`'s `SetIntegrationCredential`→`CredentialResolver.Resolve` round trip is explicitly OUT of this run's scope (issue-tracking-service + wscompat only) and is flagged here, not silently skipped — still open. `go test ./services/credential-broker-service/... ./services/scm-integration-service/... ./services/issue-tracking-service/... -v` and `go test ./services/api-gateway/internal/adapter/wscompat/... -run Credentials -v` all pass; no regressions in either full suite (`go build/vet/test ./...` clean for both `issue-tracking-service` and `api-gateway`).

---

## Changes to make

### `credential-broker-service/internal/usecase/get_credential_metadata_by_owner_test.go` (new)

- No credential exists → `Found: false`, nil error (not an error).
- Existing credential → metadata returned, `ConfigJSON` round-trips.
- Compile-time guarantee: assert via `reflect` that
  `GetCredentialMetadataByOwnerResponse` (the generated proto type) has no
  `[]byte`/`bytes`-typed field anywhere — same "no field capable of
  holding a secret" discipline `credential-broker-service.md` §9 documents:

```go
func TestGetCredentialMetadataByOwnerResponse_NoSecretField(t *testing.T) {
	typ := reflect.TypeOf(credentialbrokerv1.GetCredentialMetadataByOwnerResponse{})
	assertNoBytesField(t, typ) // walks fields recursively, including the embedded CredentialMetadata; fails on any []byte field
}
```

(`assertNoBytesField` is a small new reflection helper — walk struct
fields recursively via `reflect.Type`, fail if any field's `Kind()` is
`reflect.Slice` with `Elem().Kind() == reflect.Uint8`.)

### `list_credentials_by_category_test.go` (new)

3 credentials across 2 tenants (fake `CredentialMetadataRepository`
returning canned rows), filter returns only the requesting tenant's rows;
empty `tenant_id` → `CREDENTIAL_MISSING_SCOPE` without calling the repo.

### `scm-integration-service/internal/usecase/set_integration_credential_test.go` (new)

```go
package usecase_test

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/services/scm-integration-service/internal/domain"
	"github.com/stablyai/orca-go/services/scm-integration-service/internal/usecase"
)

type fakeCredentialWriter struct {
	writeRawFunc func(ctx context.Context, tenantID string, provider domain.ScmProvider, token, configJSON string) error
}

func (f *fakeCredentialWriter) Write(context.Context, string, domain.ScmProvider, usecase.OAuthToken) error {
	panic("not used by SetIntegrationCredential")
}
func (f *fakeCredentialWriter) WriteRaw(ctx context.Context, tenantID string, provider domain.ScmProvider, token, configJSON string) error {
	return f.writeRawFunc(ctx, tenantID, provider, token, configJSON)
}

func TestSetIntegrationCredential_CallsWriteRawWithOwnerIDConvention(t *testing.T) {
	var gotTenant, gotToken, gotConfig string
	var gotProvider domain.ScmProvider
	writer := &fakeCredentialWriter{writeRawFunc: func(_ context.Context, tenantID string, provider domain.ScmProvider, token, configJSON string) error {
		gotTenant, gotProvider, gotToken, gotConfig = tenantID, provider, token, configJSON
		return nil
	}}
	uc := usecase.NewSetIntegrationCredential(writer)

	err := uc.Execute(context.Background(), usecase.SetIntegrationCredentialInput{
		TenantID: "t1", Provider: domain.ScmProviderBitbucket, Token: "tok-123", ConfigJSON: `{"baseUrl":"https://x"}`,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotTenant != "t1" || gotProvider != domain.ScmProviderBitbucket || gotToken != "tok-123" || gotConfig != `{"baseUrl":"https://x"}` {
		t.Errorf("WriteRaw called with unexpected args: tenant=%q provider=%v token=%q config=%q", gotTenant, gotProvider, gotToken, gotConfig)
	}
}

func TestSetIntegrationCredential_EmptyToken_Errors(t *testing.T) {
	uc := usecase.NewSetIntegrationCredential(&fakeCredentialWriter{})
	err := uc.Execute(context.Background(), usecase.SetIntegrationCredentialInput{TenantID: "t1"})
	if err == nil {
		t.Fatal("expected SCM_NO_TOKEN error")
	}
}
```

(`domain.ScmProviderBitbucket` — check the actual constant name in
`domain/scm_provider.go` before using; adjust if named differently.)

### `get_integration_credential_status_test.go` (new)

Not-configured (`GetStatus` returns `false, "", nil`) vs. configured
(`true, configJSON, nil`) cases; `ConfigJSON` presence matches `Configured`.

### `issue-tracking-service`: same 2 tests (`set_integration_credential_test.go`,
`get_integration_credential_status_test.go`, mirroring scm's shape with
`domain.Provider`), plus:

`revoke_auth_test.go` (new — this service has none today):

```go
package usecase_test

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/services/issue-tracking-service/internal/domain"
	"github.com/stablyai/orca-go/services/issue-tracking-service/internal/usecase"
)

type fakeCredentialRevoker struct {
	revokeByOwnerFunc func(ctx context.Context, tenantID string, provider domain.Provider) error
}

func (f *fakeCredentialRevoker) RevokeByOwner(ctx context.Context, tenantID string, provider domain.Provider) error {
	return f.revokeByOwnerFunc(ctx, tenantID, provider)
}

func TestRevokeAuth_CallsRevokeByOwner(t *testing.T) {
	called := false
	revoker := &fakeCredentialRevoker{revokeByOwnerFunc: func(context.Context, string, domain.Provider) error {
		called = true
		return nil
	}}
	uc := usecase.NewRevokeAuth(revoker)
	if err := uc.Execute(context.Background(), usecase.RevokeAuthInput{TenantID: "t1", Provider: domain.ProviderJira}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("expected RevokeByOwner to be called")
	}
}

func TestRevokeAuth_NoTenant_Errors(t *testing.T) {
	uc := usecase.NewRevokeAuth(&fakeCredentialRevoker{})
	if err := uc.Execute(context.Background(), usecase.RevokeAuthInput{}); err == nil {
		t.Fatal("expected ISSUETRACKING_NO_TENANT error")
	}
}
```

(`domain.ProviderJira` — check the actual constant name in
`domain/issue.go` before using.)

### `services/api-gateway/internal/adapter/wscompat/channels_credentials_test.go` (new)

Table-driven over all 5 `service` values × 4 channels (20 cases): a
`fakeScmClient`/`fakeIssueClient` pair (each embedding the respective
generated interface, overriding only the 4 methods `channels_credentials.go`
calls — same pattern as `fakeInfraFleetClient` in `channels_test.go`),
asserting each `service` value dispatches to the correct client's correct
RPC. A 6th, unknown `service` value → `CREDENTIALS_UNKNOWN_SERVICE`
without calling either client:

```go
func TestCredentialsSet_UnknownService_ErrorsWithoutCallingEitherClient(t *testing.T) {
	scmCalled, issueCalled := false, false
	scm := &fakeScmClient{setIntegrationCredentialFunc: func(context.Context, *scmintegrationv1.SetIntegrationCredentialRequest) (*scmintegrationv1.SetIntegrationCredentialResponse, error) {
		scmCalled = true
		return nil, nil
	}}
	issue := &fakeIssueClient{setIntegrationCredentialFunc: func(context.Context, *issuetrackingv1.SetIntegrationCredentialRequest) (*issuetrackingv1.SetIntegrationCredentialResponse, error) {
		issueCalled = true
		return nil, nil
	}}
	r := NewRegistry()
	registerCredentialsChannels(r, scm, issue)

	args := argsJSON(t, map[string]any{"service": "gitlab-not-a-real-service", "token": "x"})
	_, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "credentials.set", args)
	if err == nil {
		t.Fatal("expected CREDENTIALS_UNKNOWN_SERVICE error")
	}
	if scmCalled || issueCalled {
		t.Error("neither client should be called for an unrecognized service")
	}
}
```

Also `credentials.list`: fake both clients returning 2 configured
providers each, assert the merged `services` array has all 4, `mode:
"server"` always present.

### Cross-service consistency test (optional, integration-tier — flag if
skipped for this pass)

`scm-integration-service` integration suite:
`SetIntegrationCredential(bitbucket, token)` then `CredentialResolver.Resolve`
(the path `ListIssues`/`CreatePullRequest` already use) returns that same
token — proves the tenant-wide convergence SOL-007's mapping section
claims, not just an assertion in prose. Requires a real or fake
`credential-broker-service` backing both calls; if no integration test
harness exists yet for this service, note that gap explicitly rather than
skipping silently.

---

## Verify

```bash
cd /opt/repos/orca/backend-go
go test ./services/credential-broker-service/... -v
go test ./services/scm-integration-service/... -v
go test ./services/issue-tracking-service/... -v
go test ./services/api-gateway/internal/adapter/wscompat/... -run Credentials -v
```

Expected: all new tests pass; no regressions in existing suites for these
4 packages.
