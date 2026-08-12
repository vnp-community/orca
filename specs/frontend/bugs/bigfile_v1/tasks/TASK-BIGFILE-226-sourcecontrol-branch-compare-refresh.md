# TASK-BIGFILE-226 — Move: `useSourceControlBranchCompareRefresh` hook

**Loại:** Move — custom hook extraction · **Effort:** L · **Phụ thuộc:**
TASK-BIGFILE-020..025 đã xong, khuyến nghị chạy SAU TASK-BIGFILE-225 (xác
nhận pattern hook + `reset()` hoạt động đúng trước khi làm cụm lớn hơn)
**Status:** ⬜ Todo
**Solution:** `../solutions/SOLUTION-FE-BIGFILE-004-sourcecontrol.md` (Giai đoạn 2)

## Bối cảnh

Sinh ra từ TASK-BIGFILE-026 (Investigate `SourceControlInner`). Cụm "branch
compare refresh + git history refresh scheduling" — xác nhận qua grep KHÔNG
đọc/ghi `isExecutingBulk`, `pendingDiscard`, `commitDrafts`, hay bất kỳ state
nào của luồng Create-PR-intent. Có 3 điểm NGOÀI cụm gọi vào hàm
`refreshBranchCompareRef.current()` sau khi commit/remote action thành công
— đây là phụ thuộc 1 CHIỀU (ngoài gọi vào, cụm không gọi ra ngoài), an toàn
cho ranh giới hook.

**QUAN TRỌNG — đọc lại dòng thật trước khi tách**: số dòng dưới đây đo tại
thời điểm viết task này (sau TASK 020–025, `SourceControl.tsx` = 7,086
dòng). Nếu file đã đổi (kể cả do TASK-BIGFILE-225 chạy trước), PHẢI grep lại
theo TÊN BIẾN/HÀM, không tin số dòng literal.

## Input

- File nguồn: `frontend/src/renderer/src/components/right-sidebar/SourceControl.tsx`
- Khối chính cần đọc: dòng **4,552–4,857** (~305 dòng) — 6 `useRef`, 2
  `useCallback` chính (`runBranchCompare`, `refreshBranchCompare`), 1
  `useCallback` phụ (`refreshGitHistory`) + 1 `useRef` bọc nó
  (`refreshGitHistoryRef`), và 6 `useEffect`.
- State/ref/hàm cần chuyển (giữ nguyên tên): `branchCompareInFlightRef`,
  `branchCompareRerunRef`, `branchCompareRunPromiseRef`,
  `refreshBranchCompareRef`, `branchCompareStatusHeadRef`,
  `branchCompareRemoteStatusRef`, `runBranchCompare`, `refreshBranchCompare`,
  `refreshGitHistory`, `refreshGitHistoryRef`, `gitHistoryByWorktree` (+
  setter), `gitHistoryRequestSeqRef`, `gitHistoryRequestByWorktreeRef`.
- 3 điểm gọi NGOÀI cụm cần sửa để gọi qua giá trị hook trả về thay vì biến
  cục bộ (xác nhận lại dòng thật bằng grep `refreshBranchCompareRef` trước
  khi sửa — ước tính tại thời điểm viết: ~1,969, ~2,415, ~2,503):
  ```
  void refreshBranchCompareRef.current()
  refreshBranchCompare: refreshBranchCompareRef.current   // x2
  ```
- Input hook cần nhận từ component cha: `activeWorktreeId`, `worktreePath`,
  `isFolder`, `isBranchVisible`, `compareBaseRef`, `activeRepoSettings`,
  `branchName`, `remoteStatus`, `activeGitStatusHead`, `isGitHistoryExpanded`,
  `isGitHistoryVisible`, và các store action đã dùng trong khối (
  `beginGitBranchCompareRequest`, `setGitBranchCompareResult`,
  `clearGitBranchCompare`, `fetchUpstreamStatus` — xác nhận danh sách đầy đủ
  khi đọc lại đoạn code, KHÔNG suy đoán từ danh sách này).
- Trước khi sửa: `gitnexus impact` cho `runBranchCompare` và
  `refreshBranchCompare` — dừng nếu risk HIGH/CRITICAL. Nếu impact cho thấy
  điểm gọi NGOÀI 3 vị trí đã liệt kê ở trên, dừng lại và xác nhận lại (khác
  kỳ vọng).

## Output

- File mới: `frontend/src/renderer/src/components/right-sidebar/use-source-control-branch-compare-refresh.ts`
  — export 1 hook `useSourceControlBranchCompareRefresh(input): { refreshBranchCompare, refreshGitHistory, gitHistoryState string reader helper hoặc gitHistoryByWorktree, ... }`.
  Vì `refreshBranchCompareRef`/`refreshGitHistoryRef` tồn tại để né stale-
  closure giữa các `useCallback` phụ thuộc lẫn nhau — GIỮ NGUYÊN pattern ref
  này bên trong hook (không refactor sang cách khác trong task Move này).
- `SourceControl.tsx`: xoá khối 4,552–4,857, gọi
  `const branchCompare = useSourceControlBranchCompareRefresh({...})`, sửa
  3 điểm gọi ngoài cụm dùng `branchCompare.refreshBranchCompare`/
  `branchCompare.refreshGitHistory`, sửa JSX đọc `gitHistoryState`/
  `isGitHistoryExpanded` cho khớp field hook trả về.

## Các bước

1. `gitnexus impact` theo Input ở trên.
2. Đọc lại đúng dòng 4,552–4,857 (KHÔNG tin số dòng nếu file đã đổi do
   TASK-225 hoặc task khác chạy trước) — copy nguyên văn, không refactor nội
   dung từng hàm.
3. Grep `refreshBranchCompareRef\.current\(\)` và
   `refreshBranchCompare:\s*refreshBranchCompareRef` trong TOÀN BỘ
   `SourceControl.tsx` để xác nhận đúng 3 điểm gọi ngoài cụm (không nhiều
   hơn) trước khi sửa.
4. Tạo file hook mới, paste, đổi biến cục bộ thành input/output hook.
5. Sửa `SourceControl.tsx`: gọi hook, sửa 3 điểm gọi ngoài, sửa JSX liên
   quan `gitHistoryState`/`isGitHistoryExpanded` nếu cần.
6. `pnpm exec tsc --noEmit` + `pnpm exec oxlint` trên 2 file đã đổi.
7. Chạy toàn bộ test trong `frontend/src/renderer/src/components/right-sidebar/`
   — đặc biệt các test liên quan compare summary, git history panel, và các
   test gọi commit/remote action (vì 3 điểm gọi ngoài cụm nằm trong các luồng
   đó).

## Xác minh xong

- [ ] `pnpm run typecheck && pnpm run lint`
- [ ] `gitnexus detect_changes({scope: "all"})` — risk low
- [ ] `node scripts/find-frontend-bigfiles.mjs` — `SourceControl.tsx` giảm
      ~305 dòng
- [ ] Test liên quan pass — đặc biệt: polling branch compare khi mở/đóng
      sidebar, refresh sau commit/push/pull thành công, và git history panel
      mở lần đầu

## Rollback

```
git checkout -- frontend/src/renderer/src/components/right-sidebar/SourceControl.tsx
rm frontend/src/renderer/src/components/right-sidebar/use-source-control-branch-compare-refresh.ts
```
