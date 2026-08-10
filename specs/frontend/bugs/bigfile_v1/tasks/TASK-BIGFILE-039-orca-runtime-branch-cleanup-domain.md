# TASK-BIGFILE-039 — Move (composition): Branch cleanup / managed-worktree removal domain

**Loại:** Move — composition pattern (KHÔNG phải barrel, class có `this`)
· **Effort:** M (chứa 1 method khổng lồ — xem cảnh báo) · **Phụ thuộc:**
TASK-BIGFILE-008, 009
**Status:** ⬜ Todo
**Solution:** `../solutions/SOLUTION-FE-BIGFILE-002-orca-runtime.md` (Giai
đoạn 3) · Sinh ra từ `TASK-BIGFILE-035`

## ⚠️ Cảnh báo — `removeManagedWorktree` dài ~490 dòng

Method này TỰ NÓ đã dài gần bằng 1 file nhỏ. Di chuyển file KHÔNG giải
quyết việc này — chỉ đổi chỗ. Task này CHỈ di chuyển (không chia nhỏ nội
dung method). Nếu muốn chia nhỏ `removeManagedWorktree` thành nhiều hàm
con, làm ở 1 task refactor RIÊNG sau này (không gộp vào đây — giữ task
này đúng nghĩa "Move cơ học").

## Input

- File nguồn: `frontend/src/main/runtime/orca-runtime.ts`
- Đọc **đúng dòng 16,705–17,325** (KHÔNG đọc phần khác của file).
- Method cần chuyển (5 method, xác nhận lại khi đọc):
  `closeHeadlessBrowserPagesForWorktree`(⚠️ tên gợi ý dùng chéo domain
  browser — xác nhận kỹ), `rememberPreservedBranchCleanupTarget`,
  `preserveBranchHeadFallback`, `forceDeletePreservedBranch`,
  `removeManagedWorktree`
- Field private cần: `preservedBranchCleanupByWorktreeId`,
  `removeManagedWorktreeInFlight`.

## Output

- File mới: `frontend/src/main/runtime/orca-runtime-branch-cleanup.ts` —
  class mới (ví dụ `BranchCleanupDomain`) nhận dependency qua constructor.
- `orca-runtime.ts`: thêm field `private branchCleanup = new
  BranchCleanupDomain({ ... })`, 5 method forward — GIỮ NGUYÊN chữ ký.

## Các bước

1. `gitnexus impact({target: "removeManagedWorktree", direction: "upstream"})`
   — method này rất có thể được gọi từ nhiều IPC handler, kiểm tra kỹ,
   dừng nếu risk HIGH/CRITICAL.
2. Đọc dòng 16,705–17,325, xác nhận method + field + kiểm tra riêng
   `closeHeadlessBrowserPagesForWorktree` có dùng field domain browser
   (`offscreenBrowserBackend`, `agentBrowserBridge`) hay không — nếu có,
   giữ lại ở `OrcaRuntimeService` hoặc inject browser dependency vào domain
   mới qua constructor.
3. Tạo `orca-runtime-branch-cleanup.ts`, copy nguyên văn (KHÔNG chia nhỏ
   `removeManagedWorktree`), đổi `this.xxx` → `this.deps.xxx`.
4. Sửa `orca-runtime.ts`: thêm field, forward 5 method.

## Xác minh xong

- [ ] `pnpm run typecheck && pnpm run lint`
- [ ] `gitnexus detect_changes({scope: "all"})` — risk low
- [ ] `node scripts/find-frontend-bigfiles.mjs` — giảm ~500-600 dòng
- [ ] Test worktree-removal/branch-cleanup liên quan pass — đây là thao
      tác phá huỷ (xoá worktree/branch), rủi ro cao nếu có lỗi logic khi
      copy — test thủ công thêm nếu không có test tự động đủ bao phủ

## Rollback

```
git checkout -- frontend/src/main/runtime/orca-runtime.ts
rm frontend/src/main/runtime/orca-runtime-branch-cleanup.ts
```
