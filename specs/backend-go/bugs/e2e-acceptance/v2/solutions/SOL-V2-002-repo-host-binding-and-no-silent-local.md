# SOL-V2-002: Repo luôn gắn host; không bao giờ chạy git "cục bộ" một cách im lặng

**Giải quyết:** BUG-V2-002 (gốc của [TASK-FE-E2E-015](../../../../../frontend/bugs/e2e-acceptance-v1/tasks/TASK-FE-E2E-015-web-add-project-drops-dev-server.md): `WORKTREE_CREATE_FAILED ... chdir <repo>: no such file or directory`)
**Service:** project-service, git-gateway-service, api-gateway (wscompat + REST), infra-fleet-service (transport), agent (`fs.readDir`), deploy/dev (cấu hình)
**Phối hợp với:** [SOL-FE-E2E-004](../../../../../frontend/bugs/e2e-acceptance-v1/solutions/SOL-FE-E2E-004-web-repo-host-binding.md) (frontend luôn truyền `devServerId`, chọn/hiển thị/đổi host)
**Tasks:** TASK-V2-028 đến TASK-V2-034 (xem [README](../tasks/README.md))

Mọi trích dẫn `file:dòng` dưới đây là mã THẬT đã đọc ngày 2026-10-10 trên nhánh `feat/e2e-acceptance-fixes`. Tài liệu này CHỈ ĐỌC mã, không sửa mã sản phẩm. Phạm vi ảnh hưởng dùng `codegraph` (tên chính xác), xem mục 8.

## Quyết định đã chọn (người dùng đã chốt, 2026-10-10)

Yêu cầu gốc: "bước tạo project cho phép chọn dev server, project được tạo gắn với dev server đó, không block việc tạo worktree và làm việc trên Orca diễn ra bình thường". Các quyết định dưới đây là cơ sở của toàn bộ tài liệu và task; không còn câu hỏi mở.

1. **Repo luôn gắn một dev server.** Không có "repo local" trong triển khai Docker/web, trừ khi cấu hình khai báo tường minh (`ORCA_LOCAL_REPO_ROOT`, mặc định tắt; compose dev có thể đặt `/data/repos`). Không bao giờ chạy git trong container một cách im lặng.
2. **Project gắn dev server khi tạo; repo thừa hưởng dev server của project khi request không nêu** (`repo.add` thiếu `devServerId` thì dùng `project.dev_server_id` nếu nó tồn tại; không có thì `PROJECT_REPO_HOST_REQUIRED`). `devServerId` tường minh trong request luôn thắng. `GetRepo` trả host HIỆU LỰC (repo, nếu rỗng thì project) nên repo cũ có project gắn host chạy được mà không cần migration.
3. **Chọn sẵn dev server duy nhất đang kết nối** (từ hai host trở lên bắt chọn; không host thì chặn có thông báo).
4. **Repo cũ không tự migrate.** Chỉ có công cụ gắn lại dry-run (`report_unbound_repos.py`) và hành động "Đổi host" ở UI.
5. **Hộp thoại Browse folder phải có giới hạn** (trần mục, không rớt agent, lỗi hiển thị chứ không đóng im lặng).
6. **Không block worktree:** với host gắn đúng và đang kết nối, tạo worktree chạy bình thường; host ngắt thì lỗi có mã, thử lại được (không `chdir`).

## 1. Chuỗi nguyên nhân (đã đối chiếu mã)

Lỗi người dùng thấy là hệ quả của ba lỗ hổng độc lập nối với nhau. Sửa một cái chưa đủ.

| # | Lỗ hổng | Bằng chứng trong mã |
|---|---|---|
| L1 | Frontend web không gửi host khi tạo repo | `web-preload-api.ts` `createReposApi().add` (dòng 1497) và `addRemote` (1572) gọi `repo.add {projectId,url,displayName}`; `store/slices/repos.ts` `addRepoPath` (3012-3060) cũng vậy. Chi tiết ở SOL-FE-E2E-004 mục 1 |
| L2 | project-service nhận repo không có host và coi là "local" | `usecase/add_repo.go:44-77`: chỉ kiểm khi `DevServerID != ""` (dòng 57), rỗng thì `domain.NewRepo(..., "")` (`domain/repo.go:54-62` chỉ cấm `projectID`/`url` rỗng) rồi ghi `project.repos.dev_server_id = NULL` (`adapter/postgres/repo_repository.go:39`). Proto ghi sẵn ngữ nghĩa này: `project.proto` `AddRepoRequest.dev_server_id` "empty = local repo" |
| L3 | git-gateway coi "không có dev server" là "chạy git ngay trong container", và cả "dev server không tới được" cũng vậy | `usecase/ports.go:673-691` (`dispatchExecutorForRepo`): `DevServerID == ""` trả `local, repo.URL` (dòng 674-676); host có nhưng `!reachable` cũng rơi xuống `return ctx, local, repo.URL, nil` (dòng 688). Bản sao cho filesystem ở `ports.go:699-717` (dòng 701 và 714). `Clone` (`usecase/clone.go:46-50`) và `InitRepo` (`usecase/init_repo.go:51-55`) khởi tạo `executor := uc.local` rồi chỉ đổi sang relay khi `reachable`, nên host không tới được thì clone/init chạy trong container. `ConnectionResolver.ResolveConnection` (`adapter/grpcclient/resolver.go:113-176`) trả `Connected:false` (dòng 157) khi host không tới được, và `resolveLocal` (dòng 208-239) nuốt lỗi `GetRepo` ("degrades to Path-only") nên cũng rơi về local |

Executor `local` là `localgit.New()` (`cmd/server/main.go:98`): tiến trình `git` trong container `orca-go-git-gateway`. Đường dẫn của repo là đường dẫn trên máy người dùng (Mac) nên container báo `chdir ...: no such file or directory`.

### 1.1 Điều làm lỗi khó đoán

