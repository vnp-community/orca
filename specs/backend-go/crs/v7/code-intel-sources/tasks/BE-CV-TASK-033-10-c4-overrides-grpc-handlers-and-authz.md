# BE-CV-TASK-033-10: Handler gRPC `GetC4Overrides`/`SaveC4Overrides` và quyền `c4_write`

**From Solution:** BE-CV-SOL-033-c4-overrides-yaml
**Priority:** P0
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/adapter/grpc/c4_overrides_handlers.go` (mới) và `_test.go`
**Depends on:** BE-CV-TASK-033-09; BE-CV-SOL-013
**Status:** [ ] TODO

---

## Context

Solution mục 2.D, hợp đồng §6.3.

## Việc cần làm

1. Validate tham số; `Get` action `read`; `Save` action `c4_write` (owner/admin); member bị `CODEINTEL_NOT_AUTHORIZED`.
2. Lỗi theo PQ-02 (tiền tố `CODEINTEL_`, hậu tố `{currentVersion}` ≤ 2 KiB cho conflict).
3. Không log `document`; chỉ kích thước, `container`, `version`.
4. Phiên thiết bị bị từ chối.

## Kiểm thử

`go test ./services/code-intel-service/internal/adapter/grpc/... -run C4Overrides` (chưa chạy): từng vai trò; conflict có hậu tố đúng regex PQ-02; quá 64 KiB.

## Tiêu chí hoàn thành

- [ ] Chỉ owner/admin ghi.
- [ ] Không rò `document` vào log.

## Rủi ro và lưu ý

- Giới hạn gateway 96 KiB thuộc 040.
