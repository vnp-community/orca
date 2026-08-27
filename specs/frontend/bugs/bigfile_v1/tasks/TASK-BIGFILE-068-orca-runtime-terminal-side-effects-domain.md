# TASK-BIGFILE-068 — Move: PTY side-effect fact + OSC 7 cwd-tracking domain (rủi ro cao)

**Loại:** Move — composition pattern, rủi ro CAO (tiếp nối chấp nhận rủi
ro của TASK-BIGFILE-067) · **Effort:** L · **Phụ thuộc:** TASK-BIGFILE-060,
063, 064, 067
**Status:** ✅ Done
**Solution:** `../solutions/SOLUTION-FE-BIGFILE-002-orca-runtime.md`

## Bối cảnh

Ngay sau TASK-BIGFILE-067 (pty-title-tracker), khảo sát cụm liền kề:
`processAgentStatusOscForPty`/`flushPendingTerminalSideEffectFacts`/
`recordTerminalSideEffectFact`/`emitTerminalSideEffectBatch`/
`resolveTerminalSideEffectAttribution`/`getTerminalSideEffectSnapshot`
(pty:sideEffect fact scanning/emission — anh em của pty-title-tracker,
CÙNG bị `onPtyData` gọi trực tiếp) và
`extractLastOsc7CwdForPty`/`recordOsc7MetadataForPty`/`pathFlavorForPty`
(OSC 7 cwd tracking — đã là host dep của `headless-terminal`, TASK-064).
Cùng hình dạng rủi ro với 067 (nhiều host dep, nhưng phần lớn đã
public+forwarded từ các domain trước) — tách ngay tiếp nối, đúng chỉ đạo
"xử lý triệt để" của người dùng.

## Phát hiện quan trọng: composition wiring của 067 nằm giữa 2 đoạn ứng viên

