# TASK-BIGFILE-071 — Move: worktree.ps domain

**Loại:** Move — composition pattern, rủi ro trung bình (ngoài vùng
onPtyData) · **Effort:** L · **Phụ thuộc:** TASK-BIGFILE-040, 051, 065, 070
**Status:** ✅ Done
**Solution:** `../solutions/SOLUTION-FE-BIGFILE-002-orca-runtime.md`

## Bối cảnh

Ngay sau TASK-BIGFILE-070 (`listTerminals`), cụm liền kề cùng hình dạng:
`getWorktreePs` + helper riêng `attachAgentRowsToSummaries` — dựng danh
sách worktree kèm agent row inline cho mobile sidebar. Cùng mẫu "list
command + private summary helper" như 070, tìm được qua đúng sweep
gap-analysis đó.

## Kết quả thực thi (2026-08-11)

- Domain: `getWorktreePs` (public), `attachAgentRowsToSummaries` (private,
  chỉ tự tham chiếu) — dòng gốc 3762–4109, 348 dòng.
- 10 host dependency, tất cả method core đã tồn tại sẵn: `getStore`,
  `getGraph`, `listResolvedWorktrees` (đã public từ TASK-040),
  `isRuntimeWorktreeVisible`, `refreshPtyWorktreeRecordsFromController`,
  `getAgentLaunchPlatformForRepo`, `getSummaryForRuntimeWorktreeId`,
  `getLatestAgentStatusByPaneKey` (field trực tiếp, mẫu closure đã dùng ở
  TASK-051/069), `getAgentStatusSnapshot` (bọc `getAgentStatusSnapshotFn?.()
  ?? []` — field callback nullable, gói thành method trả mảng luôn để đơn
  giản hoá interface phía domain mới), `buildAgentOrchestrationByPaneKey`.
- Chỉ 1 method cần forwarding: `getWorktreePs` (đã public sẵn, không có
  caller nội bộ nào khác trong `orca-runtime.ts`).
- 5 free-function/const move-only sau khi kiểm tra kỹ (không dùng nơi
  khác): `compareWorktreePs`, `getLeafWorktreeStatus`,
  `getSavedTabWorktreeStatus`, `mergeWorktreeStatus` (từ
  `orca-runtime-tail-buffer.ts`), hằng số module-scope
  `DEFAULT_WORKTREE_PS_LIMIT`. `folderWorkspaceToWorktree`,
  `DEFAULT_WORKSPACE_STATUS_ID`, `maxTimestamp` — giữ nguyên (dùng ở nơi
  khác ngoài cụm, xác nhận qua `tsc` sạch ngay lần chạy đầu không cần sửa
  thêm).
- `RuntimeWorktreeAgentRow` — move-only (không dùng nơi khác trong
  `orca-runtime.ts` sau khi domain chuyển đi).
- `orca-runtime.ts`: 8,143 → **7,808 dòng**. File mới: 408 dòng (357 dòng
  non-blank/non-comment) — đã đăng ký `config/max-lines-baseline.txt` +
  `eslint-disable max-lines` inline.
- Xác minh: `tsc --noEmit --composite false` giữ đúng baseline 251 lỗi (0
  lỗi mới, chỉ cần dọn 6 import/const move-only sau lần chạy đầu — không
  có lỗi logic/tham chiếu nào). `oxlint` sạch (exit 0) cả 2 config.
  `max-lines-ratchet`: 647 vi phạm pre-existing không đổi.

## Rủi ro còn lại / khuyến nghị

- Rủi ro trung bình — KHÔNG nằm trong hot path `onPtyData` (chỉ gọi khi
  client yêu cầu `worktree.ps`). Khuyến nghị kiểm thử thủ công: mobile
  sidebar hiển thị đúng agent row inline (nguồn hook OSC 9999 lẫn OSC
  fallback), sort order, linked PR/issue badge, folder-workspace entries,
  active worktree highlight.
- Phần còn lại của `orca-runtime.ts` (~7,808 dòng): `onPtyData` (dispatcher
  chính, không tách được thêm), `createTerminal`/`splitPtyBackedTerminal`/
  `launchAgentTerminal` (tạo terminal, đan xen sâu với `graph`/
  `headlessTerminals`), `waitForTerminal` và các method chờ/waiter khác,
  cùng nhiều method utility nhỏ lẻ (getTerminalPaneKey, resolveTerminalPane,
  v.v.) không đủ lớn để tách riêng.

## Tổng kết luỹ kế

`orca-runtime.ts`: 26,730 → **7,808 dòng (70.8% giảm)** qua 39 task
(TASK-BIGFILE-036 đến 071, trừ 057 đã huỷ rồi tái thực thi ở 067; 041 và
063 là state-container Extract).
