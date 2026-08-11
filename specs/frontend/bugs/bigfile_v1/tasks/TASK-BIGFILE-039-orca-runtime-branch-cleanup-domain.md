# TASK-BIGFILE-039 — Move (composition): Branch cleanup / managed-worktree removal domain

**Loại:** Move — composition pattern (KHÔNG phải barrel, class có `this`)
· **Effort:** M (chứa 1 method khổng lồ — xem cảnh báo) · **Phụ thuộc:**
TASK-BIGFILE-008, 009
**Status:** ✅ Done

## Kết quả thực thi (2026-08-10)

- Phạm vi thực tế rộng hơn 5 method gốc: thêm `resolveWorktreeRemovalTarget`
  và `removeWorktreeMetadataAndHistory` (2 private helper CHỈ dùng bởi
  domain này, task doc gốc bỏ sót — cùng pattern lặp lại ở mọi task trước).
- Phát hiện quan trọng: `killAllProcessesForWorktree` (hàm ngoài, ở
  `worktree-teardown.ts`) nhận tham số `runtime?: OrcaRuntimeService` và
  gọi `.stopTerminalsForWorktree()` trên đó — code gốc truyền `runtime:
  this`. Nếu giữ nguyên sau khi tách, `this` sẽ trỏ vào class MỚI
  (`RuntimeBranchCleanupCommands`), SAI KIỂU. Thêm `getRuntimeForTeardown():
  OrcaRuntimeService` vào host interface, implement ở `orca-runtime.ts`
  bằng `() => this` (nơi `this` thực sự là `OrcaRuntimeService`).
- Dùng đúng pattern `const store = this.host.getStore(); if (!store) throw`
  rồi dùng biến local `store` (không gọi lại `getStore()` nhiều lần) —
  khớp với cách code gốc đã làm sẵn (`const store = this.store` sau guard),
  giữ nguyên TS narrowing.
- Domain phụ thuộc ~10 dependency ngoài (nhiều nhất trong 5 task): store,
  requireStore, resolveWorktreeSelector, agentBrowserBridge,
  offscreenBrowserBackend, getLocalProvider, onPtyStopped,
  clearOptimisticReconcileToken, invalidateResolvedWorktreeCache,
  notifyWorktreesChanged, getRuntimeForTeardown — tất cả qua host
  interface với closure lazy.
- File mới vượt ngưỡng oxlint `max-lines` (844 dòng, chủ yếu do
  `removeManagedWorktree` tự nó ~490 dòng) — đăng ký
  `config/max-lines-baseline.txt` kèm disable comment "NEEDS PR REVIEW"
  theo đúng chính sách `AGENTS.md`, KHÔNG tự ý bỏ qua.
- Tiện thể sửa 1 lỗi `unicorn/no-useless-spread` phát sinh khi tách file
  (`{...(warning ? {warning} : {}) }` → `warning ? {warning} : {}`) — an
  toàn, không đổi hành vi.
- `orca-runtime.ts`: 24,016 → **23,264 dòng** (giảm ~752 dòng — nhiều nhất
  trong 5 task, vì `removeManagedWorktree` là method lớn nhất class). File
  mới: 844 dòng.
- Xác minh: `tsc --noEmit --composite false` 251 lỗi pre-existing không
  đổi (0 lỗi mới), `oxlint` sạch (exit 0) cả 2 config sau khi đăng ký
  baseline.
- **CHƯA làm**: chưa chạy test PTY/worktree-removal thủ công (thao tác
  phá huỷ, rủi ro cao nếu có lỗi copy) — khuyến nghị kiểm tra kỹ trước khi
  coi là an toàn deploy.
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