- **Cửa sổ 30 giây.** `PollFleetHealth` chạy mỗi 30 giây (`infra-fleet-service/cmd/server/main.go:888`, `fleetHealthPollInterval = 30 * time.Second`). `DevServerReachability.IsReachable` (`git-gateway .../grpcclient/reachability.go:22-39`) đọc mẫu sức khoẻ đó và trả `false` khi "chưa có mẫu". Mỗi lần agent rớt (kể cả do lỗi ở mục 1.3), repo CÓ gắn host vẫn bị chạy cục bộ trong tối đa một chu kỳ, và người dùng thấy đúng cùng lỗi `chdir`. Tức là lỗi này không chỉ ảnh hưởng repo "mồ côi" mà cả repo gắn đúng host.
- **Lỗi bị bọc mất mã.** Cả 13 usecase theo repo bọc mọi lỗi điều phối thành `Internal` mã chung và giấu nguyên nhân khỏi client (`apperrors.ToGRPCStatus`, `common/apperrors/apperrors.go:101-141`, chỉ gửi `Code: Message`): `WORKTREE_RESOLVE_FAILED` ở `create_worktree.go:82`, `detect_worktrees.go:44`, `prefetch_create_base.go:31`, `resolve_pr_base.go:37`, `resolve_mr_base.go:37`; `GITGATEWAY_RESOLVE_FAILED` ở `base_ref_default.go:40`, `check_hooks.go:47`, `get_remote_url.go:39`, `read_ephemeral_vm_recipes.go:55`, `read_issue_command.go:41`, `scan_setup_script_imports.go:36`, `search_refs.go:40`, `write_issue_command.go:37`.
- **UI nói dối về host (nay được sửa bằng cách làm backend khớp).** `store/slices/repos.ts:1317` điền `devServerId: repo.devServerId || project.devServerId` khi liệt kê repo, nên repo không host trông như gắn với `devServerId` của project, trong khi backend (`get_repo.go:63`) chỉ dùng cột của repo và chạy cục bộ. Quyết định 2 làm backend thừa hưởng giống frontend. Ở repro, project `My Repos` mang `local-mac` (đang `disconnected`), KHÁC `dev-local-mac` mà người dùng đã chọn.
- **Cùng lỗi, thao tác khác.** Mọi thao tác theo repo đều dính: `worktree.detectedList` (`WORKTREE_DETECT_FAILED`, đã thấy ở TASK-FE-E2E-014), `repo.hooksCheck` (`GITGATEWAY_CHECK_HOOKS_FAILED: read hooks dir`), `repo.baseRefDefault`, `repo.searchRefs`, issue command, setup script scan, và mọi thao tác theo worktree đi qua `ConnectionResolver` (48 điểm gọi `dispatchExecutor`/`dispatchFilesystemExecutor`). Request-service đã tự chặn đúng cách một lần: `request-service/internal/adapter/grpcclient/ai_connection_resolver.go:59` trả `ErrNoDevServer` khi repo mặc định không có `dev_server_id` (tiền lệ "không đoán local").

### 1.2 Lỗi ở các kênh khác cùng luồng Add Project (gateway)

- `repo.clone` (`api-gateway .../channels_repo_ssh_status_workspace.go:456-478`) đọc `{devServerId,url,destPath}` và trả `{worktreePath,defaultBranch}`, nhưng frontend web gọi `{url,destination}` và đọc `.repo` (`web-preload-api.ts:1555-1559`, `use-onboarding-flow.ts:1142`, `store/slices/repos.ts:3358`): tên trường lệch, nên luôn thiếu `devServerId` (`GITGATEWAY_MISSING_DEV_SERVER_ID`).
- `repo.create` (`channels_repo_ssh_status_workspace.go:524-551`) đọc `{devServerId,destPath,...}`; web preload gửi `{parentPath,name,kind}` (`web-preload-api.ts:1583-1586`). Cùng lệch.
- Hộp thoại clone/create (`useAddRepoCloneFlow.ts:160-190`, `useCreateRepo.ts:155-197`) gọi ĐÚNG `repo.clone`/`repo.create` với `devServerId` rồi gọi `repo.add` THIẾU `devServerId`: repo được clone/tạo trên host nhưng đăng ký local. Cùng lỗi gốc.
- REST `POST /projects/{id}/repos` (`api-gateway .../httpgateway/project_routes.go:242-270`) không có trường `dev_server_id` trong body (`addRepoRequestBody`), nên mọi repo thêm qua REST luôn không host.
- `repo.add` bị loại khỏi MCP (`mcpserver/tools/excluded_channels.yaml:511`, `backlog`), không phải đường tạo repo hiện tại.

### 1.3 Duyệt thư mục làm rớt agent (phát hiện phụ của TASK-FE-E2E-015)

`log infra-fleet: devserveragent: connection lost: failed to read: websocket: message too big: read limited at 32769 bytes`.

Nguyên nhân gốc là cấu hình, KHÔNG phải kích thước nghiệp vụ: `github.com/coder/websocket v1.8.15` (`infra-fleet-service/go.mod:6`) mặc định giới hạn đọc một thông điệp là 32768 byte (`read.go`, "By default, the connection has a message read limit of 32768 bytes", đã đọc trong module cache) và khi vượt thì đóng kết nối bằng `StatusMessageTooBig`. Mọi điểm tạo kết nối đều không gọi `SetReadLimit`: `agentwsserver/server.go:132,143` (`websocket.Accept`), `devserveragent/session.go:241` (`websocket.Dial`), `devserveragent/client.go:371` rồi `transport.go:48` `newWSTransport`. Trong khi đó chính mã đặt trần khung là 16 MiB (`devserveragent/frame.go:24`, `MaxMessageSize`). Hậu quả mở rộng: BẤT KỲ phản hồi một khung nào của agent lớn hơn 32 KiB (liệt kê thư mục lớn, `git.diff`, `fs.readFile`, danh sách trạng thái git rất dài) làm rớt agent ~30 giây (agent tự nối lại; mục 1.1). Mỗi mục `fs.readDir` của agent mang cả `path`, `name`, `type`, `size` (`agent/src/relay/fs-agent-extensions.ts:71-91`, `readDirRecursive` còn `stat` từng file), nên 450 mục là đủ vượt 32 KiB. Gateway `devServer.browseDir` (`channels.go:984-1045`) và `files.browseServerDir` (`channels_files.go:46-111`) không giới hạn số mục, và `rpcTimeout` chỉ 8 giây (`channels.go:105`).

## 2. Ngữ nghĩa "local repo" (`dev_server_id` rỗng): hợp lệ hay sai

