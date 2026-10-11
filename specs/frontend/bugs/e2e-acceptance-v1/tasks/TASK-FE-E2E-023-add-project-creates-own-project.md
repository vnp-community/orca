# TASK-FE-E2E-023: "Add Project" tạo PROJECT RIÊNG cho mỗi repo thêm vào (thư mục có sẵn, clone, create)

**From Solution:** SOL-FE-E2E-005 (mục 1.2, 3.2) · **Resolves:** mọi repo bị nhét vào project mặc định của dev server (ví dụ "Demo (this Mac)")
**Priority:** P0 · **Area:** frontend (web preload + store + hộp thoại Add Project)
**Files:**
- frontend/src/renderer/src/lib/own-project-registration.ts (mới) + `.test.ts`
- frontend/src/preload/api-types.ts (`repos.add/addRemote/clone/create`: thêm `projectName?: string`)
- frontend/src/renderer/src/web/web-preload-api.ts (`createReposApi`: `add` 1511, `clone` 1572, `addRemote` 1602, `create` 1615)
- frontend/src/renderer/src/store/slices/repos.ts (`AddRepoPathRouteOptions`; `addRepoPath` 3103-3260)
- frontend/src/renderer/src/lib/clone-repo-onto-dev-server.ts
- frontend/src/renderer/src/components/sidebar/useAddRepoServerPathFlow.ts, useAddRepoCloneFlow.ts, useCreateRepo.ts, AddRepoDialog.tsx, AddRepoDialogStepContent.tsx, AddRepoServerStartStep.tsx, NonGitFolderDialog.tsx (+ test tương ứng)
**Depends on:** backend `project.create` tạo repo từ `repoPath` (agent AE); frontend có đường bù `repo.add` nếu backend cũ
**Status:** `[x]` XONG (2026-10-11): mã, vitest, Playwright mock và kiểm live. Kiểm live (2026-10-11): e2e `--real addproject` XANH 2/2 (Claude thật); `--real all` 7/7 XANH (24.1 phút); `--repro add-project-worktree` xanh; dữ liệu thật của người dùng không đổi.

---

## Context

`addRepoPath` và các đường clone/create/`addRemote` đều gọi `getOrCreateDefaultProject`/`getOrCreateDefaultProjectId` nên chọn project SỚM NHẤT của host (`lib/repo-host-binding.ts:90-120`). Request chạy trên `repos[0]` của project (`ai_connection_resolver.go:54-62`), nên project dùng chung gây nhầm repo.

## Việc cần làm

- [x] `lib/own-project-registration.ts`: `registerRepoInOwnProject(rpc, {devServerId, path, projectName, kind?})` = kiểm trùng (task 025) → `project.create {name, devServerId, repoPath, visibility:'private'}` → `repo.list {projectId}` lấy `RemoteRepoView` (đường dẫn so theo `normalizeRuntimePathForComparison`); thiếu repo thì bù `repo.add {projectId,url,displayName,devServerId}`; lỗi bù thì ném lỗi nêu tên project (không tự xoá). `deriveProjectNameFromPath(path)` = basename, bỏ `.git`, rỗng thì `devServerId`.
- [x] Preload web: `add/addRemote/clone/create` nhận `projectName`; có `projectName` thì `registerRepoInOwnProject` (thay `getOrCreateDefaultProjectId`), vắng thì giữ nguyên. Gọi `requireRepoDevServerId` TRƯỚC mọi RPC (như hiện tại).
- [x] Store `addRepoPath`: `options.ownProject?: {name: string}`; có (và có `devServerId`) thì cả hai nhánh dùng `registerRepoInOwnProject` (nhánh `local` qua preload với `projectName`, nhánh `environment` qua `callRuntimeRpc`); toast thành công nêu tên project; modal `confirm-non-git-folder` mang `projectName` để `addNonGitFolder` giữ project riêng. Vắng `ownProject` thì giữ đường project mặc định (onboarding, `AddProjectFromFolderDialog`, desktop).
- [x] `useAddRepoServerPathFlow`: state `projectName` (mặc định = tên thư mục theo `serverPath`, ngừng tự cập nhật khi người dùng sửa tay), truyền `ownProject`; `AddRepoServerStartStep` (bước "Open host project") thêm ô "Project name" `data-testid="add-project-name"`.
- [x] Clone/Create: `cloneRepoOntoDevServer`, `useAddRepoCloneFlow`, `useCreateRepo` truyền tên repo làm tên project khi có `devServerId` (`addRuntimeRepoRemote` KHÔNG đổi: không có caller Add Project, xem "Kết quả thực thi").
- [x] Nested import không đổi (backend đã tạo project cho từng repo).
- [x] Không đổi nhánh desktop (local/SSH, không `devServerId`).

## Tiêu chí hoàn thành

