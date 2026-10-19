# TASK-V2-035: Repo luôn gắn dev server, không fallback im lặng về local, công cụ gắn lại dữ liệu cũ, duyệt thư mục lớn không rớt agent (triển khai theo quyết định của người dùng)

**From:** yêu cầu của người dùng "bước tạo project cho phép chọn dev server, project được tạo gắn với dev server đó, không block việc tạo worktree" · **Resolves:** [TASK-FE-E2E-015](../../../../../frontend/bugs/e2e-acceptance-v1/tasks/TASK-FE-E2E-015-web-add-project-drops-dev-server.md) (`WORKTREE_CREATE_FAILED ... chdir <repo>: no such file or directory`) phía backend; hiện thực phần backend của [SOL-V2-002](../solutions/SOL-V2-002-repo-host-binding-and-no-silent-local.md), gồm TASK-V2-028, 029, 030 (bước đọc websocket + trần danh sách) và 031 (công cụ gắn lại)
**Priority:** P0 · **Service/Area:** project-service, git-gateway-service, infra-fleet-service, api-gateway (wscompat + REST), deploy/dev
**Files:**
- project-service: `internal/usecase/repo_host_policy.go` (mới: `RepoHostPolicy`, `PROJECT_REPO_HOST_REQUIRED`, `ProjectDirectory`), `add_repo.go`, `create_project.go`, `rebind_repo_dev_server.go`, `import_nested.go`, `internal/config/config.go`, `cmd/server/main.go` (+ test: `add_repo_host_binding_test.go`, `repo_host_policy_test.go`, `config_test.go`; test cũ chỉnh theo chữ ký mới)
- git-gateway-service: `internal/usecase/repo_host_dispatch.go` (mới: `RepoHostRouting`, `RepoHostRouter`, `RouteRepoHost`, `HostDispatchError`, 3 mã lỗi), `ports.go` (`dispatchExecutorForRepo`, `dispatchFilesystemExecutorForRepo`), `clone.go`, `init_repo.go`, 13 usecase theo repo (chỉ đổi kiểu tham số `RepoHostRouting`), `internal/adapter/grpcclient/resolver.go`, `relay_executor.go`, `internal/adapter/grpc/host_dispatch_status.go` (mới) + `server.go` (`toStatus`), `internal/config/config.go`, `cmd/server/main.go` (+ test: `dispatch_test.go`, `grpcclient_test.go`, `relay_executor_dev_server_dropped_test.go`, `host_dispatch_status_test.go`, `config_test.go`)
- infra-fleet-service: `internal/adapter/devserveragent/{frame.go,transport.go,session.go}`, `internal/adapter/agentwsserver/server.go` (+ test: `large_response_test.go`, `large_message_test.go`)
- api-gateway: `wscompat/channels_browse_dir_limit.go` (mới), `channels.go` (`devServer.browseDir`), `channels_files.go` (`files.browseServerDir`), `channels_repo_ssh_status_workspace.go` (chỉ chú thích), `httpgateway/project_routes.go` (`dev_server_id` tuỳ chọn ở `POST /v1/projects/{id}/repos`) (+ test: `channels_browse_dir_limit_test.go`, `project_routes_test.go`)
- deploy/dev: `docker-compose.yml` (`x-go-common-env`: `ORCA_ALLOW_LOCAL_REPOS`), `scripts/rebind-unbound-repos.sh` + `scripts/rebind_unbound_repos.py` (mới)
**Depends on:** không (frontend truyền `devServerId` ở TASK-FE-E2E-016 đến 018, làm song song)
**Status:** `[x]` XONG và đã kiểm live (2026-10-11): project-service giải quyết host (request > project > `PROJECT_REPO_HOST_REQUIRED`), git-gateway không fallback im lặng (`GITGATEWAY_REPO_HOST_UNBOUND`, `GITGATEWAY_DEV_SERVER_UNREACHABLE`, `GITGATEWAY_HOST_CHECK_FAILED`), cờ `ORCA_ALLOW_LOCAL_REPOS` mặc định tắt, `SetReadLimit` 16 MiB + browse có giới hạn, công cụ `rebind-unbound-repos.sh` (dry-run). Kiểm live (2026-10-11): `run_ui_flow.py --repro add-project-worktree` XANH (worktree test-02 tạo được, repo có devServerId = dev-local-mac); `--real all` 6/6 XANH (21.8 phút); backend `--strict --ai-stub` 114 PASS, 0 FAIL.

