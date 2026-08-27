# TASK-BIGFILE-064 — Move: Headless (server-owned) PTY emulator domain

**Loại:** Move — composition pattern, rủi ro cao (chấp nhận rõ ràng bởi
người dùng, tương đương TASK-BIGFILE-051) · **Effort:** L · **Phụ thuộc:**
TASK-BIGFILE-063
**Status:** ✅ Done
**Solution:** `../solutions/SOLUTION-FE-BIGFILE-002-orca-runtime.md`

## Bối cảnh

Sau TASK-BIGFILE-063 (`RuntimePtyTranscriptStore`), khảo sát cụm còn lại
mà TASK-063 gợi ý ("headlessTerminals cluster... ứng viên state-container
riêng"). Khác với dự đoán ban đầu (chỉ tách field), rà kỹ method-body
dependency (đúng bài học TASK-057) cho thấy cụm này **CÓ THỂ tách được
như 1 domain Move đầy đủ** — không đan xen sâu như pty-title-tracker.
Trình bày rõ quy mô/rủi ro cho người dùng qua `AskUserQuestion` (477 dòng
gốc, 14 host dependency, ≥4 method cần public+forwarding vì bị gọi từ
`onPtyData`/`createTerminal`-adjacent core) — người dùng chọn **"Tách
(chấp nhận rủi ro cao, giống TASK-051)"**.

## Kết quả thực thi (2026-08-11)

- Domain: 16 method — `seedHeadlessTerminal`, `applyPushedViewAttributesToAll`
  (mới, thay cho vòng lặp trực tiếp field), `isAlternateScreen` (mới, thay
  `isTerminalAlternateScreen` cũ), `maybeHydrateHeadlessFromRenderer`,
  `shouldAnswerQueriesForLiveChunk`, `trackHeadlessTerminalData`,
  `createPtyHeadlessTerminalState` (private), `ensureNativeWindowsConptyDa1Override`,
  `getOrCreateHeadlessTerminal` (private), `resizeHeadlessTerminal`,
  `clearHeadlessTerminalBuffer`, `serializeTerminalBufferFromAvailableState`,
  `serializeRendererTerminalBuffer`, `withVisibleSnapshotFallback`,
  `readRendererVisibleSnapshotLines` (private), `serializeHeadlessTerminalBuffer`,
  `disposeHeadlessTerminal` — dòng gốc 3083–3568 (486 dòng, kể cả comment
  mở đầu).
- 14 host dependency: `getGraph`, `getStore`, `getPtyController`,
  `getPtyTranscripts` (từ TASK-063), `getHeadlessHydrationState`,
  `getTerminalSize`, `hasRemoteTerminalViewSubscriber` (đã public từ
  TASK-062), `recordOsc7MetadataForPty`, `recordRecentPtyOutputForPathProvenance`,
  `getTrackedRawTitleForPty`, `applySeededAgentStatus`, `pathFlavorForPty`,
  `preferTrackedLastTitle` (generic method, giữ nguyên chữ ký).
- **Quyết định thiết kế quan trọng**: `applySeededAgentStatus` (method
  gốc nằm trong cụm) được xếp làm **host dependency thay vì chuyển đi**,
  vì thân method của nó gọi sâu vào cụm pty-title-tracker
  (`getOrCreatePtyTitleTrackerEntry`, `nextTitleObservationSequence`,
  `setPtyManagementTitleFromObservedTitle`, `getLeavesForPty`, `graph.ptysById`)
  — đúng loại đan xen đã khiến TASK-057 bị huỷ. Giữ method này ở
  `orca-runtime.ts` giảm đáng kể số host dep cần thiết (nếu chuyển đi sẽ
  cần thêm ~5 host dep nữa, đẩy domain này gần mức rủi ro của
  pty-title-tracker).
- **2 điểm chạm trực tiếp field bị bỏ sót ở lần rà đầu, phát hiện qua
  `grep -n "this\.headlessTerminals\b"` toàn file (đúng quy trình đã sửa
  từ bài học TASK-062)**: (1) `isTerminalAlternateScreen` (method public
  KHÔNG thuộc cụm, dòng ~3079) đọc trực tiếp field — sửa thành gọi
  `this.headlessTerminalCommands.isAlternateScreen(ptyId)`; (2) constructor
  có closure `registerTerminalViewAttributesApplier((attributes) => { for
  (const state of this.headlessTerminals.values()) {...} })` — thêm
  method mới `applyPushedViewAttributesToAll` trên class mới, constructor
  gọi qua field đã khởi tạo sẵn (xác nhận field-initializer JS chạy TRƯỚC
  thân constructor bất kể vị trí khai báo trong source, kiểm chứng bằng
  script Node độc lập trước khi tin tưởng).
- **Lỗi tạm thời tự gây ra và tự sửa**: khi xoá cụm bằng Python script,
  vô tình xoá luôn `applySeededAgentStatus` (nằm trong range bị xoá dù
  quyết định giữ lại) — `tsc TS2339` bắt ngay, thêm lại method này vào
  `orca-runtime.ts` (cạnh `getOrCreatePtyTitleTrackerEntry`, cùng cụm
  pty-title-tracker về mặt vị trí).
- `RuntimeHeadlessTerminal`/`HeadlessSeedMetadata` (2 type nội bộ, dòng
  658/666) — chuyển hẳn (chỉ domain này dùng, kể cả field).
- 7 import free-function/const move-only sau khi dọn:
  `HeadlessEmulator`, `isNativeWindowsConptyPty`, `shouldModelAnswerHiddenPtyQueries`,
  `getTerminalViewAttributes`, `MOBILE_SUBSCRIBE_SCROLLBACK_ROWS`,
  `buildVisibleSnapshotReadFallback`, `shouldFallbackToVisibleTerminalSnapshot`
  — `registerConptyDa1OverrideInstaller`/`registerTerminalViewAttributesApplier`
  giữ nguyên (dùng trong constructor, không thuộc domain).
- `orca-runtime.ts`: 9,980 → **9,563 dòng** (giảm 417 dòng — mức giảm lớn
  nhất trong 1 task kể từ TASK-053). File mới: 536 dòng — đã đăng ký
  `config/max-lines-baseline.txt` + `eslint-disable max-lines` inline.
- Xác minh: `tsc --noEmit --composite false` giữ đúng baseline 251 lỗi (0
  lỗi mới, sau khi sửa 1 lỗi tạm thời `TS2339` do bỏ sót `applySeededAgentStatus`
  trong lần xoá đầu). `oxlint` sạch (exit 0) cả 2 config sau khi thêm
  disable. `max-lines-ratchet`: 647 vi phạm pre-existing không đổi.

## Rủi ro còn lại / khuyến nghị

- Domain rủi ro cao nhất được tách trong effort này kể từ TASK-051 (zero
  test coverage, đan xen với `onPtyData`/`createTerminal` core). Khuyến
  nghị kiểm thử thủ công kỹ trước khi merge: seed headless emulator từ
  daemon snapshot, hydrate từ renderer buffer khi PTY được pane nhận lại,
  ConPTY DA1 override trên Windows, resize/clear buffer, serialize buffer
  cho mobile/hidden-output-recovery, dispose khi PTY ngắt kết nối/bị prune.
- Phần còn lại của `orca-runtime.ts` (~9,500 dòng) vẫn là lõi thật:
  `graph`, cụm pty-title-tracker (`getTrackedRawTitleForPty`…`disposePtyTitleTracker`,
  bao gồm `applySeededAgentStatus` vừa thêm lại), cụm OSC-status processing
  (`onPtyData`, `processAgentStatusOscForPty`) — cần thiết kế lại sâu hơn
  (không phải Move đơn giản) hoặc bổ sung test coverage trước khi tách
  tiếp. `browserScreencast` (412 dòng, 1 method) vẫn cần Investigate
  riêng để refactor nội bộ trước khi cân nhắc Move.

## Tổng kết luỹ kế

`orca-runtime.ts`: 26,730 → **9,563 dòng (64.2% giảm)** qua 32 task
(TASK-BIGFILE-036 đến 064, trừ 057 đã huỷ; 041 và 063 là state-container
Extract).
