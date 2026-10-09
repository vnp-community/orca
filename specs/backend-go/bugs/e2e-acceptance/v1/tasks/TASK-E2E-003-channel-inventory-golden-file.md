# TASK-E2E-003: Xuất tệp vàng danh mục kênh để frontend so khớp (P2)

**From Solution:** SOL-E2E-001 · **Resolves:** BUG-E2E-001
**Priority:** P1 · **Service/Area:** api-gateway
**Files:**
- backend-go/services/api-gateway/internal/adapter/wscompat/channel_inventory_golden_test.go (mới)
- backend-go/services/api-gateway/internal/adapter/wscompat/testdata/channel_inventory.json (mới)
**Depends on:** TASK-E2E-001
**Status:** `[x]` XONG (cập nhật 2026-10-09, chưa commit)

---

## Context

Frontend khai báo tên kênh trong `REQUEST_RPC_METHODS` và các bảng khác, không có nguồn sự thật chung với gateway.

## Việc cần làm

- [x] Test gọi `NewRegistry` + đăng ký production (xem `register_production.go`) rồi ghi `[{name, kind}]` đã sắp xếp.
- [x] Chế độ so khớp mặc định; `UPDATE_GOLDEN=1` ghi lại (giống `check-agent-capability-golden.sh`).
- [x] Ghi README ngắn nêu frontend đọc tệp này (TASK-FE-E2E-001).

## Tiêu chí hoàn thành

- [x] Đổi tên một kênh làm test đỏ cho tới khi cập nhật tệp vàng.
- [x] Tệp ổn định (sắp xếp, không có thời gian).

## Kiểm thử

Test vàng; xác nhận chạy hai lần cho cùng kết quả.

## Kết quả thực thi

Xong. `channel_inventory_golden_test.go` + `testdata/channel_inventory.json` (cập nhật bằng `UPDATE_GOLDEN=1`, kèm `testdata/README.md`). Chạy hai lần cho cùng kết quả. FE đọc tệp này ở TASK-FE-E2E-001.
