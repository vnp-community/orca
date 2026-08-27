# TASK-BIGFILE-072 — Move: terminal-waiter domain (exit/tui-idle waiters)

**Loại:** Move — composition pattern, rủi ro cao (trực tiếp trong hot path
`onPtyData`/`onPtyExit`) · **Effort:** L · **Phụ thuộc:** TASK-BIGFILE-041,
60, 63, 64, 67, 68, 69
**Status:** ✅ Done
**Solution:** `../solutions/SOLUTION-FE-BIGFILE-002-orca-runtime.md`

## Bối cảnh

Sau TASK-BIGFILE-071, sweep gap-analysis tiếp theo trên `orca-runtime.ts`
(7,808 dòng) không còn phát hiện cụm "list command" nào nữa — các gap lớn
còn lại hoặc là forwarding-field noise của các domain đã tách (Linear, false
positive kiểu TASK-066), hoặc là chính `waitForTerminal` (212 dòng, gap 320
tại dòng 3545).

Khác các task trước, cụm này **không liền mạch**: `waitForTerminal` (public,
đăng ký waiter) nằm cách xa các private helper resolve/reject/poll/teardown
của chính nó hơn 3,000 dòng — `resolveExitWaiters`,
`resolveTuiIdleWaiters`, `resolvePtyExitWaiters`, `resolvePtyTuiIdleWaiters`,
`startTuiIdleFallbackPoll`, `startPtyTuiIdleFallbackPoll`,
`getAdoptedPtyExplicitIdleStatus`, `resolveWaiter`, `bindTerminalWaiterAbort`,
`rejectWaitersForHandle`, `rejectAllWaiters`, `removeWaiter`. Toàn bộ vận
hành trên `graph.waitersByHandle`, được gọi trực tiếp từ `onPtyExit`
(`resolveExitWaiters`/`resolvePtyExitWaiters`) và từ closure host-wiring của
`ptyTitleTrackerCommands` (`resolveTuiIdleWaiters`/`resolvePtyTuiIdleWaiters`,
TASK-067) — tức là nằm ngay trong hot path `onPtyData`/`onPtyExit`, cùng mức
rủi ro với TASK-067/068/069.

Lưu ý phân biệt: đây là hệ thống waiter riêng cho điều kiện `exit`/`tui-idle`
của `orca wait`, KHÔNG liên quan tới `RuntimeTerminalMessageWaiterCommands`
(đã tách từ trước) — hệ đó phục vụ `waitForMessage`/message-arrival, khoá
theo `graph.messageWaitersByHandle` khác hẳn.

## Kết quả thực thi (2026-08-11)

- Domain (3 đoạn không liền mạch, gộp vào 1 file mới):
  - Đoạn A: `waitForTerminal` (public async, dòng gốc 3545–3756, 212 dòng).
  - Đoạn B: `resolveExitWaiters` → `getAdoptedPtyExplicitIdleStatus` (dòng
    gốc 6834–7083, 250 dòng) — **loại trừ** `deliverPendingMessages` nằm xen
    giữa `startPtyTuiIdleFallbackPoll`/`getAdoptedPtyExplicitIdleStatus` và
    `resolveWaiter`: method này dùng chung bởi `ptyTitleTrackerCommands`
    (TASK-067) và `terminalMessageWaiterCommands` (đã tách trước), không
    thuộc domain waiter này dù nằm cạnh — giữ nguyên tại `orca-runtime.ts`.
  - Đoạn C: `resolveWaiter` → `removeWaiter` (dòng gốc 7157–7217, 61 dòng).
- 6 host dependency: `getGraph`, `getPtyController` (closure field trực
  tiếp), `getLivePtyForHandle`, `getLiveLeafForHandle`, `issueHandle`,
  `getLeafKey` — tất cả method core đã tồn tại, dùng rộng rãi ở nơi khác
  trong `orca-runtime.ts` nên STAYS, không đổi visibility.
  `getAdoptedPtyExplicitIdleStatus` xác nhận chỉ tự tham chiếu trong cụm →
  MOVE (không phải host dep) dù ban đầu nghi ngờ do nằm cạnh
  `deliverPendingMessages`.