| Bối cảnh | `dev_server_id` rỗng có hợp lệ? | Căn cứ |
|---|---|---|
| Desktop Electron | KHÔNG áp dụng: desktop không dùng project-service; repo local nằm trong DB cục bộ của app, git chạy bằng IPC trên máy người dùng. Thay đổi ở đây KHÔNG ảnh hưởng desktop | `window.api.repos.*` (nhánh `target.kind === 'local'` trong `addRepoPath`) |
| Desktop trỏ vào một Orca runtime (`environment`) | Có thể hợp lệ nếu runtime đó là máy đa-người-dùng chia sẻ filesystem với git-gateway | không có cấu hình khai báo hiện nay |
| Triển khai Docker web (deploy/dev) | CHỈ hợp lệ cho đường dẫn nằm trong volume `git-gateway-repos` gắn tại `/data/repos` (convention đã ghi trong comment `deploy/dev/docker-compose.yml` trước service `git-gateway-service`: "where any repo NOT bound to a remote dev server ... actually lives on disk"). Mọi đường dẫn khác (như `/Users/...`, `/private/tmp/...`) là SAI | `docker-compose.yml:527-529` (`git-gateway-repos:/data/repos`), `git-gateway-runtime.Dockerfile` dòng 21-26 |
| Chạy git-gateway trực tiếp trên máy dev (không Docker), cùng máy người dùng | Hợp lệ (chia sẻ filesystem) | nhưng KHÔNG có biến môi trường nào khai báo điều này |

Không có cấu hình triển khai nào hiện nay cho biết "git-gateway chia sẻ filesystem với người dùng": `git-gateway-service/internal/config/config.go` chỉ có các địa chỉ service; `deploy/dev/docker-compose.yml` chỉ có volume và `x-go-common-env`. Phải thêm cấu hình tường minh (mục 3.3), nếu không không có cách phân biệt "local hợp lệ" với "lỡ quên host".

## 3. Giải pháp

### 3.1 Nguyên tắc

1. **Mọi repo có một host rõ ràng**: một dev server, hoặc "local có khai báo" (đường dẫn dưới gốc cục bộ đã cấu hình). Không còn trạng thái ngầm "không có gì thì chạy ở đây".
2. **Fail closed, mã lỗi rõ, thử lại được**: không bao giờ đổi chỗ chạy git khi không chắc. Lỗi nêu việc cần làm.
3. **Backend không đoán, chỉ thừa hưởng host đã khai báo.** Client truyền host tường minh (luôn thắng); nếu thiếu thì dùng host đã khai báo trên project (quyết định 2); không có cả hai thì lỗi có mã.
4. **Cộng, không phá**: proto không đổi trường cũ; kênh gateway chỉ thêm trường tuỳ chọn; có cờ cấu hình để lùi lại (mục 3.7).

### 3.2 project-service: chính sách `AddRepo`

Sửa `usecase/add_repo.go` (Execute từ dòng 44). Cần đọc project (`ProjectRepository.Get(ctx, tenantID, id)`, `ports.go:24`; `NewAddRepo` nhận thêm cổng này, một nơi dựng ở `cmd/server/main.go:312`) để lấy `dev_server_id` của project. Ma trận quyết định, theo thứ tự:

| `dev_server_id` của request | `project.dev_server_id` | `ORCA_LOCAL_REPO_ROOT` | Kết quả |
|---|---|---|---|
| không rỗng, không tồn tại | bất kỳ | bất kỳ | `PROJECT_DEV_SERVER_NOT_FOUND` (đã có, dòng 61) |
| không rỗng, tồn tại | bất kỳ | bất kỳ | gắn host tường minh như hiện nay |
| rỗng | không rỗng và tồn tại | bất kỳ | THỪA HƯỞNG: repo lưu `dev_server_id = project.dev_server_id` (ghi vào hàng repo, không để NULL) |
| rỗng | rỗng hoặc không còn tồn tại | rỗng (mặc định) | `PROJECT_REPO_HOST_REQUIRED` (mới, `KindInvalidArgument`) |
| rỗng | rỗng | đặt, `url` tuyệt đối dưới gốc đó (sau `path.Clean`, không `..`, khớp ranh giới thư mục) | cho phép, repo "local khai báo" |
| rỗng | rỗng | đặt, `url` ngoài gốc hoặc là URL git từ xa | `PROJECT_REPO_HOST_REQUIRED` |

Ghi `dev_server_id` hiệu lực vào hàng repo (không chỉ suy ra lúc đọc) để `repo.list` phản ánh đúng sự thật.

#### 3.2.0 Tạo project gắn dev server (không cần repo ngay)

`CreateProject` hiện đòi `repo_path` khi có `dev_server_id` (`create_project.go:78-80`, `PROJECT_REPO_PATH_REQUIRED`). Nới (cộng, tương thích): cho phép `dev_server_id` mà không có `repo_path` = "project gắn host, thêm repo sau": vẫn kiểm host tồn tại và tới được (dòng 56-77), bỏ qua `fs.checkPath` và không tạo repo. Có `repo_path` thì hành vi cũ. Frontend dùng đường này ở bước tạo/lấy project mặc định theo host (SOL-FE-E2E-004 mục 3.2).

#### 3.2.3 `GetRepo` trả host hiệu lực

`usecase/get_repo.go:63` trả `DevServerID: repo.DevServerID`. Đổi thành hiệu lực: repo, nếu rỗng thì `project.dev_server_id` (để repo cũ có project gắn host vẫn chạy, không cần migration). `ListRepos` không đổi (frontend đã tự thừa hưởng ở `store/slices/repos.ts:1317`, nay khớp với backend). Nếu host hiệu lực là host cũ đã ngắt (ca `local-mac`), git-gateway trả `GITGATEWAY_DEV_SERVER_UNREACHABLE` rõ ràng thay vì `chdir`.

Thông điệp: `PROJECT_REPO_HOST_REQUIRED: choose the host (dev server) where this repository lives; repos without a host can only live under <root>`. Không đưa đường dẫn người dùng vào log ở mức info.

Cờ rollout `PROJECT_REPO_HOST_POLICY=enforce|warn` (mặc định `enforce`; `warn` = ghi log có cấu trúc và cho qua như cũ). Đây là cần gạt rollback không cần đổi nhị phân, dùng trong giai đoạn chuyển tiếp khi còn tab frontend cũ (mục 6).

