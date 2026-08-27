# TASK-BIGFILE-058 — Move: Connection-subscription cleanup + mobile notification/fit-override domain

**Loại:** Move — composition pattern · **Effort:** S · **Phụ thuộc:**
TASK-BIGFILE-054
**Status:** ✅ Done
**Solution:** `../solutions/SOLUTION-FE-BIGFILE-002-orca-runtime.md`

## Bối cảnh

5/8 (thực thi) "đảo an toàn" từ TASK-BIGFILE-054, áp dụng đúng bài học
phương pháp rút ra sau TASK-057 (huỷ) — kiểm tra method-body dependency
(`this.X` trong thân method) TRƯỚC khi xếp loại an toàn, không chỉ dựa
field-span.

## 2 đoạn không liền mạch, gộp thành 1 domain

`subscribeToFitOverrideChanges`/`notifyFitOverrideListeners` (dòng gốc
3055–3079) đứng tách biệt khỏi `registerSubscriptionCleanup`…
`dismissMobileNotification` (dòng gốc 3755–3863) — cùng chủ đề "quản lý
lifecycle theo connection/subscription" nên gộp vào 1 file hợp lý (không
tách 2 file nhỏ riêng).

## Kết quả thực thi (2026-08-11)

- Domain: 9 method — `subscribeToFitOverrideChanges`,
  `notifyFitOverrideListeners`, `registerSubscriptionCleanup`,
  `cleanupSubscription`, `cleanupSubscriptionsByPrefix`,
  `cleanupSubscriptionsForConnection`, `onNotificationDispatched`,
  `getMobileNotificationListenerCount`, `dispatchMobileNotification`,
  `dismissMobileNotification` (10 thật, đếm nhầm 9 lúc đầu).
- Kiểm tra method-body dependency (đúng bài học 057): CHỈ 1 host dependency
  thật — `getPushManager()` (dùng trong `dispatchMobileNotification` để
  bắn web push khi agent-task-complete). `addListenerToMap` (free function,
  dùng chung với `dataListeners` ở domain khác) — giữ nguyên ở
  `orca-runtime.ts` (đã `export`), import lại trong file mới.
- 2 method cần public + forwarding (đã public sẵn từ trước, chỉ cần thêm
  forwarding field): `notifyFitOverrideListeners` (gọi từ
  `mobileFloorCommands` host wiring, TASK-037), `registerSubscriptionCleanup`/
  `cleanupSubscription` (gọi từ `browserScreencast`, method đơn lẻ khổng lồ
  chưa tách — dòng ~9826/9836/9843).
- `MobileNotificationEvent` (từ `./orca-runtime-types`) — cùng biến thể
  STAYS/MOVE đã gặp ở TASK-056: có mặt trong khối
  `export type {...} from './orca-runtime-types'` cuối file nhưng đó là
  re-export TRỰC TIẾP từ module gốc, không tính là "dùng" import cục bộ —
  xoá khỏi import cục bộ sau khi 2 method dùng nó chuyển đi (`tsc TS6196`
  xác nhận, sửa ngay).
- `orca-runtime.ts`: 10,236 → **10,123 dòng**. File mới: 179 dòng — dưới
  ngưỡng 300, không cần đăng ký `max-lines-baseline.txt`.
- Xác minh: `tsc --noEmit --composite false` giữ đúng baseline 251 lỗi (0
  lỗi mới, sau khi sửa 1 lỗi tạm thời `TS6196`). `oxlint` sạch (exit 0) cả
  2 config. `max-lines-ratchet`: 647 vi phạm pre-existing không đổi.

## Rủi ro còn lại / khuyến nghị

- Move cơ học thuần tuý, rủi ro thấp (chỉ 1 host dep thật). Khuyến nghị
  kiểm thử thủ công: mobile client reconnect (subscription cleanup không
  leak), desktop notification fan-out + web push khi agent hoàn thành task,
  fit-override khi khôi phục từ mobile-fit.
- Còn 2 domain từ TASK-054 (060 pty-foreground-agent-refresh, 062
  remote-terminal-view-subscriber) — cả 2 cần áp dụng đúng bài học 057
  trước khi thực thi.

## Tổng kết luỹ kế

`orca-runtime.ts`: 26,730 → **10,123 dòng (62.1% giảm)** qua 28 task
(TASK-BIGFILE-036 đến 061 trừ 057 đã huỷ, cộng 058).
