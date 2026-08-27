# TASK-008: Thêm `rpcTimeout` và per-RPC deadline cho gRPC channel handlers

**From Solution:** SOL-003  
**Priority:** P1  
**Service:** `api-gateway`  
**File:** `services/api-gateway/internal/adapter/wscompat/channels.go`  
**Depends on:** TASK-001 (SOL-001 phải được apply trước — `rpcTimeout < invokeTimeout` mới có ý nghĩa)  
**Status:** `[x]` DONE

---

## Context

`devServer.list`, `devServer.add`, và `fleet.health.checkAll` gọi gRPC với `ctx` từ
`invokeTimeout` (25s). Khi `infra-fleet-service` chậm hoặc không phản hồi, call block
đến tận 25s. Cần thêm per-RPC deadline ngắn hơn (8s) để fail fast và để lại margin
cho write-back (SOL-001).

---

## Thay đổi cần thực hiện

**File:** `services/api-gateway/internal/adapter/wscompat/channels.go`

### Bước 1: Thêm constant `rpcTimeout`

Thêm sau `package wscompat` declaration và trước block `import`:

```go
// rpcTimeout is the per-RPC deadline applied to each outbound gRPC call inside
// a channel handler. Shorter than handler.go's invokeTimeout (25s) so a slow
// or unreachable downstream service fails fast with a meaningful error message
// rather than occupying the full dispatch window.
//
// Invariant: rpcTimeout (8s) < invokeTimeout (25s) — verified by
// TestRPCTimeoutShorterThanInvokeTimeout in channels_test.go.
const rpcTimeout = 8 * time.Second
```

Đảm bảo `"time"` đã có trong import block.

### Bước 2: Cập nhật `registerDevServerChannels`

Thay toàn bộ function `registerDevServerChannels`:

```go
func registerDevServerChannels(r *Registry, client infrafleetv1.InfraFleetServiceClient) {
	r.Register("devServer.list", func(ctx context.Context, id Identity, _ []json.RawMessage) (any, error) {
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		// Per-RPC deadline: fail fast so the write-back (SOL-001 / TASK-001)
		// still has time to deliver the error to the frontend before invokeTimeout.
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

### Bước 3: Cập nhật `registerFleetChannels`

Thay toàn bộ function `registerFleetChannels`:

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

## Verify sau khi thay đổi

```bash
cd /Users/binhnt/Work/blockchain/vnp-blc/orca/backend-go/services/api-gateway
go build ./internal/adapter/wscompat/...
go vet ./internal/adapter/wscompat/...
go test ./internal/adapter/wscompat/... -count=1 -v
```

Expected: build thành công, tất cả tests hiện có vẫn pass (các tests trong channels_test.go
đã có mock cho ListDevServers nên vẫn chạy đúng với rpcCtx).