`Rebind` (`usecase/rebind_repo_dev_server.go`) giữ nguyên ngữ nghĩa "gắn lại"; xem điểm cần cải thiện ở mục 5.

#### 3.2.1 Kiểm tra khi thêm (tuỳ chọn, `PROJECT_ADD_REPO_VERIFY_HOST`)

Theo tiền lệ `CreateProject` (`create_project.go:60-100`: `IsReachable` fail-closed, `fs.checkPath` qua `DevServerRelay`): khi có `dev_server_id` và `url` là đường dẫn tuyệt đối, kiểm host tới được (`PROJECT_DEV_SERVER_UNREACHABLE`, `KindFailedPrecondition`, đã có mã này ở `create_project.go`) và thư mục tồn tại (`PROJECT_REPO_PATH_NOT_FOUND`, đã có). Mặc định TẮT ở giai đoạn 1 để không đổi hành vi e2e hiện có (các suite dùng dev server stub), bật sau khi e2e xanh (quyết định D5).

#### 3.2.2 Kênh gateway

- `repo.add` (`channels_repo_ssh_status_workspace.go:112-137`): đã chuyển `devServerId` (dòng 126-129). Chỉ cập nhật chú thích sai "empty = local repo" và để lỗi gRPC đi qua nguyên vẹn (đã đúng, `err` trả thẳng).
- REST `handleAddRepo` (`project_routes.go:247-268`): thêm `dev_server_id` tuỳ chọn vào `addRepoRequestBody` và chuyển vào `AddRepoRequest.DevServerId`.
- `repo.clone`/`repo.create` KHÔNG tự đăng ký repo (chỉ chạy git). Không đổi trong đợt này; ghi nhận ở mục 9 (đề xuất một kênh `repo.cloneAndAdd` nguyên tử cho đợt sau, để tránh trạng thái "đã clone nhưng chưa đăng ký").

### 3.3 Cấu hình "local khai báo"

Một biến duy nhất, dùng chung cho hai service, đặt ở `x-go-common-env` của `deploy/dev/docker-compose.yml` để không lệch:

- `ORCA_LOCAL_REPO_ROOT` (mặc định mã: rỗng = KHÔNG cho phép repo local). Compose dev đặt `${ORCA_LOCAL_REPO_ROOT:-/data/repos}` vì volume `git-gateway-repos` đã tồn tại và được ghi là convention (nếu người dùng chọn bỏ hẳn local, đặt rỗng; quyết định D1).
- Một hàm thuần dùng chung `IsUnderLocalRepoRoot(root, path)` (chuẩn hoá bằng `path.Clean`, bắt buộc tuyệt đối, khớp theo ranh giới thư mục để `/data/reposX` không khớp `/data/repos`), viết ở từng service (mỗi service tự đóng gói theo quy ước repo, KHÔNG import chéo `internal/`) và test cùng một bảng ca. Đường dẫn Windows (`C:\...`) không bao giờ nằm dưới gốc POSIX.
- git-gateway kiểm lại cùng quy tắc ở thời điểm chạy (phòng thủ chiều sâu: DB có thể chứa hàng cũ).

### 3.4 git-gateway: một điểm quyết định host, không fallback im lặng

Thay logic rải rác bằng MỘT hàm (đặt trong tệp mới `usecase/repo_host_dispatch.go`, tên theo nghiệp vụ, không `helpers/utils`), `ports.go:673-717` gọi vào nó:

```
resolveRepoHost(ctx, reach, policy, repo) -> (ctx, target, repoPath, error)
  repo.DevServerID != "":
     reach.IsReachable lỗi    -> GITGATEWAY_HOST_CHECK_FAILED   (Unavailable, thử lại được)
     !reachable               -> GITGATEWAY_DEV_SERVER_UNREACHABLE (Unavailable, thử lại được)
     reachable                -> relay (ctx mang DevServerID như hiện nay)
  repo.DevServerID == "":
     policy.AllowsLocal(repo.URL) -> local
     ngược lại                -> GITGATEWAY_REPO_HOST_UNBOUND   (FailedPrecondition)
```

Áp dụng đồng nhất cho:
1. `dispatchExecutorForRepo` và `dispatchFilesystemExecutorForRepo` (13 điểm gọi, danh sách ở mục 8).
2. `Clone` và `InitRepo`: bỏ `executor := uc.local` khi `!reachable`; trả `GITGATEWAY_DEV_SERVER_UNREACHABLE`. Giữ `GITGATEWAY_MISSING_DEV_SERVER_ID` cho `dev_server_id` rỗng (đường local của clone/init chỉ hợp lệ khi `ORCA_LOCAL_REPO_ROOT` đặt và đích dưới gốc đó).
3. `ConnectionResolver.ResolveConnection`/`resolveLocal` (cổng của 48 điểm gọi theo worktree): khi `!resp.GetConnected()` và repo có host mà `!reachable` thì TRẢ LỖI thay vì `Connected:false`; khi `GetRepo` lỗi (hai nhánh "degrades to Path-only") thì trả `GITGATEWAY_HOST_CHECK_FAILED` thay vì rơi về local; khi repo không có host thì áp `policy.AllowsLocal(path)`.

Giữ lỗi có mã qua các usecase: 13 usecase hiện bọc lỗi điều phối thành `*_RESOLVE_FAILED` (mục 1.1). Thêm helper `hostResolutionError(err, code, msg)` trả nguyên `*apperrors.AppError` nếu `errors.As` ra một trong ba mã trên, ngược lại mới bọc như cũ. Các usecase đổi một dòng mỗi cái.

Nhận biết agent rớt giữa chừng (mẫu 30 giây có thể cũ): ở điểm `relay` chung của `RelayExecutor` (`adapter/grpcclient/relay_executor.go`, đọc `DevServerIDFromContext` ở dòng 97) và ở `create_worktree.go:156`, nếu lỗi relay mang `INFRA_DEV_SERVER_NOT_CONNECTED` (`infra-fleet .../relay_by_dev_server.go:56`, `FailedPrecondition`) thì đổi thành `GITGATEWAY_DEV_SERVER_UNREACHABLE` thay vì `WORKTREE_CREATE_FAILED` kèm `chdir` hoặc lỗi chung. Như vậy "dev server ngắt trong lúc tạo worktree" ra lỗi rõ chứ không phải lỗi git.

