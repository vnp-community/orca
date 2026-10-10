# TASK-V2-028: project-service: repo luôn có host (thừa hưởng từ project), `PROJECT_REPO_HOST_REQUIRED`, project gắn dev server không cần repo, `GetRepo` trả host hiệu lực

**From Solution:** SOL-V2-002 (mục 3.2, 3.2.0, 3.2.3, 3.3) · **Resolves:** L2 của chuỗi nguyên nhân; [TASK-FE-E2E-015](../../../../../frontend/bugs/e2e-acceptance-v1/tasks/TASK-FE-E2E-015-web-add-project-drops-dev-server.md) việc 2 và tiêu chí 3
**Priority:** P0 · **Service/Area:** project-service, api-gateway (wscompat + REST), deploy/dev
**Files:**
- backend-go/services/project-service/internal/usecase/add_repo.go (struct ~30, `NewAddRepo`, `Execute` 44-77: thêm cổng `ProjectRepository` để đọc `dev_server_id` của project; ma trận ở SOL mục 3.2)
- backend-go/services/project-service/internal/usecase/create_project.go (78-80 `PROJECT_REPO_PATH_REQUIRED`: cho phép `dev_server_id` không có `repo_path`; 56-77 giữ kiểm host tồn tại/tới được)
- backend-go/services/project-service/internal/usecase/get_repo.go (63: trả host hiệu lực)
- backend-go/services/project-service/internal/usecase/local_repo_root.go (mới: `IsUnderLocalRepoRoot`) + test bảng ca
- backend-go/services/project-service/internal/config (biến `ORCA_LOCAL_REPO_ROOT`, `PROJECT_REPO_HOST_POLICY=enforce|warn`, `PROJECT_ADD_REPO_VERIFY_HOST`), cmd/server/main.go (312 dựng `NewAddRepo`)
- backend-go/services/api-gateway/internal/adapter/wscompat/channels_repo_ssh_status_workspace.go (112-137: sửa chú thích "empty = local repo"; lỗi đi nguyên vẹn), channels_tenant_project.go (449-480: `project.create` đã nhận `devServerId`,`repoPath`)
- backend-go/services/api-gateway/internal/adapter/httpgateway/project_routes.go (242-268: thêm `dev_server_id` vào `addRepoRequestBody`)
- deploy/dev/docker-compose.yml (`x-go-common-env`: `ORCA_LOCAL_REPO_ROOT`)
**Depends on:** không (frontend 016 không chờ task này; làm ở chế độ `warn` trước)
**Status:** `[x]` XONG qua TASK-V2-035 (2026-10-11), đã kiểm live; các mục dự kiến bị đơn giản hoá theo quyết định của người dùng (không có chế độ warn/legacy) ghi ở TASK-V2-035.
**Triển khai thực tế (2026-10-10, chưa commit, chưa redeploy):** [TASK-V2-035](./TASK-V2-035-repo-host-binding-and-no-silent-local.md) — một phần (AddRepo/CreateProject/ImportNested/Rebind, cờ `ORCA_ALLOW_LOCAL_REPOS`; khác SOL: không có `ORCA_LOCAL_REPO_ROOT`, `warn`, `GetRepo` host hiệu lực)

---

## Context

`AddRepo.Execute` chỉ kiểm khi `DevServerID != ""` (dòng 57) và ghi `NULL` khi rỗng (`postgres/repo_repository.go:39`). Quyết định đã chọn: repo thừa hưởng host của project khi request không nêu; không có cả hai thì lỗi có mã; local chỉ khi cấu hình khai báo.

## Việc cần làm

- [ ] `AddRepo`: áp ma trận (host tường minh thắng; rỗng thì dùng `project.dev_server_id` nếu host tồn tại, và GHI nó vào hàng repo; rỗng cả hai thì `PROJECT_REPO_HOST_REQUIRED` `KindInvalidArgument`, trừ khi `ORCA_LOCAL_REPO_ROOT` đặt và `url` tuyệt đối nằm dưới gốc).
- [ ] `PROJECT_REPO_HOST_POLICY=warn`: ghi log `project_repo_host_policy outcome=warned` và cho qua như cũ (cần gạt rollback); `enforce` mặc định.
- [ ] `CreateProject`: nới cho `dev_server_id` không `repo_path` (không `fs.checkPath`, không tạo repo); có `repo_path` giữ hành vi cũ. Test cả hai.
- [ ] `GetRepo`: `DevServerID` = repo, nếu rỗng thì project (qua `ProjectRepository.Get`); không lỗi nếu project đọc thất bại (giữ giá trị rỗng, git-gateway sẽ trả `UNBOUND`).
- [ ] `PROJECT_ADD_REPO_VERIFY_HOST` (mặc định tắt): với `url` tuyệt đối kiểm `IsReachable` (`PROJECT_DEV_SERVER_UNREACHABLE`) và `fs.checkPath` (`PROJECT_REPO_PATH_NOT_FOUND`), theo mẫu `create_project.go:56-100`.
- [ ] REST: `dev_server_id` tuỳ chọn.
- [ ] Compose: `ORCA_LOCAL_REPO_ROOT: ${ORCA_LOCAL_REPO_ROOT:-/data/repos}` trong `x-go-common-env` (hoặc rỗng nếu người dùng muốn tắt hẳn).
- [ ] Đọc không đổi `ListRepos` và proto; `buf lint`/`buf breaking` xanh.

## Tiêu chí hoàn thành

- `repo.add` không `devServerId` vào project có host: repo lưu host của project; vào project không host: `PROJECT_REPO_HOST_REQUIRED`; `repo.add` có `devServerId`: như trước.
- `project.create {devServerId}` không `repoPath`: tạo project gắn host, không repo.
- `GetRepo` của repo cũ (rỗng) trong project có host trả host của project.

## Kiểm thử

Go unit: `add_repo_test.go` (mở rộng 9 test hiện có; thêm ma trận 6 hàng, gốc local biên `/data/reposX`, `..`, Windows path), `create_project_test.go`, `get_repo_test.go`; wscompat `channels_repo_*_test.go` (lỗi đi nguyên mã), `httpgateway` test body. Hợp đồng/tích hợp Postgres và MySQL: `repo_repository_test.go` (cả hai dialect; không migration nên chỉ xác nhận ghi `dev_server_id` thừa hưởng). Lệnh: `go test ./...` trong `services/project-service` và `services/api-gateway`; `-tags integration` cho repository.

## Rủi ro · Ước lượng

MEDIUM (đổi ngữ nghĩa `repo.add` thiếu host; client cũ nhận lỗi sau `enforce`). Thứ tự bật: `warn` → quan sát → `enforce`. 1 đến 1.5 ngày.
