# TASK-BIGFILE-076 — Move: terminal agent-status domain

**Loại:** Move — composition pattern, rủi ro trung bình-cao (22 method,
5 đoạn không liền mạch, phát hiện 2 lỗi thật trong quá trình audit) ·
**Effort:** L (lớn nhất về SỐ METHOD trong toàn effort) · **Phụ thuộc:**
TASK-BIGFILE-051, 065, 067, 071, 073
**Status:** ✅ Done
**Solution:** `../solutions/SOLUTION-FE-BIGFILE-002-orca-runtime.md`

## Bối cảnh

Theo yêu cầu người dùng: tiếp tục theo 3 hướng cụ thể — `getTerminalAgentStatus`,
`getPaneKeyForTerminalHandle`/`getTerminalAgentStatusSnapshot`, và `onPtyExit`
(task riêng, xem TASK-BIGFILE-077). `codegraph`/`gitnexus` impact scan xác nhận
`getPaneKeyForTerminalHandle`/`getTerminalAgentStatusSnapshot` không có test
coverage trực tiếp.

Khảo sát ban đầu tưởng đây là 1 cụm nhỏ (~150 dòng), nhưng audit `this.*` mở
rộng dần phát hiện đây là MỘT domain lớn duy nhất bị rải rác hữu cơ khắp
~2.700 dòng file: "terminal agent-status" (đọc trạng thái agent hiện tại) và
"pane-key/orchestration-context resolution" (định danh pane/handle) đan xen
chặt — lookup trạng thái agent phải đi qua pane key, lookup orchestration
context cũng vậy. Quyết định gộp thành 1 domain thay vì tách nhỏ hơn vì chia
cắt thêm sẽ tạo vòng phụ thuộc chéo giữa 2 domain mới không cần thiết.

Cũng lần thứ 3 trong session này regex gap-analysis bỏ sót method — lần này
là `terminalHasShellForegroundProcess` (`private async` kết hợp 2 modifier,
cùng lỗi regex đã sửa ở TASK-075) và toàn bộ cụm `isTerminalRunningAgent`
(150 dòng, phát hiện qua đọc thủ công phần thân method, không qua regex).

## Kết quả thực thi (2026-08-12)

- Domain: 22 method, 5 đoạn không liền mạch (dòng gốc trước khi tách):
  2780–2799 (`getTerminalPaneKey`, `resolveTerminalPane`), 2862–3030
  (`getTerminalAgentStatus`, `getTerminalAgentStatusPtyId`,
  `assertTerminalAgentStatusPtyBinding`, `getTerminalAgentStatusSnapshot`,
  `terminalHasShellForegroundProcess`), 3071–3110
  (`getFreshExplicitAgentStatusForHandle`), 5060–5095 (`getAgentStatusForHandle`,
  `getAgentStatusOrchestrationContextForPaneKey`,
  `getAgentStatusTerminalHandleForPaneKey`, `getAgentStatusLaunchConfigForPaneKey`),
  5136–5245 (`getAgentStatusOrchestrationContextForHandle`,
  `getRecentCompletedDispatchForTerminal`, `getTerminalHandleForPaneKey`,
  `getPtyRecordForPaneKey`, `getPaneKeyForTerminalHandle`), 5279–5439
  (`isTerminalRunningAgent`, `isPtyRunningAgent`,
  `isRecognizedForegroundAgentProcess`, `isAgentWrapperForegroundProcess`,
  `getPrimaryLeafForPty`).
- Loại trừ (đã xác nhận qua audit dùng-ở-nơi-khác, không phải suy đoán từ vị
  trí): `shouldDelayPtyBackedMobileSnapshotForForegroundAgent` (host dep của
  `ptyTitleTrackerCommands`, TASK-067), `getOrchestrationDbIfAvailable` (đã
  xác định STAYS từ TASK-065), `buildAgentOrchestrationByPaneKey` (host dep
  của `worktreePsCommands`, TASK-071), `setPtyManagementTitleFromObservedTitle`,
  `nextTitleObservationSequence`.
- 12 host dependency: `getGraph`, `getPtyController`, `getLivePtyForHandle`,
  `getLiveLeafForHandle`, `getOrchestrationDbIfAvailable`, `getLeafKey`,
  `getRuntimeId` (dùng method public sẵn có thay vì field `runtimeId` thô),
  `getLatestAgentStatusByPaneKey`, `issuePtyHandle`, `issueHandle`,
  `getLeavesForPty`, `getAgentStatusSnapshot` (bọc `getAgentStatusSnapshotFn?.()
  ?? []`, mẫu đã dùng ở TASK-071/074).
- 8 method public giữ forwarding field: `getTerminalPaneKey`,
  `resolveTerminalPane`, `getTerminalAgentStatus`, `getAgentStatusForHandle`,
  `getAgentStatusOrchestrationContextForPaneKey`,
  `getAgentStatusTerminalHandleForPaneKey`, `getAgentStatusLaunchConfigForPaneKey`,
  `isTerminalRunningAgent`.
