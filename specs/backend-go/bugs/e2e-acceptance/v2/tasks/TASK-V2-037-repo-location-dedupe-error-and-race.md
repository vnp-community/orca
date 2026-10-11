# TASK-V2-037: Chặn đăng ký trùng repo (tenant, dev server, đường dẫn chuẩn hoá): mã `PROJECT_REPO_ALREADY_REGISTERED`, chuẩn hoá, chống đua

**From:** lượt test Request #1136, lỗi (3) · **Solution:** [SOL-V2-003](../solutions/SOL-V2-003-project-own-project-and-repo-dedupe.md) mục 2.1 đến 2.3, 2.7
**Priority:** P0 · **Service/Area:** project-service (domain, usecase, adapter hai dialect, migration `0032`)
**Files (dự kiến):**
- domain: `internal/domain/repo_location.go` (`NormalizeRepoLocation`, `RepoRegistrationKey`, `RepoConflict`, `ErrRepoAlreadyRegistered`), `repo.go` (`Repo.RegistrationKey`), test `repo_location_test.go`
- usecase: `internal/usecase/repo_registration.go` (kiểm vị trí + dựng thông điệp cố định + rút gọn quyền), `add_repo.go`, `rebind_repo_dev_server.go`, `update_repo.go`, `import_nested.go`, `setup_existing_folder.go`, `ports.go` (`FindRepoAtLocation`, `UpdateDevServerID` thêm khoá), test từng usecase, `fakes_test.go`
- adapter: `internal/adapter/postgres/repo_repository.go`, `mysql/repo_repository.go`, hai `project_group_repository.go` (`ImportNested` ghi khoá; sửa `Scan` sai ở Postgres), `scanRepo`/`repoColumns` có `registration_key`
- migration: `migrations/postgres/0032_repo_registration_key.{up,down}.sql`, `migrations/mysql/0032_repo_registration_key.{up,down}.sql`
**Depends on:** không · **Status:** `[x]` XONG (2026-10-11): mã, test (đơn vị, hợp đồng, tích hợp hai dialect, đua) và kiểm live. Kiểm live (2026-10-11): e2e `--real addproject` XANH 2/2 (Claude thật); `--real all` 7/7 XANH (24.1 phút); `--repro add-project-worktree` xanh; dữ liệu thật của người dùng không đổi.

## Quyết định (xem SOL mục "Quyết định đã chọn")

Mã `PROJECT_REPO_ALREADY_REGISTERED`, `KindAlreadyExists`; thông điệp cố định:
`PROJECT_REPO_ALREADY_REGISTERED: repo <url> on dev server <devServerId> is already registered in project "<tên>" (<projectId>)`.
Người gọi không xem được project kia: `... is already registered in another project`. Khoá = SHA-256(tenant, dev server, đường dẫn chuẩn hoá) trong `repos.registration_key` + chỉ mục UNIQUE (nhiều NULL cho dữ liệu cũ, không migrate/xoá).

## Việc cần làm

