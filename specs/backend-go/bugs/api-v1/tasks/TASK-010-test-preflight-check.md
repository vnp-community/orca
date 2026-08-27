# TASK-010: Viết tests cho `preflight.check` và cập nhật comment trong channels.go

**From Solution:** SOL-004  
**Priority:** P2 — sau khi TASK-001, TASK-008 đã được apply  
**Service:** `api-gateway`  
**Files:**
- `services/api-gateway/internal/adapter/wscompat/channels.go` (cập nhật comment)
- `services/api-gateway/internal/adapter/wscompat/channels_test.go` (thêm tests)
**Depends on:** TASK-001, TASK-008  
**Status:** `[x]` DONE

---

## Context

`preflight.check` là local handler (không có gRPC call), nhưng vẫn có thể timeout do:
- Cause A: Parent HTTP ctx bị cancel → đã fix bởi TASK-001 (writeCtx fresh)
- Cause B: `writeMu` contention khi nhiều goroutines cùng timeout → đã giảm thiểu
  bởi TASK-008 (rpcTimeout 8s thay vì 25s)

Task này thêm regression tests và cập nhật comment trong code để document methodology
debug nếu `preflight.check` vẫn chậm sau khi các fix trên được apply.

---

## Thay đổi 1: Cập nhật comment trong `channels.go`

**File:** `services/api-gateway/internal/adapter/wscompat/channels.go`

Tìm function `registerPreflightChannels` và thay toàn bộ comment:

```go
// ── preflight.check ──────────────────────────────────────────────────────────
//
// Registered as a fast, LOCAL (no downstream call) response — see
// docs/execution-plan.md §7. This handler is intentionally local-only: if it is
// observed to time out in production after SOL-001 (TASK-001) and SOL-003
// (TASK-008) are applied, the cause is writeMu contention (BUG-004 Cause B) —
// look for "wscompat: writeMu contention detected" log entries on the same
// connection around the same timestamp. The contention arises when multiple
// concurrent devServer.list/fleet.health.checkAll calls all hit their rpcTimeout
// simultaneously and queue up on writeMu.
//
// frontend/src/preload/api-types.ts's PreflightStatus asks about `gh`/`glab`
// CLI installed+authenticated state — that concept doesn't map onto
// backend-go's design: scm-integration-service is a direct OAuth API client,
// deliberately NOT a `gh`/`glab` CLI wrapper. Reporting installed:false/
// authenticated:false for both is the honest answer.
func registerPreflightChannels(r *Registry) {
	r.Register("preflight.check", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		return map[string]any{
			"git":  map[string]any{"installed": true}, // git-gateway-service's local executor requires the real git binary
			"gh":   map[string]any{"installed": false, "authenticated": false},
			"glab": map[string]any{"installed": false, "authenticated": false},
		}, nil
	})
}
```

---

## Thay đổi 2: Thêm tests trong `channels_test.go`

**File:** `services/api-gateway/internal/adapter/wscompat/channels_test.go`

Thêm vào cuối file:

```go
// ── preflight.check tests (SOL-004 / TASK-010) ──────────────────────────────

// TestPreflightCheckChannel_CompletesInstantly verifies that preflight.check
// returns within 50ms — it makes no downstream calls and should be sub-millisecond
// in practice. Regression guard for BUG-004.
func TestPreflightCheckChannel_CompletesInstantly(t *testing.T) {
	r := NewRegistry()
	registerPreflightChannels(r)

	start := time.Now()
	result, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1"}, "preflight.check", nil)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if elapsed > 50*time.Millisecond {
		t.Errorf("preflight.check took %s, want < 50ms (local handler, no gRPC call)", elapsed)
	}

	m, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("unexpected result type %T, want map[string]any", result)
	}

	// Verify git key exists and installed=true (git-gateway-service uses real git binary).
	gitInfo, ok := m["git"].(map[string]any)
	if !ok {
		t.Fatalf("result['git'] is %T, want map[string]any", m["git"])
	}
	if gitInfo["installed"] != true {
		t.Errorf("result['git']['installed'] = %v, want true", gitInfo["installed"])
	}

	// Verify gh and glab report installed=false (no CLI wrappers in backend-go).
	for _, tool := range []string{"gh", "glab"} {
		info, ok := m[tool].(map[string]any)
		if !ok {
			t.Fatalf("result[%q] is %T, want map[string]any", tool, m[tool])
		}
		if info["installed"] != false {
			t.Errorf("result[%q]['installed'] = %v, want false (no CLI in backend-go)", tool, info["installed"])
		}
		if info["authenticated"] != false {
			t.Errorf("result[%q]['authenticated'] = %v, want false", tool, info["authenticated"])
		}
	}
}

// TestPreflightCheckChannel_ReturnsExpectedKeys verifies the response has
// exactly the keys the frontend expects (git, gh, glab).
func TestPreflightCheckChannel_ReturnsExpectedKeys(t *testing.T) {
	r := NewRegistry()
	registerPreflightChannels(r)

	result, err := r.Dispatch(context.Background(), Identity{}, "preflight.check", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	m, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("unexpected result type %T", result)
	}

	for _, key := range []string{"git", "gh", "glab"} {
		if _, exists := m[key]; !exists {
			t.Errorf("preflight.check response missing expected key %q", key)
		}
	}
}
```

---

## Verify sau khi thay đổi

```bash
cd /Users/binhnt/Work/blockchain/vnp-blc/orca/backend-go/services/api-gateway
go test ./internal/adapter/wscompat/... \
    -run "TestPreflightCheck" \
    -v -count=1
```

Expected output:
```
--- PASS: TestPreflightCheckChannel_CompletesInstantly (0.00s)
--- PASS: TestPreflightCheckChannel_ReturnsExpectedKeys (0.00s)
PASS
```
