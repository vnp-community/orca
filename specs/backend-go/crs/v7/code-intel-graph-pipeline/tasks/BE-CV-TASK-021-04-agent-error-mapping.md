# BE-CV-TASK-021-04: Ánh xạ lỗi agent/infra-fleet/gRPC sang `apperrors` (kèm `data` từ trailer)

**From Solution:** BE-CV-SOL-021-agent-collector
**Priority:** P0
**Service:** `code-intel-service`
**File:** `.../internal/adapter/infrafleetclient/agent_error_mapping.go` (mới) và test
**Depends on:** TASK-021-03
**Status:** [x] DONE

---

## Context

Bảng 2.C của solution (PQ-02, PQ-03). Cần `apperrors.KindUnavailable` (SOL-010).

## Việc cần làm

1. Hàm `MapRelayError(err error, trailer metadata.MD) error`: đọc tiền tố `^CODEINTEL_[A-Z0-9_]+: `, `INFRA_*`, mã gRPC.
2. Đọc trailer `x-orca-agent-error-data-bin` (≤ 4 KiB); hỏng/mất → bỏ `data`, vẫn giữ mã. Đính `AgentErrorData` vào lỗi (không vào message).
3. Mã lạ → `CODEINTEL_TOOL_FAILED`, message ≤ 200 ký tự, bỏ ký tự điều khiển.
4. `PATH_NOT_ALLOWED` gọi hook audit (cổng của SOL-013; no-op nếu chưa có).
5. Không dùng `CODEINTEL_INVALID_ARGUMENT`.

## Kiểm thử

- `go test ./internal/adapter/infrafleetclient/ -run ErrorMapping -race`.
- Bảng: mỗi dòng 2.C một ca (đúng Kind + Code); `AMBIGUOUS_SYMBOL` mang `candidates`; trailer hỏng; mã lạ; `INFRA_DEV_SERVER_NOT_FOUND`.

## Tiêu chí hoàn thành

- [x] Mọi dòng bảng có test.
- [x] Không còn mã `INVALID_ARGUMENT`.

## Rủi ro và lưu ý

- Dữ liệu `candidates` có đường dẫn: chỉ chuyển nguyên, che ở tầng gateway.