- 5 call site ngoài cụm cần cập nhật sang `this.terminalWaiterCommands.*`:
  `onPtyExit` (`resolvePtyExitWaiters`, `resolveExitWaiters`), host-wiring
  closure của `ptyTitleTrackerCommands` (`resolveTuiIdleWaiters`,
  `resolvePtyTuiIdleWaiters`), `invalidateLeafHandle`
  (`rejectWaitersForHandle`), `markRendererReloading` +
  `markGraphUnavailable` (`rejectAllWaiters`, 2 chỗ).
- Chỉ `waitForTerminal` cần forwarding field (API công khai). 11 method còn
  lại private, không forwarding, chỉ gọi qua `this.terminalWaiterCommands.*`
  từ trong lớp hoặc từ closure host-wiring.
- 8 import move-only từ `orca-runtime-tail-buffer.ts` dọn sạch sau khi
  domain chuyển đi (không dùng nơi nào khác): `buildPtyTerminalWaitBlockedResult`,
  `buildPtyTerminalWaitResult`, `buildTerminalWaitBlockedResult`,
  `buildTerminalWaitResult`, `detectExplicitIdleStatusFromTitle`,
  `TUI_IDLE_DEFAULT_TIMEOUT_MS`, `TUI_IDLE_POLL_INTERVAL_MS`,
  `TUI_IDLE_QUIESCENCE_MS`. `buildTerminalWaitText`, `detectTerminalWaitBlockedReason`,
  `getTerminalState`, `isKnownReadyPromptPreview` — STAYS (dùng nơi khác).
- `orca-runtime.ts`: 7,808 → **7,290 dòng**. File mới
  `orca-runtime-terminal-waiter.ts`: 600 dòng (515 dòng non-blank/non-comment)
  — đã đăng ký `config/max-lines-baseline.txt` + `eslint-disable max-lines`
  + `eslint-disable unicorn/no-useless-spread` (mẫu clone-trước-khi-iterate
  giống `orca-runtime.ts`/`orca-runtime-terminal-message-waiter.ts`, vì
  resolve waiter xoá chính waiter đó khỏi Set đang duyệt).
- Xác minh: `tsc --noEmit --composite false` giữ đúng baseline 251 lỗi (0
  lỗi mới trong file mới, chỉ cần dọn 8 import move-only sau lần chạy đầu).
  `oxlint` sạch (exit 0) cả 2 config sau khi thêm 2 inline-disable ở trên.
  `max-lines-ratchet`: 647 vi phạm pre-existing không đổi (phải `git add`
  file mới trước khi chạy check — script chỉ quét `git ls-files`, file
  untracked không được tính nên báo "stale" giả tạo lúc đầu).

## Rủi ro còn lại / khuyến nghị

- **Rủi ro cao** — cùng mức TASK-067/068/069: `resolveExitWaiters`/
  `resolvePtyExitWaiters` chạy trong `onPtyExit`, `resolveTuiIdleWaiters`/
  `resolvePtyTuiIdleWaiters` chạy trong `onPtyData` (qua closure
  `ptyTitleTrackerCommands`). Khuyến nghị kiểm thử thủ công kỹ trước khi
  merge: `orca wait --for exit` và `orca wait --for tui-idle` trên cả PTY
  thường và PTY-backed (background/daemon), timeout, abort signal
  (Ctrl+C khi đang wait), graph reload giữa lúc đang wait (phải reject với
  `terminal_handle_stale`), và đường polling fallback (agent không phát OSC
  title chuẩn — quan trọng cho daemon/SSH terminals).
- Phần còn lại của `orca-runtime.ts` (~7,290 dòng): `onPtyData` (dispatcher
  chính, không tách được thêm), `createTerminal`/`splitPtyBackedTerminal`/
  `launchAgentTerminal` (tạo terminal, đan xen sâu với `graph`/
  `headlessTerminals`), cùng nhiều method utility nhỏ lẻ (getTerminalPaneKey,
  resolveTerminalPane, v.v.) không đủ lớn để tách riêng. Sweep gap-analysis
  toàn diện đã chạy lại sau task này — không còn cụm non-contiguous nào khác
  cùng hình dạng được phát hiện.

## Tổng kết luỹ kế

`orca-runtime.ts`: 26,730 → **7,290 dòng (72.7% giảm)** qua 40 task
(TASK-BIGFILE-036 đến 072, trừ 057 đã huỷ rồi tái thực thi ở 067; 041 và 063
là state-container Extract).
