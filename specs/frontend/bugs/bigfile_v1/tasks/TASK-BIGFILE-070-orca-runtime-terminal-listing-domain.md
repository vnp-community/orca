# TASK-BIGFILE-070 — Move: terminal.list + visual-layout-tree domain

**Loại:** Move — composition pattern, rủi ro trung bình (ngoài vùng
onPtyData) · **Effort:** L · **Phụ thuộc:** TASK-BIGFILE-040, 051, 065
**Status:** ✅ Done
**Solution:** `../solutions/SOLUTION-FE-BIGFILE-002-orca-runtime.md`

## Bối cảnh

Sau khi hoàn tất 3 domain PTY-core rủi ro cao nhất (TASK-067/068/069),
rà lại toàn bộ file bằng gap-analysis đã sửa (kiểm tra nội dung thật,
không chỉ khoảng cách dòng thô — bài học từ vụ đo nhầm `browserScreencast`
ở TASK-066) — phát hiện `listTerminals` + 6 helper `buildTerminalVisual*`
(dựng cây visual layout cho response `terminal.list`) là 1 cụm liền mạch,
tự chứa gần như hoàn toàn, nằm NGOÀI vùng `onPtyData`-adjacent nguy hiểm.

## Kết quả thực thi (2026-08-11)

- Domain: `listTerminals` (public), `buildTerminalVisualLayouts`,
  `buildTerminalVisualGroups`, `buildTerminalVisualTab`,
  `collectVisibleTerminalLeafIds`, `buildTerminalVisualPane`,
  `buildTerminalVisualGroupLayout` (5 helper private, chỉ tự tham chiếu)
  — dòng gốc 2954–3290, 337 dòng.
- 13 host dependency, tất cả đều là method core đã tồn tại sẵn (không cần
  tạo mới): `getGraph`, `getLeafKey`, `peekResolvedWorktreeCache` (qua
  `resolvedWorktreeCommands`, TASK-040), `getMobileSessionTabsByWorktree`
  (field trực tiếp, mẫu closure đã dùng ở 3 nơi khác),
  `resolveWorktreeSelector`, `refreshPtyWorktreeRecordsFromController`,
  `listKnownResolvedWorktreesForExplicitTarget`,
  `getValidatedExplicitWorktreeIdSelector`, `getResolvedWorktreeMap` (đã
  public từ TASK-040), `buildTerminalSummary`, `buildResolvedWorktreeFromId`,
  `buildPtyTerminalSummary`, `assertStableReadyGraph`.
- Chỉ 1 method cần public + forwarding: `listTerminals` (đã public sẵn,
  chỉ cần forwarding field — gọi từ `resolveActiveTerminal`, không thuộc
  cụm).
- 7 type/free-function move-only sau khi kiểm tra kỹ (không dùng nơi khác
  trong `orca-runtime.ts`): `RuntimeTerminalListResult`,
  `RuntimeTerminalVisualGroupNode`, `RuntimeTerminalVisualLayout`,
  `RuntimeTerminalVisualLayoutNode`, `RuntimeTerminalVisualPaneNode`,
  `RuntimeTerminalVisualTab`, `RuntimeMobileSessionTerminalTab`,
  `includeTargetResolvedWorktree`, `TabGroupLayoutNode`,
  `TerminalPaneLayoutNode`. Hằng số `DEFAULT_TERMINAL_LIST_LIMIT` (module
  scope, chỉ domain này dùng) — chuyển hẳn.
- 2 lỗi import quen thuộc gặp lại (đúng mẫu đã thấy nhiều lần):
  `TerminalPaneLayoutNode` từ `'../../shared/types'` (đoán nhầm ban đầu
  là `runtime-types`), sửa ngay theo `tsc TS2459`.
- `orca-runtime.ts`: 8,468 → **8,143 dòng**. File mới: 401 dòng (372 dòng
  non-blank/non-comment) — đã đăng ký `config/max-lines-baseline.txt` +
  `eslint-disable max-lines` inline.
- Xác minh: `tsc --noEmit --composite false` giữ đúng baseline 251 lỗi (0
  lỗi mới). `oxlint` sạch (exit 0) cả 2 config. `max-lines-ratchet`: 647
  vi phạm pre-existing không đổi.

## Rủi ro còn lại / khuyến nghị

- Rủi ro trung bình — KHÔNG nằm trong hot path `onPtyData` (chỉ gọi khi
  client yêu cầu `terminal.list`), nhưng vẫn zero test coverage. Khuyến
  nghị kiểm thử thủ công: `terminal.list` trả đúng cả flat list lẫn
  `visualLayouts` (group/tab/pane-split lồng nhau), filter theo
  worktree selector, `requireFreshPtyLiveness` option.
- Phần còn lại của `orca-runtime.ts` (~8,143 dòng): `onPtyData` (dispatcher
  chính), `createTerminal`/`splitPtyBackedTerminal`/`launchAgentTerminal`
  (tạo terminal, đan xen sâu với `graph`/`headlessTerminals`),
  `getWorktreePs` (dispatcher tương tự `listTerminals` nhưng gắn chặt với
  `attachAgentRowsToSummaries`/agent-status) — có thể còn cơ hội tách
  `getWorktreePs` theo mẫu tương tự `listTerminals` nếu rà kỹ tiếp.

## Tổng kết luỹ kế

`orca-runtime.ts`: 26,730 → **8,143 dòng (69.5% giảm)** qua 38 task
(TASK-BIGFILE-036 đến 070, trừ 057 đã huỷ rồi tái thực thi ở 067; 041 và
063 là state-container Extract).
