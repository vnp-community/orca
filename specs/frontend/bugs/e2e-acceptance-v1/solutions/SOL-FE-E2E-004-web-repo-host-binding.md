# SOL-FE-E2E-004: Web luôn gắn host (dev server) cho project/repo; chọn, hiển thị, đổi host; Browse folder có giới hạn

**Resolves:** [TASK-FE-E2E-015](../tasks/TASK-FE-E2E-015-web-add-project-drops-dev-server.md) (`WORKTREE_CREATE_FAILED ... chdir ... no such file or directory`) · **Gốc:** frontend web bỏ host đã chọn nên repo lưu `dev_server_id` rỗng
**Tasks:** TASK-FE-E2E-016 đến 021 · **Backend đi kèm:** [SOL-V2-002](../../../../backend-go/bugs/e2e-acceptance/v2/solutions/SOL-V2-002-repo-host-binding-and-no-silent-local.md) (TASK-V2-028 đến 034)

Mọi `file:dòng` là mã THẬT đã đọc ngày 2026-10-10 (frontend ở `frontend/src/...`). Tài liệu chỉ đọc mã, không sửa mã sản phẩm.

## Quyết định đã chọn (người dùng đã chốt)

Yêu cầu: "bước tạo project cho phép chọn dev server, project được tạo gắn với dev server đó, không block việc tạo worktree và làm việc trên Orca diễn ra bình thường".

1. Mọi project/repo tạo từ web mang một dev server. Không có "repo local" ở web (trừ cờ cấu hình backend `ORCA_LOCAL_REPO_ROOT`, mặc định tắt).
2. Project mặc định được chọn/tạo THEO HOST đã chọn và gắn host đó (`project.create {devServerId}`); repo thêm vào mang `devServerId` tường minh, và backend thừa hưởng host của project nếu request không nêu.
3. Chọn sẵn dev server duy nhất đang kết nối; từ hai host bắt chọn; không host thì chặn có hướng dẫn.
4. Repo cũ không tự migrate: UI hiện host và hành động "Đổi host" (`repo.rebindDevServer`).
5. Browse folder có giới hạn, không đóng im lặng.
6. Tạo worktree không bị chặn khi host đã gắn và đang kết nối; host ngắt thì lỗi có mã, có Retry.

## 1. Hiện trạng

### 1.1 Host được biểu diễn thế nào

- `ExecutionHostId` (`shared/execution-host.ts:6-19`): `local` | `ssh:<targetId>` | `runtime:<envId>` | `devServer:<devServerId>`; `parseExecutionHostId` (dòng 110-117), `toDevServerExecutionHostId` (64), `getRepoExecutionHostId` (164-175: repo không có `connectionId`/`devServerId` thì thành `local`). SSH dùng `connectionId` (chỉ desktop); web dùng `devServerId`.
- Hộp thoại Add Project: `use-add-repo-host-selection.ts`. Ở web chỉ liệt kê dev server `available` (`use-sidebar-host-scope-options.ts:42-45`), mặc định phần tử ĐẦU (dòng 62-66, 90-93: "first connected dev server", thứ tự tuỳ ý), và khi không còn host nào thì rơi về `local` (dòng 83 `selectedHost?.id ?? defaultHostId`). Chọn host gọi `setActiveDevServerId` (dòng 120), một trạng thái TOÀN CỤC (`store/slices/dev-servers.ts:10,66,83`, không lưu qua tải lại).
- `AddRepoDialog.tsx:66-74` tách `selectedDevServerId`; chỉ dùng cho nút Browse (`339-359`) và hai hook clone/create (`useAddRepoCloneFlow`, `useCreateRepo` nhận `devServerId`). `useAddRepoServerPathFlow` (đường chính) và `useAddRepoLocalFolderFlow` KHÔNG nhận host.

### 1.2 TOÀN BỘ đường tạo repo/project từ web và host mỗi đường mang

