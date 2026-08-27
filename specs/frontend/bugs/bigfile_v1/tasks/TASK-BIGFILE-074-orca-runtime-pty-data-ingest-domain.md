# TASK-BIGFILE-074 — Move: onPtyData (pty-data-ingest domain)

**Loại:** Move — composition pattern, rủi ro cao (hot path duy nhất chạy trên
MỌI byte output của MỌI terminal) · **Effort:** L · **Phụ thuộc:**
TASK-BIGFILE-063, 064, 067, 068, 069, 051
**Status:** ✅ Done
**Solution:** `../solutions/SOLUTION-FE-BIGFILE-002-orca-runtime.md`

## Bối cảnh

Cụm thứ hai trong 3 cụm người dùng yêu cầu ("decompose cả onPtyData"). Đã
từng đánh giá ở TASK-069/071 là "dispatcher chính, không tách được thêm" —
đánh giá đó chỉ đúng cho phương pháp "tách callee ra khỏi dispatcher" (đã
tận dụng hết, ~40 domain đã tách trước đó). Áp dụng đúng kỹ thuật đã dùng ở
TASK-BIGFILE-072 cho `waitForTerminal`: tách CHÍNH dispatcher ra thành một
domain riêng, giữ toàn bộ ~40 domain đã tách làm host dependency — không có
gì mâu thuẫn, chỉ là góc nhìn khác của cùng composition pattern.

`onPtyData` là method có tần suất gọi cao nhất toàn bộ file (mọi byte output
từ mọi PTY đều đi qua đây), theo `gitnexus`/`codegraph` impact scan không có
test coverage nào bao phủ trực tiếp (`⚠️ no covering tests found` cho bản
frontend). Rủi ro cao nhất trong toàn bộ effort BUG-FE-BIGFILE-002, ngang
hoặc hơn TASK-067/068/069 (những task đó tách callee được gọi TỪ trong
onPtyData; task này tách chính onPtyData).

## Kết quả thực thi (2026-08-12)

- Domain: nguyên vẹn method `onPtyData` (dòng gốc 1988–2192, 205 dòng) — di
  chuyển 100% logic, không rút gọn/viết lại. Xác minh bằng diff nguyên văn
  (chuẩn hoá `this.host.getX()` → `this.X` rồi so với `git show HEAD:...`)
  — khớp byte-for-byte, không phát hiện sai lệch logic nào (khác TASK-073's
  `inferCapturedClaudeAgentTeamsMode`, lần này sạch ngay từ đầu).
- 19 host dependency: `getGraph`, `getPtyTranscripts`, `getAgentDetector`
  (field `AgentDetector | null`), `getDataListeners` (field `Map`), cùng 10
  method public đã forward từ các domain trước — dùng kiểu qua
  `RuntimeXCommands['method']`: `recordOsc7MetadataForPty`/
  `processAgentStatusOscForPty`/`flushPendingTerminalSideEffectFacts`
  (`RuntimeTerminalSideEffectsCommands`, TASK-068), `shouldAnswerQueriesForLiveChunk`/
  `maybeHydrateHeadlessFromRenderer`/`trackHeadlessTerminalData`
  (`RuntimeHeadlessTerminalCommands`, TASK-064), `scheduleWaitBlockedCheck`
  (`RuntimePtyWaitBlockedCheckCommands`), `getOrCreatePtyTitleTrackerEntry`
  (`RuntimePtyTitleTrackerCommands`, TASK-067), `emitTerminalAgentStatusEvents`
  (`RuntimeAgentRowSnapshotCommands`, TASK-069), `touchMobileSessionSnapshotsForPty`
  (`RuntimeMobileSessionTabsCommands`, TASK-051) — cộng 5 method private
  (`recordRecentPtyOutputForPathProvenance`, `getOrCreatePtyWorktreeRecord`,
  `recordPtyWorktree`, `getLeavesForPty`, `makeRuntimePaneKey`), tất cả đã
  tồn tại, chỉ bọc closure, không đổi visibility.
- `onPtyData` giữ forwarding field public (không có internal caller nào khác
  trong `orca-runtime.ts`, gọi từ IPC layer/ssh-relay-session bên ngoài).
- 6 import move-only dọn theo (đều hoá ra CHỈ dùng trong onPtyData, dù lần
  khảo sát nhanh ban đầu tưởng nhầm là "dùng nơi khác" do đếm occurrence
  thô — `tsc` bắt chính xác toàn bộ 6 lỗi TS6133 ngay lần chạy đầu):
  `extractOscTitleScanTail`, `buildPreview`, `computeTerminalTailWaitState`,
  `tailGainedNewerBlockedReason`, `appendNormalizedToTailBuffer`,
  `normalizeTerminalChunk`, `tailStateMatches` (7 tổng cộng).
- `orca-runtime.ts`: 6,708 → **6,527 dòng**. File mới
  `orca-runtime-pty-data-ingest.ts`: 286 dòng (216 dòng non-blank/non-comment)
  — **dưới ngưỡng 300**, không cần đăng ký `max-lines-baseline.txt`.
- Xác minh: `tsc --noEmit --composite false` giữ đúng baseline 251 lỗi (0
  lỗi thật — toàn bộ 7 lỗi ban đầu là import move-only, không có lỗi kiểu
  narrow-qua-method-call như TASK-073). `oxlint` sạch (exit 0) cả 2 config,
  không cần disable nào (không dùng pattern clone-trước-khi-iterate).
  `max-lines-ratchet`: 647 vi phạm pre-existing không đổi.

## Rủi ro còn lại / khuyến nghị

- **Rủi ro cao nhất trong toàn bộ effort này** — không có unit test bao phủ
  trực tiếp `onPtyData`. Khuyến nghị kiểm thử thủ công BẮT BUỘC trước khi
  merge: gõ phím/nhận output trên terminal thường, terminal PTY-backed
  (background/daemon), SSH/remote PTY, output lớn/nhanh (agent CLI in ồ ạt),
  OSC title transitions (working→idle, permission), OSC 7 cwd tracking, OSC
  9999 agent-status hook payload, mobile subscribe qua `terminal.subscribe`
  nhận đúng data event, dev-server URL advertised watcher, Command Code
  scrape fallback (không hook).
- Còn lại cụm cuối theo yêu cầu người dùng: rà quét method utility nhỏ lẻ
  (TASK-BIGFILE-075+) — theo đánh giá TASK-071, các method như
  `getTerminalPaneKey`/`resolveTerminalPane` không đủ lớn để tách riêng
  từng cái; cần đánh giá lại xem có cụm nào gộp được sau khi 073/074 đã dọn
  bớt.

## Tổng kết luỹ kế

`orca-runtime.ts`: 26,730 → **6,527 dòng (75.6% giảm)** qua 42 task
(TASK-BIGFILE-036 đến 074, trừ 057 đã huỷ rồi tái thực thi ở 067; 041 và 063
là state-container Extract).
