# TASK-BIGFILE-066 — Move: Browser screencast streaming domain

**Loại:** Move — composition pattern, rủi ro thấp · **Effort:** S ·
**Phụ thuộc:** TASK-BIGFILE-016, 037, 058
**Status:** ✅ Done
**Solution:** `../solutions/SOLUTION-FE-BIGFILE-002-orca-runtime.md`

## Bối cảnh

Sau TASK-BIGFILE-065 rà lại `browserScreencast` — trước đó ước tính nhầm
là method đơn lẻ ~412 dòng "cần Investigate riêng để refactor nội bộ
trước". Đo lại chính xác (dòng gốc 8658–8802) cho thấy method này thực
tế chỉ **145 dòng**, và 2 field riêng (`activeBrowserScreencastsByConnection`/
`activeBrowserScreencastsByPage`) chỉ dùng trong đúng method này (+ 1
external call site) — hoàn toàn tách được bằng Move thông thường, không
cần refactor nội bộ trước. Ước tính ban đầu "412 dòng" trong TASK-054 là
sai (đo nhầm khoảng cách tới method tiếp theo, bỏ qua rằng các dòng ở
giữa là forwarding field của domain browser khác, không phải thân
`browserScreencast`).

## Kết quả thực thi (2026-08-11)

- Domain: `browserScreencast` (public), `cancelBrowserScreencastForPage`
  (mới — thay cho việc đọc field trực tiếp) — dòng gốc 8658–8802, 146
  dòng (kể cả dòng trắng cuối).
- 5 host dependency, toàn bộ đã public+forwarded từ domain khác trước đó:
  `browserScreencast` (qua `browserCommands`, TASK-016),
  `getBrowserDriver`/`setBrowserDriver` (qua `mobileFloorCommands`,
  TASK-037), `registerSubscriptionCleanup`/`cleanupSubscription` (qua
  `connectionSubscriptionNotifyCommands`, TASK-058).
- **Phát hiện quan trọng**: `activeBrowserScreencastsByPage` bị đọc trực
  tiếp tại 1 nơi khác — composition wiring của `mobileFloorCommands`
  (`cancelBrowserScreencastForPage: (browserPageId) =>
  this.activeBrowserScreencastsByPage.get(browserPageId)?.cancel(true)`,
  TASK-037). Sửa bằng cách thêm method public mới
  `cancelBrowserScreencastForPage` trên class mới, cập nhật wiring của
  `mobileFloorCommands` gọi qua forwarding field thay vì đọc field trực
  tiếp — phát hiện TRƯỚC khi chạy `tsc` (grep toàn file theo đúng quy
  trình đã sửa từ bài học TASK-062/064), không phải sửa lỗi phát sinh.
- `BrowserError` (từ `'../browser/cdp-bridge'`), `BrowserScreencastResult`
  (từ `'../../shared/runtime-types'`) — move-only, xoá khỏi
  `orca-runtime.ts`.
- `orca-runtime.ts`: 9,179 → **9,040 dòng**. File mới: 188 dòng — dưới
  ngưỡng 300, không cần đăng ký `max-lines-baseline.txt`.
- Xác minh: `tsc --noEmit --composite false` giữ đúng baseline 251 lỗi (0
  lỗi mới, sạch ngay sau khi xoá 2 import move-only). `oxlint` sạch
  (exit 0) cả 2 config. `max-lines-ratchet`: 647 vi phạm pre-existing
  không đổi.

## Rủi ro còn lại / khuyến nghị

- Move cơ học thuần tuý, rủi ro thấp (5 host dep đều đã public sẵn từ
  trước, không cần tạo mới forwarding nào ở domain khác). Khuyến nghị
  kiểm thử thủ công: bắt đầu/huỷ screencast trình duyệt trên mobile/web,
  chuyển đổi giữa các trang cùng lúc (CDP chỉ hỗ trợ 1 screencast/trang),
  cleanup khi client ngắt kết nối giữa chừng.
- Phần còn lại của `orca-runtime.ts` (~9,040 dòng) vẫn chủ yếu là lõi PTY
  thật (`onPtyData`, `createTerminal`, `graph`, pty-title-tracker,
  OSC-status processing, `attachAgentRowsToSummaries`/`getWorktreePs`) —
  không còn method/cụm đơn lẻ nào ngoài vùng lõi này để tách an toàn nữa
  (đã rà bằng gap-analysis toàn file).

## Tổng kết luỹ kế

`orca-runtime.ts`: 26,730 → **9,040 dòng (66.2% giảm)** qua 34 task
(TASK-BIGFILE-036 đến 066, trừ 057 đã huỷ; 041 và 063 là state-container
Extract).