`policy` là một giá trị cấu hình (`LocalRepoPolicy{Root string, Mode}`) dựng trong `cmd/server/main.go` và truyền vào `DevServerReachability`-cạnh-nó (xem TASK-V2-029 về cách thay `reachability` bằng một `RepoHostDispatcher` để không thêm tham số vào 13 constructor).

Chế độ `GIT_GATEWAY_HOST_DISPATCH_MODE=strict|legacy` (mặc định `strict`): `legacy` giữ đúng hành vi cũ và ghi log cảnh báo mỗi lần rơi về local; chỉ dùng để lùi lại khi cần (mục 6.4).

### 3.5 Mã lỗi (bảng thống nhất)

| Mã | Service | Kind (gRPC) | Khi nào | Hành vi UI mong đợi |
|---|---|---|---|---|
| `PROJECT_REPO_HOST_REQUIRED` | project-service | InvalidArgument | `repo.add` không có host và không thuộc gốc local | chặn, mời chọn host (FE đã chặn từ trước; mã này để bắt client cũ/kênh tắt) |
| `PROJECT_DEV_SERVER_NOT_FOUND` | project-service | InvalidArgument | host không tồn tại (đã có) | báo host không còn |
| `PROJECT_DEV_SERVER_UNREACHABLE` | project-service | FailedPrecondition | (khi bật verify) host chưa kết nối (đã có ở `CreateProject`) | báo host đang offline, thử lại |
| `GITGATEWAY_REPO_HOST_UNBOUND` | git-gateway | FailedPrecondition | repo không có host, ngoài gốc local | "Repo này chưa gắn host", nút "Chọn host" (rebind) |
| `GITGATEWAY_DEV_SERVER_UNREACHABLE` | git-gateway | Unavailable | host có nhưng chưa kết nối / agent rớt | "Host `<tên>` đang offline hoặc vừa ngắt, thử lại sau"; có nút Retry |
| `GITGATEWAY_HOST_CHECK_FAILED` | git-gateway | Unavailable | không xác định được host (lỗi đọc sức khoẻ hoặc `GetRepo`) | báo tạm thời, thử lại |
| `INFRA_DEV_SERVER_NOT_CONNECTED` | infra-fleet | FailedPrecondition | đã có (`relay_by_dev_server.go:56`) | frontend nhận ra cùng nhóm "host offline" |
| `DEVSERVER_BROWSE_TRUNCATED` (không phải lỗi, cờ `truncated` ở kết quả) | api-gateway | — | liệt kê vượt trần | UI hiện "Hiển thị N/M mục" |
| `INFRA_AGENT_RESPONSE_TOO_LARGE` | infra-fleet | ResourceExhausted | một khung agent vượt `MaxMessageSize` | báo thu hẹp thư mục, không rớt kết nối |

Tên mã theo tiền tố service như quy ước hiện có (`GITGATEWAY_*`, `PROJECT_*`, `INFRA_*`). Phía frontend nhận dạng bằng chuỗi con (như `workspace-create-error-format.ts` đang làm), nên cách viết `REPO_HOST_UNBOUND`/`DEV_SERVER_UNREACHABLE` rút gọn vẫn khớp.

### 3.6 Duyệt thư mục: giới hạn kích thước và không rớt agent

1. **Gốc (infra-fleet):** gọi `conn.SetReadLimit(MaxMessageSize + HeaderLength)` trong `newWSTransport` (`transport.go:48`) và ở hai chỗ đọc trước khi bọc transport: bắt tay inbound (`agentwsserver/server.go:167`, `conn.Read(hctx)` trong `handleConnection`) và `session.go:308`. Test: phản hồi giả 200 KiB không làm rớt phiên (đỏ trước, xanh sau). Cùng đợt thêm log rõ khi khung vượt trần 16 MiB (hiện chỉ lỗi `frame payload too large`, `frame.go:73-74`).
2. **Giới hạn nghiệp vụ (gateway + agent), cộng không phá:** `devServer.browseDir` và `files.browseServerDir` nhận thêm tham số tuỳ chọn `limit` (mặc định 500, tối đa 2000), `directoriesOnly` (mặc định `false`), `query` (lọc chứa chuỗi, không phân biệt hoa thường); kết quả thêm `truncated: boolean` và `totalEntries`. Gateway truyền xuống agent `fs.readDir` các tham số mới (`maxEntries`, `directoriesOnly`, `nameContains`, `includeSize:false`) để agent không `stat` từng file; agent cũ bỏ qua tham số lạ nên gateway tự cắt sau khi nhận (an toàn nhờ bước 1). Thứ tự: thư mục trước, tên theo `localeCompare`, như `fs-handler.ts:166-180`. Cập nhật `specs/agent/api/agent-rpc-catalog-git-fs.md` dòng 92 (tham số mới, tương thích ngược).
3. **Thời gian:** `rpcTimeout` 8 giây (`channels.go:105`) giữ nguyên; với giới hạn mục và bỏ `stat` thì liệt kê thư mục 2000 mục phải xong trong ngân sách này; test ghi lại thời gian.
4. **Lỗi hiển thị được:** nếu agent rớt hoặc `INFRA_DEV_SERVER_NOT_CONNECTED`, kênh trả lỗi có mã (đã đi qua nguyên vẹn); phần UI ở TASK-FE-E2E-020.

Điểm phụ (ghi nhận, KHÔNG sửa ở đây): `devServer.browseDir` coi `~` là `/` (comment `channels.go:972-978`, "Home-directory resolution gap"); `fs.readDir` Phần A không báo symlink nên thư mục symlink (`/tmp`, `/var`) hiện như file.

### 3.7 Quan sát

