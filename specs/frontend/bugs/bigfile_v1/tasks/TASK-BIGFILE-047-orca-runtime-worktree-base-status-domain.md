# TASK-BIGFILE-047 — Move: Worktree base-status/drift & managed PR/MR base domain

**Loại:** Move — composition pattern · **Effort:** M · **Phụ thuộc:** không
(ranh giới xác định bằng grep field/method toàn file)
**Status:** ✅ Done
**Solution:** `../solutions/SOLUTION-FE-BIGFILE-002-orca-runtime.md` (Giai
đoạn 3, mở rộng ngoài 5 domain gốc — theo lựa chọn "worktree còn lại" của
người dùng)

## Bối cảnh: "git-sourcecontrol" hoá ra đã xong từ trước

Trước khi bắt đầu task này, rà lại danh sách domain "an toàn" người dùng
chọn (git-sourcecontrol, project-groups, worktree còn lại) thì phát hiện
git-sourcecontrol **đã được tách từ trước** (`orca-runtime-git.ts`, 950
dòng, `RuntimeGitCommands` — commit/diff/stage/branch-compare/merge/rebase/
push/pull/fetch đều đã có, đã wire composition đầy đủ ở `orca-runtime.ts`).
Không còn gì để tách ở domain này — đã báo lại cho người dùng, người dùng
chọn tiếp tục với "worktree còn lại".

## Kết quả thực thi (2026-08-11)

- Rà toàn bộ ~20 method còn dính dáng tới "worktree" rải rác từ dòng 888 đến
  14363 trong `orca-runtime.ts` cũ — KHÔNG liền mạch, rơi vào 4 cụm riêng
  biệt:
  1. **Cụm base-status/drift/PR-MR-base** (dòng ~11455-12112, 658 dòng, 9
     method) — liền mạch hoàn toàn, không đụng PTY. → **Tách trong task này.**
  2. Cụm managed-worktree list/show/sleep/activate/create (dòng
     ~9437-10200+) — xen lẫn `scanWorkspacePorts`/`killWorkspacePort` (domain
     khác), `activateManagedWorktree` một mình ~550 dòng — chưa phân tích
     sâu, để lại cho task sau.
  3. Cụm dừng terminal theo worktree (`stopTerminalsForWorktree`,
     `stopExactTerminalsForWorktree`, `hasTerminalsForWorktree`, dòng
     ~13516-13668) — **PTY-adjacent, cố tình bỏ qua** theo lựa chọn của
     người dùng tránh PTY-lifecycle core.
  4. Cụm worktree lineage (`hydrateInferredWorktreeLineage`,
     `listWorktreeLineage`, `listWorkspaceLineage`, dòng ~14318-14370) — nhỏ,
     nhưng `validateLineageParent` (dùng chung với cụm 1) vẫn ở lại
     `orca-runtime.ts` vì cụm này chưa tách.
- Domain tách: `recordOptimisticReconcileToken`,
  `clearOptimisticReconcileToken`, `emitWorktreeBaseStatus`,
  `reconcileWorktreeBaseStatus`, `probeWorktreeDrift`,
  `updateManagedWorktreeMeta`, `persistManagedWorktreeSortOrder`,
  `resolveManagedPrBase`, `resolveManagedMrBase` + private helper
  `resolveGitLabIssueSourceRemote`. 13 host dependency (giữ tối thiểu:
  `getStore`, `requireStore`, `resolveWorktreeSelector`, `resolveRepoSelector`,
  `showManagedWorktree`, `notifyWorktreesChanged`, `notifyReposChanged`,
  `invalidateResolvedWorktreeCache`, `validateLineageParent`,
  `getOrStartRemoteFetch`, `fetchRemoteWithCache`, `resolveRemoteTrackingBase`,
  `getNotifier`).
- `resolveWorktreeSelector`/`showManagedWorktree` dùng thẳng type
  `ResolvedWorktree` (export từ `orca-runtime.ts`) thay vì thu hẹp xuống
  shape tối thiểu như các domain trước — vì `validateLineageParent` (ở lại
  `orca-runtime.ts`, dùng chung với cụm lineage chưa tách) đòi hỏi đúng type
  đầy đủ này; thu hẹp sẽ vỡ type khi wiring host.
- `optimisticReconcileTokens` (Map riêng, chỉ domain này dùng) chuyển hẳn
  thành private field của class mới, không qua host — cùng mẫu với
  `RuntimeGraphStore`/`RuntimeRemoteFetchCache`'s state fields.
- `omitUndefinedProperties` và `RuntimeLineageError` (dùng chung với cụm
  lineage chưa tách) — thêm `export` ở `orca-runtime.ts`, import lại vào file
  mới, thay vì di chuyển hẳn.
- `DRIFT_PROBE_SUBJECT_LIMIT` (hằng số, chỉ domain này dùng) — chuyển hẳn vào
  file mới.
- `orca-runtime.ts`: 17,331 → **16,708 dòng** (giảm ~623 dòng). File mới:
  758 dòng — vượt ngưỡng 300 dòng hiệu quả của oxlint, đã đăng ký
  `config/max-lines-baseline.txt` + `eslint-disable max-lines` inline theo
  đúng quy trình AGENTS.md.
- Xác minh: `tsc --noEmit --composite false` 251 lỗi pre-existing không đổi
  (0 lỗi mới), `oxlint` sạch (exit 0) cả 2 config (`default` + `react-doctor`).
  `pnpm check:max-lines-ratchet`: diff giữa trước/sau thay đổi giống hệt
  (không tạo thêm "New max-lines bypass" nào ngoài 647 lỗi môi trường có sẵn,
  không liên quan tới domain này).

## Việc tiếp theo

- Cụm managed-worktree list/show/sleep/activate/create (dòng
  ~9437-10200+) — cần tách riêng `scanWorkspacePorts`/`killWorkspacePort` ra
  trước, rồi phân tích `activateManagedWorktree`/`createManagedWorktree`
  (rất lớn, có thể đụng PTY startup — cần kiểm tra kỹ trước khi quyết định
  tách).
- Cụm dừng terminal theo worktree và cụm lineage: giữ nguyên trong
  `orca-runtime.ts`, không động tới trừ khi người dùng đổi hướng khỏi việc
  tránh PTY-lifecycle core.
