# TASK-BIGFILE-050 — Move: Repo lifecycle domain (add/create/clone/show/update/remove)

**Loại:** Move — composition pattern · **Effort:** M · **Phụ thuộc:** không
(ranh giới xác định bằng grep field/method toàn file)
**Status:** ✅ Done
**Solution:** `../solutions/SOLUTION-FE-BIGFILE-002-orca-runtime.md`

## Bối cảnh

`addRepo`/`cloneRepo` là 2 host dependency được TASK-BIGFILE-046
(project-groups) cố tình để lại trong `orca-runtime.ts` vì khi đó chúng là
một phần của domain "repo lifecycle" lớn hơn, chưa được scoping. Sau khi
domain PTY-lifecycle core (TASK-BIGFILE-049) hoàn tất, domain repo-lifecycle
còn lại này là ứng viên "an toàn" tiếp theo — không đụng PTY (ngoại trừ 1
method cần loại trừ).

## Kết quả thực thi (2026-08-11)

- Domain: `addRepo`, `createRepo`, `cloneRepo` + private helper
  `cloneRepoAfterPathLock`, `showRepo`, `setRepoBaseRef`, `updateRepo`,
  `removeProject`, `reorderRepos`, `searchRepoRefs` + private helper
  `searchRemoteRepoRefs`, `getRepoBaseRefDefault` + private helper
  `getRemoteRepoBaseRefDefault`. 11 method, chỉ **4 host dependency**
  (`getStore`, `notifyReposChanged`, `invalidateResolvedWorktreeCache`,
  `resolveRepoSelector`) — domain sạch nhất về mặt phụ thuộc kể từ
  project-groups.
- `inspectTerminalProcess` nằm NGAY GIỮA khối này (giữa `removeProject` và
  `reorderRepos`) nhưng dùng `this.ptyController` trực tiếp — PTY-adjacent,
  không thuộc domain repo lifecycle — cố tình loại trừ, giữ nguyên vị trí
  trong `orca-runtime.ts` (cùng mẫu với `scanWorkspacePorts`/
  `killWorkspacePort` bị loại trừ khỏi domain managed-worktree ở
  TASK-BIGFILE-048's notes).
- `getHostedReviewExecutionOptions`, `getLocalGitExecutionOptionArgs`,
  `getAgentLaunchPlatformForRepo`, `getAgentLaunchPlatformForWorkspace`,
  `backfillForkUpstreams` nằm ngay sau khối này nhưng là host dependency
  DÙNG CHUNG bởi nhiều domain đã tách trước (issue-tracking,
  worktree-base-status, worktree-creation) — giữ nguyên, không di chuyển.
- `runtimeRepoMatchesExecutionHost` (hàm tự do module-scope, chỉ dùng trong
  domain này) và `DEFAULT_REPO_SEARCH_REFS_LIMIT` (hằng số) — chuyển hẳn vào
  file mới.
- `cloneInFlightByPath` (Map riêng, chỉ `cloneRepo`/`cloneRepoAfterPathLock`
  dùng) — chuyển hẳn thành private field của class mới.
- Sau khi tách xong, phát hiện TOÀN BỘ import block `'../git/repo'`
  (`getBaseRefDefault`, `isGitRepo`, `getRepoName`, `searchBaseRefDetails`,
  v.v. — 12 tên) không còn dùng ở đâu trong `orca-runtime.ts` nữa — xoá
  sạch cả block. Tương tự `gitExecFileAsync` (từ `../git/runner`), không
  còn call site nào trong `orca-runtime.ts` sau khi domain PTY-lifecycle
  (TASK-049) và domain này đã lấy hết — xoá luôn.
- `orca-runtime.ts`: 14,567 → **13,949 dòng** (giảm ~618 dòng). File mới:
  681 dòng — vượt ngưỡng 300 dòng hiệu quả của oxlint, đã đăng ký
  `config/max-lines-baseline.txt` + `eslint-disable max-lines` inline theo
  đúng quy trình AGENTS.md.
- Xác minh: `tsc --noEmit --composite false` 251 lỗi pre-existing không đổi
  (0 lỗi mới), `oxlint` sạch (exit 0) cả 2 config (`default` +
  `react-doctor`). `pnpm check:max-lines-ratchet`: diff giữa trước/sau thay
  đổi giống hệt (không tạo thêm "New max-lines bypass" nào ngoài lỗi môi
  trường có sẵn).

## Tổng kết luỹ kế

`orca-runtime.ts`: 26,730 → **13,949 dòng (47.8% giảm)** qua 19 task
(TASK-BIGFILE-036 đến 050).
