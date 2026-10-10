# TASK-FE-E2E-016: Truyền `devServerId` từ hộp thoại Add Project xuống `repo.add` (preload + store + hook) và project mặc định theo host

**From Solution:** SOL-FE-E2E-004 (mục 3.1, 3.2) · **Resolves:** [TASK-FE-E2E-015](./TASK-FE-E2E-015-web-add-project-drops-dev-server.md) (đường #1, #2, #3, #11, #12 của bảng 1.2)
**Priority:** P0 · **Area:** frontend (web preload + store + hook)
**Files:**
- frontend/src/renderer/src/lib/repo-host-binding.ts (mới) + `.test.ts`
- frontend/src/preload/api-types.ts (dòng 988 `add`, 1027 `clone`, 1037 `addRemote`, 1044 `create`: thêm `devServerId?: string`)
- frontend/src/renderer/src/web/web-preload-api.ts (`createReposApi` 1491; `add` 1497; `getOrCreateDefaultProjectId` 1473; `mergeRepoViewIntoRepo` 1427)
- frontend/src/renderer/src/store/slices/repos.ts (`AddRepoPathRouteOptions` 1623; `getOrCreateDefaultProject` 1096; `addRepoPath` 3012-3060; `scanNestedRepos` 2459 (2495); `importNestedRepos` 2521 (2544); `createFolderWorkspace` 2604 (2636); `getFolderWorkspacePathStatus` 2433; `addNonGitFolder` ~3404)
- frontend/src/renderer/src/components/sidebar/AddRepoDialog.tsx (66-74, 169-201), useAddRepoServerPathFlow.ts (37, 87, 142), useAddRepoLocalFolderFlow.ts (49, 156), AddProjectFromFolderDialog.tsx (100), NonGitFolderDialog.tsx (dữ liệu modal)
**Depends on:** không (gateway đã nhận `devServerId`: `channels_repo_ssh_status_workspace.go:112-137`)
**Status:** `[x]` XONG (2026-10-11): mã, vitest, Playwright mock và kiểm live. Kiểm live (2026-10-11): `run_ui_flow.py --repro add-project-worktree` XANH (worktree test-02 tạo được, repo có devServerId = dev-local-mac); `--real all` 6/6 XANH (21.8 phút); backend `--strict --ai-stub` 114 PASS, 0 FAIL.

---

## Context

Hộp thoại biết host (`selectedDevServerId`, `AddRepoDialog.tsx:66-74`) nhưng `addRepoPath(path, kind)` và `createReposApi().add({path})` không có tham số host, nên `repo.add` luôn thiếu `devServerId` (SOL-FE-E2E-004 mục 1.2).

## Việc cần làm

- [x] `lib/repo-host-binding.ts`: `resolveRepoHostBinding(hostId)`, `RepoHostRequiredError` (mã `REPO_HOST_REQUIRED`), `requireDevServerIdForWeb`, `isUnboundWebRepo`. Desktop (`!isWebClientLocation()`) không bị ép.
- [x] Kiểu preload: thêm `devServerId?: string` cho `repos.add/addRemote/clone/create` (tuỳ chọn, tương thích Electron).
- [x] `web-preload-api.ts`: `add` gọi `requireDevServerIdForWeb` TRƯỚC `getOrCreateDefaultProjectId` (không tạo project mồ côi) rồi gửi `devServerId` trong `repo.add`; ghi chú ngắn vì sao.
- [x] `repos.ts`: `AddRepoPathRouteOptions` thêm `devServerId`; `addRepoPath` truyền nó ở nhánh `local` (3017: `window.api.repos.add({path,kind,devServerId})`) và nhánh `environment` (3036-3041); web thiếu host thì ném `RepoHostRequiredError`, toast có mã (xem 018). `addNonGitFolder` và modal `confirm-non-git-folder` mang `devServerId`.
- [x] Project mặc định THEO HOST: `getOrCreateDefaultProject(target, devServerId)` (repos.ts) và `getOrCreateDefaultProjectId(devServerId)` (preload): chọn project của người gọi có `devServerId` bằng host (sớm nhất), không có thì `project.create {name:<tên host>, devServerId, visibility:'private'}` (cần TASK-V2-028 nới `repoPath`); cache khoá theo host; `resetDefaultProjectCacheForTests` giữ nguyên chữ ký. Project cũ không bị đụng.
- [x] Hook: `useAddRepoServerPathFlow` và `useAddRepoLocalFolderFlow` nhận `getHostBinding()` (hoặc `devServerId`) từ `AddRepoDialog`, truyền vào `addRepoPath` và `scanNestedRepos`.
- [x] `scanNestedRepos`, `importNestedRepos`, `createFolderWorkspace`, `getFolderWorkspacePathStatus`: tham số `devServerId` tường minh (ưu tiên), dự phòng `get().activeDevServerId` kèm `console.warn`.
- [x] Không đổi nhánh desktop (`target.kind==='local'` ngoài web).

## Tiêu chí hoàn thành

- Với host `dev-local-mac` chọn trong hộp thoại, `repo.add` gửi `devServerId` = id của nó; `repo.list` trả đúng; repo hiện trong project gắn host đó.
- Web không có host thì không có RPC nào được gửi, người dùng thấy lỗi `REPO_HOST_REQUIRED` rõ.
- `run_ui_flow.py --repro add-project-worktree` XANH (kết hợp TASK-FE-E2E-021 cập nhật spec nếu cần).

## Kiểm thử

Vitest: `repo-host-binding.test.ts`; `web-preload-api.test.ts` (khối `web repos preload API`, 2196-): payload `repo.add` có `devServerId`, thiếu host thì không gọi RPC; `repos.test.ts`/`repos-runtime-default-project.test.ts`: cả hai nhánh `addRepoPath`, project mặc định theo host; `useAddRepoServerPathFlow.test.ts`. Lệnh: `pnpm exec vitest run --config config/vitest.config.ts <đường dẫn các test trên>` (trong `frontend/`).

## Rủi ro · Ước lượng

MEDIUM (store dùng chung; `addRepoPath` có nhiều caller: `useAddRepoLocalFolderFlow`, `AddProjectFromFolderDialog`, `use-onboarding-flow:860`, `addNonGitFolder`; chạy `codegraph callers addRepoPath` trước khi sửa). 1 đến 1.5 ngày.

## Kết quả thực thi

**Trạng thái:** mã + vitest + Playwright mock XANH; CHƯA chạy spec live (`--repro add-project-worktree`) vì người điều phối chưa redeploy frontend/backend và đã dặn không chạy.

**File đổi (frontend/src/...)**
- `renderer/src/lib/repo-host-binding.ts` (mới) + `.test.ts`: `RepoHostRequiredError` (mã `REPO_HOST_REQUIRED`), `resolveRepoHostBinding`, `requireRepoDevServerId` (luôn ném; dùng ở preload web), `requireDevServerIdForWeb` (chỉ ép ở web), `isUnboundWebRepo`, `pickProjectBoundToDevServer`, `resolveProjectIdForDevServer` (chọn project của người gọi gắn host đó; không có thì `project.create {name:<tên host>, devServerId, visibility:'private'}`; nếu tạo lỗi thì lùi về project sớm nhất, repo vẫn mang `devServerId`).
- `preload/api-types.ts`: `devServerId?: string` cho `repos.add/addRemote/clone/create` (tuỳ chọn, tương thích Electron).
- `renderer/src/web/web-preload-api.ts`: `add/addRemote/clone/create` gọi `requireRepoDevServerId` TRƯỚC mọi RPC; `getOrCreateDefaultProjectId(devServerId)` có cache theo host; `repo.add` gửi `devServerId`; `add/addRemote` đặt `kind` khi có.
- `renderer/src/store/slices/repos.ts`: `getOrCreateDefaultProject(target, devServerId?)` cache theo host (`resetDefaultProjectCacheForTests` giữ nguyên chữ ký); `AddRepoPathRouteOptions`/`FolderWorkspacePathStatusRouteOptions`/`NestedRepoScanControls`/`importNestedRepos args` có `devServerId`; `addRepoPath` truyền host ở CẢ HAI nhánh (local leg: `window.api.repos.add({path,kind,devServerId})` chỉ thêm khóa khi có; environment leg: `repo.add {..., devServerId}` trong project theo host), web thiếu host thì ném `RepoHostRequiredError` trước RPC (toast mô tả rõ); modal `confirm-non-git-folder` mang `devServerId`; `scanNestedRepos/importNestedRepos/createFolderWorkspace/fetchFolderWorkspacePathStatus` ưu tiên host tường minh, dự phòng `activeDevServerId` kèm MỘT lần `console.warn` (path-status gọi dày nên không warn mỗi lần).
- `components/sidebar/AddRepoDialog.tsx`, `useAddRepoServerPathFlow.ts`, `useAddRepoNestedImportFlow.ts`, `NonGitFolderDialog.tsx`: host của hộp thoại (`selectedDevServerId`) đi xuống `addRepoPath`, `scanNestedRepos`, `importNestedRepos`, `addNonGitFolder`. `useAddRepoLocalFolderFlow` KHÔNG đổi: nó chỉ chạy với bộ chọn thư mục gốc của desktop; web luôn là host `devServer` nên không bao giờ vào nhánh này.
- Test: `lib/repo-host-binding.test.ts`; `web/web-preload-api.test.ts` (khối mới `host binding (devServerId)`, kể cả test hợp đồng đọc tag JSON của gateway `channels_repo_ssh_status_workspace.go`); `store/slices/repos-host-binding.test.ts` (8 test: hai nhánh, project theo host, web không host, dự phòng toàn cục, desktop giữ nguyên payload, quét lồng); `useAddRepoServerPathFlow.test.ts`; `repos`-fixture không đổi.

**Lệch so với solution:** `getOrCreateDefaultProject(target, devServerId)` nhận host qua tham số thứ hai (như solution) nhưng project mới đặt tên theo tên dev server (đọc `devServer.list`), không phải `My Repos`; project cũ không bị đụng.

**Lệnh đã chạy và kết quả thật (2026-10-10)**
- `NODE_OPTIONS=--no-experimental-webstorage pnpm exec vitest run --config config/vitest.config.ts <các tệp đã đổi/mới + components/sidebar + ProjectDevServerSection>` (trong `frontend/`): 189 tệp, 1647 xanh, 4 đỏ. 4 đỏ là `Sidebar.test.tsx` (2) và `WorktreeCardMeta.interaction.test.tsx` (2), ĐỎ SẴN trên HEAD (đối chiếu bằng bản sao sạch của HEAD).
- Toàn bộ vitest frontend: HEAD 28 tệp đỏ / 85 test; sau thay đổi 25 tệp đỏ / 79 test; tập tệp đỏ sau thay đổi là tập con của tập đỏ trên HEAD (không test nào xanh ở HEAD mà đỏ bây giờ). Một phần chênh lệch do bản sao HEAD thiếu cây `backend-go` (`request-rpc-contract.test.ts`) và do tôi chép test mới vào bản sao để tái hiện.
- `./node_modules/.bin/tsc --noEmit -p frontend/tsconfig.json --composite false`: không có lỗi mới ở tệp tôi đụng; còn lại là lỗi CÓ SẴN (`ui.ts` `previousViewBefore*`, `use-onboarding-flow.ts` `StepId`, `web-preload-api.ts` 740/1506/2567 `onboarding`/`scanRemote`/`arch`, `CreateProjectDialog.test.tsx` `MockAppState`, `web-preload-api.test.ts` ~4281).
- `./node_modules/.bin/oxlint -c .oxlintrc.json <mọi tệp ts/tsx đã đổi/mới>`: sạch (exit 0), kể cả 3 tệp CÓ SẴN vượt max-lines mà tôi đã tách (xem 017/018); `./node_modules/.bin/oxfmt --list-different` trên cùng tập: sạch sau `--write`. Không thêm `max-lines` disable nào (`git diff` không có dòng `+...-disable`).
- `npx playwright test -c tests/configs/playwright.web.config.ts --project=request-web`: 67 xanh, 1 skipped (có sẵn); gồm 9 test mới ở `add-project-host-binding.web.e2e.ts` (a-f) và `repo-host-rebind.web.e2e.ts` (g-i). Đã xoá `test-results` ở gốc sau đó.

**Blast radius (codegraph, không có gitnexus)**
- `addRepoPath`: callers `useOnboardingFlow`, `AddProjectFromFolderDialog`, `useAddRepoLocalFolderFlow`, `useAddRepoServerPathFlow`, test repos; không đổi hành vi khi không truyền `devServerId` (desktop) và khóa `devServerId` chỉ có mặt khi có giá trị.
- `getOrCreateDefaultProject`: callers `useAddRepoCloneFlow`, `useCreateRepo`, `createRepoSlice` (+ `lib/clone-repo-onto-dev-server.ts`, `runtime-repo-client.ts` mới); tham số thứ hai tuỳ chọn.
- `createReposApi` (web preload): chỉ web; Electron dùng preload khác.

**Chưa kiểm:** `run_ui_flow.py --repro add-project-worktree` (cần stack đã redeploy cả frontend lẫn backend AA); kiểm Electron bằng tay (chỉ kiểm qua vitest: payload desktop không đổi).