- Add Project thư mục `/x/foo` trên dev server D: có đúng một `project.create {name:'foo', devServerId:D, repoPath:'/x/foo', visibility:'private'}`, không `project.list`→chọn project cũ; repo hiện trong sidebar và được mở; sửa tên thành "Bar" thì `name:'Bar'`.
- Add Project lần hai thư mục khác: project thứ hai riêng.
- Không host thì không RPC nào (như hiện tại).
- Onboarding/`AddProjectFromFolderDialog` không đổi payload.

## Kiểm thử

Vitest: `own-project-registration.test.ts`, `web-preload-api.test.ts` (khối `host binding`), `store/slices/repos-own-project.test.ts` (hai nhánh, fallback bù `repo.add`, desktop không đổi), `useAddRepoServerPathFlow.test.ts`, `useAddRepoCloneFlow.test.ts`, `useCreateRepo.default-checkout.test.ts`, `AddRepoStartSteps.test.tsx`. Playwright mock: `add-project-own-project.web.e2e.ts`.

## Blast radius (codegraph, không có gitnexus)

- `addRepoPath` (14 callers): `useOnboardingFlow`, `AddProjectFromFolderDialog`, `useAddRepoLocalFolderFlow`, `useAddRepoServerPathFlow`, test repos; tham số mới tuỳ chọn, vắng thì không đổi.
- `getOrCreateDefaultProject` (5): `useAddRepoCloneFlow`, `useCreateRepo`, `createRepoSlice`; không bị sửa chữ ký.
- `addRuntimeRepoRemote` (8 callers: onboarding, SSH): không đổi.
- `useAddRepoServerPathFlow`: `AddRepoDialog` + test.
- Rủi ro TRUNG BÌNH (store dùng chung, preload web).

## Rủi ro · Ước lượng

TRUNG BÌNH. 1,5 ngày.

## Kết quả thực thi

**Trạng thái:** XONG (mã, vitest, Playwright mock; chưa kiểm live; chưa kiểm Electron thật).

**Tệp đổi (frontend/src/...)**
- `renderer/src/lib/own-project-registration.ts` (mới) + `.test.ts` (10 test): `registerRepoInOwnProject`, `findRegisteredRepoRemote`, `deriveProjectNameFromPath`, kiểu `ProjectRpc` (cùng logic cho preload web `callRuntimeResult` và store `callRuntimeRpc(target,...)`).
- `preload/api-types.ts`: `projectName?: string` cho `repos.add/clone/create` (KHÔNG cho `addRemote`: không có caller Add Project).
- `renderer/src/web/web-preload-api.ts`: helper `registerRepoOnDevServer` (có `projectName` thì project riêng, vắng thì project mặc định như cũ) dùng bởi `add/clone/addRemote/create`.
- `renderer/src/store/slices/repos.ts`: `AddRepoPathRouteOptions.ownProject?: {name}`; `addRepoPath` đi project riêng ở cả hai nhánh khi có `devServerId` + `ownProject`; modal `confirm-non-git-folder` mang `projectName`.
- `renderer/src/lib/clone-repo-onto-dev-server.ts`, `components/sidebar/{useAddRepoCloneFlow,useCreateRepo,NonGitFolderDialog,useAddRepoServerPathFlow,AddRepoDialog,AddRepoDialogStepContent,AddRepoServerStartStep}.ts(x)`, `components/sidebar/add-repo-host-routing.ts` (mới; tách khỏi `AddRepoDialog.tsx` để khỏi vượt max-lines) + test.
- Selector mới: `add-project-name` (ô "Project name", `#server-project-name`) ở bước "Open host project".
- Test: `web-preload-api.test.ts` (+3), `store/slices/repos-own-project.test.ts` (mới, 5), `useAddRepoServerPathFlow.test.ts` (+3), `useAddRepoCloneFlow.test.ts` và `useCreateRepo.default-checkout.test.ts` (viết lại phần đăng ký repo theo project riêng), Playwright mock `add-project-own-project.web.e2e.ts` (a, b) và sửa `add-project-host-binding.web.e2e.ts` (a, e, f) + mock gateway `support/mock-project-host-gateway.ts` (`project.create` có `repoPath` thì tạo repo như project-service).

**Lệch so với tài liệu:** (1) `addRuntimeRepoRemote` không đổi (không có caller). (2) Nhánh `kind: 'folder'` ("Open as Folder") cũng dùng project riêng. (3) Clone/Create đặt tên project = tên repo (không thêm ô nhập).

**Blast radius (codegraph):** `addRepoPath` (14 callers) không đổi hành vi khi vắng `ownProject`; `getOrCreateDefaultProject` (5 callers) không sửa chữ ký, các caller `useAddRepoCloneFlow`/`useCreateRepo` chỉ rẽ sang đường mới khi có `devServerId`.
