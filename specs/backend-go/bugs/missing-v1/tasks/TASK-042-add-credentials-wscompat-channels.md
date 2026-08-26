# TASK-042: Add `credentials.*` wscompat channels (fan-out to `scm-integration-service`/`issue-tracking-service`)

**From Solution:** SOL-007 (`wscompat` channel wiring section)
**Priority:** P1
**Service:** `api-gateway`
**File:** `services/api-gateway/internal/adapter/wscompat/channels_credentials.go` (new), `channels.go`, `cmd/server/main.go`
**Depends on:** TASK-039, TASK-040, TASK-041
**Status:** `[x]` DONE — `channels_credentials.go` now fans out to both backing services: `registerCredentialsChannels(r *Registry, scmClient scmintegrationv1.ScmIntegrationServiceClient, issueTrackingClient issuetrackingv1.IssueTrackingServiceClient)` wires `credentials.set`/`credentials.status`/`credentials.list`/`credentials.revoke` for all 5 providers — jira/linear against issue-tracking-service (unchanged from the prior pass) and bitbucket/azure-devops/gitea against `scm-integration-service`'s already-implemented `SetIntegrationCredential`/`GetIntegrationCredentialStatus`/`ListIntegrationCredentials`/`RevokeAuth` RPCs (`scm-integration-service/internal/adapter/grpc/server.go`), added via a new `scmCredentialProviders` map alongside the existing `issueCredentialProviders` one, following the same decode → provider-lookup → RPC → response-shape pattern each handler already used. `channels.go`'s single call site (`registerCredentialsChannels(r, scmClient, issueTrackingClient)`) now passes the `scmClient` param `RegisterRealChannels` already had dialed for other channel groups — no new client dial, no `main.go` change needed. `credentials.list` now fans out to BOTH services unconditionally and merges, matching the frontend's single-call-spans-all-5-providers contract. Response shapes unchanged (`{success: bool}` for set/revoke, `{configured, mode, config?}` for status, `{services, mode}` for list) — verified still correct against `frontend/src/preload/api-types.ts`. `channels_credentials_test.go` extended with bitbucket/azure-devops/gitea set/revoke/status cases plus a merged-across-both-services list case and an updated unknown-service case (using `github`/`gitlab` — real `ScmProvider` values NOT in `scmCredentialProviders`, since bitbucket/azure-devops/gitea are no longer unknown); `channels_scm_test.go`'s shared `fakeScmIntegrationClient` gained the 3 new RPC fields/methods. `go build/vet/test ./...` clean for `api-gateway`. Also verified for real (not just fakes) as part of TASK-043's cross-service test below — a genuine live-stack round trip through this exact code caught and led to fixing a real, previously-latent `credential-broker-service` schema bug (`owner_id` was `UUID`-typed but every real caller writes a provider name or composite string; see TASK-043's status note and `credential-broker-service/migrations/0003_owner_id_text.up.sql`).

---

## Context

**No new `registry.go` rule** — per SOL-007's routing decision,
`credential-broker-service` is reached only indirectly via the owning
domain services (`scm-integration-service` for bitbucket/azure-devops/
gitea, `issue-tracking-service` for linear/jira), never directly from
`api-gateway`. `credentials.*`'s 4 channels reuse the existing gRPC clients
`api-gateway` already dials for `/v1/scm` and `/v1/issues`, fanning out per
`service` param to whichever of the two owns it.

`main.go` already dials `scmClient`/`issueTrackingClient` (used for REST
routes) but does not pass them into `wscompat.RegisterRealChannels` yet —
this task adds both as new parameters.

`mode: "server"` is hardcoded on every response — see SOL-007's "`mode`
field" section: a request reaching `wscompat`'s `credentials.*` handlers
at all is, by construction, never running in Electron's local IPC path.

---

## Changes to make

### New file: `services/api-gateway/internal/adapter/wscompat/channels_credentials.go`

