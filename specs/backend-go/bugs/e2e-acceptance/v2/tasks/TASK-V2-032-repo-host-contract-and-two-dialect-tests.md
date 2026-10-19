# TASK-V2-032: Test hợp đồng và tích hợp hai dialect cho chính sách host của repo

**From Solution:** SOL-V2-002 (mục 3.2, 3.8) · **Resolves:** tiêu chí kiểm thử của TASK-FE-E2E-015 (Go test)
**Priority:** P1 · **Service/Area:** project-service, api-gateway, git-gateway-service
**Files:**
- backend-go/services/project-service/internal/adapter/postgres/repo_repository_test.go, mysql/repo_repository_test.go (tag `integration`), usecase/add_repo_test.go, get_repo_test.go
- backend-go/services/api-gateway/internal/adapter/wscompat/channels_repo_*_test.go, testdata/channel_inventory.json (không đổi; `TestChannelInventoryMatchesGolden` phải xanh)
- backend-go/services/git-gateway-service/internal/usecase/dispatch_test.go, internal/adapter/grpcclient/grpcclient_test.go
**Depends on:** TASK-V2-028, TASK-V2-029
**Status:** `[~]` MỘT PHẦN (2026-10-11): chỉ có test đơn vị/hợp đồng ở project-service, git-gateway, infra-fleet, api-gateway; không có test tích hợp hai dialect vì không đổi repo/migration.

---

## Việc cần làm

- [ ] Hợp đồng repo: ghi `dev_server_id` thừa hưởng, đọc lại qua `GetRepo`/`ListRepos`, NULL ↔ `""` ở cả Postgres (`COALESCE(dev_server_id::text,'')`) và MySQL.
- [ ] Không có migration mới: xác nhận bằng test rằng migration `0017` vẫn là nguồn cột (không thêm `0032`); nếu bất kỳ task nào thêm migration thì bắt buộc cả `postgres/` và `mysql/` và chạy up/down.
- [ ] Bộ test bảng chung (cùng dữ liệu) cho `IsUnderLocalRepoRoot` ở project-service và git-gateway để hai bên không lệch.
- [ ] `buf lint` không tăng cảnh báo, `buf breaking` xanh so với `main`.
- [ ] `go test ./...` ở `project-service`, `api-gateway`, `git-gateway-service`, `infra-fleet-service`; `-tags integration` project-service trên Postgres và MySQL (testcontainers; môi trường Colima như ghi ở TASK-V2-027: `DOCKER_HOST`, `TESTCONTAINERS_RYUK_DISABLED`).

## Tiêu chí hoàn thành

Mọi lệnh trên xanh, trừ các lỗi CÓ SẴN đã ghi ở README v2 (`common/tracing`, test socket `infra-fleet`, `localgit` symlink macOS, 14 test worktree `project-service` giống hệt `main`).

## Rủi ro · Ước lượng

LOW. 1 ngày.
