# TASK-V2-039: Test tích hợp hai dialect cho chống trùng repo, migration `0032` và `CreateWithOwner` (có test đua)

**From:** [SOL-V2-003](../solutions/SOL-V2-003-project-own-project-and-repo-dedupe.md) mục 2.3, 2.7, 6.3 · **Priority:** P0 · **Service/Area:** project-service (`-tags integration`, Postgres + MySQL qua testcontainers)
**Files (dự kiến):** `internal/adapter/postgres/repo_registration_test.go`, `internal/adapter/mysql/repo_registration_test.go`, (cập nhật) `project_group_repository_test.go` hai dialect
**Depends on:** TASK-V2-036, TASK-V2-037 · **Status:** `[x]` XONG (2026-10-11): mã, test (đơn vị, hợp đồng, tích hợp hai dialect, đua) và kiểm live. Kiểm live (2026-10-11): e2e `--real addproject` XANH 2/2 (Claude thật); `--real all` 7/7 XANH (24.1 phút); `--repro add-project-worktree` xanh; dữ liệu thật của người dùng không đổi.

## Việc cần làm

- [x] Migration: `up` rồi `down` rồi `up` hai dialect bằng `migrate`; tạo hai repo trùng (không khoá) TRƯỚC khi chạy `0032` rồi chứng minh chỉ mục tạo được và cả hai hàng còn nguyên.
- [x] `AddRepo` với khoá: lần hai cùng khoá trả `ErrRepoAlreadyRegistered`; khoá rỗng (NULL) không xung đột (dữ liệu cũ).
- [x] **Đua**: N goroutine (N >= 8) `AddRepo` cùng khoá song song, đúng 1 thành công, còn lại `ErrRepoAlreadyRegistered`, bảng chỉ có 1 hàng; chạy `-race`.
- [x] `FindRepoAtLocation`: trùng dạng `/a/b` và `/a/b/` ở hàng cũ không khoá; cô lập tenant (cùng dev server id + đường dẫn ở tenant khác không thấy); dev server khác không thấy; `excludeRepoID` loại chính nó; local (NULL).
- [x] `CreateWithOwner`: thành công ghi project + owner + repo cùng lúc; trùng khoá quay lui hết (không còn project/hội viên); không repo vẫn tạo owner.
- [x] `UpdateDevServerID` với khoá mới, trùng thì lỗi; `Update` đổi `url` đổi khoá.
- [x] `ImportNested` hai dialect: thành công, trùng với repo có sẵn thì lỗi và không để lại project (Postgres cũng chạy được sau khi sửa `Scan`).

## Kiểm thử

`go test -tags integration ./internal/adapter/...` (và `-race` cho test đua) với `DOCKER_HOST=unix:///Users/datbeohbbh/.colima/default/docker.sock TESTCONTAINERS_RYUK_DISABLED=true`, `migrate` trong `PATH`. Lỗi có sẵn trên `main`: 14 test worktree project-service Postgres.

## Kết quả thực thi

Mỗi dialect có `internal/adapter/{postgres,mysql}/repo_registration_test.go` (tag `integration`): khoá trùng bị từ chối và khoá NULL không xung đột; đua 16 goroutine `AddRepo` đúng một thành công; đua 12 goroutine `CreateWithOwner` không để lại project/hội viên mồ côi; `CreateWithOwner` thành công (có và không repo); `FindRepoAtLocation` (hàng cũ không khoá khác chính tả, cô lập tenant, khác dev server, `excludeRepoID`, local, dev server hoa/thường); `UpdateDevServerID`/`Update` đổi khoá và xung đột; `ImportNested` thành công và quay lui khi có đường dẫn đã đăng ký (Postgres: lần đầu có test cho hàm này, sau khi sửa `Scan`); migration `0032` up/down/up với dữ liệu trùng sẵn có.

Phát hiện nhờ test đua: MySQL báo deadlock 1213 khi nhiều client cùng khoá (`INSERT ... SELECT`); đã xử lý bằng thử lại (xem SOL 2.3), test xanh 4/4 lần liên tiếp.

Lệnh (env colima, `migrate` trong PATH): `go test -tags integration -count=1 ./internal/adapter/postgres/` → chỉ 13 test worktree FAIL (có sẵn, `insert outbox event: invalid input syntax for type uuid`, không liên quan); `go test -tags integration -count=1 ./internal/adapter/mysql/` → ok (283 giây). `-race` cho các test đua: ok.
