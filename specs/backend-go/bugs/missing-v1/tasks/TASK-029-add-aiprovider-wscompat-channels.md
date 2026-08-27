# TASK-029: Add `aiProvider.*` wscompat channels

**From Solution:** SOL-005 (`wscompat` channel wiring section)
**Priority:** P1
**Service:** `api-gateway`
**File:** `services/api-gateway/internal/adapter/wscompat/channels_ai_provider.go` (new), `channels.go`, `cmd/server/main.go`
**Depends on:** TASK-024, TASK-026, TASK-027, TASK-028
**Status:** `[x]` DONE — `channels_ai_provider.go` wires all 6 channels (`aiProvider.create/list/update/delete/writeCredential/testConnection`) to `aiproviderv1.AiProviderServiceClient`, following this package's `decodeArg`/`AttachIdentity`/`rpcTimeout` conventions. `channels.go` and `main.go` were deliberately left untouched per this batch's shared-file-avoidance convention — see this task's companion report for the exact `registerAiProviderChannels(r, aiProviderClient)` call + `RegisterRealChannels` signature addition the integration pass still needs to make.

---

## Context

6 channels: `aiProvider.create` (existing `CreateAccount` RPC — pure
channel-wiring gap, mirrors `handleCreateAccount` in
`ai_provider_routes.go`), `aiProvider.list`/`update`/`delete` (Group B,
TASK-026), `aiProvider.writeCredential` (Group C, TASK-027),
`aiProvider.testConnection` (Group D, TASK-028 — inert until the agent
implements `ai.testProviderConnection`, see TASK-028's Context; this
channel wiring itself is fully buildable and correct regardless).

`main.go` already dials `aiProviderClient` (`aiProviderClient :=
aiproviderv1.NewAiProviderServiceClient(aiProviderConn)`) but does not pass
it into `wscompat.RegisterRealChannels` yet — this task adds that
parameter, the same kind of signature change TASK-005/TASK-006 made for
`rateLimits`.

---

## Changes to make

### New file: `services/api-gateway/internal/adapter/wscompat/channels_ai_provider.go`

```go
// Package wscompat — aiProvider.* channels. See SOL-005
// (specs/backend-go/bugs/missing-v1/solutions/SOL-005-aiprovider-channels.md)
// for the full design this file wires up.
package wscompat

import (
	"context"
	"encoding/json"

	aiproviderv1 "github.com/stablyai/orca-go/proto/gen/go/orca/aiprovider/v1"

	gatewaygrpc "github.com/stablyai/orca-go/services/api-gateway/internal/adapter/grpc"
	"github.com/stablyai/orca-go/services/api-gateway/internal/usecase"
)

func registerAIProviderChannels(r *Registry, client aiproviderv1.AiProviderServiceClient) {
	r.Register("aiProvider.create", handleAIProviderCreate(client))
	r.Register("aiProvider.list", handleAIProviderList(client))
	r.Register("aiProvider.update", handleAIProviderUpdate(client))
	r.Register("aiProvider.delete", handleAIProviderDelete(client))
	r.Register("aiProvider.writeCredential", handleAIProviderWriteCredential(client))
	r.Register("aiProvider.testConnection", handleAIProviderTestConnection(client))
}

// attachAIProviderIdentity is shared by every handler below —
// ai-provider-service's usecases require tenant via ctx
// (tenant.RequireTenantID), same AttachIdentity requirement as
// devServer.*/fleet.* (see channels.go's doc comment on that section).
func attachAIProviderIdentity(ctx context.Context, id Identity) (context.Context, context.CancelFunc) {
	ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
	return context.WithTimeout(ctx, rpcTimeout)
}

func handleAIProviderCreate(client aiproviderv1.AiProviderServiceClient) ChannelHandler {
	return func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type createArgs struct {
			Type string `json:"type"`
		}
		in, err := decodeArg[createArgs](args, 0)
		if err != nil {
			return nil, err
		}
		rpcCtx, cancel := attachAIProviderIdentity(ctx, id)
		defer cancel()
		resp, err := client.CreateAccount(rpcCtx, &aiproviderv1.CreateAccountRequest{
			TenantId: id.TenantID,
			Type:     aiproviderv1.ProviderType(aiproviderv1.ProviderType_value[in.Type]),
		})
		if err != nil {
			return nil, err
		}
		return resp.GetAccount(), nil
	}
}

func handleAIProviderList(client aiproviderv1.AiProviderServiceClient) ChannelHandler {
	return func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type listArgs struct {
			DevServerID string `json:"devServerId"`
		}
		in, err := decodeArg[listArgs](args, 0)
		if err != nil {
			return nil, err
		}
		rpcCtx, cancel := attachAIProviderIdentity(ctx, id)
		defer cancel()
		resp, err := client.ListAccounts(rpcCtx, &aiproviderv1.ListAccountsRequest{DevServerId: in.DevServerID})
		if err != nil {
			return nil, err
		}
		return resp.GetAccounts(), nil
	}
}

func handleAIProviderUpdate(client aiproviderv1.AiProviderServiceClient) ChannelHandler {
	return func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type updateArgs struct {
			AccountID string `json:"accountId"`
			Label     string `json:"label"`
			ModelHint string `json:"modelHint"`
			BaseURL   string `json:"baseUrl"`
		}
		in, err := decodeArg[updateArgs](args, 0)
		if err != nil {
			return nil, err
		}
		rpcCtx, cancel := attachAIProviderIdentity(ctx, id)
		defer cancel()
		resp, err := client.UpdateAccount(rpcCtx, &aiproviderv1.UpdateAccountRequest{
			AccountId: in.AccountID, Label: in.Label, ModelHint: in.ModelHint, BaseUrl: in.BaseURL,
		})
		if err != nil {
			return nil, err
		}
		return resp.GetAccount(), nil
	}
}

func handleAIProviderDelete(client aiproviderv1.AiProviderServiceClient) ChannelHandler {
	return func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type deleteArgs struct {
			AccountID string `json:"accountId"`
		}
		in, err := decodeArg[deleteArgs](args, 0)
		if err != nil {
			return nil, err
		}
		rpcCtx, cancel := attachAIProviderIdentity(ctx, id)
		defer cancel()
		if _, err := client.DeleteAccount(rpcCtx, &aiproviderv1.DeleteAccountRequest{AccountId: in.AccountID}); err != nil {
			return nil, err
		}
		// matches annotation.delete's response shape (channels.go)
		return map[string]bool{"ok": true}, nil
	}
}

func handleAIProviderWriteCredential(client aiproviderv1.AiProviderServiceClient) ChannelHandler {
	return func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type writeCredentialArgs struct {
			AccountID     string `json:"accountId"`
			EncryptedBlob string `json:"encryptedBlob"` // base64 in the JSON envelope
			IV            string `json:"iv"`             // base64 in the JSON envelope
		}
		in, err := decodeArg[writeCredentialArgs](args, 0)
		if err != nil {
			return nil, err
		}
		blob, err := base64.StdEncoding.DecodeString(in.EncryptedBlob)
		if err != nil {
			return nil, fmt.Errorf("decoding encryptedBlob: %w", err)
		}
		iv, err := base64.StdEncoding.DecodeString(in.IV)
		if err != nil {
			return nil, fmt.Errorf("decoding iv: %w", err)
		}
		rpcCtx, cancel := attachAIProviderIdentity(ctx, id)
		defer cancel()
		resp, err := client.WriteCredential(rpcCtx, &aiproviderv1.WriteCredentialRequest{
			AccountId: in.AccountID, EncryptedBlob: blob, Iv: iv,
		})
		if err != nil {
			return nil, err
		}
		return resp.GetAccount(), nil
	}
}

func handleAIProviderTestConnection(client aiproviderv1.AiProviderServiceClient) ChannelHandler {
	return func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type testConnectionArgs struct {
			AccountID string `json:"accountId"`
			TraceID   string `json:"traceId"`
		}
		in, err := decodeArg[testConnectionArgs](args, 0)
		if err != nil {
			return nil, err
		}
		rpcCtx, cancel := attachAIProviderIdentity(ctx, id)
		defer cancel()
		resp, err := client.TestConnection(rpcCtx, &aiproviderv1.TestConnectionRequest{
			AccountId: in.AccountID, TraceId: in.TraceID,
		})
		if err != nil {
			return nil, err
		}
		return map[string]any{"success": resp.GetSuccess(), "message": resp.GetMessage()}, nil
	}
}
```

