# TASK-BIGFILE-065 — Move: Worktree/workspace parent-lineage resolution domain

**Loại:** Move — composition pattern, rủi ro thấp (ngoài vùng PTY-core) ·
**Effort:** M · **Phụ thuộc:** TASK-BIGFILE-040, 047, 049
**Status:** ✅ Done
**Solution:** `../solutions/SOLUTION-FE-BIGFILE-002-orca-runtime.md`

## Bối cảnh

Sau khi hoàn tất mọi domain PTY-core "an toàn" (TASK-054-064), rà lại toàn
bộ `orca-runtime.ts` bằng gap-analysis (khoảng cách dòng giữa các method
liên tiếp) để tìm cụm lớn tiếp theo — phát hiện `resolveLineageForWorktreeCreate`
(245 dòng gap) KHÔNG nằm trong vùng PTY-core nguy hiểm mà thuộc domain
worktree/workspace (giống TASK-047/049 đã tách trước đó), khảo sát
method-body dependency (đúng quy trình từ bài học TASK-057) cho thấy đây
là cụm sạch, rủi ro thấp.

## Kết quả thực thi (2026-08-11)

- Domain: `resolveWorkspaceParentSelector` (private), `validateLineageParent`
  (public), `resolveLineageForWorktreeCreate` (public),
  `resolveLineageCandidateForTaskId` (private), `hydrateInferredWorktreeLineage`
  (public), `listWorktreeLineage`, `listWorkspaceLineage` — dòng gốc
  6957–7361, 405 dòng.
- 7 host dependency: `getStore`, `getOrchestrationDbField` (đọc field
  `_orchestrationDb` trực tiếp — field này dùng ở rất nhiều nơi khác, KHÔNG
  chuyển), `listResolvedWorktrees` (đã forward từ TASK-040),
  `resolveWorktreeSelector`, `showTerminal`, `peekResolvedWorktreeCache`
  (qua `resolvedWorktreeCommands.peekCache()`, domain TASK-040).
- **Phát hiện quan trọng, tự sửa trước khi hoàn tất**: `getOrchestrationDbIfAvailable`
  ban đầu bị gộp nhầm vào cụm bị chuyển (nằm liền kề trong dòng gốc
  7299–7305), nhưng hoá ra được gọi từ **3 nơi khác** ngoài lineage
  (`buildAgentOrchestrationByPaneKey`, `getAgentStatusOrchestrationContextForHandle`,
  `getRecentCompletedDispatchForTerminal` — hạ tầng agent-status/dispatch
  chung, không liên quan lineage) — `tsc TS2339` bắt ngay. Sửa bằng cách
  giữ nguyên method này ở `orca-runtime.ts` (dùng chung, không chỉ riêng
  lineage), thêm vào host interface làm dependency thay vì method di
  chuyển.
- `ResolvedWorkspaceParent`, `WorktreeLineageCandidate`, `WorktreeIdRequiresFullPathError`
  (3 type/class nội bộ) — thêm `export`, import lại từ `'./orca-runtime'`.
  `WorktreeLineageResolution` đã export sẵn (dùng bởi
  `orca-runtime-worktree-creation.ts`) — không đổi.
  `ResolvedWorktreeCacheEntry` (trong `orca-runtime-resolved-worktree-cache.ts`,
  TASK-040) — thêm `export` (chưa export trước đó, chỉ dùng nội bộ file
  gốc).
- `extractOrchestrationTaskId` (hàm tự do nội bộ, dòng 1024) — chuyển hẳn
  (chỉ domain này dùng).
- `worktreeWorkspaceKey`, `WorkspaceLineage` (import move-only, không dùng
  nơi khác trong `orca-runtime.ts`) — xoá khỏi import. `WorktreeLineageWarning`,
  `WorkspaceKey`, `folderWorkspaceKey`, `parseWorkspaceKey` — giữ nguyên
  (dùng ở type/method khác ngoài cụm).
- `orca-runtime.ts`: 9,563 → **9,179 dòng** (giảm 384 dòng). File mới:
  444 dòng — đã đăng ký `config/max-lines-baseline.txt` +
  `eslint-disable max-lines` inline.
- Xác minh: `tsc --noEmit --composite false` giữ đúng baseline 251 lỗi (0
  lỗi mới, sau khi sửa 1 lỗi tạm thời `TS2339` do gộp nhầm
  `getOrchestrationDbIfAvailable`). `oxlint` sạch (exit 0) cả 2 config.
  `max-lines-ratchet`: 647 vi phạm pre-existing không đổi.

## Rủi ro còn lại / khuyến nghị

- Move cơ học, rủi ro thấp hơn hẳn các domain PTY-core (không đụng
  `onPtyData`/`createTerminal`/headless-emulator). Khuyến nghị kiểm thử
  thủ công: tạo worktree với `--parent-worktree`/`--no-parent`, lineage
  suy luận từ task orchestration context, phát hiện chu trình lineage
  (cycle detection qua `resolvedWorktreeCommands.peekCache()`).
- Phần còn lại của `orca-runtime.ts` (~9,179 dòng) vẫn chủ yếu là lõi PTY
  thật (`graph`, pty-title-tracker, OSC-status processing,
  `onPtyData`/`createTerminal`) và `browserScreencast` (412 dòng, 1
  method đơn lẻ) — cần Investigate riêng hoặc thiết kế lại sâu hơn trước
  khi tách tiếp.

## Tổng kết luỹ kế

`orca-runtime.ts`: 26,730 → **9,179 dòng (65.7% giảm)** qua 33 task
(TASK-BIGFILE-036 đến 065, trừ 057 đã huỷ; 041 và 063 là state-container
Extract).
