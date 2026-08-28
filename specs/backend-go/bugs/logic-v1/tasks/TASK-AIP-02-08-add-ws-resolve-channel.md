# TASK-AIP-02-08: Register the missing `aiProvider.resolve` WS channel

**From Solution:** SOL-AIP-02
**Priority:** P1 — pure wiring gap, no usecase logic; every other `aiProvider.*` channel already exists
**Service:** `api-gateway`
**File:** `backend-go/services/api-gateway/internal/adapter/wscompat/channels_ai_provider.go`
**Depends on:** TASK-AIP-02-01, TASK-AIP-02-05, TASK-AIP-02-06
**Status:** `[x]` DONE — registered `aiProvider.resolve` and added `handleAiProviderResolve` in channels_ai_provider.go; added `TestAiProviderResolveChannel_Success`/`_PropagatesError` to channels_ai_provider_test.go (matches `-run TestAiProviderResolve`) — both pass.

---

## Context

`channels_ai_provider.go` registers 6 `aiProvider.*` channels
(`registerAiProviderChannels`, `channels_ai_provider.go:23-30`) —
`create`/`list`/`update`/`delete`/`writeCredential`/`testConnection` — but
never `resolve`, so no WS client can call `ResolveProvider` at all today.
This closes that gap, following `handleAiProviderCreate`'s exact shape.

## Changes to make

In
`backend-go/services/api-gateway/internal/adapter/wscompat/channels_ai_provider.go`,
register the new channel:

```go
func registerAiProviderChannels(r *Registry, client aiproviderv1.AiProviderServiceClient) {
	r.Register("aiProvider.create", handleAiProviderCreate(client))
	r.Register("aiProvider.list", handleAiProviderList(client))
	r.Register("aiProvider.update", handleAiProviderUpdate(client))
	r.Register("aiProvider.delete", handleAiProviderDelete(client))
	r.Register("aiProvider.writeCredential", handleAiProviderWriteCredential(client))
	r.Register("aiProvider.testConnection", handleAiProviderTestConnection(client))
	r.Register("aiProvider.resolve", handleAiProviderResolve(client)) // NEW
}
```

Add the handler:

```go
func handleAiProviderResolve(client aiproviderv1.AiProviderServiceClient) ChannelHandler {
	return func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type resolveArgs struct {
			UserID      string `json:"userId"`
			ProjectID   string `json:"projectId"`
			DevServerID string `json:"devServerId"`
			ModelHint   string `json:"modelHint"`
			AccountID   string `json:"accountId"`
			ScopedRef   string `json:"scopedRef"`
		}
		in, err := decodeArg[resolveArgs](args, 0)
		if err != nil {
			return nil, err
		}
		rpcCtx, cancel := attachAiProviderIdentity(ctx, id)
		defer cancel()
		resp, err := client.ResolveProvider(rpcCtx, &aiproviderv1.ResolveProviderRequest{
			TenantId:    id.TenantID,
			UserId:      in.UserID,
			ProjectId:   in.ProjectID,
			DevServerId: in.DevServerID,
			ModelHint:   in.ModelHint,
			AccountId:   in.AccountID,
			ScopedRef:   in.ScopedRef,
		})
		if err != nil {
			return nil, err
		}
		return resp.GetAccount(), nil
	}
}
```

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./...
go test ./services/api-gateway/internal/adapter/wscompat/... -run TestAiProviderResolve
```

Add `TestAiProviderResolve` to `channels_ai_provider_test.go` using a fake
`AiProviderServiceClient`, matching the existing 6 tests' shape in that
file (assert the channel is registered, decodes args, and forwards the
response's `account` field verbatim).