Add `"encoding/base64"` and `"fmt"` to this file's import block (both used
above).

### `channels.go` — extend `RegisterRealChannels`

```go
func RegisterRealChannels(
	r *Registry,
	annotationClient annotationv1.AnnotationServiceClient,
	taskClient taskv1.TaskServiceClient,
	gitClient gitgatewayv1.GitGatewayServiceClient,
	automationClient automationv1.AutomationServiceClient,
	infraFleetClient infrafleetv1.InfraFleetServiceClient,
	aiProviderClient aiproviderv1.AiProviderServiceClient, // NEW
	rateLimits rateLimitReader,
) {
	registerAnnotationChannels(r, annotationClient)
	registerTaskChannels(r, taskClient)
	registerGitChannels(r, gitClient)
	registerAutomationChannels(r, automationClient)
	registerPreflightChannels(r)
	registerDevServerChannels(r, infraFleetClient)
	registerFleetChannels(r, infraFleetClient)
	registerAccountsChannels(r, infraFleetClient)   // TASK-021
	registerAIProviderChannels(r, aiProviderClient) // NEW
	registerCrashReportChannels(r)
	registerRateLimitChannels(r, rateLimits)
}
```

Add `aiproviderv1 "github.com/stablyai/orca-go/proto/gen/go/orca/aiprovider/v1"`
to `channels.go`'s import block.

### `cmd/server/main.go` — pass `aiProviderClient` through

Find the existing call site:

```go
wscompat.RegisterRealChannels(wsCompatRegistry, annotationClient, taskClient, gitClient, automationClient, infraFleetClient, rateLimiter)
```

Replace with:

```go
wscompat.RegisterRealChannels(wsCompatRegistry, annotationClient, taskClient, gitClient, automationClient, infraFleetClient, aiProviderClient, rateLimiter)
```

`aiProviderClient` is already dialed earlier in `main.go` — no new dial
needed, just threading the existing variable into this call.

---

## Verify

```bash
cd /opt/repos/orca/backend-go/services/api-gateway
go build ./... && go vet ./...
```

Expected: clean build across `wscompat` and `cmd/server`.
