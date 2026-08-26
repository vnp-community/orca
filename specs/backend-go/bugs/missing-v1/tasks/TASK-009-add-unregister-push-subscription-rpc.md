# TASK-009: Add `UnregisterPushSubscription` RPC to `notification-service`

**From Solution:** SOL-003
**Priority:** P1
**Service:** `notification-service`
**File:** `proto/orca/notification/v1/notification.proto`, `services/notification-service/internal/usecase/unregister_push_subscription.go` (new), `internal/adapter/postgres/repository.go`
**Depends on:** none
**Status:** `[ ]` TODO

---

## Changes to make

### Proto (additive)

```protobuf
message UnregisterPushSubscriptionRequest {
  string endpoint = 1; // matches push_subscriptions.endpoint's unique index
}
rpc UnregisterPushSubscription(UnregisterPushSubscriptionRequest) returns (google.protobuf.Empty);
```

Regenerate: `cd /opt/repos/orca/backend-go && buf generate proto && buf breaking proto --against '.git#branch=main'`

### Usecase

```go
// internal/usecase/unregister_push_subscription.go
func (uc *PushUseCase) UnregisterPushSubscription(ctx context.Context, endpoint string) error {
    return uc.repo.DeleteByEndpoint(ctx, endpoint) // idempotent — deleting an already-gone subscription is not an error
}
```

### Repository

```go
func (r *Repository) DeleteByEndpoint(ctx context.Context, endpoint string) error {
    _, err := r.db.ExecContext(ctx, `DELETE FROM notification.push_subscriptions WHERE endpoint = $1`, endpoint)
    return err // DELETE affecting 0 rows is not an error — idempotent by design
}
```

### gRPC server wiring

Add the RPC handler in `internal/adapter/grpc/server.go`, next to the
existing `Subscribe`/`GetVapidPublicKey` handlers.

## Verify

```bash
cd /opt/repos/orca/backend-go/services/notification-service
go build ./... && go vet ./...
```