| # | Đường (điểm vào) | Gọi gì | Host mang | Lỗi |
|---|---|---|---|---|
| 1 | Add Project, "Open host project" (`useAddRepoServerPathFlow.ts:142` `addRepoPath(path, kind)`) | `store/slices/repos.ts:3012` `addRepoPath`: nhánh `local` → `window.api.repos.add({path,kind})` (3017) → `web-preload-api.ts:1497` `repo.add {projectId,url,displayName}`; nhánh `environment` → `repo.add` cùng dạng (3036-3041) | KHÔNG | **Lỗi của người dùng** |
| 2 | Quét repo lồng (`useAddRepoServerPathFlow.ts:87` `scanNestedRepos(path, undefined, ...)`) | `repos.ts:2495` `projectGroup.scanNested {devServerId: get().activeDevServerId}` | host TOÀN CỤC (null sau khi tải lại) | dễ mất |
| 3 | Import repo lồng (`importNestedRepos`) | `repos.ts:2544` `projectGroup.importNested {devServerId: activeDevServerId}` | host toàn cục | dễ mất |
| 4 | Clone trong hộp thoại (`useAddRepoCloneFlow.ts:160-190`) | `repo.clone {devServerId,url,destPath}` (đúng) rồi `repo.add {projectId,url,displayName}` | clone CÓ, đăng ký KHÔNG | repo clone trên host nhưng lưu rỗng |
| 5 | Create project trong hộp thoại (`useCreateRepo.ts:155-197`) | `repo.create {devServerId,...}` rồi `repo.add` | tạo CÓ, đăng ký KHÔNG | như trên |
| 6 | Xác nhận thư mục không phải git (`NonGitFolderDialog`, `AddProjectFromFolderDialog.tsx:100`, `addNonGitFolder` `repos.ts:3404-3410`) | `addRepoPath(path,'folder', {runtimeEnvironmentId})` hoặc `addRuntimeRepoRemote` | modal data chỉ có `runtimeEnvironmentId` | KHÔNG |
| 7 | `addRuntimeRepoRemote` (`runtime/runtime-repo-client.ts:75-100`; gọi từ `AddRepoSteps.tsx:179`, `AddProjectFromFolderDialog.tsx:71`, `NonGitFolderDialog.tsx:57`, `onboarding/AddRepoStep.tsx:84,140`) | nhánh environment gửi `repo.add {path,kind}` (SAI dạng, thiếu `projectId`,`url`); nhánh local → `web-preload-api.ts:1572` `addRemote` bỏ `connectionId` | KHÔNG (onboarding truyền dev server id dưới tên `connectionId`) | lỗi dạng + mất host |
| 8 | Onboarding (`use-onboarding-flow.ts:860,884,922,1142`) | `addRepoPath`, `repos.add({path})`, `repo.clone {url,destination}` | KHÔNG; `repo.clone` sai tên trường (gateway đọc `devServerId`,`destPath`, `channels_repo_ssh_status_workspace.go:456-478`) | |
| 9 | `web-preload-api.ts:1555` `repos.clone`, `:1583` `repos.create`, `repos.ts:3358` (`setupProjectClone`, nhánh devServer ném "not supported yet" ở 3338-3341) | `repo.clone {url,destination}`, `repo.create {parentPath,name,kind}` | KHÔNG, sai tên trường, đọc `.repo` không có | |
| 10 | Create New Project (`components/project/CreateProjectDialog.tsx:144-197`) | `project.create` rồi `repo.add {..., devServerId}` | CÓ (đúng) nhưng `project.create` KHÔNG mang `devServerId` (chú thích dòng 171-175 đã lỗi thời: gateway nhận `devServerId`+`repoPath`, `channels_tenant_project.go:449-480`); lỗi `repo.add` chỉ toast, project mồ côi | |
| 11 | Project mặc định (`repos.ts:1096` `getOrCreateDefaultProject`, `web-preload-api.ts:1473` `getOrCreateDefaultProjectId`) | lấy project TẠO SỚM NHẤT, hoặc `project.create {name:'My Repos'}` không host | KHÔNG (host của project mặc định là di sản, ví dụ `local-mac`) | gốc của "project có host khác repo" |
| 12 | Thư mục workspace (`repos.ts:2636` `folderWorkspace.create`, `2433` path status) | `{devServerId: activeDevServerId}`; DB `folder_workspaces.dev_server_id NOT NULL` | host toàn cục | rỗng thì backend từ chối |
| 13 | Host setup (`repos.ts:3138-3170` `projectHostSetup.setupExistingFolder`, `create`) | `hostId` của args; `getProjectSetupRuntimeTarget` chỉ biết `runtime`/`local` (`repos.ts:239-247`) | `devServer:` KHÔNG được ánh xạ | CHƯA XÁC MINH có chạy ở web (không thấy `projects.setupExistingFolder` trong `web-preload-api.ts`) |
| 14 | `init-repo-as-git.ts:38-47` | `repo.create {devServerId: repo.devServerId ?? ''}` | repo rỗng host → `GITGATEWAY_MISSING_DEV_SERVER_ID` | không tạo repo, nhưng cần xử lý |
| 15 | REST `POST /projects/{id}/repos`, script e2e | không có trường host (backend) | KHÔNG | xem SOL-V2-002 |

