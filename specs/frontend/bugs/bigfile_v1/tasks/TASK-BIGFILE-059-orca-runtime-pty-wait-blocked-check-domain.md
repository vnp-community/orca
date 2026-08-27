# TASK-BIGFILE-059 — Move: PTY wait-blocked-check domain

**Loại:** Move — composition pattern · **Effort:** S · **Phụ thuộc:**
TASK-BIGFILE-054
**Status:** ✅ Done
**Solution:** `../solutions/SOLUTION-FE-BIGFILE-002-orca-runtime.md`

## Bối cảnh

3/8 "đảo an toàn" từ TASK-BIGFILE-054. `waitBlockedCheckStateByPtyId` cô
lập, chỉ 1 host dependency thật (`getGraph()`).

## Kết quả thực thi (2026-08-11)

- Domain: `scheduleWaitBlockedCheck` (public), `runWaitBlockedCheck`
  (private), `clearWaitBlockedCheckState` (public) — dòng gốc 2395–2467,
  73 dòng.
- 1 host dependency: `getGraph()`. Field `waitBlockedCheckStateByPtyId`
  chuyển hẳn vào class mới.
- 2 method cần public + forwarding: `scheduleWaitBlockedCheck` (gọi từ
  `onPtyData`), `clearWaitBlockedCheckState` (gọi từ `onPtyExit` và
  `pruneDisconnectedPtyRecords`) — phát hiện trước khi viết qua
  `grep -n`.
- 3 hằng số module-scope (`WAIT_BLOCKED_CHECK_MIN_INTERVAL_MS`,
  `WAIT_BLOCKED_KEYWORD_PATTERN`, `WAIT_BLOCKED_KEYWORD_CARRY_CHARS`, định
  nghĩa ở cuối `orca-runtime.ts`, không phải import) — chuyển hẳn, chỉ
  domain này dùng.
- `MAX_TAIL_CHARS` (import từ `./orca-runtime-tail-buffer`, phần của khối
  import lớn) — move hẳn, không dùng nơi khác. `TerminalTailWaitState`/
  `computeTerminalTailWaitState`/`tailGainedNewerBlockedReason` — giữ
  nguyên ở `orca-runtime.ts` (dùng ở `onPtyData`/type field khác), import
  lại bản sao trong file mới.
- `orca-runtime.ts`: 10,421 → **10,331 dòng**. File mới: 106 dòng — dưới
  ngưỡng 300, không cần đăng ký `max-lines-baseline.txt`.
- Xác minh: `tsc --noEmit --composite false` giữ đúng baseline 251 lỗi (0
  lỗi mới). `oxlint` sạch (exit 0) cả 2 config. `max-lines-ratchet`: 647 vi
  phạm pre-existing không đổi.

## Rủi ro còn lại / khuyến nghị

- Move cơ học thuần tuý, rủi ro thấp. Khuyến nghị kiểm thử thủ công phát
  hiện "wait blocked" trên PTY output (trust prompt, update available,
  v.v.) trước khi merge.

## Tổng kết luỹ kế

`orca-runtime.ts`: 26,730 → **10,331 dòng (61.4% giảm)** qua 26 task
(TASK-BIGFILE-036 đến 059, không tính TASK-054 Investigate).
