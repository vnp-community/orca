# TASK-011: Full regression test — chạy toàn bộ test suite sau khi apply tất cả tasks

**From Solution:** SOL-001 → SOL-004 (Verification steps)  
**Priority:** P2 — task cuối cùng sau khi tất cả tasks trước đã hoàn thành  
**Service:** `api-gateway`  
**Files:** Không thay đổi file mới — chỉ chạy lệnh verify  
**Depends on:** TASK-001 → TASK-010 (tất cả)  
**Status:** `[x]` DONE

---

## Mục tiêu

Xác nhận toàn bộ changes từ TASK-001 đến TASK-010 không break bất kỳ test nào
hiện có, và tất cả tests mới đều pass.

---

## Các lệnh verify

### Bước 1: Build toàn bộ service

```bash
cd /Users/binhnt/Work/blockchain/vnp-blc/orca/backend-go/services/api-gateway
go build ./...
```

Expected: exit code 0, không có lỗi.

### Bước 2: Run vet

```bash
cd /Users/binhnt/Work/blockchain/vnp-blc/orca/backend-go/services/api-gateway
go vet ./...
```

Expected: không có warning.

### Bước 3: Run toàn bộ test suite (không bao gồm slow tests)

```bash
cd /Users/binhnt/Work/blockchain/vnp-blc/orca/backend-go/services/api-gateway
go test ./... -count=1 -timeout 60s -v 2>&1 | grep -E "^(=== RUN|--- PASS|--- FAIL|PASS|FAIL|ok)"
```

Expected: tất cả `--- PASS`, không có `--- FAIL`.

### Bước 4: Run với -race flag (quan trọng cho concurrent write path)

```bash
cd /Users/binhnt/Work/blockchain/vnp-blc/orca/backend-go/services/api-gateway
go test ./internal/adapter/wscompat/... -count=1 -race -timeout 60s
```

Expected: `PASS`, không có "DATA RACE" warnings.

### Bước 5: Run slow timeout tests riêng (rpcTimeout tests = ~24s mỗi lần)

```bash
cd /Users/binhnt/Work/blockchain/vnp-blc/orca/backend-go/services/api-gateway
go test ./internal/adapter/wscompat/... \
    -run "TestDevServerListChannel_FailsFast|TestDevServerAddChannel_FailsFast|TestFleetHealthCheckAll_FailsFast" \
    -v -count=1 -timeout 60s
```

Expected: 3 tests pass trong ~24s tổng cộng (mỗi test ~8s).

---

## Expected test count

Sau khi apply TASK-001 → TASK-010, tổng số tests mới thêm vào:

| File | Tests mới |
|------|-----------|
| `wscompat/handler_test.go` | 3 tests (TASK-002) |
| `wscompat/channels_test.go` | 4 tests crash/rateLimit (TASK-007) + 4 tests rpcTimeout (TASK-009) + 2 tests preflight (TASK-010) = 10 tests |
| `usecase/rate_limit_test.go` | 3 tests (TASK-004) |
| **Tổng** | **16 tests mới** |

---

## Checklist hoàn thành

```
[x] TASK-001: handleInvoke/handleSend sử dụng dispatchCtx và writeCtx riêng
[x] TASK-002: 3 tests cho handler context separation
[x] TASK-003: RPS() và Burst() accessors trên RateLimiter
[x] TASK-004: 3 tests cho RateLimiter accessors
[x] TASK-005: registerCrashReportChannels + registerRateLimitChannels + RegisterRealChannels mới
[x] TASK-006: main.go truyền rateLimiter vào RegisterRealChannels
[x] TASK-007: 4 tests cho crashReports và rateLimits channels
[x] TASK-008: rpcTimeout const + per-RPC deadline trong 3 channel handlers
[x] TASK-009: 4 tests cho rpcTimeout behavior
[x] TASK-010: 2 tests cho preflight.check + comment update
[x] TASK-011: full regression verify — tất cả tests pass
```

---

## Kết quả mong đợi (so sánh Before/After)

| Frontend Error | Trước | Sau |
|---------------|-------|-----|
| `crashReports.getLatestPending timeout 30s` | ❌ | ✅ Returns `null` < 1ms |
| `rateLimits.get timeout 30s` | ❌ | ✅ Returns `{rps, burst}` < 1ms |
| `devServer.list timeout 30s` | ❌ | ✅ Returns error trong ≤ 8s |
| `preflight.check timeout 30s` | ❌ | ✅ Returns trong < 1ms |
