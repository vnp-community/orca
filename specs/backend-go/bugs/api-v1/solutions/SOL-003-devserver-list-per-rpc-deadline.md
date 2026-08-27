# SOL-003: Fix BUG-003 — Per-RPC deadline cho `devServer.list`

**Resolves:** BUG-003  
**Service:** `api-gateway`  
**Affected file:** `services/api-gateway/internal/adapter/wscompat/channels.go`  
**Status:** ✅ IMPLEMENTED (2026-08-24) — TASK-008 + TASK-009

---

## Implementation Notes

Đã áp dụng đúng như thiết kế. Thay đổi thực tế trong `channels.go`:

- Thêm `const rpcTimeout = 8 * time.Second`
- `registerDevServerChannels` / `devServer.list`: thêm `rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)` trước `ListDevServers`
- `registerDevServerChannels` / `devServer.add`: thêm `rpcCtx, cancel` trước `RegisterDevServer`
- `registerFleetChannels` / `fleet.health.checkAll`: thêm `rpcCtx, cancel` trước `GetFleetHealth`

Tests thêm mới trong `channels_test.go`:
- `TestRPCTimeoutConstant_ShorterThanInvokeTimeout` ✅
- `TestDevServerListChannel_FailsFastWhenServiceSlow` ✅ (pass ~8s)
- `TestDevServerAddChannel_FailsFastWhenServiceSlow` ✅ (pass ~8s)
- `TestFleetHealthCheckAll_FailsFastWhenServiceSlow` ✅ (pass ~8s)

## Solution Design

Thêm một per-RPC `context.WithTimeout` ngắn hơn bên trong `devServer.list` handler,
độc lập với `invokeTimeout`. Mục tiêu:

1. Fail fast khi `infra-fleet-service` không phản hồi — thay vì chờ đến 25s.
2. Trả về error message có ý nghĩa (`DeadlineExceeded: infra-fleet-service took too long`)
   thay vì để frontend timeout 30s.
3. Sau khi BUG-001 được fix, error này sẽ thực sự đến được frontend.

**Deadline hierarchy:**

```
invokeTimeout (25s) — transport layer, handler.go
  └── rpcTimeout (8s) — per-RPC, channels.go (NEW)
        └── infra-fleet-service.ListDevServers gRPC call
```

Tương tự áp dụng cho `devServer.add` và `fleet.health.checkAll` — cùng pattern,
cùng vấn đề.

**Thời gian 8s** được chọn vì:
- Đủ để `infra-fleet-service` xử lý DB query trong điều kiện bình thường (< 100ms).
- Đủ dài để tránh false positive khi service đang khởi động.
- Để lại 17s margin trước khi `invokeTimeout` (25s) hết — đủ thời gian cho write-back.

---

## Code Change — `wscompat/channels.go`

### Thêm constant (sau dòng package declaration, trước các imports)

```go
// rpcTimeout is the per-RPC deadline applied to each outbound gRPC call inside
// a channel handler. Shorter than handler.go's invokeTimeout (25s) so a slow
// or unreachable downstream service fails fast with a meaningful error message
// rather than occupying the full dispatch window. The gap (invokeTimeout -
// rpcTimeout = 17s) is writeCtx's budget in SOL-001.
const rpcTimeout = 8 * time.Second
```

### Cập nhật `registerDevServerChannels` — `devServer.list`

```go
func registerDevServerChannels(r *Registry, client infrafleetv1.InfraFleetServiceClient) {
    r.Register("devServer.list", func(ctx context.Context, id Identity, _ []json.RawMessage) (any, error) {
        ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
        // Per-RPC deadline: fail fast so the write-back (SOL-001) still has
        // time to deliver the error to the frontend before invokeTimeout.
        rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
        defer cancel()
        resp, err := client.ListDevServers(rpcCtx, &infrafleetv1.ListDevServersRequest{})
        if err != nil {
            return nil, err
        }
        views := make([]devServerView, 0, len(resp.GetDevServers()))
        for _, ds := range resp.GetDevServers() {
            views = append(views, toDevServerView(ds))
        }
        return views, nil
    })

    r.Register("devServer.add", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
        type addArgs struct {
            Name           string `json:"name"`
            ConnectionType string `json:"connectionType"`
            SSHTargetID    string `json:"sshTargetId"`
            WSUrl          string `json:"wsUrl"`
        }
        in, err := decodeArg[addArgs](args, 0)
        if err != nil {
            return nil, err
        }
        ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
        // Per-RPC deadline (same reasoning as devServer.list above).
        rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
        defer cancel()
        resp, err := client.RegisterDevServer(rpcCtx, &infrafleetv1.RegisterDevServerRequest{
            TenantId: id.TenantID,
            Host:     devServerHost(in.WSUrl, in.SSHTargetID, in.Name),
            Mode:     toConnectionMode(in.ConnectionType),
        })
        if err != nil {
            return nil, err
        }
        return toDevServerView(resp.GetDevServer()), nil
    })
}
```