- Log có cấu trúc `repo_host_dispatch` ở điểm quyết định mới (git-gateway) với `target=relay|local|rejected`, `reason=unbound|unreachable|check_failed|local_root`, `mode=strict|legacy`, KHÔNG chứa tenant, đường dẫn đầy đủ hay tên người dùng (chỉ id repo rút gọn nếu cần). Ở chế độ `legacy` ghi mức WARN.
- Log `project_repo_host_policy` ở `AddRepo` (`outcome=accepted|accepted_local|rejected|warned`).
- Số repo mồ côi: lệnh quản trị chỉ đọc (mục 4) và một dòng log khi project-service khởi động đếm `dev_server_id IS NULL` AND không dưới gốc local (một truy vấn, cả hai dialect). Chưa có `/metrics` ở git-gateway và project-service (chỉ health: `cmd/server/main.go:303`, `97`); bộ đếm Prometheus theo mẫu `request-service/internal/adapter/metrics` là mục tuỳ chọn của TASK-V2-034.

### 3.8 Proto và tương thích

- KHÔNG sửa proto cho `AddRepoRequest`/`Repo` (đã có `dev_server_id`). Nếu verify host (3.2.1) cần thêm lỗi thì chỉ là mã lỗi mới, không đổi message.
- Kênh gateway: chỉ thêm trường tuỳ chọn vào `devServer.browseDir`/`files.browseServerDir` và `truncated`/`totalEntries` vào kết quả (JSON cộng thêm); `channel_inventory.json` golden (`wscompat/testdata/channel_inventory.json`) không đổi vì không thêm kênh.
- Tương thích ngược: desktop không dùng project-service nên không đổi; frontend web cũ gửi `repo.add` không host sẽ nhận `PROJECT_REPO_HOST_REQUIRED` sau khi bật `enforce` (thông điệp nêu cần tải lại trang). Vì vậy thứ tự triển khai ở mục 6 đặt frontend trước.
- `buf lint` và `buf breaking` phải xanh (không đổi `.proto`, nhưng chạy để chắc).

## 4. Dữ liệu cũ (repo có `dev_server_id` rỗng)

- **Phát hiện (chỉ đọc):** `project.repos WHERE dev_server_id IS NULL` (Postgres: schema `project`; MySQL: bảng `repos`, `CHAR(36)`; cả hai dialect đã có cột từ migration `0017`). Qua gateway không cần SQL: `project.list` rồi `repo.list {projectId}` và lọc `devServerId == ""` (đúng cách đã chứng minh ở TASK-FE-E2E-015). Cũng phân loại: dưới `ORCA_LOCAL_REPO_ROOT` (hợp lệ, không cần làm gì) hay ngoài (cần gắn host).
- **Gắn lại an toàn:** dùng `repo.rebindDevServer {repoId,newDevServerId}` (`channels_repo_ssh_status_workspace.go:140-158`, `RebindRepoDevServer`). Lưu ý các ràng buộc ở `rebind_repo_dev_server.go:62-97`: chặn khi project có workflow/task đang chạy (`PROJECT_HAS_ACTIVE_WORKFLOWS`), kiểm host tồn tại nhưng KHÔNG kiểm tới được và KHÔNG kiểm đường dẫn có trên host mới (cải thiện ở mục 5).
- **Backfill/migration: KHÔNG tự đoán.** Không viết migration SQL gán hàng loạt, vì host đúng là thông tin chỉ người dùng biết (cùng một đường dẫn `/Users/x/repo` có thể có trên nhiều máy). Thay vào đó là công cụ quản trị `report_unbound_repos.py` (đặt cạnh `report_orphan_requests.py`, cùng cách đăng nhập từ `deploy/dev/.env`): mặc định CHỈ BÁO CÁO (dry-run); `--apply` bắt buộc kèm ánh xạ tường minh `--repo <id>=<tên-hoặc-id host>` (hoặc `--host <tên> --repo <id>...`); in kế hoạch trước, từ chối khi host đang `disconnected`, không bao giờ suy ra host; gợi ý (không tự áp) từ `devServerId` cũ của project nếu nó đang kết nối và thư mục có thật. Một repo mỗi lần gọi `repo.rebindDevServer`, dừng ở lỗi đầu tiên có thể phục hồi được, in kết quả từng repo.
- **Project `My Repos` có `dev_server_id` riêng** (`local-mac`, đang `disconnected`, khác `dev-local-mac`): với quyết định 2 đây là host mặc định của repo thiếu host trong project đó; repo thêm từ hộp thoại luôn mang host tường minh nên không bị ảnh hưởng. Không sửa cột, không migration.

## 5. Cải thiện nhỏ cho `RebindRepoDevServer` (đề xuất, TASK-V2-031)

Vì gắn lại là con đường duy nhất cứu dữ liệu cũ: (a) `newDevServerId` rỗng (tháo host) cho repo không thuộc gốc local nên bị từ chối bằng `PROJECT_REPO_HOST_REQUIRED` cùng quy tắc `AddRepo`; (b) khi bật verify, kiểm host mới tới được và đường dẫn có thật trên host mới (tránh gắn sai); (c) giữ guard "đang chạy" nhưng thông điệp nêu rõ vì sao chặn. Mặc định hành vi cũ nếu cờ tắt.

## 6. Lộ trình triển khai & kiểm chứng

### 6.1 Thứ tự PR/commit (mỗi PR nhỏ, độc lập, lùi được)

| # | PR | Người sửa | Cần redeploy | Ghi chú |
|---|---|---|---|---|
| 1 | TASK-V2-030 (đọc giới hạn websocket, bước 1 của 3.6) | backend | infra-fleet-service | an toàn nhất, không đổi hành vi nghiệp vụ; làm trước vì nó khiến dev server rớt ngẫu nhiên |
| 2 | TASK-FE-E2E-016/017/018 (frontend truyền `devServerId`, chọn host) | frontend | frontend (`BUILD_FRONTEND=1`) | BỎ lỗi của người dùng ngay cả khi backend chưa đổi (gateway đã nhận `devServerId`) |
| 3 | TASK-V2-028 (`AddRepo` chính sách) ở chế độ `warn` | backend | project-service, api-gateway | chỉ ghi log; thu thập client nào còn gửi thiếu host |
| 4 | TASK-V2-031 (công cụ báo cáo + gắn lại) | backend | không | chạy báo cáo, người dùng quyết dữ liệu cũ |
| 5 | TASK-V2-029 (git-gateway `strict`) | backend | git-gateway-service | chỉ bật sau khi báo cáo cho thấy 0 repo mồ côi ngoài gốc local, hoặc người dùng đã chấp nhận |
| 6 | TASK-V2-028 chuyển `warn` thành `enforce` | backend (cấu hình) | project-service (khởi động lại) | sau ≥1 ngày không còn log `warned` |
| 7 | TASK-V2-030 phần giới hạn mục + TASK-FE-E2E-019/020 (hiển thị/đổi host, duyệt thư mục) | cả hai | api-gateway, (agent), frontend | |
| 8 | TASK-V2-032/033/034 (test hai dialect, e2e, quan sát và tài liệu) | backend/QA | — | đóng đợt |