- [x] Chuẩn hoá: bỏ khoảng trắng hai đầu, bỏ dấu phân cách cuối (giữ gốc `/`, `C:\`), gộp phân cách lặp, Windows (`C:\`, `\\`) coi `/` = `\`, URL `://` chỉ bỏ `/` cuối; không đổi hoa/thường.
- [x] Migration `0032` hai dialect (up/down), KHÔNG backfill; chạy up/down/up thật và kiểm dữ liệu trùng sẵn có còn nguyên.
- [x] `FindRepoAtLocation` ở hai adapter, lọc tenant qua `projects.tenant_id`, `dev_server_id` bằng hoặc `IS NULL`; adapter ánh xạ lỗi unique (`23505`/`1062`, tên `uq_repos_registration_key`) sang `ErrRepoAlreadyRegistered`.
- [x] Áp dụng ở `AddRepo`, `CreateProject` (TASK-V2-036), `SetupExistingFolder`, `ImportNested` (kể cả hai ứng viên cùng đường dẫn trong một lô: `PROJECT_IMPORT_NESTED_DUPLICATE_PATH`, `InvalidArgument`), `RebindRepoDevServer` (chỉ khi dev server đổi; về cùng dev server là no-op), `UpdateRepo` (chỉ khi `url` đổi vị trí).
- [x] Thua đua: tra lại bên thắng để dựng thông điệp; không thấy thì `PROJECT_REPO_REGISTRATION_CONFLICT` (`KindAborted`).
- [x] Rút gọn thông điệp khi người gọi không là hội viên, không là admin toàn cục và project `private`.
- [x] Test: bảng chuẩn hoá (POSIX, Windows, URL, hoa/thường); mỗi usecase trùng cùng project và khác project; khác dev server cùng đường dẫn KHÔNG trùng; khác tenant cùng dev server id/đường dẫn KHÔNG trùng; repo cũ không khoá trùng vẫn bị chặn khi thêm bản thứ ba; thao tác khác trên repo cũ trùng (đổi tên, rebind về chính nó) vẫn chạy; chuỗi thông điệp chính xác; test đua mỗi dialect (task 039).
- [x] Rà e2e `orca-flows`: `suite_dev_server_agent.py` (DEV-04/05/06), `ui-live/live-project-fixture.ts` dùng `ORCA_E2E_REPO_PATH` hoặc repo tạm duy nhất; chỉ bị chặn khi tái dùng đường dẫn cố định mà project cũ chưa xoá. Ghi kết quả rà vào mục kết quả.

## Kiểm thử

`go build ./... && go vet ./... && go test ./...` project-service; `go vet -tags integration ./...`; tích hợp ở task 039.

## Kết quả thực thi và blast radius

- Miền: `domain/repo_location.go` (`NormalizeRepoLocation`, `RepoRegistrationKey`, `RepoConflict`, `ErrRepoAlreadyRegistered`), `Repo.RegistrationKey`; `NewRepo` nay từ chối url chỉ có khoảng trắng. Bảng chuẩn hoá 20 ca (POSIX, Windows `C:\`/`C:/`/UNC, URL, hoa/thường giữ nguyên, `..` không giải); cùng bảng ca ở `deploy/dev/scripts/test_report_duplicate_repos.py`.
- Migration `0032_repo_registration_key` (postgres, mysql): cột `registration_key` + chỉ mục UNIQUE `uq_repos_registration_key`, KHÔNG backfill. Test tích hợp hai dialect: dựng schema đến bản 31, chèn hai repo trùng, `up` giữ nguyên cả hai (không khoá), `down 1` bỏ cột và giữ đủ hàng, `up` lại.
- Adapter: `FindRepoAtLocation` (JOIN `projects` lọc tenant, so chuẩn hoá trong Go, dev server so không phân biệt hoa/thường), ánh xạ `23505`/`1062` đúng tên chỉ mục sang `ErrRepoAlreadyRegistered`; MySQL `AddRepo` thử lại khi deadlock (1213) do `INSERT ... SELECT`. `UpdateDevServerID` nhận thêm `registrationKey`; `Update` ghi khoá.
- Usecase (`repo_registration.go`): kiểm trước, tra lại bên thắng khi thua đua, rút gọn thông điệp khi người gọi không xem được project `private`. Áp dụng: `AddRepo`, `CreateProject`, `SetupExistingFolder` (nay tạo project+owner+repo nguyên tử, setup giữ `pending` khi trùng), `ImportNested` (cả trùng trong lô: `PROJECT_IMPORT_NESTED_DUPLICATE_PATH`), `RebindRepoDevServer` (chỉ khi đổi dev server; cùng dev server là no-op trả repo nguyên trạng), `UpdateRepo` (chỉ khi đổi vị trí).
- Mã lỗi: `PROJECT_REPO_ALREADY_REGISTERED`, `KindAlreadyExists` (gRPC `AlreadyExists`, REST 409). Thông điệp (test so chuỗi chính xác sau `ToGRPCStatus`): `PROJECT_REPO_ALREADY_REGISTERED: repo /srv/app on dev server ds-1 is already registered in project "Alpha" (alpha)`; rút gọn `... is already registered in another project`; thua đua mà bên thắng biến mất: `PROJECT_REPO_REGISTRATION_CONFLICT` (`Aborted`).
- Test usecase: `repo_registration_test.go` (cùng project/khác project, biến thể chính tả, khác dev server, khác tenant, repo cũ không khoá chặn bản thứ ba, thao tác khác trên repo cũ trùng vẫn chạy, rút gọn riêng tư, thua đua, rebind, update, import, setup). Test đua thật: 16 goroutine `AddRepo` cùng khoá mỗi dialect, đúng 1 thành công (`-race`, MySQL lặp 4 lần xanh), 12 goroutine `CreateWithOwner`.
- Rà e2e `orca-flows` (đã đọc, KHÔNG chạy): `suite_dev_server_agent.py` DEV-04 tạo project với `cfg.repo_path` (`ORCA_E2E_REPO_PATH`) hoặc repo tạm duy nhất theo `run_prefix`; DEV-05 (đường dẫn không tồn tại) và DEV-06 (dev server không tồn tại) vẫn nhận `PROJECT_REPO_PATH_NOT_FOUND`/`PROJECT_DEV_SERVER_NOT_FOUND` (kiểm dev server đứng trước kiểm trùng; đường dẫn không tồn tại không bao giờ trùng); `ui-live/live-project-fixture.ts` dùng `ORCA_E2E_REPO_PATH` hoặc repo tạm; `real-repo-fixture.ts` và `live-add-project-repro-steps.ts` dùng `mkdtemp` duy nhất mỗi lượt. Chỉ bị chặn (đúng thiết kế) khi đặt `ORCA_E2E_REPO_PATH` cố định VÀ một lượt trước chết giữa chừng để lại project cùng đường dẫn, hoặc chạy hai luồng song song cùng biến đó. Không suite nào gọi `repo.add`/`project.importNested`.
- Blast radius (`codegraph callers`): `NewAddRepo` 9, `NewImportNested` 4, `NewRebindRepoDevServer` 11, `NewUpdateRepo` 5, `NewSetupExistingFolder` 5 (mỗi cái: một nơi dựng ở `cmd/server/main.go` + test); `RepoRepository.UpdateDevServerID` một nơi gọi sản xuất (`rebind_repo_dev_server.go`). Chỉ project-service đổi hành vi; api-gateway chỉ truyền lỗi. Cần redeploy: migration `0032` + project-service.
