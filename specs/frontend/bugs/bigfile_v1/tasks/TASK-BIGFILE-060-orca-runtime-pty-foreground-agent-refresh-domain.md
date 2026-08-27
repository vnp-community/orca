# TASK-BIGFILE-060 — Move: PTY foreground-agent refresh domain

**Loại:** Move — composition pattern · **Effort:** S · **Phụ thuộc:**
TASK-BIGFILE-054
**Status:** ✅ Done
**Solution:** `../solutions/SOLUTION-FE-BIGFILE-002-orca-runtime.md`

## Bối cảnh

7/8 "đảo" từ TASK-BIGFILE-054, áp dụng đúng bài học phương pháp sau
TASK-057 (huỷ) — kiểm method-body dependency kỹ trước khi thực thi.

## Phát hiện: phụ thuộc ngược từ pty-title-tracker (057, đã huỷ)

`applyTrackedPtyTitle` (thuộc domain pty-title-tracker, KHÔNG tách vì lý
do đã ghi ở TASK-054) gọi tới `refreshPtyForegroundAgentFromController`/
`getPendingForegroundAgentRefreshForTitle`/
`delayPtyBackedMobileSnapshotForForegroundAgent` của domain này. Vẫn tách
được domain này bình thường (closure trong `OrcaRuntimeService`'s own
body vẫn gọi được method qua forwarding field), chỉ cần đảm bảo 3 method
này public + có forwarding field.

## Kết quả thực thi (2026-08-11)

- Domain: `refreshPtyForegroundAgent`, `getPendingForegroundAgentRefreshForTitle`,
  `delayPtyBackedMobileSnapshotForForegroundAgent`,
  `refreshPtyForegroundAgentFromController` (4 method public — cả 4 đều
  cần forwarding vì bị gọi từ `applyTrackedPtyTitle` hoặc nơi khác trong
  `orca-runtime.ts`), `loadPtyForegroundAgentFromController` (private, chỉ
  tự tham chiếu).
- 4 host dependency: `getGraph()`, `getPtyController()`,
  `touchMobileSessionSnapshotsForPty()` (đã public từ TASK-051),
  `recognizeAgentProcess` (free function từ
  `'../../shared/agent-process-recognition'`, giữ nguyên ở
  `orca-runtime.ts` vì dùng ở nơi khác, import lại bản sao).
- `PtyForegroundAgentRefresh` (type nội bộ, dòng 544) — thêm `export`,
  import lại từ `'./orca-runtime'`.
- `orca-runtime.ts`: 10,123 → **10,035 dòng**. File mới: 141 dòng — dưới
  ngưỡng 300, không cần đăng ký `max-lines-baseline.txt`.
- Xác minh: `tsc --noEmit --composite false` giữ đúng baseline 251 lỗi (0
  lỗi mới, sạch ngay từ lần chạy đầu). `oxlint` sạch (exit 0) cả 2 config.
  `max-lines-ratchet`: 647 vi phạm pre-existing không đổi.

## Rủi ro còn lại / khuyến nghị

- Move cơ học thuần tuý nhưng phụ thuộc ngược từ domain KHÔNG tách
  (pty-title-tracker) — rủi ro trung bình (vẫn nằm trong vùng PTY-core
  đan xen). Khuyến nghị kiểm thử thủ công: OSC title thay đổi trạng thái
  working/idle → foreground-process probe đúng, mobile snapshot không bị
  trì hoãn sai, resume session khôi phục đúng foreground agent.

## Tổng kết luỹ kế

`orca-runtime.ts`: 26,730 → **10,035 dòng (62.5% giảm)** qua 29 task
(TASK-BIGFILE-036 đến 061, trừ 057 đã huỷ, cộng 058/060).
