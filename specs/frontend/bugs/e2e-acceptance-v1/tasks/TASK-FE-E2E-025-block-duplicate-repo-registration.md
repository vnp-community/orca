# TASK-FE-E2E-025: Chặn thêm trùng repo (cùng dev server + đường dẫn) và chỉ đường tới project đang chứa nó

**From Solution:** SOL-FE-E2E-005 (mục 1.3, 3.4) · **Resolves:** sidebar có hai mục cùng đường dẫn
**Priority:** P0 · **Area:** frontend (Add Project, Create New Project, store)
**Files:**
- frontend/src/renderer/src/lib/registered-repo-lookup.ts (mới) + `.test.ts`
- frontend/src/renderer/src/lib/own-project-registration.ts (kiểm trùng + ánh xạ mã lỗi)
- frontend/src/renderer/src/components/sidebar/AddRepoDuplicateNotice.tsx (mới) + test; useAddRepoServerPathFlow.ts; AddRepoServerStartStep.tsx
- frontend/src/renderer/src/components/project/CreateProjectDialog.tsx (bỏ cảnh báo "will NOT link", chặn); ProjectSwitcher.tsx (nối `onOpenExistingProject`)
- frontend/src/renderer/src/store/slices/repos.ts (`addRepoPath`: toast có tên project + "Open project")
**Depends on:** TASK-FE-E2E-023; backend mã `PROJECT_REPO_ALREADY_REGISTERED` (AE, tuỳ chọn: frontend tự kiểm)
**Status:** `[x]` XONG (2026-10-11): mã, vitest, Playwright mock và kiểm live. Kiểm live (2026-10-11): e2e `--real addproject` XANH 2/2 (Claude thật); `--real all` 7/7 XANH (24.1 phút); `--repro add-project-worktree` xanh; dữ liệu thật của người dùng không đổi.

---

## Context

`CreateProjectDialog.tsx:116-128,321-331` chỉ cảnh báo và vẫn cho tạo; `repos.ts:3196` so trùng theo (host, repo id) nên không bao giờ bắt trùng đường dẫn.

## Việc cần làm

- [x] `registered-repo-lookup.ts`: `normalizeRepoPathKey` (bỏ `/` cuối, `\`→`/`, gộp `//`, chữ thường cho đường dẫn kiểu Windows; dùng `normalizeRuntimePathForComparison`), `findRegisteredRepo(repos, {devServerId, path})`, `RepoAlreadyRegisteredError`, `parseRepoAlreadyRegisteredMessage` (`already in project <name> (<id>)`).
- [x] `registerRepoInOwnProject`: kiểm `project.list` + `repo.list` từng project trước khi ghi; ánh xạ lỗi backend `PROJECT_REPO_ALREADY_REGISTERED`.
- [x] Add Project: trước khi gọi, tra store + tên project; trùng thì khối inline `add-project-duplicate` (destructive, nêu tên project) + nút "Open that project" (`add-project-open-existing`) = `fetchWorktrees(repoId)` + `onGitRepoReady(repoId)` như thêm thành công; KHÔNG RPC ghi.
- [x] `CreateProjectDialog`: bỏ amber; trùng (store hoặc lỗi backend) thì khối chặn `cp-duplicate-repo-blocked` + nút "Open that project" (`cp-open-existing-project`), Submit vô hiệu; prop `onOpenExistingProject`.
- [x] `addRepoPath`: lỗi trùng thì toast có tên project và hành động mở.
- [x] Desktop (repo local/SSH không `devServerId`) không đổi: chỉ áp khi có `devServerId`.

## Tiêu chí hoàn thành

- Thêm lại cùng (dev server, đường dẫn) — kể cả có `/` cuối hoặc ký tự `\` kiểu Windows — bị chặn, không `project.create`/`repo.add`.
- Cùng đường dẫn nhưng dev server khác không bị chặn.
- Thông điệp backend `PROJECT_REPO_ALREADY_REGISTERED: ... already in project P (id)` hiện cùng khối chặn.

## Kiểm thử

Vitest: `registered-repo-lookup.test.ts`, `own-project-registration.test.ts`, `AddRepoDuplicateNotice.test.tsx`, `CreateProjectDialog.test.tsx` (cập nhật các test dùng `cp-duplicate-repo-warning`), `useAddRepoServerPathFlow.test.ts`. Playwright mock: chặn trùng không có lời gọi ghi (`callsTo('project.create')` rỗng).

## Blast radius

`CreateProjectDialog` (gọi từ `ProjectSwitcher`, test); `useAddRepoServerPathFlow` (gọi từ `AddRepoDialog`); `addRepoPath` (xem task 023). TRUNG BÌNH.

## Rủi ro · Ước lượng

TRUNG BÌNH (kiểm trùng thêm N lời gọi `repo.list`). 1 ngày.

## Kết quả thực thi

**Trạng thái:** XONG (mã, vitest, Playwright mock; chưa kiểm live).

**Tệp đổi (frontend/src/renderer/src/...)**
- `lib/registered-repo-lookup.ts` (mới) + `.test.ts` (12 test): `normalizeRepoPathKey`, `findRegisteredRepo`, `RepoAlreadyRegisteredError`, `parseRepoAlreadyRegistered` (đọc đúng thông điệp backend `... is already registered in project "<name>" (<id>)` và biến thể `in another project`).
- `lib/open-registered-repo.ts` (mới), `components/sidebar/add-repo-duplicate-lookup.ts` (mới), `components/sidebar/AddRepoDuplicateNotice.tsx` (mới).
- `components/sidebar/useAddRepoServerPathFlow.ts` (kiểm trùng trước khi quét/thêm; `duplicate`, `openDuplicate`), `AddRepoDialog.tsx`, `AddRepoDialogStepContent.tsx`, `AddRepoServerStartStep.tsx`.
- `components/project/CreateProjectDialog.tsx` (bỏ cảnh báo amber, chặn + nút mở; prop `onOpenExistingProject`), `ProjectSwitcher.tsx`.
- `store/slices/repos.ts`: toast "Project already added" kèm hành động "Open project" khi lỗi trùng lọt tới `addRepoPath`.
- Selector mới: `add-project-duplicate`, `add-project-open-existing` (Add Project); `cp-duplicate-repo-blocked`, `cp-open-existing-project` (Create New Project; `cp-duplicate-repo-warning` đã bỏ).
- Test: `CreateProjectDialog.test.tsx` (khối "duplicate repo is blocked", 5 test), Playwright mock `add-project-own-project.web.e2e.ts` (c, d, e).

**Lệch so với tài liệu:** (1) thông điệp backend thật khác mô tả ban đầu (`already registered in project "<name>" (<id>)`): regex nhận cả hai dạng. (2) Khi người khác sở hữu project riêng tư, backend không trả tên/id: khối chặn không có nút mở. (3) Lỗi trùng do backend ở luồng Add Project hiện thành toast có hành động, không phải khối inline (kiểm trước bằng store đã chặn trường hợp thường gặp).