### 6.2 Redeploy (dùng `deploy/dev/scripts/redeploy-local.sh`, KHÔNG chạy trong bước viết tài liệu)

`./deploy/dev/scripts/redeploy-local.sh infra-fleet-service` (PR 1); `BUILD_FRONTEND=1 ./deploy/dev/scripts/redeploy-local.sh frontend` (PR 2); `./deploy/dev/scripts/redeploy-local.sh project-service api-gateway` (PR 3, 6); `./deploy/dev/scripts/redeploy-local.sh git-gateway-service` (PR 5). Agent trên dev server: build lại và khởi động lại `deploy/dev/scripts/start-local-agent.sh` khi đổi `fs.readDir` (PR 7). Biến môi trường mới đi qua `x-go-common-env` hoặc khối service tương ứng trong `docker-compose.yml` (không cần migration DB).

### 6.3 Kiểm chứng (không động tới dự án thật của người dùng)

1. Bước nền (trước khi sửa, chỉ đọc): chạy `python3 tests/stages/e2e/orca-flows/report_unbound_repos.py` (dry-run) và ghi lại số repo mồ côi; KHÔNG `--apply`.
2. Sau PR 2: `python3 tests/stages/e2e/orca-flows/run_ui_flow.py --repro add-project-worktree` phải chuyển từ ĐỎ (`BUG REPRODUCED`) sang XANH: `repo.list` trả `devServerId` = id của `dev-local-mac`, `worktree test-02` có trong danh sách và trên đĩa, spec vẫn dọn sạch. Spec dùng repo tạm `/private/tmp/e2e-ui-repro-*` (không phải repo của người dùng), tự gỡ khỏi Orca ở `finally`. Cần dev server `dev-local-mac` đang `connected` (`start-local-agent.sh`).
3. Sau PR 5: kịch bản mở rộng của TASK-V2-033 (ngắt dev server khi đang tạo worktree thì UI báo `GITGATEWAY_DEV_SERVER_UNREACHABLE`, không `chdir`; repo thêm qua kênh tắt không `devServerId` thì `PROJECT_REPO_HOST_REQUIRED`), chạy trên repo và project tạm của chính kịch bản, dọn trong `finally`; ngắt dev server bằng cách dừng tiến trình agent do chính kịch bản khởi chạy trên cổng riêng (không dừng `dev-local-mac` của người dùng) hoặc, nếu không có, chỉ chạy bản ghi sẵn ở lớp Go e2e với agent giả.
4. Không hồi quy: `python3 tests/stages/e2e/orca-flows/run_ui_flow.py --real all` (Claude Code thật, ~20 phút, 6/6 như README tasks FE) và `run_acceptance.py` (103/110 PASS như trước), `go test ./...` ở `project-service`, `git-gateway-service`, `api-gateway`, `infra-fleet-service`; tích hợp Postgres và MySQL ở `project-service` (`-tags integration`). Lưu ý `--real` dùng project/repo tạm (`real-repo-fixture.ts`, `live-project-fixture.ts`), không đụng repo `codex` hay các project có sẵn.
5. Dọn dữ liệu: `report_orphan_requests.py` và `report_unbound_repos.py` phải báo 0 mục `e2e-ui-repro-*`/`e2e-ui-*` còn lại.

### 6.4 Rollback

- `GIT_GATEWAY_HOST_DISPATCH_MODE=legacy` + khởi động lại git-gateway: về hành vi cũ ngay (cảnh báo log). Không cần đổi nhị phân.
- `PROJECT_REPO_HOST_POLICY=warn`: `AddRepo` nhận lại repo không host (như cũ).
- `ORCA_LOCAL_REPO_ROOT=/`: mở hoàn toàn local (chỉ để khẩn cấp; ghi chú rủi ro).
- Không có migration dữ liệu nên không có rollback DB. Mọi gắn lại (`rebind`) là thao tác có chủ đích của người dùng và đảo được bằng một `rebind` khác.
- Nếu đổi `SetReadLimit` gây vấn đề bộ nhớ: đặt trần nhỏ hơn (ví dụ 4 MiB, cùng cỡ `MaxCallRecvMsgSize` mặc định của gRPC) bằng hằng số; không cần quay lại 32 KiB.

## 7. Quyết định đã chốt và hệ quả

| # | Quyết định | Hệ quả kỹ thuật |
|---|---|---|
| D1 | Không có repo local trong Docker/web, trừ cờ `ORCA_LOCAL_REPO_ROOT` | mặc định mã tắt; compose dev có thể đặt `/data/repos` (volume đã có); đặt rỗng để tắt hẳn |
| D2 | Repo cũ không tự migrate | công cụ dry-run `--apply` tường minh (mục 4); host hiệu lực qua project (3.2.3) cứu phần lớn mà không đổi dữ liệu |
| D3 | Chọn sẵn dev server duy nhất đang kết nối | phía frontend (SOL-FE-E2E-004) |
| D4 | Repo thừa hưởng host của project khi request không nêu | 3.2 và 3.2.3; rủi ro: project mang host cũ đã ngắt gây lỗi `UNREACHABLE` rõ (không còn `chdir`); host tường minh luôn thắng |
| D5 | Kiểm host tới được và đường dẫn có thật khi thêm repo (`PROJECT_ADD_REPO_VERIFY_HOST`) | mặc định tắt giai đoạn 1, bật sau khi e2e xanh |
| D6 | Bật `strict` ở git-gateway sau khi báo cáo repo mồ côi | `GIT_GATEWAY_HOST_DISPATCH_MODE`, lùi bằng `legacy` |

## 8. Phạm vi ảnh hưởng (codegraph, tên chính xác)

Công cụ thay `gitnexus_impact` như `IMPACT-CODEGRAPH.md`; CHỈ ĐỌC.

