# TASK-BIGFILE-067 — Move: PTY title-tracker domain (rủi ro cao, tái thực thi sau TASK-057)

**Loại:** Move — composition pattern, rủi ro CAO (chấp nhận rõ ràng bởi
người dùng) · **Effort:** L · **Phụ thuộc:** TASK-BIGFILE-054 (huỷ ở
057), 060, 063, 064
**Status:** ✅ Done
**Solution:** `../solutions/SOLUTION-FE-BIGFILE-002-orca-runtime.md`

## Bối cảnh

TASK-BIGFILE-057 huỷ việc tách domain này vì method-body dependency quá
sâu (~15 host dep, nhiều method core). Sau khi TASK-BIGFILE-060
(`pty-foreground-agent-refresh`) và TASK-BIGFILE-064 (`headless-terminal`)
hoàn tất, 3 trong số các dependency từng chặn 057
(`refreshPtyForegroundAgentFromController`, `getPendingForegroundAgentRefreshForTitle`,
`delayPtyBackedMobileSnapshotForForegroundAgent`) **đã public+forwarded
sẵn** — thu hẹp đáng kể mức đan xen. Người dùng yêu cầu tiếp tục "xử lý
triệt để vấn đề PTY", chấp nhận rủi ro cao — tái thực thi domain này với
rà soát method-body dependency đầy đủ (đúng bài học từ 057).

## Kết quả thực thi (2026-08-11)

- Domain: `getTrackedRawTitleForPty`, `preferTrackedLastTitle`,
  `makeMobileTitleGateKey` (private, chỉ tự tham chiếu),
  `applySeededAgentStatus` (chuyển CÙNG cụm lần này — trước đó TASK-064
  cố ý giữ lại ở `orca-runtime.ts` vì phụ thuộc pty-title-tracker; giờ cả
  2 domain đã ở cùng 1 file nên không cần tách rời nữa),
  `getOrCreatePtyTitleTrackerEntry`, `applyTrackedPtyTitle`,
  `disposePtyTitleTracker` — dòng gốc 2496–2772, 277 dòng.
- 15 host dependency: `getGraph`, `getPtyTranscripts` (TASK-063),
  `getOnTerminalSideEffects` (đọc field `onTerminalSideEffects !== null`,
  field này vẫn ở `orca-runtime.ts` vì dùng nhiều nơi khác),
  `getLeavesForPty`, `nextTitleObservationSequence`,
  `setPtyManagementTitleFromObservedTitle`, `recordTerminalSideEffectFact`,
  `touchMobileSessionSnapshotsForPty` (đã public từ TASK-051),
  `resolveTuiIdleWaiters`, `resolvePtyTuiIdleWaiters`,
  `deliverPendingMessages`, `shouldDelayPtyBackedMobileSnapshotForForegroundAgent`,
  `refreshPtyForegroundAgentFromController`/`getPendingForegroundAgentRefreshForTitle`/
  `delayPtyBackedMobileSnapshotForForegroundAgent` (cả 3 đã public từ
  TASK-060 — không cần thêm forwarding mới cho domain khác, chỉ cần
  closure trong `orca-runtime.ts`).
- 5 method cần public + forwarding trên `OrcaRuntimeService` (bị gọi từ
  `onPtyData`/`onPtyExit`/`dropDisconnectedPtyRecord`, các method core
  KHÔNG chuyển, và từ `orca-runtime-headless-terminal.ts`'s host wiring
  đã tồn tại sẵn từ TASK-064 — `getTrackedRawTitleForPty`,
  `preferTrackedLastTitle`, `applySeededAgentStatus`): tất cả 6 method
  public trừ `makeMobileTitleGateKey`.
- **Lỗi tự gây ra và tự sửa (không phải do tsc bắt, do kiểm tra thủ
  công)**: khi viết file mới, chuỗi `\u0000` (ký tự null dùng làm dấu
  phân cách trong `makeMobileTitleGateKey`) bị công cụ ghi file diễn giải
  thành 1 byte null thực sự thay vì giữ nguyên 6 ký tự nguồn `\`+`u`+`0`+`0`+`0`+`0`
  — phát hiện qua lệnh `file` báo file là "data" thay vì "text", sửa bằng
  script Python thay byte null bằng chuỗi escape đúng.
- **Kết quả xác minh đáng chú ý**: sau khi sửa lỗi `\u0000`, `tsc` báo
  sạch **ngay từ lần chạy đầu tiên thực sự** (chỉ 4 lỗi unused-import cần
  dọn, KHÔNG có lỗi logic/tham chiếu nào) — xác nhận việc rà method-body
  dependency trước khi viết (đúng quy trình từ bài học 057) đã loại bỏ
  hoàn toàn rủi ro sót call site, kể cả với domain rủi ro cao nhất trong
  toàn bộ effort.
- `createTerminalTitleTracker`, `stripBrailleSpinnerGlyphs`,
  `createCommandCodeOutputStatusDetector`, `TerminalGitHubPRLink` — move
  hẳn khỏi `orca-runtime.ts`. `TerminalTitleTracker`, `TerminalSideEffectFact`,
  `TerminalSideEffectBatch` — giữ nguyên (dùng ở field/method khác), import
  lại bản sao.
- `orca-runtime.ts`: 9,040 → **8,796 dòng**. File mới: 364 dòng (304
  dòng non-blank/non-comment) — đã đăng ký `config/max-lines-baseline.txt`
  + `eslint-disable max-lines` inline.
- Xác minh: `tsc --noEmit --composite false` giữ đúng baseline 251 lỗi (0
  lỗi mới). `oxlint` sạch (exit 0) cả 2 config (sau khi sửa `import {
  type X }` → `import type { X }` theo `no-import-type-side-effects`).
  `max-lines-ratchet`: 647 vi phạm pre-existing không đổi.

## Rủi ro còn lại / khuyến nghị

- **Đây là domain rủi ro cao nhất đã tách trong toàn bộ effort** —
  `onPtyData` (hot path xử lý mọi byte PTY) gọi trực tiếp vào
  `getOrCreatePtyTitleTrackerEntry`/`applyTrackedPtyTitle` hàng nghìn
  lần/giây khi TUI flood, zero test coverage. Khuyến nghị BẮT BUỘC kiểm
  thử thủ công kỹ trước khi merge: agent title working→idle→working liên
  tục (Claude Code, Codex, Cursor), spinner braille glyph không gây spam
  mobile snapshot, OSC 9999 side-effect facts đúng thứ tự byte, seed title
  sau khi resume/relaunch app, dispose tracker khi PTY ngắt kết nối/bị
  prune — đặc biệt trên luồng cao tần (TUI flood, nhiều pane cùng lúc).
- Phần còn lại của `orca-runtime.ts` (~8,796 dòng): `onPtyData` chính nó,
  `createTerminal`, `graph`, cụm OSC 7/OSC-status-osc processing
  (`processAgentStatusOscForPty`, `extractLastOsc7CwdForPty`), `getWorktreePs`/
  `attachAgentRowsToSummaries` — đây là lõi PTY thật sự cuối cùng, đan xen
  nhất trong toàn bộ file. Tách tiếp cần rà kỹ hơn nữa hoặc bổ sung test
  trước.

## Tổng kết luỹ kế

`orca-runtime.ts`: 26,730 → **8,796 dòng (67.1% giảm)** qua 35 task
(TASK-BIGFILE-036 đến 067, trừ 057 đã huỷ rồi tái thực thi thành công ở
067; 041 và 063 là state-container Extract).