---

## Quyết định đã chốt (người dùng) và cách hiện thực

1. **Repo luôn gắn một dev server.** project-service giải quyết host khi tạo repo theo thứ tự: `devServerId` trong request, rồi dev server của project, rồi lỗi `PROJECT_REPO_HOST_REQUIRED` (InvalidArgument) nếu triển khai không cho phép repo local. Host nào cũng được kiểm với infra-fleet (tồn tại, theo tenant; `PROJECT_DEV_SERVER_NOT_FOUND`); project đọc bằng `ProjectRepository.Get(tenant, id)` nên không đọc project tenant khác. Áp cho `AddRepo`, `CreateProject` (có `repo_path` mà không có host thì lỗi, trước đây repo bị bỏ im lặng), `ImportNested` (có chọn repo mà không có host) và `RebindRepoDevServer` (không cho tháo host về rỗng). `SetupExistingFolder` đã đòi `dev_server_id`; `repo.clone`/`repo.create` đã đòi `devServerId` ở git-gateway.
2. **"Repo local" chỉ khi khai báo.** Cờ `ORCA_ALLOW_LOCAL_REPOS` (mặc định tắt) đọc ở project-service và git-gateway-service, đặt một lần ở `x-go-common-env` của compose dev (`${ORCA_ALLOW_LOCAL_REPOS:-false}`). Bật = hành vi cũ cho repo không gắn dev server. Dữ liệu cũ không tự migrate.
3. **git-gateway không fallback im lặng.** Một điểm quyết định `RouteRepoHost`: repo có dev server và tới được thì relay; có dev server mà không tới được thì `GITGATEWAY_DEV_SERVER_UNREACHABLE` (Unavailable, thử lại được, thông điệp nêu id dev server), kể cả khi cờ bật; không đọc được mẫu sức khoẻ hoặc không đọc được repo thì `GITGATEWAY_HOST_CHECK_FAILED` (Unavailable); không có dev server thì local nếu cờ bật, ngược lại `GITGATEWAY_REPO_HOST_UNBOUND` (FailedPrecondition, gợi ý `repo.rebindDevServer`). Áp cho `dispatchExecutorForRepo`, `dispatchFilesystemExecutorForRepo` (15 + 1 chỗ gọi), `Clone`, `InitRepo` và `ConnectionResolver.ResolveConnection` (cổng của mọi usecase theo worktree/`dispatchExecutor`). Các usecase bọc lỗi điều phối thành `*_RESOLVE_FAILED` nên `toStatus` ở adapter gRPC tìm mã bên trong chuỗi lỗi và trả mã hành động được. `RelayExecutor` đổi `INFRA_DEV_SERVER_NOT_CONNECTED` (agent rớt sau khi điều phối, mẫu sức khoẻ trễ tới 30 giây) thành `GITGATEWAY_DEV_SERVER_UNREACHABLE`.
4. **Công cụ gắn lại dữ liệu cũ.** `deploy/dev/scripts/rebind-unbound-repos.sh` (gọi `rebind_unbound_repos.py`): mặc định chỉ liệt kê repo có `devServerId` rỗng (qua `project.list` và `repo.list`); `--apply --dev-server <id|tên>` mới gắn lại, từng repo qua `repo.rebindDevServer` (đường sản phẩm: chủ project, bị chặn khi project có workflow/task đang chạy); từ chối khi dev server chưa được duyệt hoặc không `connected`; có `--repo` (lặp lại) và `--project` để giới hạn; không in bí mật; không đoán host. Dùng lại client phiên và WebSocket của `tests/stages/e2e/orca-flows` (chỉ import, không sửa).
5. **Duyệt thư mục.** Gốc rễ là cấu hình: `github.com/coder/websocket` mặc định giới hạn đọc một thông điệp 32768 byte, mọi điểm tạo kết nối của infra-fleet không đặt `SetReadLimit`, nên một phản hồi agent trên 32 KiB (khoảng 300 đến 450 mục của `fs.readDir`, ví dụ `$TMPDIR` macOS) làm infra-fleet đóng kết nối (`message too big: read limited at 32769 bytes`) và agent rớt cho tới khi tự nối lại (~30 giây). Sửa: `SetReadLimit(MaxMessageSize + HeaderLength)` (16 MiB + 13) ở `newWSTransport`, trước bắt tay phía Dial (`session.go`) và ngay sau `Accept` (`agentwsserver`). Thêm trần nghiệp vụ ở gateway: `devServer.browseDir` và `files.browseServerDir` nhận tuỳ chọn `limit` (mặc định 500, tối đa 2000), `directoriesOnly`, `query` (chứa chuỗi, không phân biệt hoa thường); kết quả thêm `truncated` và `totalEntries` (cộng thêm, tương thích ngược). **Không sửa `agent/**`** (không cần; agent vẫn trả đủ danh sách và gateway cắt; tham số `maxEntries`/`includeSize:false` ở agent là tối ưu tuỳ chọn, chưa làm).

