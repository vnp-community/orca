# TASK-BIGFILE-061 — Move: Terminal cross-agent message waiter domain

**Loại:** Move — composition pattern · **Effort:** S · **Phụ thuộc:**
TASK-BIGFILE-054
**Status:** ✅ Done
**Solution:** `../solutions/SOLUTION-FE-BIGFILE-002-orca-runtime.md`

## Bối cảnh

4/8 "đảo an toàn" từ TASK-BIGFILE-054. `messageWaitersByHandle` cô lập,
long-poll waiter cho `orchestration.check --wait`.

## Kết quả thực thi (2026-08-11)

- Domain: `deliverPendingMessagesForHandle`, `notifyMessageArrived`,
  `waitForMessage` (public), `resolveMessageWaiter`, `removeMessageWaiter`
  (private) — dòng gốc 8906–9004, 99 dòng.
- 2 host dependency: `getLiveLeafForHandle`, `deliverPendingMessages` — cả
  2 vẫn ở private trên `OrcaRuntimeService` (không cần public, vì
  composition-wiring closure định nghĩa NGAY TRONG class body nên truy
  cập private method vẫn hợp lệ — khác các trường hợp trước cần public vì
  bị gọi từ file domain khác qua `this.host.X`).
- `MessageWaiter` (type nội bộ, dòng 837) — chuyển hẳn (chỉ domain này
  dùng, kể cả field). `MESSAGE_WAIT_DEFAULT_TIMEOUT_MS` (từ
  `./orca-runtime-tail-buffer`) — move hẳn khỏi import lớn.
- Thêm `eslint-disable unicorn/no-useless-spread` inline (giống disable
  gốc) cho pattern clone `waiters` Set thành array trước khi lặp —
  `resolveMessageWaiter` có thể xoá waiter khỏi Set gốc giữa lúc lặp.
- `orca-runtime.ts`: 10,331 → **10,236 dòng**. File mới: 128 dòng — dưới
  ngưỡng 300, không cần đăng ký `max-lines-baseline.txt`.
- Xác minh: `tsc --noEmit --composite false` giữ đúng baseline 251 lỗi (0
  lỗi mới, sạch ngay từ lần chạy đầu — không có method nào bị gọi ngoài
  domain cần public). `oxlint` sạch (exit 0) cả 2 config sau khi thêm
  disable. `max-lines-ratchet`: 647 vi phạm pre-existing không đổi.

## Rủi ro còn lại / khuyến nghị

- Move cơ học thuần tuý, rủi ro thấp. Khuyến nghị kiểm thử thủ công
  `orchestration.check --wait` (long-poll, timeout, abort qua signal)
  trước khi merge.

## Tổng kết luỹ kế

`orca-runtime.ts`: 26,730 → **10,236 dòng (61.7% giảm)** qua 27 task
(TASK-BIGFILE-036 đến 061, không tính TASK-054 Investigate).