### Cập nhật `registerFleetChannels` — `fleet.health.checkAll`

```go
func registerFleetChannels(r *Registry, client infrafleetv1.InfraFleetServiceClient) {
    r.Register("fleet.health.checkAll", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
        type checkAllArgs struct {
            ServerIDs []string `json:"serverIds"`
        }
        in, err := decodeArg[checkAllArgs](args, 0)
        if err != nil {
            return nil, err
        }

        ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
        // Per-RPC deadline: GetFleetHealth involves active health checks to
        // dev servers which can be slow — 8s allows for reasonable network
        // latency while still failing before invokeTimeout.
        rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
        defer cancel()
        resp, err := client.GetFleetHealth(rpcCtx, &infrafleetv1.GetFleetHealthRequest{TenantId: id.TenantID})
        if err != nil {
            return nil, err
        }

        wanted := make(map[string]bool, len(in.ServerIDs))
        for _, sid := range in.ServerIDs {
            wanted[sid] = true
        }

        now := time.Now().UnixMilli()
        views := make([]serverHealthView, 0, len(resp.GetStatuses()))
        for _, s := range resp.GetStatuses() {
            if !wanted[s.GetDevServerId()] {
                continue
            }
            views = append(views, serverHealthView{
                ServerID:         s.GetDevServerId(),
                LastCheckedAt:    now,
                IsReachable:      s.GetReachable(),
                DiskUsagePercent: s.GetDiskPercent(),
                CPUUsagePercent:  s.GetCpuPercent(),
                MemUsagePercent:  s.GetRamPercent(),
            })
        }
        return views, nil
    })
}
```

---

## TDD — Test Cases to Add

File: `services/api-gateway/internal/adapter/wscompat/channels_test.go`

### Test 1: `devServer.list` fails fast khi service chậm

```go
// TestDevServerListChannel_FailsFastWhenServiceSlow verifies that devServer.list
// returns an error in < rpcTimeout+1s when infra-fleet-service blocks, NOT
// after the full invokeTimeout (25s). This is the regression guard for BUG-003.
func TestDevServerListChannel_FailsFastWhenServiceSlow(t *testing.T) {
    fake := &fakeInfraFleetClient{
        listDevServersFunc: func(ctx context.Context, in *infrafleetv1.ListDevServersRequest) (*infrafleetv1.ListDevServersResponse, error) {
            // Block until the caller's context is cancelled (simulating a slow service).
            <-ctx.Done()
            return nil, ctx.Err()
        },
    }

    r := NewRegistry()
    registerDevServerChannels(r, fake)

    start := time.Now()
    _, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1"}, "devServer.list", nil)
    elapsed := time.Since(start)

    if err == nil {
        t.Fatal("want error from slow service, got nil")
    }
    // Must fail within rpcTimeout (8s) + small margin, not after 25s.
    if elapsed > rpcTimeout+2*time.Second {
        t.Errorf("devServer.list took %s, want < %s (rpcTimeout)", elapsed, rpcTimeout+2*time.Second)
    }
}
```

### Test 2: Verify rpcTimeout < invokeTimeout (property test)

```go
// TestRPCTimeoutShorterThanInvokeTimeout is a compile-time/value property
// that documents the required relationship between the two constants.
// If someone raises rpcTimeout above invokeTimeout, this test fails.
func TestRPCTimeoutShorterThanInvokeTimeout(t *testing.T) {
    if rpcTimeout >= invokeTimeout {
        t.Errorf("rpcTimeout (%s) must be shorter than invokeTimeout (%s) to leave room for write-back (SOL-001)", rpcTimeout, invokeTimeout)
    }
    // Also assert write margin is meaningful (> 5s).
    margin := invokeTimeout - rpcTimeout
    if margin < 5*time.Second {
        t.Errorf("write margin (invokeTimeout - rpcTimeout = %s) must be > 5s to accommodate writeTimeout from SOL-001", margin)
    }
}
```

---

## Verification

```bash
cd services/api-gateway
go test ./internal/adapter/wscompat/... -run "TestDevServerList|TestRPCTimeout" -v -timeout 30s
go test ./... -count=1
```

---

## Files Changed

| File | Change |
|------|--------|
| `internal/adapter/wscompat/channels.go` | Thêm `rpcTimeout` const; bọc gRPC calls trong `devServer.list`, `devServer.add`, `fleet.health.checkAll` bằng `context.WithTimeout(ctx, rpcTimeout)` |
| `internal/adapter/wscompat/channels_test.go` | Thêm 2 test functions |
