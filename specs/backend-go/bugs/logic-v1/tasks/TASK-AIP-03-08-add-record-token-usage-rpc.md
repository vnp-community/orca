# TASK-AIP-03-08: Add `RecordTokenUsage` gRPC RPC (service-to-service only, no REST/WS)

**From Solution:** SOL-AIP-03
**Priority:** P1
**Service:** `ai-provider-service` proto
**File:** `backend-go/proto/orca/aiprovider/v1/aiprovider.proto`
**Depends on:** TASK-AIP-03-05
**Status:** `[ ]` TODO

---

## Context

`RecordTokenUsage` (`TASK-AIP-03-05`) has no wire surface yet. Per §7's
"Called by" table this RPC is called only by `task-service`/
`workflow-service` right after a spawn completes — never from a
browser/mobile client — so it deliberately gets no `httpgateway`/
`wscompat` route, matching how `PushCiphertext` and other internal-only
RPCs in this catalog stay off the gateway entirely.

## Changes to make

In `backend-go/proto/orca/aiprovider/v1/aiprovider.proto`, add to the
service block:

```protobuf
service AiProviderService {
  // ... existing RPCs ...
  rpc RecordTokenUsage(RecordTokenUsageRequest) returns (RecordTokenUsageResponse);
}
```

Add messages:

```protobuf
message RecordTokenUsageRequest {
  string account_id    = 1;
  int64  tokens_used   = 2;
  int64  request_count = 3;
  double cost_usd      = 4;
}
message RecordTokenUsageResponse {
  int64  tokens_used   = 1;
  double cost_usd      = 2;
  int64  request_count = 3;
}
```

## Regenerate stubs

```bash
cd /opt/repos/orca/backend-go
buf generate proto
buf breaking proto --against '.git#branch=main'
```

Wire the handler in
`backend-go/services/ai-provider-service/internal/adapter/grpc/server.go`
— add `recordTokenUsage *usecase.RecordTokenUsage` to `Server`, thread it
through `New(...)`'s parameter list, and add:

```go
func (s *Server) RecordTokenUsage(ctx context.Context, req *aiproviderv1.RecordTokenUsageRequest) (*aiproviderv1.RecordTokenUsageResponse, error) {
	state, err := s.recordTokenUsage.Execute(ctx, usecase.RecordTokenUsageInput{
		AccountID:    req.GetAccountId(),
		TokensUsed:   req.GetTokensUsed(),
		RequestCount: req.GetRequestCount(),
		CostUSD:      req.GetCostUsd(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &aiproviderv1.RecordTokenUsageResponse{
		TokensUsed: state.TokensUsed, CostUsd: state.CostUSD, RequestCount: state.RequestCount,
	}, nil
}
```

Update `cmd/server/main.go`'s `aiprovidergrpc.New(...)` call to pass
`recordTokenUsageUC` (constructed in `TASK-AIP-03-07`) as the new trailing
argument.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./...
go test ./services/ai-provider-service/internal/adapter/grpc/... -run TestRecordTokenUsage
```

Expected: `buf breaking` reports only additions; add a gRPC-layer test
asserting `RecordTokenUsageRequest` fields thread into
`RecordTokenUsageInput` unmodified and the response mirrors
`domain.QuotaState`'s three fields verbatim.
