# TASK-BIGFILE-048 — Follow-up: 3 method sót lại của issue-tracking domain

**Loại:** Move — composition pattern (bổ sung vào class đã có, không tạo
file mới) · **Effort:** S · **Phụ thuộc:** TASK-BIGFILE-042
**Status:** ✅ Done
**Solution:** `../solutions/SOLUTION-FE-BIGFILE-002-orca-runtime.md`

## Bối cảnh

Trong lúc rà ranh giới cho cụm "managed-worktree list/create/activate"
(theo kế hoạch ở TASK-BIGFILE-047's "Việc tiếp theo"), phát hiện 3 method
`getRepoSlug`, `getRepoUpstream`, `listRepoWorkItems` nằm NGAY TRƯỚC
composition wiring của `issueTrackingCommands` (domain đã tách ở
TASK-BIGFILE-042) — đúng mẫu "task doc under-scoped từ shallow grep" đã gặp
lặp lại ở hầu hết các task trước.

## Kết quả thực thi (2026-08-11)

- 3 method chuyển vào `orca-runtime-issue-tracking.ts` (không tạo file
  mới). Cả 3 host dependency cần dùng (`resolveRepoSelector`,
  `getHostedReviewExecutionOptions`, `getLocalGitExecutionOptionArgs`) đã
  có sẵn trong `RuntimeIssueTrackingCommandHost` từ TASK-BIGFILE-042 — không
  cần thêm host dependency nào.
- Naming collision với hàm tự do cùng tên (`getRepoSlug`, `getRepoUpstream`,
  `listWorkItems`, tất cả từ `../github/client`) — alias khi import
  (`getGitHubRepoSlug`, `getGitHubRepoUpstream`, `listGitHubWorkItems`),
  cùng mẫu đã dùng nhiều lần trước đó.
- `getRepoUpstream` (hàm tự do) vẫn được dùng ở `backfillForkUpstreams`
  (private startup hook, ở lại `orca-runtime.ts`, không thuộc domain issue-
  tracking) — giữ nguyên import đó trong `orca-runtime.ts`, chỉ bỏ
  `getRepoSlug`/`listWorkItems` (chỉ dùng trong 3 method vừa chuyển).
- `orca-runtime.ts`: 16,708 → **16,676 dòng**. `orca-runtime-issue-
  tracking.ts`: 1,210 → 1,254 dòng (không cần đăng ký lại baseline, đã có
  entry từ TASK-042).
- Xác minh: `tsc --noEmit --composite false` 251 lỗi pre-existing không đổi
  (0 lỗi mới), `oxlint` sạch (exit 0) cả 2 config.

## Rà soát cụm "managed-worktree còn lại" (kết luận: không tiếp tục)

Trong lúc rà, xác nhận `activateManagedWorktree` (~558 dòng) và
`createManagedWorktree` (~1,310 dòng) — 2 method LỚN NHẤT còn lại trong
toàn bộ `orca-runtime.ts` — mật độ tham chiếu pty/terminal/handle/spawn rất
cao (70 và 136 lần tương ứng trong thân method) → xác nhận đây chính là
PTY-lifecycle-core mà người dùng đã chọn tránh, không tách.

Phần còn lại của cụm (`listManagedWorktrees`, `listDetectedManagedWorktrees`,
`showManagedWorktree`, `sleepManagedWorktree`,
`prefetchManagedWorktreeCreateBase`, ~125 dòng tổng) bị kẹp giữa
`scanWorkspacePorts`/`killWorkspacePort` (domain khác) và 2 method PTY khổng
lồ ở trên, đồng thời phụ thuộc 2 private helper dùng chung với
`getWorktreePs` (`isRuntimeWorktreeVisible`, `toRuntimeDetectedWorktree`,
không tách được) — tách sẽ cần ~8 host dependency cho ~125 dòng giảm được,
tỷ lệ effort/lợi ích thấp. Quyết định: **không tách**, dừng việc rà domain
"worktree còn lại" ở đây.