```go
// Package wscompat — credentials.* channels. See SOL-007
// (specs/backend-go/bugs/missing-v1/solutions/SOL-007-credentials-channels.md)
// for the routing decision (fan-out to the owning domain service, no direct
// api-gateway -> credential-broker-service call).
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

// scmProviders/issueProviders is the (service string) <-> (owning service,
// provider enum) mapping table from SOL-007 §(c), expressed as code.
var scmProviders = map[string]scmintegrationv1.ScmProvider{
	"bitbucket":    scmintegrationv1.ScmProvider_SCM_PROVIDER_BITBUCKET,
	"azure-devops": scmintegrationv1.ScmProvider_SCM_PROVIDER_AZURE_DEVOPS,
	"gitea":        scmintegrationv1.ScmProvider_SCM_PROVIDER_GITEA,
}
var issueProviders = map[string]issuetrackingv1.IssueProvider{
	"linear": issuetrackingv1.IssueProvider_ISSUE_PROVIDER_LINEAR,
	"jira":   issuetrackingv1.IssueProvider_ISSUE_PROVIDER_JIRA,
}

func scmProviderName(p scmintegrationv1.ScmProvider) string {
	for name, v := range scmProviders {
		if v == p {
			return name
		}
	}
	return ""
}
func issueProviderName(p issuetrackingv1.IssueProvider) string {
	for name, v := range issueProviders {
		if v == p {
			return name
		}
	}
	return ""
}

func registerCredentialsChannels(r *Registry, scm scmintegrationv1.ScmIntegrationServiceClient, issue issuetrackingv1.IssueTrackingServiceClient) {
	r.Register("credentials.set", handleCredentialsSet(scm, issue))
	r.Register("credentials.revoke", handleCredentialsRevoke(scm, issue))
	r.Register("credentials.status", handleCredentialsStatus(scm, issue))
	r.Register("credentials.list", handleCredentialsList(scm, issue))
}

type credentialsServiceArgs struct {
	Service string `json:"service"`
}

type credentialsSetArgs struct {
	Service string `json:"service"`
	Token   string `json:"token"`
	Config  any    `json:"config"`
}

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

func handleCredentialsRevoke(scm scmintegrationv1.ScmIntegrationServiceClient, issue issuetrackingv1.IssueTrackingServiceClient) ChannelHandler {
	return func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		in, err := decodeArg[credentialsServiceArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()

		if provider, ok := scmProviders[in.Service]; ok {
			_, err := scm.RevokeAuth(rpcCtx, &scmintegrationv1.RevokeAuthRequest{TenantId: id.TenantID, Provider: provider})
			return map[string]bool{"ok": err == nil}, err
		}
		if provider, ok := issueProviders[in.Service]; ok {
			_, err := issue.RevokeAuth(rpcCtx, &issuetrackingv1.RevokeAuthRequest{TenantId: id.TenantID, Provider: provider})
			return map[string]bool{"ok": err == nil}, err
		}
		return nil, fmt.Errorf("CREDENTIALS_UNKNOWN_SERVICE: %q is not a recognized credentials.* service", in.Service)
	}
}

func handleCredentialsStatus(scm scmintegrationv1.ScmIntegrationServiceClient, issue issuetrackingv1.IssueTrackingServiceClient) ChannelHandler {
	return func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		in, err := decodeArg[credentialsServiceArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()

		if provider, ok := scmProviders[in.Service]; ok {
			resp, err := scm.GetIntegrationCredentialStatus(rpcCtx, &scmintegrationv1.GetIntegrationCredentialStatusRequest{TenantId: id.TenantID, Provider: provider})
			if err != nil {
				return nil, err
			}
			return map[string]any{"configured": resp.GetConfigured(), "mode": "server"}, nil
		}
		if provider, ok := issueProviders[in.Service]; ok {
			resp, err := issue.GetIntegrationCredentialStatus(rpcCtx, &issuetrackingv1.GetIntegrationCredentialStatusRequest{TenantId: id.TenantID, Provider: provider})
			if err != nil {
				return nil, err
			}
			return map[string]any{"configured": resp.GetConfigured(), "mode": "server"}, nil
		}
		return nil, fmt.Errorf("CREDENTIALS_UNKNOWN_SERVICE: %q is not a recognized credentials.* service", in.Service)
	}
}

// handleCredentialsList fans out to BOTH services and merges — the
// frontend's { services, mode } spans all 5 providers in one call.
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

### `channels.go` — extend `RegisterRealChannels`

```go
func RegisterRealChannels(
	r *Registry,
	annotationClient annotationv1.AnnotationServiceClient,
	taskClient taskv1.TaskServiceClient,
	gitClient gitgatewayv1.GitGatewayServiceClient,
	automationClient automationv1.AutomationServiceClient,
	infraFleetClient infrafleetv1.InfraFleetServiceClient,
	aiProviderClient aiproviderv1.AiProviderServiceClient,
	scmClient scmintegrationv1.ScmIntegrationServiceClient,        // NEW
	issueTrackingClient issuetrackingv1.IssueTrackingServiceClient, // NEW
	rateLimits rateLimitReader,
) {
	registerAnnotationChannels(r, annotationClient)
	registerTaskChannels(r, taskClient)
	registerGitChannels(r, gitClient)
	registerAutomationChannels(r, automationClient)
	registerPreflightChannels(r)
	registerDevServerChannels(r, infraFleetClient)
	registerFleetChannels(r, infraFleetClient)
	registerAccountsChannels(r, infraFleetClient)
	registerBrowserProfileChannels(r, infraFleetClient)
	registerBrowserChannels(r, infraFleetClient)
	registerAIProviderChannels(r, aiProviderClient)
	registerCredentialsChannels(r, scmClient, issueTrackingClient) // NEW
	registerCrashReportChannels(r)
	registerRateLimitChannels(r, rateLimits)
}
```

Add `issuetrackingv1 "github.com/stablyai/orca-go/proto/gen/go/orca/issuetracking/v1"`
and `scmintegrationv1 "github.com/stablyai/orca-go/proto/gen/go/orca/scmintegration/v1"`
to `channels.go`'s import block.

### `cmd/server/main.go` — pass `scmClient`/`issueTrackingClient` through

Find the `wscompat.RegisterRealChannels(...)` call site (already extended
by TASK-029 to include `aiProviderClient`) and append the 2 existing,
already-dialed client variables:

```go
wscompat.RegisterRealChannels(wsCompatRegistry, annotationClient, taskClient, gitClient, automationClient, infraFleetClient, aiProviderClient, scmClient, issueTrackingClient, rateLimiter)
```

Both `scmClient` and `issueTrackingClient` are already dialed earlier in
`main.go` for the REST routes (`deps.SCMClient`/`deps.IssueTrackingClient`)
— no new dial needed.

---

## Verify

```bash
cd /opt/repos/orca/backend-go/services/api-gateway
go build ./... && go vet ./...
```

Expected: clean build across `wscompat` and `cmd/server`.
