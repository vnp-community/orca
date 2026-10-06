# FE-CV-TASK-095-02: Đồng bộ sáu bản sao `shared/` và test parity

**From Solution:** [FE-CV-SOL-095-review-telemetry](../solutions/FE-CV-SOL-095-review-telemetry.md) mục 1 (6 bản sao)
**Priority:** P0
**Area:** frontend
**File:** `desktop/src/shared/`, `backend/src/shared/`, `tests/vendor-shared/shared/`, `agent/src/shared/`, `mobile/src/vendor-shared/shared/` (mỗi nơi: `telemetry-events.ts`, `review-telemetry-events.ts`) và `frontend/src/shared/telemetry-shared-copies-parity.test.ts` (mới)
**Depends on:** FE-CV-TASK-095-01
**Status:** [ ] TODO

## Context

- Đã kiểm `cmp`: hiện cả sáu bản `telemetry-events.ts` và `mcp-telemetry-events.ts` giống nhau; không có script đồng bộ.
- Validator desktop import bản `desktop/src/shared/`.
- Các thay đổi ngoài `frontend/` cần chủ sở hữu từng gói duyệt (quy ước 8.1).

## Việc cần làm

1. Sao chép hai tệp mới/sửa sang năm nơi còn lại bằng copy tay; ghi cách làm vào PR.
2. Thêm test parity đọc sáu đường dẫn và so nội dung (báo rõ nếu gói vắng).
3. Không đổi tệp khác trong các gói đó.

## Kiểm thử

- Parity xanh; thử lệch một bản → test đỏ.
- Chạy: `pnpm --filter orca-frontend test -- <đường dẫn>` (chưa chạy).

## Tiêu chí hoàn thành

- [ ] Sáu bản khớp byte.
- [ ] PR nêu rõ các gói bị chạm.

## Rủi ro

- Cơ chế đồng bộ chính thức chưa có (câu hỏi mở 1).
