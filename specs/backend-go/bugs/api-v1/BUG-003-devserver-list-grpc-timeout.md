# BUG-003: `devServer.list` hits raw gRPC timeout — no per-call deadline on infra-fleet-service calls

**Service:** `api-gateway` → `infra-fleet-service`  
**Files:**
- `services/api-gateway/internal/adapter/wscompat/channels.go`
- `services/infra-fleet-service/internal/adapter/grpc/server.go`  
**Severity:** Medium — `devServer.list` hangs for the full 25s `invokeTimeout` when infra-fleet-service is unreachable or slow  
**Symptom:** "Request timed out: devServer.list" in browser console  
**Status:** ✅ Fixed (2026-08-24) — [SOL-003](./solutions/SOL-003-devserver-list-per-rpc-deadline.md), TASK-008 + TASK-009

---

## Description

The `devServer.list` channel calls `client.ListDevServers(ctx, ...)` with the dispatch
context (25s timeout from `invokeTimeout`). When `infra-fleet-service` is:
- Not yet started (still initializing),
- Running but its database is slow,
- Unreachable due to a network partition,

…the gRPC call blocks for the full 25 seconds. Combined with BUG-001 (error write on
cancelled context), the client then sees the frontend's own 30s timeout fire:

```
Uncaught (in promise) Error: Request timed out: devServer.list
```

The problem is that the `invokeTimeout` is the **only** deadline — there is no shorter
per-RPC deadline that would return a fast, meaningful error to the client while leaving
headroom for the write-back.

---

## Root Cause Chain

```
Frontend invokes devServer.list
  → handleInvoke goroutine: ctx has 25s deadline
    → devServer.list handler: calls infra-fleet-service.ListDevServers(ctx)
      → gRPC call hangs (infra-fleet-service unreachable)
        → ctx reaches 25s deadline, gRPC returns DeadlineExceeded
    → handleInvoke tries to write ErrorMessage with cancelled ctx (BUG-001)
      → write silently dropped
  → Frontend 30s timer fires: "Request timed out: devServer.list"
```

---

## Secondary Issue: No gRPC deadline propagated to infra-fleet-service usecase

`infra-fleet-service`'s `ListDevServers` usecase does a database query. The incoming
gRPC `ctx` deadline is passed through to the query, but **only** if the database
driver respects the context. If the driver blocks ignoring the context, the RPC will not
return until the database itself times out (which may be longer than the 25s gRPC
deadline).

---

## Fix

### Short-term: Add per-channel RPC deadline

Add a shorter sub-deadline inside the `devServer.list` handler so downstream failures
are caught quickly, leaving time for the write-back before the dispatch ctx expires:

```go
r.Register("devServer.list", func(ctx context.Context, id Identity, _ []json.RawMessage) (any, error) {
    ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
    // Per-RPC deadline: 8s gives infra-fleet-service time to respond while
    // staying well under the 25s invokeTimeout so write-back still works.
    rpcCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
    defer cancel()
    resp, err := client.ListDevServers(rpcCtx, &infrafleetv1.ListDevServersRequest{})
    if err != nil {
        return nil, err
    }
    // ... map response
})
```

### Long-term: Fix BUG-001 first

Once BUG-001 is fixed (write-back uses a fresh context), the 25s `invokeTimeout` being
the only deadline is less catastrophic — the error message will actually reach the
client. The per-channel sub-deadline is still good practice for fast failure.

---

## References

- `services/api-gateway/internal/adapter/wscompat/channels.go` line 363–375 — `devServer.list` handler
- `services/api-gateway/internal/adapter/wscompat/handler.go` line 125 — `invokeTimeout = 25 * time.Second`
- `services/infra-fleet-service/internal/adapter/grpc/server.go` line 85 — `ListDevServers`
