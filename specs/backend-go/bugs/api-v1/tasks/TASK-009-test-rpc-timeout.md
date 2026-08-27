# TASK-009: Viết tests cho `rpcTimeout` và fail-fast behavior

**From Solution:** SOL-003 (TDD tests)  
**Priority:** P1  
**Service:** `api-gateway`  
**File:** `services/api-gateway/internal/adapter/wscompat/channels_test.go`  
**Depends on:** TASK-008  
**Status:** `[x]` DONE

---

## Thay đổi cần thực hiện

**File:** `services/api-gateway/internal/adapter/wscompat/channels_test.go`

Thêm vào cuối file (sau các tests hiện có và sau phần tests của TASK-007):

```go
// ── rpcTimeout tests (SOL-003 / TASK-008) ───────────────────────────────────

// TestRPCTimeoutConstant_ShorterThanInvokeTimeout documents the required
// relationship: rpcTimeout < invokeTimeout. Failing this test means the
// per-RPC deadline no longer leaves margin for write-back (SOL-001 / TASK-001).
func TestRPCTimeoutConstant_ShorterThanInvokeTimeout(t *testing.T) {
	if rpcTimeout >= invokeTimeout {
		t.Errorf("rpcTimeout (%s) must be < invokeTimeout (%s); "+
			"rpcTimeout occupies the dispatch window, invokeTimeout must envelope it",
			rpcTimeout, invokeTimeout)
	}
	// Write margin must be at least 5s (writeTimeout from SOL-001).
	margin := invokeTimeout - rpcTimeout
	if margin < 5*time.Second {
		t.Errorf("write margin (invokeTimeout - rpcTimeout = %s) must be >= 5s "+
			"to accommodate writeTimeout (SOL-001)", margin)
	}
}

// TestDevServerListChannel_FailsFastWhenServiceSlow verifies that devServer.list
// returns an error within rpcTimeout + small margin when infra-fleet-service
// blocks, NOT after the full invokeTimeout (25s). Regression guard for BUG-003.
func TestDevServerListChannel_FailsFastWhenServiceSlow(t *testing.T) {
	fake := &fakeInfraFleetClient{
		listDevServersFunc: func(ctx context.Context, in *infrafleetv1.ListDevServersRequest) (*infrafleetv1.ListDevServersResponse, error) {
			// Simulate a slow/hung service: block until the per-RPC context
			// is cancelled (i.e. until rpcTimeout fires).
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
	// Must fail within rpcTimeout (8s) + 2s margin, not after 25s.
	maxAllowed := rpcTimeout + 2*time.Second
	if elapsed > maxAllowed {
		t.Errorf("devServer.list took %s, want < %s (rpcTimeout + margin); "+
			"infra-fleet-service timeout not being enforced", elapsed, maxAllowed)
	}
}

// TestDevServerAddChannel_FailsFastWhenServiceSlow verifies the same rpcTimeout
// enforcement for devServer.add.
func TestDevServerAddChannel_FailsFastWhenServiceSlow(t *testing.T) {
	fake := &fakeInfraFleetClient{
		registerDevServerFunc: func(ctx context.Context, in *infrafleetv1.RegisterDevServerRequest) (*infrafleetv1.RegisterDevServerResponse, error) {
			<-ctx.Done()
			return nil, ctx.Err()
		},
	}

	r := NewRegistry()
	registerDevServerChannels(r, fake)

	args := argsJSON(t, map[string]any{"name": "slow-server", "connectionType": "relay-ssh"})

	start := time.Now()
	_, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1"}, "devServer.add", args)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("want error from slow service, got nil")
	}
	if elapsed > rpcTimeout+2*time.Second {
		t.Errorf("devServer.add took %s, want < %s", elapsed, rpcTimeout+2*time.Second)
	}
}

// TestFleetHealthCheckAll_FailsFastWhenServiceSlow verifies rpcTimeout
// enforcement for fleet.health.checkAll.
func TestFleetHealthCheckAll_FailsFastWhenServiceSlow(t *testing.T) {
	fake := &fakeInfraFleetClient{
		getFleetHealthFunc: func(ctx context.Context, in *infrafleetv1.GetFleetHealthRequest) (*infrafleetv1.GetFleetHealthResponse, error) {
			<-ctx.Done()
			return nil, ctx.Err()
		},
	}

	r := NewRegistry()
	registerFleetChannels(r, fake)

	args := argsJSON(t, map[string]any{"serverIds": []string{"ds-1"}})

	start := time.Now()
	_, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1"}, "fleet.health.checkAll", args)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("want error from slow service, got nil")
	}
	if elapsed > rpcTimeout+2*time.Second {
		t.Errorf("fleet.health.checkAll took %s, want < %s", elapsed, rpcTimeout+2*time.Second)
	}
}
```

---

## Verify sau khi thay đổi

```bash
cd /Users/binhnt/Work/blockchain/vnp-blc/orca/backend-go/services/api-gateway

# Chạy chỉ các tests mới — timeout 30s để giới hạn thời gian test (rpcTimeout = 8s)
go test ./internal/adapter/wscompat/... \
    -run "TestRPCTimeout|TestDevServerListChannel_FailsFast|TestDevServerAddChannel_FailsFast|TestFleetHealthCheckAll_FailsFast" \
    -v -count=1 -timeout 30s
```

> ⚠️ **Lưu ý thời gian chạy:** Các tests `_FailsFastWhenServiceSlow` sẽ chờ đến
> khi `rpcTimeout` (8s) hết. Đây là expected — test xác nhận call timeout trong 8s,
> không phải 25s. Tổng thời gian chạy 3 tests: ~24s.

Expected output (sau ~24s):
```
--- PASS: TestRPCTimeoutConstant_ShorterThanInvokeTimeout (0.00s)
--- PASS: TestDevServerListChannel_FailsFastWhenServiceSlow (8.00s)
--- PASS: TestDevServerAddChannel_FailsFastWhenServiceSlow (8.00s)
--- PASS: TestFleetHealthCheckAll_FailsFastWhenServiceSlow (8.00s)
PASS
```
