# TASK-BIGFILE-077 — Move: onPtyExit (pty-exit domain)

**Loại:** Move — composition pattern, rủi ro cao (exit hot path) ·
**Effort:** M · **Phụ thuộc:** TASK-BIGFILE-037, 051, 063, 064, 067, 068,
069, 072
**Status:** ✅ Done
**Solution:** `../solutions/SOLUTION-FE-BIGFILE-002-orca-runtime.md`

## Bối cảnh

Cụm thứ ba theo yêu cầu người dùng ("getTerminalAgentStatus,
getPaneKeyForTerminalHandle/getTerminalAgentStatusSnapshot, và onPtyExit").
Cùng kỹ thuật TASK-BIGFILE-074 áp dụng cho `onPtyData`: tách chính dispatcher
`onPtyExit` (đối tác exit của `onPtyData`), giữ ~15 domain đã tách làm host
dependency. Tần suất gọi thấp hơn `onPtyData` nhiều (một lần mỗi vòng đời
PTY, không phải mỗi output chunk) nhưng vẫn là exit hot path trực tiếp.

## Kết quả thực thi (2026-08-12)

- Domain: `onPtyExit` (dòng gốc 2578–2627, 50 dòng) + `failActiveDispatchOnExit`
  (dòng gốc 2633–2669, 37 dòng, duy nhất được gọi từ `onPtyExit`) — 1 đoạn
  liền mạch, không có "excluded chunk ở giữa" như các task trước.
- 17 host dependency — toàn bộ đã tồn tại sẵn (method/forwarding field từ
  TASK-037, 051, 063, 064, 067, 068, 069, 072): `getGraph`, `getPtyTranscripts`,
  `getRawOrchestrationDb` (field thô `_orchestrationDb`, KHÔNG dùng
  `getOrchestrationDbIfAvailable()` — bản gốc cố tình đọc field trực tiếp để
  không force-create DB orchestration khi xử lý PTY exit), `getAgentDetector`,
  `getLeafKey`, `getLeavesForPty`,
  `clearRemoteTerminalViewSubscriberCountForPty`, `clearWaitBlockedCheckState`,
  `disposePtyTitleTracker`, `clearAgentRowSnapshotsForPty`,
  `removeTeamForLeaderHandle`, `clearStateForExitedPty`,
  `disposeHeadlessTerminal`, `resolvePtyExitWaiters`, `resolveExitWaiters`,
  `pruneDisconnectedPtyTranscript`, `touchMobileSessionSnapshotsForPty`,
  `pruneDisconnectedPtyRecords`.
- `onPtyExit` giữ forwarding field public (không có internal caller nào
  khác trong `orca-runtime.ts`, gọi từ IPC layer bên ngoài).
- Xác minh fidelity bằng diff nguyên văn (chuẩn hoá `this.host.X` → `this.X`,
  bao gồm cả việc khôi phục biến cục bộ `db` về truy cập field trực tiếp
  `this._orchestrationDb` trong `failActiveDispatchOnExit`) so với
  `git show HEAD:...` — khớp byte-for-byte.
- Không có import move-only nào cần dọn (mọi free-function/type dùng trong
  cụm này đều re-import sạch từ đầu, không sót lỗi biên dịch nào).
- `orca-runtime.ts`: 5,816 → **5,750 dòng**. File mới `orca-runtime-pty-exit.ts`:
  140 dòng (103 non-blank/non-comment) — dưới ngưỡng 300, không cần đăng ký
  `max-lines-baseline.txt`.
- Xác minh: `tsc --noEmit --composite false` giữ đúng baseline 251 lỗi —
  **sạch ngay từ lần chạy đầu tiên**, không có lỗi move-only hay lỗi thật
  nào cần sửa (task đầu tiên trong cả loạt 073–077 không cần vòng sửa lỗi
  thứ hai). `oxlint` sạch (exit 0) cả 2 config. `max-lines-ratchet`: 647 vi
  phạm pre-existing không đổi.

## Rủi ro còn lại / khuyến nghị

- Rủi ro cao — exit hot path, không có test coverage trực tiếp. Khuyến nghị
  kiểm thử thủ công: kill terminal (thường, PTY-backed, SSH/remote), agent
  crash bất ngờ giữa dispatch đang active (xác nhận `failActiveDispatchOnExit`
  set lại task về `pending` + gửi escalation message cho coordinator), Claude
  Agent Teams leader PTY exit (giải phóng team map), renderer reload trong
  lúc PTY đang exit, mobile session snapshot cập nhật ngay lập tức
  (`touchMobileSessionSnapshotsForPty` với `immediate: true`).
- Đã hoàn tất cả 3 hướng người dùng yêu cầu lần này (076: agent-status +
  pane-key domain, 077: onPtyExit). Cùng với 073/074/075 trước đó, đã xử lý
  toàn bộ 3 cụm đề xuất ban đầu (createTerminal/splitTerminal/launchAgentTerminal,
  onPtyData, utility methods) cộng thêm 2 hướng mới này.

## Tổng kết luỹ kế

`orca-runtime.ts`: 26,730 → **5,750 dòng (78.5% giảm)** qua 45 task
(TASK-BIGFILE-036 đến 077, trừ 057 đã huỷ rồi tái thực thi ở 067; 041 và 063
là state-container Extract).
