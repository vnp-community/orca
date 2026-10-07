# TASK-REQ-001-06: Deploy dev (compose, migrate, build-local), Dockerfile, workflow CI hai dialect

**From Solution:** BE-REQ-SOL-001
**Priority:** P1
**Service:** `deploy/dev`, `.github/workflows`, `request-service/deploy`
**File:** `backend-go/services/request-service/deploy/Dockerfile` (mới), `deploy/dev/docker-compose.yml`, `deploy/dev/scripts/migrate.sh`, `deploy/dev/scripts/build-local.sh`, `.github/workflows/backend-go-request-service.yml` (mới)
**Depends on:** TASK-REQ-001-05
**Status:** [x] DONE

---

## Context

Stack dev thật ở `/opt/repos/orca/deploy/dev/docker-compose.yml`: mỗi service là binary mount `./bin/<svc>/orca` vào image chung (anchor `x-go-defaults`, `entrypoint: ["/app/bin/orca"]`); `task-service` ở dòng 325, `migrate-task` ở dòng 705 (dùng image `migrate/migrate:v4.17.0`, volume `./bin/task-service/migrations`). `migrate.sh` dòng 27 có `SERVICES=` riêng (tên DB, không phải tên service; `request` khớp DB). `build-local.sh` dòng 43 có danh sách service. Dockerfile mẫu `task-service/deploy/Dockerfile` và `mcp-service/deploy/Dockerfile` dùng `golang:1.25-bookworm`, distroless runtime. Workflow mẫu `.github/workflows/backend-go-task-service.yml` (matrix `dialect: [postgres, mysql]`, cài `golang-migrate` theo tag dialect).

## Việc cần làm

1. `deploy/Dockerfile`: sao `mcp-service/deploy/Dockerfile`, đổi tên binary và thư mục thành `request-service`. Giữ `COPY policy ./policy` ở stage build; bỏ `COPY --from=build /src/policy/orca-authz` ở runtime (request-service chưa dùng OPA, CR-REQ-010 sẽ quyết định). `COPY services/request-service/migrations /migrations`.
2. `docker-compose.yml`: thêm service `request-service` (`<<: *go-defaults`, `container_name: orca-go-request`, volume `./bin/request-service/orca:/app/bin/orca:ro`, `DATABASE_DSN: postgresql://orca:${POSTGRES_PASSWORD}@postgres:5432/request?sslmode=disable`, `depends_on: postgres`). Thêm `migrate-request` (`<<: *migrate-defaults`, volume `./bin/request-service/migrations:/migrations:ro`, `command` tới DB `request`). Chưa thêm `REQUEST_SERVICE_ADDR` cho `api-gateway` (thuộc CR-REQ-016).
3. `migrate.sh`: thêm `request` vào `SERVICES` (cuối danh sách, sau `issuestatussync`).
4. `build-local.sh`: thêm `request-service` vào danh sách ở dòng 43; kiểm script có sao thư mục `migrations` vào `bin/<svc>/migrations` cho service mới (đọc script trước khi sửa).
5. `.github/workflows/backend-go-request-service.yml`: sao workflow `task-service`, đường dẫn kích hoạt thêm `backend-go/proto/orca/request/**`, `backend-go/common/**`, `backend-go/go.work`, chính file workflow; các bước `go build`, `go vet`, `go test ./...`, integration theo `matrix.dialect` (`./internal/adapter/postgres/...` hoặc `./internal/adapter/mysql/...`); thêm bước `buf lint` và `buf breaking` chạy trực tiếp (không qua `make proto-lint`).
6. Cập nhật README service: mục "Triển khai dev".

## Kiểm thử

- `docker build -f backend-go/services/request-service/deploy/Dockerfile backend-go` (build context là `backend-go`, như mẫu) thành công.
- Chạy `deploy/dev/scripts/migrate.sh request` trên compose local: `migrate-request` kết thúc mã 0, bảng `request.outbox_events` tồn tại (`psql \dt request.*`).
- Workflow: đẩy nhánh thử, cả hai job `postgres` và `mysql` xanh. Chưa kiểm chứng.
- `actionlint` (nếu repo có) cho file workflow.

## Tiêu chí hoàn thành

- [x] Image build được; service khởi động trong compose với Postgres, `/healthz` OK.
- [x] `migrate.sh request` chạy `0001` thành công trên DB `request`.
- [x] Workflow có job cho cả `postgres` và `mysql`, chạy `buf breaking`.
- [x] `build-local.sh` đóng gói `request-service` và migrations.

## Rủi ro và lưu ý

- `docker-compose.yml` là file nhiều service cùng sửa (CR-REQ-016 thêm `REQUEST_SERVICE_ADDR` cho gateway); rebase trước khi sửa để tránh xung đột.
- Image Go `1.25` so với `go.work` `1.26.0`: nếu build lỗi do chỉ thị `go`, báo lại, không tự nâng version một mình (mọi service đang dùng 1.25).
- Không đưa bí mật vào compose; chỉ biến đã có (`POSTGRES_PASSWORD`, `VAULT_TOKEN` qua anchor).
