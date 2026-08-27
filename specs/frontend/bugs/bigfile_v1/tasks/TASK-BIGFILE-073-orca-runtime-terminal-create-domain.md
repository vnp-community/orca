# TASK-BIGFILE-073 — Move: terminal-create/split domain

**Loại:** Move — composition pattern, rủi ro trung bình (ngoài onPtyData, nhưng
cụm entangled nhất file) · **Effort:** L · **Phụ thuộc:** TASK-BIGFILE-036, 037,
040, 051, 052
**Status:** ✅ Done
**Solution:** `../solutions/SOLUTION-FE-BIGFILE-002-orca-runtime.md`

## Bối cảnh

Theo yêu cầu người dùng ("tiếp tục phân tách... với cả 3 cụm đề xuất"): cụm
`createTerminal`/`splitPtyBackedTerminal`/`launchAgentTerminal` là cụm đầu
tiên trong 3 cụm còn lại được nêu ở TASK-071/072 (cụm thứ hai là decompose
`onPtyData`, thứ ba là các method utility nhỏ lẻ).

Domain gồm `createTerminal` (public, 2 nhánh: background-spawn cho headless/
CLI và renderer-IPC cho desktop UI), `launchAgentTerminal` (public, gọi lại
`createTerminal`), `splitTerminal` + `splitPtyBackedTerminal` (private) — và
4 helper wait-for-handle độc quyền của cụm này: `waitForTerminalHandle`,
`waitForLeafPtyId` (public, gọi qua RPC `terminal.ts` cho mobile subscribe),
`resolveHandleForTab`, `waitForNewLeafInTab`.

`countLeavesInTab` nằm liền kề `resolveHandleForTab` nhưng KHÔNG thuộc cụm —
xác nhận qua audit: dùng bởi `closeTerminal` (method khác, không tách), comment
"Why: a leaf appears in the graph before its PTY spawns..." trước
`countLeavesInTab` trong bản gốc thực ra mô tả logic của `resolveHandleForTab`
ngay sau nó — giữ nguyên comment đó gắn với `resolveHandleForTab` khi tách,
`countLeavesInTab` ở lại không comment.

## Kết quả thực thi (2026-08-12)

- Domain: `createTerminal`, `launchAgentTerminal`, `waitForTerminalHandle`,
  `waitForLeafPtyId`, `resolveHandleForTab` (dòng gốc 4293–4781, trừ
  `countLeavesInTab` 4760–4772 ở lại), `splitTerminal` + `splitPtyBackedTerminal`
  (4840–4960), `waitForNewLeafInTab` (5007–5049).
