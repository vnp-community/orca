# TASK-V2-036: `project.create` tạo PROJECT RIÊNG, nguyên tử, đúng mọi trường hợp, trả đủ id project/repo

**From:** lượt test Request #1136, lỗi (2) "Add Project phải tạo được project riêng chứa repo, gắn dev server đã chọn" · **Solution:** [SOL-V2-003](../solutions/SOL-V2-003-project-own-project-and-repo-dedupe.md) mục 2.4, 2.5, 2.6
**Priority:** P0 · **Service/Area:** project-service, api-gateway (wscompat + REST), proto `project.v1` (chỉ cộng)
**Files (dự kiến):**
- project-service: `internal/usecase/create_project.go` (thứ tự kiểm mới, gọi `CreateWithOwner`, không còn `AddRepo` nuốt lỗi), `ports.go` (`ProjectRepository.CreateWithOwner`), `internal/adapter/postgres/repository.go` + `internal/adapter/mysql/repository.go` (`CreateWithOwner`: một giao dịch project + owner + repo), `internal/adapter/grpc/server.go` (`CreateProject` trả `repo`), `internal/usecase/create_project_test.go`, `fakes_test.go`
- proto: `proto/orca/project/v1/project.proto` (`CreateProjectResponse.repo = 2`), mã sinh (`buf generate`, header `v6.33.4`)
- api-gateway: `internal/adapter/wscompat/channels_tenant_project.go` (`project.create` trả `repo`, `repoId`), `internal/adapter/httpgateway/project_routes.go` (`dev_server_id`, `repo_path` ở `POST /v1/projects`), test cạnh mã
**Depends on:** TASK-V2-037 (khoá vị trí repo; làm cùng đợt) · **Status:** `[x]` XONG (2026-10-11): mã, test (đơn vị, hợp đồng, tích hợp hai dialect, đua) và kiểm live. Kiểm live (2026-10-11): e2e `--real addproject` XANH 2/2 (Claude thật); `--real all` 7/7 XANH (24.1 phút); `--repro add-project-worktree` xanh; dữ liệu thật của người dùng không đổi.

## Vấn đề (đã xác minh)

`CreateProject` ghi project (`create_project.go:138`) rồi gắn repo bằng `_, _ = uc.repos.AddRepo(...)` (dòng 146), nuốt cả lỗi: project có thể không có repo mà người dùng không biết; hội viên owner thêm ở lần gọi thứ hai của gateway (`channels_tenant_project.go:483`); phản hồi không có id repo; kiểm tên/visibility chỉ chạy sau lời gọi mạng (dòng 116-131); REST bỏ `dev_server_id`/`repo_path` (`project_routes.go:85-91`).

## Việc cần làm

- [x] `CreateWithOwner` ở hai dialect: một giao dịch ghi project, hội viên owner (người tạo) và repo đầu tiên (position 0, `dev_server_id`, khoá); lỗi bất kỳ thì quay lui hết, không còn project mồ côi.
- [x] `CreateProject.Execute`: kiểm tên và visibility trước; `repo_path` rỗng khi có dev server vẫn `PROJECT_REPO_PATH_REQUIRED`; `repo_path` có hoặc không có `/` cuối đều tạo repo (cùng vị trí); repo lưu `dev_server_id` của request; trả `(project, repo)`.
- [x] Không tạo project khi: dev server không tồn tại / không tới được / `repo_path` không có trên host / vị trí đã đăng ký (TASK-V2-037) / visibility sai; thông điệp mã y như cũ.
- [x] Project không dev server + không `repo_path` vẫn tạo được (không repo, có owner); project tên trùng vẫn tạo project mới id khác.
- [x] Proto: `CreateProjectResponse.repo`; gRPC server điền; gateway WS trả `{...project, repo, repoId}`; REST nhận `dev_server_id`, `repo_path`.
- [x] Test: tạo thành công đủ project+repo+owner; visibility (`private`, `team`, `department`, `company`, rỗng, sai); tên trùng; `repo_path` có và không có `/` cuối; mỗi lỗi không để lại project (fake đếm `Create`/`CreateWithOwner`); gateway trả `repoId`; thành viên/tenant: tenant khác không thấy.

## Kiểm thử

`go build ./... && go vet ./... && go test ./...` ở project-service và api-gateway; `go test -tags integration` cho `CreateWithOwner` (task 039); `buf lint` (307), `buf breaking`.

## Kết quả thực thi và blast radius

- `ProjectRepository.CreateWithOwner` (Postgres `repository.go`, MySQL `repository.go`): một giao dịch ghi project + owner + repo (position 0, `registration_key`); trùng khoá hoặc bất kỳ lỗi nào thì quay lui hết (test hai dialect: 12 goroutine, đúng một project/repo/hội viên còn lại).
- `CreateProject.Execute` trả `(project, *repo, error)`; thứ tự mới: tên, visibility, chính sách host, dev server tồn tại/tới được, `repo_path` bắt buộc khi có host, vị trí chưa đăng ký, `fs.checkPath`, ghi. `repo_path` được cắt khoảng trắng hai đầu; `repo_path` không có host và cờ local bật tạo repo local (trước đây bỏ im lặng). Dev server có `repo_path` không phải kho git vẫn được chấp nhận (chế độ thư mục không-git; project-service không có client git), thao tác git sau đó báo lỗi git có mã.
- Proto: `CreateProjectResponse.repo = 2` (cộng); `buf lint` 307 (không đổi), `buf breaking` xanh, header sinh mã giữ nguyên (`protoc (unknown)`; các tệp khác chỉ đổi header nên đã khôi phục từ `HEAD`).
- Gateway: `project.create` trả `{...project, repo, repoId}` (`repo`/`repoId` bỏ khi không có repo; `repo` cùng khoá camelCase với `repo.add`: `id, projectId, url, displayName, position, devServerId, hookSettings`); vẫn gọi `AddMember` sau đó (upsert, vô hại). REST `POST /v1/projects` nhận `dev_server_id`, `repo_path` và thêm trường `repo` cạnh các trường project (cùng thân cũ).
- Test: `create_project_own_project_test.go` (11), `create_project_test.go`, `repo_host_policy_test.go` (cập nhật), `channels_tenant_project_test.go` (+2), `project_routes_test.go` (+2).
- Lệnh: `go build ./... && go vet ./... && go test ./...` xanh ở `project-service` và `api-gateway`; `go test -tags integration ./internal/adapter/mysql` xanh (283 giây); Postgres tích hợp xanh trừ 13 test worktree có sẵn (`insert outbox event: invalid input syntax for type uuid`, giống `main`).
- Blast radius (`codegraph callers`, tên chính xác): `NewCreateProject` chỉ dựng ở `cmd/server/main.go` (cộng test); gateway là client duy nhất của RPC `CreateProject`. `ProjectRepository.Create` giữ nguyên. Cần redeploy: project-service, api-gateway.
