# BE-CV-TASK-013-09: `AgentCallGate`: đồng thời theo dev server/tenant, lớp method, nhả khe

**From Solution:** BE-CV-SOL-013-agent-call-gate-and-quotas
**Priority:** P0
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/usecase/agent_call_gate.go`, `internal/domain/agent_method_class.go`, `internal/adapter/callgate/token_bucket_gate.go` và `_test.go` (mới); `internal/config/config_limits.go` (mới)
**Depends on:** BE-CV-TASK-010-02, 010-03
**Status:** [x] DONE

---

## Context

SOL-013-gate mục 2.A–2.B. Mẫu limiter: `api-gateway/internal/usecase/rate_limit.go` (`rate.NewLimiter`, map + mutex). Thêm `golang.org/x/time v0.15.0` vào `go.mod` của service (trực tiếp). Số liệu là giả định chưa đo (O-15).

## Việc cần làm

1. `config_limits.go`: các biến `CODEINTEL_MAX_INFLIGHT_PER_DEV_SERVER` (4), `CODEINTEL_MAX_HEAVY_PER_DEV_SERVER` (2), `CODEINTEL_MAX_INFLIGHT_PER_TENANT` (16), `CODEINTEL_HEAVY_PER_MINUTE` (30, burst 10), `CODEINTEL_LIGHT_PER_MINUTE` (120, burst 30), `CODEINTEL_GATE_WAIT` (2s), `CODEINTEL_USER_RPS` (20, burst 40), `CODEINTEL_REINDEX_PER_DEV_SERVER` (1), `CODEINTEL_REINDEX_PER_TENANT` (2), `CODEINTEL_REINDEX_COOLDOWN` (5m), `CODEINTEL_REINDEX_PER_USER_PER_HOUR` (6); giá trị ≤ 0 → lỗi cấu hình.
2. `agent_method_class.go`: `ClassOf(method string) GateClass` theo bảng SOL 2.B; lạ → `ClassHeavy`; test kiểm đủ method trong hợp đồng agent §4–5.
3. `token_bucket_gate.go`: `Acquire` theo SOL 2.A: bucket `(tenant,user,class)` → semaphore tenant → dev server → dev-server-heavy với chờ `GateWait`; `release` idempotent và nhả ngược thứ tự; nhả khi `ctx` hết hạn trong lúc chờ; map theo khoá có dọn mục rảnh; đồng hồ tiêm.
4. Lỗi: `apperrors.New(KindResourceExhausted, "CODEINTEL_CONCURRENCY_LIMIT"|"CODEINTEL_RATE_LIMITED", …)` kèm `CodedData{retryAfterSeconds, scope}` (kiểu `domain.CodedData` của 013-07).
5. Cổng nil-an-toàn: `NoopGate` cho test/dev.

## Kiểm thử

- `go test -race ./services/code-intel-service/internal/... -run 'Gate|MethodClass'`
- 5 `heavy` đồng thời, giới hạn 2: 2 chạy, 3 chờ rồi `CONCURRENCY_LIMIT` sau 2 s (đồng hồ giả); nhả khe → người chờ vào; ctx huỷ giữa chờ không rò khe (đếm semaphore về 0); tenant A không làm đầy hạn mức tenant B; bucket `heavy` 30/phút vượt → `RATE_LIMITED` có `retryAfterSeconds`.

## Tiêu chí hoàn thành

- [x] Không rò khe (test đếm); `-race` sạch.
- [x] Mọi method hợp đồng có lớp.

## Rủi ro và lưu ý

- Hạn mức nhân theo số bản sao.
- Giá trị mặc định chưa đo; không hard-code ở nơi khác.
