# BUG-BE-HLD-015 — Cảnh báo quota AI Provider ở ngưỡng 80% không tồn tại; chỉ phát hiện SAU khi đã vượt

**Mức độ:** 🟡 MEDIUM (Feature gap)
**Status:** 🔴 Open
**Module:** `backend/src/main/ai-providers/ProviderHealthChecker.ts`, `AIProviderService.ts`
**Phát hiện:** 2026-08-09 (audit `backend/` code vs thiết kế — `audit/backend/backend-vs-design-review.md` §5.14/F35)

---

## Mô tả

`docs/features/F35-ai-provider-account-management.md` yêu cầu cảnh báo sớm khi usage đạt 80% quota, để admin kịp xử lý trước khi bị chặn hoàn toàn.

Thực tế: grep `"80%"`/`quotaAlert` trong `backend/src/main/ai-providers/`: 0 kết quả. `ProviderHealthChecker` (`ProviderHealthChecker.ts:98-105`) chỉ set status `quota_exceeded` khi provider **trả lỗi chứa "quota"** — đây là phát hiện **sau khi đã vượt quota**, không phải cảnh báo sớm chủ động dựa trên số liệu `orca_provider_usage` đã track sẵn (bảng này đã có, xem migration 0008).

## Hậu quả

- Admin không có thời gian phản ứng trước khi 1 AI provider account bị chặn hoàn toàn do hết quota — workflow/task đang chạy dở sẽ fail đột ngột thay vì được cảnh báo trước.

## Bằng chứng

- Grep `"80%"`, `quotaAlert` trong `backend/src/main/ai-providers/*.ts`: 0 kết quả.
- `backend/src/main/ai-providers/ProviderHealthChecker.ts:98-105` — chỉ set `quota_exceeded` reactive theo lỗi trả về.
- Bảng `orca_provider_usage` (migration 0008) đã có đủ dữ liệu (`tokens_used`, `requests`, theo `period`) để tính % quota đã dùng — chỉ thiếu logic đọc + so sánh ngưỡng.

## Đề xuất fix

1. Trong `ProviderHealthChecker` (chạy mỗi 15 phút, đã có sẵn interval), thêm bước: đọc `orca_provider_usage` hiện tại của account, so với `quotaLimitDay` (đã dùng ở `ProviderResolver.resolve()`), nếu ≥80% → emit sự kiện cảnh báo (dùng cơ chế webhook/notification đã có ở Fleet Health cho tiện tái sử dụng pattern).
2. Thêm field `quotaWarningThresholdPercent` (default 80) vào config account nếu cần cho phép admin tuỳ chỉnh ngưỡng.

## Tham khảo

- Audit: `audit/backend/backend-vs-design-review.md` §5.14 (F35)
- Doc gốc: `docs/features/F35-ai-provider-account-management.md`
- Liên quan: [BUG-BE-HLD-014](./BUG-BE-HLD-014-ai-provider-key-rotation-not-implemented.md)