## Mã lỗi mới và cách frontend nhận

Lỗi đến frontend qua gateway dạng `rpc error: code = <Code> desc = <MÃ>: <thông điệp>`; frontend nhận dạng bằng chuỗi con của `<MÃ>` (cách `workspace-create-error-format.ts` đang làm).

| Mã | Service | gRPC | Khi nào | Frontend nên |
|---|---|---|---|---|
| `PROJECT_REPO_HOST_REQUIRED` | project-service | InvalidArgument | `repo.add`/`project.create (repoPath)`/`project.importNested`/`repo.rebindDevServer` không có host và không có dev server của project (cờ local tắt) | chặn bước thêm project, mời chọn host; gặp từ client cũ thì báo tải lại trang |
| `GITGATEWAY_REPO_HOST_UNBOUND` | git-gateway | FailedPrecondition | repo cũ không gắn dev server (cờ tắt) | "Repo chưa gắn host", đưa nút gắn host (`repo.rebindDevServer`) |
| `GITGATEWAY_DEV_SERVER_UNREACHABLE` | git-gateway | Unavailable | dev server của repo offline hoặc agent vừa rớt | báo host offline, nút Retry; không phải lỗi git |
| `GITGATEWAY_HOST_CHECK_FAILED` | git-gateway | Unavailable | không xác định được host (lỗi đọc sức khoẻ/`GetRepo`) | báo tạm thời, thử lại |
| `truncated`/`totalEntries` (trường, không phải lỗi) | api-gateway | — | thư mục vượt `limit` | hiện "Hiển thị N/M mục", dùng `query` để lọc |

## Việc cần làm

- [x] project-service: `RepoHostPolicy`, `ProjectDirectory`, `AddRepo` (request, rồi project, rồi lỗi), `CreateProject`, `ImportNested`, `RebindRepoDevServer`, cờ cấu hình, nối `main.go`.
- [x] git-gateway: `RouteRepoHost`, `RepoHostRouter`, resolver, `Clone`/`InitRepo`, ánh xạ relay và `toStatus`, cờ cấu hình, nối `main.go`.
- [x] infra-fleet: `SetReadLimit` ở ba điểm (test đỏ trước, xanh sau, tái hiện đúng dòng log `read limited at 32769 bytes`).
- [x] api-gateway: trần danh sách, tuỳ chọn, `truncated`/`totalEntries`; REST `dev_server_id`.
- [x] deploy/dev: `ORCA_ALLOW_LOCAL_REPOS` trong compose, công cụ gắn lại (dry-run mặc định).
- [x] Test: bảng định tuyến đủ nhánh (reachable, unreachable, không dev server cờ tắt/bật, lỗi đọc sức khoẻ), resolver, mapping gRPC, request > project > lỗi, cô lập tenant, validate infra-fleet, không lưu repo mồ côi, trần danh sách.
- [ ] Redeploy (người điều phối): project-service, git-gateway-service, infra-fleet-service, api-gateway. Không có migration.
- [ ] Chạy công cụ ở chế độ dry-run trên stack thật để biết số repo cũ; người dùng quyết định `--apply` (repo `codex` và project `My Repos` là dữ liệu của người dùng, chưa đụng).
- [ ] Chạy `run_ui_flow.py --repro add-project-worktree` cùng frontend 016 đến 018 (cần redeploy cả frontend).
- [ ] Commit.