| Symbol | Kết quả | Rủi ro |
|---|---|---|
| `dispatchExecutorForRepo` | 13 điểm gọi sản xuất (`codegraph callers` + grep `internal/usecase`): `base_ref_default`, `check_hooks`, `create_worktree`, `detect_worktrees`, `get_remote_url`, `prefetch_create_base`, `read_issue_command`, `resolve_mr_base`, `resolve_pr_base`, `scan_setup_script_imports`, `search_refs`, `write_issue_command` (12, `GitExecutor`) + `read_ephemeral_vm_recipes` qua `dispatchFilesystemExecutorForRepo` | HIGH ở tầng git-gateway (đổi ngữ nghĩa lỗi của 13 usecase); thấp ở tầng khác vì lỗi vẫn là gRPC có mã |
| `DevServerReachability` (git-gateway `usecase/ports.go:335`) | `codegraph impact` ra 141 ký hiệu, gồm `NewClone`, `NewInitRepo`, 13 usecase và test; task-service có bản riêng `adapter/grpcclient/dev_server_reachability.go` (không đổi) | MEDIUM: nếu thay bằng `RepoHostDispatcher` thì sửa 15 constructor và test; cách rẻ hơn là giữ interface và thêm policy trong chính hàm dispatch |
| `ConnectionResolver.ResolveConnection`/`resolveLocal` | cổng chung của 48 điểm gọi `dispatchExecutor`/`dispatchFilesystemExecutor` (grep) | HIGH: đổi hành vi mọi thao tác theo worktree với repo không host hoặc host không tới được; cần chạy cả bộ `dispatch_test.go` |
| `AddRepo` (project-service) | `NewAddRepo` 1 nơi dựng (`cmd/server/main.go:312`) + gRPC `server.go:372` + REST gateway | MEDIUM: thêm lỗi từ chối; client cũ không gửi host bị chặn |
| `RebindRepoDevServer` | 1 nơi dựng; kênh `repo.rebindDevServer`; UI `ProjectDevServerSection` | LOW |
| `newWSTransport`, `websocket.Accept/Dial` (infra-fleet) | 3 đường tạo kết nối + test `agentwsserver`, `devserveragent` | LOW (chỉ nâng trần đọc) nhưng đổi hành vi chống khung lớn: cần test khung 16 MiB+1 |
| Test sẽ phải đổi | `usecase/detect_worktrees_test.go:12,53`, `dispatch_test.go:1873`, `read_ephemeral_vm_recipes_test.go:46`, `grpcclient_test.go:289,318` (đang khẳng định hành vi fallback về local) | cần sửa theo hành vi mới, KHÔNG xoá |

Chưa kiểm được bằng công cụ: lời gọi qua gRPC giữa service (đồ thị dừng ở `*_grpc.pb.go`); task-service tạo worktree qua `worktree_provisioner.go` gọi `CreateWorktree` của git-gateway nên cũng nhận mã lỗi mới ở thực thi task. Cần xác nhận nó xử lý `FailedPrecondition`/`Unavailable` từ git-gateway như lỗi có thể thử lại hoặc lỗi cuối, không lặp vô hạn (xem TASK-V2-029 mục kiểm thử).

## 9. Phương án đã cân nhắc và loại

| Phương án | Lý do loại |
|---|---|
| Chỉ sửa frontend | Không chặn kênh tắt, REST, script, tab cũ; không sửa fallback nguy hiểm khi host rớt; để nguyên cửa sổ 30 giây |
| Thừa hưởng host project mà KHÔNG kiểm tồn tại/kết nối | (đã loại; quyết định D4 chỉ thừa hưởng khi host tồn tại, và git-gateway báo `UNREACHABLE` rõ nếu offline) |
| Mount thư mục người dùng vào container git-gateway để "local" hoạt động | Phá mô hình bảo mật (container thấy toàn bộ filesystem máy), không áp dụng cho nhiều dev server, vẫn sai trên máy khác |
| Migration SQL gán hàng loạt `dev_server_id` | Không có thông tin để chọn đúng host; sai thì worktree tạo nhầm máy; thay bằng công cụ dry-run có xác nhận |
| Giữ fallback "unreachable thì local" nhưng chỉ ghi log | Đây chính là nguồn của lỗi `chdir` gây hiểu nhầm; fail closed rẻ hơn nhiều |
| Thêm cờ `isLocal` vào bảng repo | Thêm migration hai dialect cho thông tin suy ra được từ (host rỗng + dưới gốc local); chưa cần |
| Kênh nguyên tử `repo.cloneAndAdd` ngay đợt này | Hợp lý nhưng mở rộng phạm vi (hợp đồng mới, e2e mới); ghi cho đợt sau, hiện sửa bằng truyền `devServerId` ở bước `repo.add` |
| Giảm `fs.readDir` bằng cách chỉ nâng giới hạn websocket | Đủ để không rớt, nhưng thư mục 20 000 mục vẫn chậm quá 8 giây và tạo JSON hàng MB; cần trần mục |

## 10. Chưa xác minh (nói thẳng)

- Chưa chạy bất kỳ test hay stack nào ở bước viết tài liệu này (chỉ đọc mã). Các số `file:dòng` đúng tại thời điểm đọc; kiểm lại bằng `codegraph` trước khi sửa.
- "Mọi khung > 32 KiB làm rớt agent" suy ra từ mã (không có `SetReadLimit`, thư viện mặc định 32768, log khớp 32769); chưa tái hiện riêng cho `git.diff`/`fs.readFile`. TASK-V2-030 có test tái hiện.
- Chưa xác nhận thanh sidebar/repo của tab cũ có bị `PROJECT_REPO_HOST_REQUIRED` làm treo UI hay không (cần kiểm ở e2e PR 6).
- Chưa kiểm cách task-service phản ứng với mã lỗi mới của git-gateway khi tạo worktree cho task (mục 8).
- Chưa xác minh `agent` thật (bản đang chạy trên `dev-local-mac`) đã có `fs.readDir` Phần A hay Phần B; gateway đang dùng tham số `{path, depth}` của Phần A (`channels.go:1005`).
- `docs/reference/git-compatibility.md` mà AGENTS.md dẫn tới không có trong cây; đợt này KHÔNG thêm lệnh git mới (chỉ đổi nơi chạy/quyết định), nên không cần `GitCapabilityCache`.
