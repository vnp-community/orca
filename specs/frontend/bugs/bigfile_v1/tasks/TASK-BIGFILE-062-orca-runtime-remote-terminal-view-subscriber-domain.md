# TASK-BIGFILE-062 — Move: Remote terminal-view subscriber domain

**Loại:** Move — composition pattern · **Effort:** S · **Phụ thuộc:**
TASK-BIGFILE-054
**Status:** ✅ Done
**Solution:** `../solutions/SOLUTION-FE-BIGFILE-002-orca-runtime.md`

## Bối cảnh

8/8 (cuối cùng) "đảo" từ TASK-BIGFILE-054.
`remoteTerminalViewSubscriberCounts` ref-count theo PTY cho xterm remote
view (mobile/web/remote desktop RPC subscribe stream).

## Phát hiện quan trọng: `onPtyExit` chạm trực tiếp vào field (bỏ sót lúc đầu)

Đúng như TASK-054 cảnh báo ("bị `onPtyExit` chạm") nhưng lúc rà ban đầu bỏ
sót 1 call site: `onPtyExit` có `this.remoteTerminalViewSubscriberCounts.delete(ptyId)`
trực tiếp (không qua method nào cả) — `tsc TS2551` bắt lỗi ngay sau khi
xoá field. Sửa bằng cách thêm method public mới
`clearRemoteTerminalViewSubscriberCountForPty(ptyId)` trên class mới,
`onPtyExit` gọi qua instance đã lưu sẵn
(`this.remoteTerminalViewSubscriberCommands.clearRemoteTerminalViewSubscriberCountForPty(ptyId)`)
— không cần forwarding field trên `OrcaRuntimeService` vì `onPtyExit`
nằm cùng class body, có thể gọi thẳng vào sub-command instance.

## Phát hiện thứ 2: public field ghi trực tiếp từ bên ngoài

`onRemoteTerminalViewPresenceChanged` là **public field** (không phải
method) được `ipc/pty.ts` gán trực tiếp
(`runtime.onRemoteTerminalViewPresenceChanged = (id) => ...`) — khác mọi
domain trước (chỉ có method cần forwarding). Field này KHÔNG chuyển vào
class mới (vì public field không "forward" được như method qua
`.bind()`) — giữ nguyên trên `OrcaRuntimeService`, class mới đọc qua host
getter `getOnRemoteTerminalViewPresenceChanged()`.

## Kết quả thực thi (2026-08-11)

- Domain: `notifyRemoteTerminalViewPresenceChanged` (public, gọi từ
  `mobileFloorCommands` host wiring TASK-037),
  `registerRemoteTerminalViewSubscriber`, `hasRemoteTerminalViewSubscriber`
  — dòng gốc 2988–3027, 40 dòng.
- 2 host dependency: `hasMobileSubscriber` (qua `mobileFloorCommands`,
  domain khác), `getOnRemoteTerminalViewPresenceChanged` (đọc public field
  giữ nguyên trên `OrcaRuntimeService`).
- `orca-runtime.ts`: 10,035 → **10,006 dòng** (lần đầu xuống dưới 10,000
  dòng). File mới: 73 dòng — dưới ngưỡng 300, không cần đăng ký
  `max-lines-baseline.txt`.
- Xác minh: `tsc --noEmit --composite false` giữ đúng baseline 251 lỗi (0
  lỗi mới, sau khi sửa 1 lỗi tạm thời `TS2551` do bỏ sót call site trong
  `onPtyExit`). `oxlint` sạch (exit 0) cả 2 config. `max-lines-ratchet`:
  647 vi phạm pre-existing không đổi.

## Rủi ro còn lại / khuyến nghị

- Move cơ học thuần tuý, rủi ro thấp sau khi sửa call site bỏ sót. Khuyến
  nghị kiểm thử thủ công: mở/đóng remote terminal-view RPC stream (đếm
  ref đúng, không leak khi disconnect đột ngột), daemon background mark
  resync đúng khi có/không subscriber.
- **Đây là task cuối cùng trong danh sách 8 "đảo an toàn" từ TASK-BIGFILE-054.**
  Phần còn lại của `orca-runtime.ts` (~10,000 dòng) là lõi thật
  (`graph`, `headlessTerminals`, cụm 10 field OSC/transcript-per-PTY,
  `ptyTitleTrackersByPtyId`) — cần thiết kế state-owner riêng (như
  `RuntimeGraphStore` ở TASK-041) trước khi tách thêm được, hoặc method
  đơn lẻ khổng lồ `browserScreencast` (412 dòng) cần refactor nội bộ
  trước (task Investigate riêng, khác kiểu Move).

## Tổng kết luỹ kế

`orca-runtime.ts`: 26,730 → **10,006 dòng (62.6% giảm)** qua 30 task
(TASK-BIGFILE-036 đến 062, trừ 057 đã huỷ).