- 26 host dependency — cụm dày đặc nhất từ trước đến nay (hơn cả headless-terminal
  TASK-064's 14): `getStore`, `getGraph`, `getPtyController`, `getNotifier`
  (interface `RuntimeTerminalCreateNotifier` cục bộ, chỉ 2 method cần —
  `RuntimeNotifier` gốc không export, cùng mẫu các domain trước),
  `getClaudeAgentTeams`, `getAuthoritativeWindow`,
  `getAvailableAuthoritativeWindow`, `assertGraphReady`, `resolveWorktreeSelector`,
  `resolveTerminalWorkspaceLaunchScope`, `resolveAgentTerminalCreateOptions`,
  `resolveWorkspaceTerminalStartupCwd`, `buildTerminalWorkspaceEnv`,
  `createPreAllocatedTerminalHandle`, `registerPreAllocatedHandleForPty`,
  `registerPty`, `getOrCreatePtyWorktreeRecord`, `nextTitleObservationSequence`,
  `setPtyManagementTitleFromObservedTitle`, `issuePtyHandle`, `issueHandle`,
  `publishPtyBackedMobileSessionTerminal`/`persistHeadlessTerminalSplit`
  (kiểu qua `RuntimeMobileSessionTabsCommands['...']` — public, tránh chép
  tay object shape phức tạp), `buildStartupForAgent`/
  `markLocalWorkspaceTrustedForAgent`/`markRemoteWorkspaceTrustedForAgent`
  (kiểu qua `RuntimeWorktreeCreationCommands['...']`), `getLivePtyForHandle`,
  `getLiveLeafForHandle`, `resolveLeafForHandle`, `getLeafKey`.
- 4 method public giữ nguyên là forwarding field trên `OrcaRuntimeService`
  (`createTerminal`, `launchAgentTerminal`, `splitTerminal`, `waitForLeafPtyId`)
  → **không cần sửa call site nào khác trong file** — các closure host-wiring
  đã tồn tại từ trước (`worktreeCreationCommands`, `mobileSessionTerminalCommands`,
  `handleAgentTeamsTmuxCompat`) gọi `this.createTerminal(...)`/
  `this.splitTerminal(...)` vẫn hoạt động nguyên vẹn vì các field đó giờ trỏ
  sang `terminalCreateCommands` — khác các task 067–072 vốn luôn cần cập nhật
  vài call site.
- 3 free function move-only (không dùng nơi khác sau khi domain chuyển đi):
  `createTerminalRevealWarning`, `resolveTerminalPresentation`,
  `inferCapturedClaudeAgentTeamsMode`. `copySleepingAgentLaunchConfig` — STAYS
  (dùng ở `focusTerminal` và nơi khác), thêm `export` để file mới import lại.
  6 import move-only dọn theo: `SETUP_AGENT_SEQUENCE_STARTUP_COMMAND_ENV`,
  `isValidHostTerminalTabId` (giữ `isValidTerminalTabId` sibling), `ipcMain`
  (giữ `BrowserWindow`), `buildClaudeAgentTeamsLaunchPlan`,
  `addClaudeTeammateModeAuto`, `addClaudeTeammateModeInProcess`. 2 type
  move-only: `RuntimeTerminalCreate`, `RuntimeTerminalSplit` (giữ
  `RuntimeTerminalPresentation`, còn dùng nơi khác).
- **Bug tự phát hiện khi transcribe `inferCapturedClaudeAgentTeamsMode`**:
  viết nhầm logic hàm khi soạn file mới (nhớ nhầm từ một đoạn code không liên
  quan đọc trước đó trong phiên) — `tsc` bắt ngay lập tức
  (`TS2322: Type '"auto"' is not assignable to type 'ClaudeAgentTeamsMode'`)
  vì literal `'auto'`/return-path sai không khớp type `ClaudeAgentTeamsMode`.
  Xác minh lại bằng cách diff nguyên văn thân hàm gốc (qua `git show HEAD:...`)
  so với bản mới sau khi chuẩn hoá `this.host.` → `this.` — phát hiện và sửa
  đúng logic gốc (`'native-panes-shim'`/`'in-process'`/`'off'`/`currentMode`,
  không phải `'auto'`/`'in-process'`/`currentMode`/`currentMode` như viết
  nhầm). Diff toàn bộ thân 4 method còn lại khớp 100% ngoài các thay thế
  `this.X` → `this.host.getX()` có chủ đích và reformat cosmetic — không phát
  hiện sai lệch logic nào khác.
- **2 lỗi `tsc` thật (không phải move-only)**: `this.host.getPtyController()!.spawn(...)`
  — TS không narrow được optional-chaining qua lệnh gọi method (khác truy cập
  field trực tiếp `this.ptyController` ở bản gốc, TS narrow được qua nhiều
  statement). Sửa bằng cách gán `const ptyController = this.host.getPtyController()`
  một lần rồi tái sử dụng biến đã narrow, ở cả `createTerminal` lẫn
  `splitPtyBackedTerminal`.
- `orca-runtime.ts`: 7,289 → **6,708 dòng**. File mới
  `orca-runtime-terminal-create.ts`: 793 dòng (cụm dày đặc nhất, tương xứng
  26 host dependency) — đã đăng ký `config/max-lines-baseline.txt` +
  `eslint-disable max-lines`.
- Xác minh: `tsc --noEmit --composite false` giữ đúng baseline 251 lỗi sau
  khi sửa 2 lỗi thật + dọn 11 import/type/function move-only. `oxlint` sạch
  (exit 0) cả 2 config — không cần thêm disable nào khác lần này (không có
  spread-clone pattern hay import-type-side-effects trong cụm này).
  `max-lines-ratchet`: 647 vi phạm pre-existing không đổi.

## Rủi ro còn lại / khuyến nghị

- Rủi ro trung bình — KHÔNG nằm trong hot path `onPtyData`/`onPtyExit` (chỉ
  gọi khi tạo/split terminal, tần suất thấp hơn nhiều), nhưng là cụm entangled
  nhất file (26 host dependency, chạm gần như mọi domain đã tách trước đó).
  Khuyến nghị kiểm thử thủ công kỹ: tạo terminal thường (renderer-backed) và
  headless/CLI (background-spawn) trên cả local lẫn SSH/remote repo, launch
  agent terminal (Claude/Codex/Cursor/Gemini, cả Claude Agent Teams
  auto/in-process mode), split terminal thường và split PTY-backed
  (background terminal), mobile session terminal creation (đường dùng chung
  `createTerminal` qua `mobileSessionTerminalCommands`).
- Còn lại 2 cụm theo yêu cầu người dùng: decompose `onPtyData` (TASK-BIGFILE-074,
  tiếp theo) và rà quét method utility nhỏ lẻ (TASK-BIGFILE-075+).

## Tổng kết luỹ kế

`orca-runtime.ts`: 26,730 → **6,708 dòng (74.9% giảm)** qua 41 task
(TASK-BIGFILE-036 đến 073, trừ 057 đã huỷ rồi tái thực thi ở 067; 041 và 063
là state-container Extract).