Điểm chung: host đã chọn trong hộp thoại chỉ dùng để DUYỆT thư mục; không đường nào (trừ #10 và nửa đầu #4/#5) chuyển nó xuống `repo.add`.

### 1.3 Browse folder

- `RemoteFileBrowser.tsx:120-148` `fetchListing` gọi `devServer.browseDir {id,path}` không giới hạn; `resolvePathInput` (243-342) liệt kê MỌI thư mục cha của đường dẫn gõ vào (mỗi đoạn một `fetchListing`), nên `/private/var/folders/.../T` (~1000 mục) làm vượt khung 32 KiB của infra-fleet (SOL-V2-002 mục 1.3) và rớt agent.
- Lỗi đã được hiển thị trong chính trình duyệt (`setError`, dòng 169-173, 282-290), nên "đóng im lặng" KHÔNG đến từ nó. Nguyên nhân suy ra từ mã (CHƯA kiểm bằng test): khi dev server rớt, `useSidebarHostScopeOptions` bỏ nó khỏi danh sách (chỉ `available`), `selectedHostId` rơi về `local` (`use-add-repo-host-selection.ts:83`), `useAddRepoHostChangeReset` xoá trạng thái host (`use-add-repo-host-change-reset.ts:19-26`) và điều kiện `browsing && (runtimeEnvironmentId || devServerId)` ở `AddRepoServerStartStep.tsx:43` thành sai nên trình duyệt thoát ra. Task 018 viết test tái hiện trước khi sửa.

## 2. Nguyên nhân gốc

Mô hình hai nửa: hộp thoại biết host nhưng store/preload/`addRepoPath` không có tham số host; `activeDevServerId` toàn cục che khuyết điểm ở vài đường và vắng ở các đường khác; project mặc định không gắn host; không có trạng thái "chưa có host" nên mọi đường rơi vào "local" rồi backend chạy git trong container.

## 3. Thiết kế

### 3.1 Mô hình host → devServerId (một nguồn)

Tệp mới `lib/repo-host-binding.ts`: `resolveRepoHostBinding(hostId)` bọc `parseExecutionHostId` và trả `{kind:'devServer', devServerId} | {kind:'ssh', connectionId} | {kind:'runtime', environmentId} | {kind:'local'}`; `RepoHostRequiredError` (mã `REPO_HOST_REQUIRED`); `requireDevServerIdForWeb(binding)` ném lỗi này khi `isWebClientLocation()` và không có dev server; `isUnboundWebRepo(repo)` = web && `!repo.devServerId && !repo.connectionId`. Desktop (Electron) không đổi: host `local`/`ssh`/`runtime` đi đường cũ; chỉ web bắt buộc `devServerId`. Không đổi dạng `ExecutionHostId`.

### 3.2 Luôn truyền `devServerId` ở MỌI đường (#1 đến #12)

- Kiểu preload (`preload/api-types.ts:988,1027,1037,1044`): thêm `devServerId?: string` cho `repos.add`, `addRemote`, `clone`, `create`.
- `web-preload-api.ts`: `add/addRemote/clone/create` gọi `requireDevServerIdForWeb` TRƯỚC mọi RPC (không tạo project mồ côi); `repo.add` gửi `devServerId`; `clone`/`create` sửa tên trường theo gateway (`destPath`, `devServerId`) và dạng kết quả (`{worktreePath,defaultBranch}`, không `.repo`) rồi đăng ký bằng `repo.add` mang `devServerId` (hoặc dùng kênh nguyên tử nếu backend bổ sung sau).
- Store `addRepoPath(path, kind, options)`: `options` thêm `devServerId` (cạnh `runtimeEnvironmentId`, `repos.ts:1623`); cả hai nhánh (3017 và 3036) gửi nó; web thiếu thì ném `RepoHostRequiredError`. `addNonGitFolder` và modal `confirm-non-git-folder` mang `devServerId` trong dữ liệu modal.
- Hook hộp thoại: `useAddRepoServerPathFlow`, `useAddRepoLocalFolderFlow`, `useAddRepoCloneFlow`, `useCreateRepo`, nhập host qua tham số `getHostBinding()` do `AddRepoDialog` cấp (từ `selectedDevServerId`, dòng 66-74), không đọc `activeDevServerId`.
- `scanNestedRepos`, `importNestedRepos`, `createFolderWorkspace`, `getFolderWorkspacePathStatus`: nhận `devServerId` tường minh; `activeDevServerId` chỉ còn là dự phòng cho caller không có ngữ cảnh hộp thoại (có `console.warn` khi dùng).
- `runtime-repo-client.ts:75-100` `addRuntimeRepoRemote`: thêm `devServerId`, đổi nhánh environment sang dạng Go `{projectId,url,displayName,devServerId}` (dùng `getOrCreateDefaultProject`); `onboarding/AddRepoStep.tsx` truyền `devServerId` thật thay vì nhét vào `connectionId`.
- Project mặc định THEO HOST (`getOrCreateDefaultProject` ở `repos.ts:1096` và `web-preload-api.ts:1473`): với host đã chọn, tìm project của người gọi có `devServerId` bằng host đó (sớm nhất); không có thì `project.create {name:<tên host>, devServerId, visibility:'private'}` (backend cho phép không `repoPath`, SOL-V2-002 mục 3.2.0), rồi `repo.add` mang `devServerId` tường minh. Cache khoá theo host. Project cũ `My Repos` không bị đụng; repo đã có trong đó giữ nguyên.
- `CreateProjectDialog.tsx:171-197`: gửi `devServerId` ngay ở `project.create` (và `repoPath` nếu có) thay vì chỉ ở `repo.add`; nếu `repo.add` lỗi thì hiện lỗi trong hộp thoại và giữ nút thử lại thay vì chỉ toast; sửa chú thích lỗi thời.
- Giữ `devServerId: repo.devServerId || project.devServerId` ở `repos.ts:1317`: backend nay thừa hưởng host của project (SOL-V2-002 mục 3.2.3) nên hai bên khớp; thêm cờ `devServerInherited` ở view để UI ghi "kế thừa từ project".

### 3.3 Chọn host trong hộp thoại (`use-add-repo-host-selection.ts`)

- 0 dev server kết nối (web): trạng thái chặn, KHÔNG rơi về `local`: panel "Chưa có dev server kết nối" + nút "Mở Dev Servers" (`openSettingsTarget({pane:'servers'})`, đã dùng ở `AddRepoHostSelectorSlot.tsx:20-23`); mọi nút Browse/Add vô hiệu.
- 1 host: chọn sẵn, hiện tên host rõ ràng.
- ≥2 host: không chọn sẵn (bỏ `hostOptions[0]`, dòng 62-66 và 90-93); placeholder "Chọn host", nút Browse/Add vô hiệu cho tới khi chọn. Nhớ host dùng gần nhất (`localStorage`, bọc try/catch, chỉ là tiện ích; chỉ áp nếu host còn trong danh sách).
- Host đang chọn mà ngắt giữa chừng: GIỮ lựa chọn (trạng thái `unavailable`), không rơi về `local`, không thoát bước Browse; hiện thông báo inline "Host X đã ngắt, đang nối lại" và vô hiệu Add; tự bật lại khi host `connected` (`devServers` store). Đây là sửa cho "đóng im lặng".
- Desktop: toàn bộ nhánh `!isWeb` giữ nguyên.

### 3.4 Hiển thị host của repo và "Đổi host"

- Sidebar (web): dòng project hiện nhãn host (muted, từ `devServers` store) hoặc biểu tượng cảnh báo "Chưa gắn host" cho `isUnboundWebRepo`. Nút "+" tạo worktree (`repo-header-create-state.ts:15-`, dùng ở `WorktreeList.tsx:214`) KHÔNG chết: với repo không host bấm sẽ mở hộp thoại "Chọn host" (STYLEGUIDE: có hành động trực tiếp cho trạng thái thiếu dữ liệu).
- Settings → Repository: hàng "Host" (tên, trạng thái connected/disconnected) + nút "Đổi host" mở chọn dev server → `repo.rebindDevServer {repoId,newDevServerId}` (đã có ở `ProjectDevServerSection.tsx:73-87`). Tách logic thành hook `useRepoHostRebind` để dùng chung cho `ProjectDevServerSection`, Settings và hộp thoại "Chọn host"; xử lý `PROJECT_HAS_ACTIVE_WORKFLOWS` ("dừng task đang chạy trước"), xoá cache worktree, lấy lại danh sách repo; nếu mở từ panel lỗi tạo worktree thì tự Retry.

### 3.5 Lỗi có mã trong UI (`lib/workspace-create-error-format.ts`)

Thêm trước nhóm `NOT_A_GIT_REPO_ANCHORS` (dòng 14-24; giữ anchor `worktree_detect_failed` vì lỗi detect không còn chứa chữ "not a git repository"): `GITGATEWAY_REPO_HOST_UNBOUND`/`REPO_HOST_UNBOUND` → kind `host-unbound` (nút "Chọn host"); `GITGATEWAY_DEV_SERVER_UNREACHABLE`/`DEV_SERVER_UNREACHABLE`/`INFRA_DEV_SERVER_NOT_CONNECTED` → kind `host-unreachable` (thông báo host offline + Retry); `PROJECT_REPO_HOST_REQUIRED` → chọn host. `WorktreeCreationPanel.tsx` hiển thị nút theo kind (đã có mẫu `not-a-git-repo`).

### 3.6 Browse folder

- Gọi `browseDir` với `limit` (500) và dùng `truncated/totalEntries` của backend (SOL-V2-002 mục 3.6): dải "Hiển thị N/M mục, gõ để thu hẹp"; khi truncated, lọc đi qua tham số `query` phía máy chủ (debounce).
- `resolvePathInput`: đường dẫn tuyệt đối thì gọi MỘT `browseDir(đường dẫn cuối)` trước; chỉ khi lỗi mới duyệt từng đoạn (và không liệt kê cha khi không cần). Cache giữ nguyên.
- Lỗi: giữ trình duyệt mở, hiện lỗi inline + nút Thử lại; với `INFRA_DEV_SERVER_NOT_CONNECTED`/rớt agent hiện "Host đang nối lại" và tự tải lại khi host `connected`. Không bao giờ đóng im lặng (kèm sửa 3.3).
- Phát hiện phụ không sửa: `~` ở backend thành `/`; thư mục symlink hiện như file.

### 3.7 i18n, STYLEGUIDE, đa nền tảng, SSH/Electron

- Mọi chuỗi mới qua `translate()` với khoá rõ (ví dụ `auto.components.sidebar.AddRepoHostGate.*`, `auto.components.settings.RepositoryHost.*`, `auto.lib.workspace.create.error.format.hostUnbound.*`), thêm đủ 5 ngôn ngữ `i18n/locales/{en,es,ja,ko,zh}.json` và một test phủ dạng `task-open-in-git-locale-coverage.test.ts`. Các script `verify:localization-*` ở `package.json` gốc trỏ `config/scripts/...` KHÔNG có trong cây này (chỉ `check-max-lines-ratchet.mjs`), nên coi là chưa chạy được, dựa vào test phủ.
- STYLEGUIDE: tệp thật ở `guides/STYLEGUIDE.md` (AGENTS.md ghi sai `docs/STYLEGUIDE.md`). Chỉ dùng token/primitive sẵn có: lỗi chặn nằm inline (`text-destructive`), có hành động trực tiếp, không tooltip cho thông báo quan trọng (mục "Empty and error states"), không thêm màu/cỡ chữ/bóng mới. Tên tệp theo nghiệp vụ (`repo-host-binding.ts`, `use-repo-host-rebind.ts`), không `helpers/utils`; không thêm `max-lines` disable (đã có `config/max-lines-baseline.txt`: tách tệp nếu vượt).
- Đa nền tảng: không phím tắt mới; đường dẫn host xử lý theo kiểu host (Windows/Posix) bằng tiện ích sẵn có, không giả định `/`.
- SSH/Electron: nhánh `ssh`/`runtime`/`local` của desktop không đổi hành vi; tham số `devServerId` là tuỳ chọn ở kiểu preload; Electron không dùng project-service nên không gặp lỗi mới. Web không có SSH (đã ẩn ở `AddRepoHostSelectorSlot.tsx:34`).

## 4. Kiểm thử

- Vitest: `repo-host-binding.test.ts`; `web-preload-api.test.ts` (mở rộng khối `web repos preload API`, dòng 2196-): `add/addRemote/clone/create` gửi `devServerId`, thiếu thì ném lỗi trước mọi RPC; `store/slices/repos.test.ts` + `repos-runtime-default-project.test.ts`: `addRepoPath` truyền host ở cả hai nhánh, project mặc định theo host; `useAddRepoServerPathFlow.test.ts`; test hook chọn host (0/1/≥2 host, host ngắt giữ lựa chọn); `workspace-create-error-format.test`; `RemoteFileBrowser` (truncated, một lần gọi cho đường tuyệt đối, lỗi inline).
- Playwright mock (`tests/stages/e2e/legacy/request-web`, `support/mock-request-ws.ts` có `on(method,handler)` và `callsTo`): spec mới kiểm payload `repo.add` có `devServerId` ở các đường 1, 4, 5, 6; chọn host khi ≥2; chặn khi 0 host; "Đổi host".
- Live: `ui-live/add-project-worktree-repro.web.e2e.ts` phải XANH (TASK-FE-E2E-021).

## 5. Lộ trình triển khai & kiểm chứng

1. TASK-FE-E2E-016 + 017 + 018 (cùng một PR hoặc ba commit liên tiếp): frontend gửi host. Redeploy: `BUILD_FRONTEND=1 ./deploy/dev/scripts/redeploy-local.sh frontend`. Kiểm: `python3 tests/stages/e2e/orca-flows/run_ui_flow.py --repro add-project-worktree` chuyển ĐỎ → XANH (repo tạm `/private/tmp/e2e-ui-repro-*`, tự gỡ). Chỉ cần frontend vì gateway đã nhận `devServerId`.
2. Backend TASK-V2-028/030 song song (không phụ thuộc UI); TASK-V2-029 chỉ sau báo cáo repo mồ côi.
3. TASK-FE-E2E-019 + 020 sau khi backend giới hạn browse (TASK-V2-030) lên.
4. TASK-FE-E2E-021 đóng đợt: i18n, test, cập nhật spec live; chạy `run_ui_flow.py --real all` để chắc không hồi quy (project/repo tạm của fixture, không đụng repo `codex` hay project có sẵn).
5. Khi kiểm bằng tay hay live, KHÔNG thêm/gắn lại/xoá repo thật của người dùng; chỉ repo tạm, dọn trong `finally`.

## 6. Rủi ro, thay thế đã loại, chưa xác minh

- Rủi ro: tab frontend cũ gửi `repo.add` thiếu host: backend thừa hưởng host project hoặc báo `PROJECT_REPO_HOST_REQUIRED` (thông điệp nêu tải lại trang). Project theo host thêm project mới cho mỗi host (chấp nhận; nhóm bằng tên host).
- Loại: chỉ đọc `activeDevServerId` toàn cục (mất sau tải lại, đua giữa hộp thoại); tự suy host bằng "host duy nhất" ở tầng store (che lỗi thiếu host của các đường khác); ẩn cảnh báo repo mồ côi (người dùng không biết vì sao).
- Chưa xác minh: nguyên nhân "đóng im lặng" (3.3 suy từ mã); `projectHostSetup.*` có dùng được ở web (#13); hành vi `project.create` không `repoPath` (cần backend 028); chưa chạy bất kỳ test nào ở bước viết tài liệu này.