- **2 lỗi thật phát hiện qua `tsc` sau khi tưởng đã audit đủ** (cả 2 đã được
  ghi nhận nghi ngờ ở bước khảo sát ban đầu — "4070 (external)" — nhưng
  không được xử lý triệt để lúc viết file mới):
  - `getTerminalHandleForPaneKey` (private, dự định chỉ nội bộ domain mới)
    thực ra được gọi từ `prepareClaudeAgentTeamsLeader` (method KHÔNG chuyển
    đi) — `tsc TS2551`.
  - `getAgentStatusOrchestrationContextForHandle` (private, dự định chỉ nội
    bộ) thực ra được gọi từ `buildAgentOrchestrationByPaneKey` (method STAYS)
    — `tsc TS2551`, 2 chỗ gọi.
  - Sửa: bỏ `private` ở cả 2 method trong domain mới (chuyển thành public),
    thêm forwarding field tương ứng trên `OrcaRuntimeService` — 2 call site
    cũ tiếp tục hoạt động nguyên vẹn, không cần sửa.
- **1 lỗi transcription thật khi soạn file mới** (bắt qua `tsc TS2305`/`TS2459`):
  viết nhầm nguồn import — `copySleepingAgentLaunchConfig` (thực ra từ
  `./orca-runtime`, đã `export` từ TASK-073) và `detectAgentStatusFromTitle`
  (thực ra từ `'../../shared/agent-detection'`) bị viết nhầm thành import từ
  `./orca-runtime-tail-buffer`. Sửa bằng cách kiểm tra nguồn thật trong
  `orca-runtime.ts` rồi sửa đúng.
- Xác minh fidelity bằng diff nguyên văn (chuẩn hoá `this.host.X` → `this.X`,
  bao gồm cả pattern `getAgentStatusSnapshotFn?.() ?? []`) so với
  `git show HEAD:...` — khớp, chỉ khác 4 chỗ capture `ptyController` vào biến
  cục bộ (mẫu TS-narrow-qua-method-call đã thấy ở TASK-073, áp dụng ngay từ
  lúc viết thay vì đợi lỗi).
- 18 import/type/const move-only dọn sạch sau `tsc`: `isShellProcess`,
  `AGENT_STATUS_STALE_AFTER_MS`, `AgentStatusEntry`,
  `buildOrchestrationTaskDisplayMetadata`, `isAgentForegroundWrapperProcess`/
  `isExpectedAgentProcess`/`recognizeAgentProcess` (cả import block),
  `RuntimeTerminalAgentStatus`, `RuntimeTerminalResolvePane`,
  `buildTerminalWaitText`, `classifyAgentTitle`, `classifyLatestAgentTitle`,
  `detectTerminalWaitBlockedReason`, `getLatestAgentCandidateTitle`,
  `getLatestAgentCandidateTitleInfo`, `isKnownReadyPromptPreview`,
  `mapExplicitAgentStateToRuntimeTerminalStatus`,
  `terminalTitleBlocksExplicitAgentStatus`,
  `FOREGROUND_AGENT_WRAPPER_RETRY_INTERVAL_MS`/`_TIMEOUT_MS` (2 const,
  chuyển vào file mới).
- 1 lỗi `oxlint no-import-type-side-effects` (import agent-status-types chỉ
  còn toàn `type X` inline sau khi bỏ `AGENT_STATUS_STALE_AFTER_MS` — chuyển
  sang `import type {...}` top-level).
- `orca-runtime.ts`: 6,336 → **5,816 dòng**. File mới
  `orca-runtime-terminal-agent-status.ts`: 643 dòng (561 non-blank/non-comment)
  — file domain lớn thứ hai toàn effort (sau `orca-runtime-terminal-create.ts`)
  — đã đăng ký `config/max-lines-baseline.txt` + `eslint-disable max-lines`.
- Xác minh: `tsc --noEmit --composite false` giữ đúng baseline 251 lỗi.
  `oxlint` sạch (exit 0) cả 2 config. `max-lines-ratchet`: 647 vi phạm
  pre-existing không đổi (4 "new bypass" báo động giả từ file renderer
  `agent-status.ts`/`.test.ts` không liên quan, xác nhận qua `git status`
  không có thay đổi ở các file đó).

## Rủi ro còn lại / khuyến nghị

- Rủi ro trung bình-cao — KHÔNG trong hot path `onPtyData`/`onPtyExit`, nhưng
  `getTerminalAgentStatus`/`isTerminalRunningAgent` là logic phức tạp, nhiều
  nhánh (foreground-process polling, title classification, management-UI
  suppression) không có test bao phủ. Khuyến nghị kiểm thử thủ công kỹ:
  `orca terminal status` trên nhiều agent (Claude/Codex/Cursor/Gemini,
  Claude Agent Teams), trạng thái permission/idle/working, terminal PTY-backed
  vs renderer-backed, orchestration context hiển thị đúng task/dispatch cho
  worktree.ps sidebar, foreground-process wrapper retry (agent khởi động qua
  shell wrapper).
- 2 lần phát hiện lỗi thật qua `tsc` (không phải chỉ move-only) trong task
  này — nhắc lại bài học: với domain có nhiều method private bị nghi ngờ
  "chỉ dùng nội bộ", PHẢI grep toàn file (`this.methodName(`) cho TỪNG method
  trước khi quyết định giữ `private`, không chỉ dựa vào lần audit ban đầu.
- Còn lại: `onPtyExit` (TASK-BIGFILE-077, tiếp theo).

## Tổng kết luỹ kế

`orca-runtime.ts`: 26,730 → **5,816 dòng (78.2% giảm)** qua 44 task
(TASK-BIGFILE-036 đến 076, trừ 057 đã huỷ rồi tái thực thi ở 067; 041 và 063
là state-container Extract).