Range ứng viên ban đầu (`processAgentStatusOscForPty` → `pathFlavorForPty`)
bị cắt làm 2 đoạn bởi chính block composition wiring của
`ptyTitleTrackerCommands` (thêm ở TASK-067, nằm giữa
`getTerminalSideEffectSnapshot` và `extractLastOsc7CwdForPty`) — xử lý
như 2 đoạn rời, gộp vào cùng 1 file (đúng mẫu "excluded chunk in the
middle" đã gặp nhiều lần).

## Lỗi tự gây ra và tự sửa (không phải do tsc)

Khi cắt bằng script Python, tính sai ranh giới đoạn 2 khiến 2 dòng
docstring của method NGAY SAU (`emitTerminalAgentStatusEvents`, không
thuộc phạm vi cần chuyển) bị cuốn theo — phát hiện qua đọc lại thủ công
đoạn code còn sót (comment mồ côi thiếu dòng mở `/**`), sửa bằng cách
thêm lại dòng bị mất trước khi chạy `tsc`.

## Kết quả thực thi (2026-08-11)

- Domain: 13 method — `processAgentStatusOscForPty`,
  `flushPendingTerminalSideEffectFacts`, `ingestSyntheticTitleFrame`,
  `setPtyTransientFactDelegation`, `emitDaemonPtyTransientFact`,
  `notePtyDataGap`, `recordTerminalSideEffectFact` (private→public),
  `emitTerminalSideEffectBatch` (private), `resolveTerminalSideEffectAttribution`
  (private), `getTerminalSideEffectSnapshot`, `extractLastOsc7CwdForPty`
  (private), `recordOsc7MetadataForPty`, `pathFlavorForPty` — dòng gốc
  2284–2491 + 2529–2586, 266 dòng.
- 9 host dependency: `getGraph`, `getPtyTranscripts` (TASK-063),
  `getOnTerminalSideEffects` (trả về chính callback, khác kiểu boolean
  đơn giản của TASK-067 — method này THỰC SỰ gọi callback
  `onTerminalSideEffects(batch)`, không chỉ kiểm tra null),
  `getLeavesForPty`, `getOrCreatePtyTitleTrackerEntry` (đã public từ
  TASK-067), `getOrCreatePtyWorktreeRecord`, `makeRuntimePaneKey`,
  `touchMobileSessionSnapshotsForPty` (đã public từ TASK-051),
  `disposeHeadlessTerminal` (đã public từ TASK-064).
- **Quyết định thiết kế**: `pathFlavorForPty` ban đầu định để làm host
  dep (đã là host dep của `headless-terminal.ts` từ TASK-064), nhưng nhận
  ra nó hoàn toàn tự chứa (chỉ dùng `splitWorktreeIdForFilesystem`/
  `isWindowsAbsolutePathLike`, không phụ thuộc gì khác) nên chuyển HẲN
  vào file này thay vì giữ ở `orca-runtime.ts` — `headless-terminal.ts`'s
  host wiring KHÔNG cần đổi (vẫn gọi qua forwarding field trên
  `OrcaRuntimeService`, giờ trỏ sang class mới thay vì method cũ).
- 5 method cần public + forwarding (bị gọi từ `onPtyData` hoặc từ
  `orca-runtime-headless-terminal.ts`/`orca-runtime-pty-title-tracker.ts`'s
  host wiring): `processAgentStatusOscForPty`, `flushPendingTerminalSideEffectFacts`,
  `recordTerminalSideEffectFact`, `recordOsc7MetadataForPty`,
  `pathFlavorForPty`.
- Sau khi chuyển, 6 free-function/type trở thành move-only thật sự (KHÔNG
  còn dùng ở đâu khác trong `orca-runtime.ts` — một số suýt tưởng nhầm là
  "STAYS" ở TASK-067 vì lúc đó vẫn còn 1 usage trong cụm này, giờ mới thật
  sự hết): `isCursorNativeAgentTitle`, `normalizeTerminalTitle`,
  `extractLastOsc7Uri`, `extractOscScanTail`, `parseFileUriPathParts`,
  `createAgentStatusOscProcessor`, `PtyTransientFact`.
- `orca-runtime.ts`: 8,796 → **8,571 dòng**. File mới: 317 dòng (257 dòng
  non-blank/non-comment, DƯỚI ngưỡng 300) — không cần đăng ký
  `config/max-lines-baseline.txt`.
- Xác minh: `tsc --noEmit --composite false` giữ đúng baseline 251 lỗi (0
  lỗi mới, sau khi sửa comment mồ côi + dọn 7 import move-only thật sự).
  `oxlint` sạch (exit 0) cả 2 config. `max-lines-ratchet`: 647 vi phạm
  pre-existing không đổi.

## Rủi ro còn lại / khuyến nghị

- Cùng mức rủi ro cao như TASK-067 — `processAgentStatusOscForPty`/
  `flushPendingTerminalSideEffectFacts` gọi từ `onPtyData` cho MỌI byte
  PTY nhận được, zero test coverage. Khuyến nghị kiểm thử thủ công cùng
  batch với TASK-067: OSC 9999 agent-status parsing đúng thứ tự byte, OSC
  7 cwd tracking (file tap trên mobile resolve đúng path), daemon
  transient-fact relay khi PTY bị background/keep-tail-thinning, PTY data
  gap reset đúng state khi daemon drop byte.
- Phần còn lại của `orca-runtime.ts` (~8,571 dòng): `onPtyData` chính nó
  (dispatcher, gọi vào tất cả domain đã tách), `createTerminal`, `graph`,
  `getWorktreePs`/`attachAgentRowsToSummaries` (đọc `graph` trực tiếp) —
  đây là lõi cuối cùng, phần lớn là orchestration logic khó tách rời hơn
  (không còn "cụm nhiều method" rõ ràng, mà là 1-2 method dispatcher lớn
  gọi vào rất nhiều domain đã tách).

## Tổng kết luỹ kế

`orca-runtime.ts`: 26,730 → **8,571 dòng (67.9% giảm)** qua 36 task
(TASK-BIGFILE-036 đến 068, trừ 057 đã huỷ rồi tái thực thi ở 067; 041 và
063 là state-container Extract).