## Khác với SOL-V2-002 (do quyết định của người dùng, ghi để không nhầm)

- Một cờ bool `ORCA_ALLOW_LOCAL_REPOS` thay cho `ORCA_LOCAL_REPO_ROOT` (không có gốc thư mục, không có `IsUnderLocalRepoRoot`).
- Không có chế độ `PROJECT_REPO_HOST_POLICY=warn` hay `GIT_GATEWAY_HOST_DISPATCH_MODE=legacy`; lùi lại bằng `ORCA_ALLOW_LOCAL_REPOS=true` (chỉ khôi phục repo không host, KHÔNG khôi phục fallback khi dev server không tới được).
- Có fallback sang dev server của project (SOL quyết định D4 khuyên không); đổi lại cột `projects.dev_server_id` (di sản, có thể khác host người dùng đã chọn) thành nguồn thứ hai: client web phải gửi `devServerId` tường minh, nếu không repo sẽ gắn host của project.
- Công cụ gắn lại đặt ở `deploy/dev/scripts` (không phải `tests/.../report_unbound_repos.py`), có `--apply` nhưng không tự kiểm đường dẫn tồn tại trên host mới.
- Chưa làm: `GetRepo` trả host hiệu lực, kiểm host tới được khi thêm repo (`PROJECT_ADD_REPO_VERIFY_HOST`), log/metric `repo_host_dispatch` (TASK-V2-028, 031, 034).

## Kiểm thử

Chạy trong `backend-go/services/<service>`:

- `git-gateway-service`: `go build ./... && go vet ./... && go test ./...` PASS; `go test -race ./internal/...` PASS.
- `project-service`: `go build ./... && go vet ./... && go vet -tags integration ./... && go test ./...` PASS (không đụng repository/migration nên không chạy tích hợp hai dialect).
- `api-gateway`: `go build ./... && go vet ./... && go test ./...` PASS (`TestChannelInventoryMatchesGolden` giữ nguyên, không thêm kênh).
- `infra-fleet-service`: `go build ./... && go vet ./... && go test ./...` PASS trừ hai test socket unix có sẵn (`bind: invalid argument`, `backendrelaysshprovisioner`, `ephemeralsshconn`).

## Kết quả thực thi và blast radius

Blast radius tính bằng `codegraph callers` (tên chính xác): `dispatchExecutorForRepo` 15 (13 usecase + 3 test), `dispatchFilesystemExecutorForRepo` 1, `dispatchExecutor` 20 (đã đạt trần hiển thị của công cụ), `ResolveConnection` 20, `newWSTransport` 3, `NewAddRepo` 9 (1 `main.go` + test). gitnexus không có.

- Hành vi đổi cho người dùng Docker: repo cũ không có dev server nay báo `GITGATEWAY_REPO_HOST_UNBOUND` ở mọi thao tác (worktree, hooks, ref, file, remove worktree) thay vì `chdir ...: no such file or directory`; sẽ không xoá được worktree của repo cũ cho tới khi gắn lại host (cố ý: không chạy git sai chỗ).
- Dev server mất kết nối: mọi thao tác theo repo báo `GITGATEWAY_DEV_SERVER_UNREACHABLE` thay vì chạy cục bộ; cửa sổ trễ 30 giây của mẫu sức khoẻ vẫn còn nhưng nay trả lỗi thử lại được.
- Các service khác đọc `repo.devServerId` (request-service `ai_connection_resolver`, task-service `project_execution_resolver`) đã từ chối repo không host (`ErrNoDevServer`), nên không phụ thuộc "repo local"; không đổi.
- e2e orca-flows tạo project qua `project.create {devServerId, repoPath}` vẫn đi nhánh có host (không đổi). Không suite nào gọi `repo.add` không host (đã rà `tests/` và `backend-go`).
- Rủi ro còn lại: repo `codex` và các repo thêm từ web trước đây có `devServerId` rỗng sẽ lỗi cho tới khi gắn lại; tab frontend cũ gọi `repo.add` không `devServerId` vào project có host riêng sẽ gắn nhầm host của project (cần frontend 016 đến 018).
