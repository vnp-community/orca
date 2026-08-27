# TASK-BIGFILE-046 — Move: Project/project-group/folder-workspace domain

**Loại:** Move — composition pattern · **Effort:** M · **Phụ thuộc:** không
(ranh giới xác định bằng grep field/method toàn file)
**Status:** ✅ Done
**Solution:** `../solutions/SOLUTION-FE-BIGFILE-002-orca-runtime.md` (Giai
đoạn 3, mở rộng ngoài 5 domain gốc — theo lựa chọn "domain an toàn" của
người dùng: git-sourcecontrol, project-groups, worktree còn lại)

## Kết quả thực thi (2026-08-11)

- Domain: `listRepos`, `enrichMissingRepoGitRemoteIdentities`,
  `listProjects`, `updateProject`, project host setup CRUD (create/update/
  delete/list), `listProjectGroups`/`listFolderWorkspaces`, project-group
  CRUD (create/update/delete/move), folder-workspace CRUD (create/update/
  delete/path-status), `scanNestedRepos`, `browseServerDir`, `isGitAvailable`,
  `importNestedRepos`, sparse-preset list/save. 26 method, 5 host dependency
  (`getStore`, `resolveRepoSelector`, `notifyReposChanged`,
  `invalidateResolvedWorktreeCache`, `addRepo`, `cloneRepo`).
- `addRepo`/`cloneRepo` (repo lifecycle) nằm NGAY SAU khối này trong source
  nhưng là domain riêng, lớn hơn, chưa tách — giữ làm host dependency thay vì
  mở rộng phạm vi, giữ đây là một Move thuần.
- Naming collision (giống TASK-BIGFILE-042/037): 3 method trùng tên với hàm
  tự do mà chúng gọi (`enrichMissingRepoGitRemoteIdentities`,
  `getFolderWorkspacePathStatus`, `scanNestedRepos`) — alias khi import
  (`... as ...Impl`).
- 4 hàm tự do private chỉ dùng trong domain này
  (`normalizeSparsePresetName`, `normalizeSparsePresetDirectoriesForSave`,
  `sanitizeNestedRepoRuntimeImportError`, `resolveServerBrowsePath`) — chuyển
  hẳn vào file mới thay vì export/import lại, theo tiền lệ
  `sameStringSet`/`labelsForIds` ở TASK-BIGFILE-044.
- `orca-runtime.ts`: 17,852 → **17,331 dòng** (giảm ~521 dòng, gồm cả việc
  xoá 4 hàm tự do private ở trên). File mới: 642 dòng — vượt ngưỡng 300 dòng
  hiệu quả của oxlint, đã đăng ký `config/max-lines-baseline.txt` +
  `eslint-disable max-lines` inline theo đúng quy trình AGENTS.md.
- Xác minh: `tsc --noEmit --composite false` 251 lỗi pre-existing không đổi
  (0 lỗi mới trong 2 file), `oxlint` sạch (exit 0) cả 2 config
  (`default` + `react-doctor`).
- `pnpm check:max-lines-ratchet`: baseline entry mới không tạo "added"
  mới (xác nhận bằng diff giữa lần chạy trước/sau thay đổi — giống hệt
  647 lỗi "New max-lines bypass" tồn tại sẵn từ trước, không liên quan tới
  domain này — các file mirror ở `agent/`, `backend/`, `desktop/` không nằm
  trong baseline gốc, vấn đề môi trường/đồng bộ có sẵn, không thuộc phạm vi
  sửa của task này).

## Việc tiếp theo

- `addRepo`/`cloneRepo` (repo lifecycle, ngay sau khối này) là ứng viên tách
  tiếp theo — lớn hơn project-groups, chưa phân tích ranh giới.
- Tiếp tục theo lựa chọn người dùng: git-sourcecontrol, rồi worktree còn lại
  (nếu tìm được ranh giới an toàn).
