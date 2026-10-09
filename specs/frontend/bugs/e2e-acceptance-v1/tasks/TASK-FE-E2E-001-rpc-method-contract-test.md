# TASK-FE-E2E-001: Test hợp đồng: mọi kênh frontend gọi đều có ở gateway

**From Solution:** SOL-FE-E2E-001 · **Resolves:** FE-E2E-C
**Priority:** P0 · **Area:** frontend
**Files:**
- frontend/src/shared/request-rpc-methods.ts
- frontend/src/shared/request-rpc-contract.test.ts (mới)
- backend-go/services/api-gateway/internal/adapter/wscompat/testdata/channel_inventory.json (đọc)
**Depends on:** TASK-E2E-003
**Status:** `[ ]` TODO

---

## Context

Không có kiểm tra nào so tên kênh frontend với gateway; `clarification.*` thiếu mà mọi test xanh.

## Việc cần làm

- [ ] Vitest đọc tệp vàng bằng `fs` (đường dẫn tương đối từ gốc repo), so với giá trị của `REQUEST_RPC_METHODS` và các bảng kênh Request khác.
- [ ] Danh sách cho phép kênh chỉ-Electron, mỗi mục có lý do.
- [ ] Thông báo lỗi nêu tên kênh thiếu và gợi ý cập nhật tệp vàng backend.

## Tiêu chí hoàn thành

- [ ] Xoá một kênh khỏi tệp vàng làm test đỏ.
- [ ] Test xanh ở HEAD sau TASK-E2E-001/003.

## Kiểm thử

Vitest.
