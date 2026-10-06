# TASK-REQ-001-01: Module `request-service`, cấu hình, đăng ký workspace

**From Solution:** BE-REQ-SOL-001
**Priority:** P0
**Service:** `request-service` (mới), `go.work`, `Makefile`
**File:** `backend-go/services/request-service/go.mod` (mới), `internal/config/config.go` (mới), `internal/config/config_test.go` (mới), `README.md` (mới), `backend-go/go.work`, `backend-go/Makefile`, `backend-go/deploy/postgres-init-databases.sh`
**Depends on:** Không (task đầu tiên của toàn series)
**Status:** [ ] TODO

---

## Context

Đã đọc `go.work` (19 module service, thứ tự chữ cái, `project-service` rồi `scm-integration-service`), `Makefile` dòng 7 đến 11 (biến `SERVICES`), `postgres-init-databases.sh` (biến `DATABASES`), `task-service/internal/config/config.go` (nhúng `commonconfig.Base`) và `common/config`. Mọi `go.mod` service ghi `go 1.25.0` dù `go.work` ghi `go 1.26.0`; module mới theo `1.25.0`. `request-service` chưa tồn tại.

## Việc cần làm

1. Tạo `backend-go/services/request-service/go.mod`: `module github.com/stablyai/orca-go/services/request-service`, `go 1.25.0`. Sao danh sách `require` tối thiểu từ `services/task-service/go.mod` (pgx v5, go-sql-driver/mysql, grpc, uuid, nats), phần `replace` nếu có thì giữ cùng kiểu. Chạy `go mod tidy` sau khi có code ở các task sau; ở task này chỉ cần `go build ./...` qua với package `config`.
2. Tạo `internal/config/config.go` với `type Config struct` như SOL-001 mục B (nhúng `commonconfig.Base`; thêm `DatabaseCredentialsFile`, `NATSURL`, `TaskServiceAddr`, `AIProviderServiceAddr`, `ProjectServiceAddr`, `RequestFlowEnabled`) và `func Load() Config` theo cách `task-service` đọc env (`DATABASE_CREDENTIALS_FILE`, `NATS_URL`, `TASK_SERVICE_ADDR`, `AI_PROVIDER_SERVICE_ADDR`, `PROJECT_SERVICE_ADDR`, `REQUEST_FLOW_ENABLED`; bool theo `strconv.ParseBool`, mặc định `false`).
3. Thêm `./services/request-service` vào `go.work` giữa `./services/project-service` và `./services/scm-integration-service`.
4. Thêm `request-service` vào `SERVICES` trong `Makefile` (giữ thứ tự chữ cái; dòng `notification-service` hiện đứng riêng, thêm `request-service` sau `project-service`).
5. Thêm `request` vào `DATABASES` của `deploy/postgres-init-databases.sh`.
6. Tạo `README.md` của service: mục "Chạy local", biến môi trường, và bảng **real vs stub** (ở task này chỉ có `config` là thật). Bảng sẽ được TASK-REQ-001-05 và TASK-REQ-002-07 cập nhật.
7. Cấm tên `helpers`, `utils`, `common`, `misc`; không thêm `max-lines` disable.

## Kiểm thử

- `config_test.go`: `TestLoad_Defaults` (cổng 9090/8080, `RequestFlowEnabled=false`, địa chỉ rỗng); `TestLoad_Overrides` (đặt env, kiểm từng trường); `TestLoad_InvalidBool` (giá trị `REQUEST_FLOW_ENABLED=maybe` rơi về `false` hoặc trả lỗi, theo cách `task-service` xử lý bool sai; ghi rõ lựa chọn vào test).
- Lệnh: `cd /opt/repos/orca/backend-go && go test ./services/request-service/... && go build ./services/request-service/...`; `make vet` có chạm module mới.

## Tiêu chí hoàn thành

- [ ] `go build ./services/request-service/...` và `go vet` xanh trong workspace.
- [ ] `go.work`, `Makefile`, `postgres-init-databases.sh` có `request-service` / `request` đúng thứ tự.
- [ ] `config_test.go` xanh.
- [ ] README có bảng real vs stub.
- [ ] `grep -ri "helpers\|utils\|misc" services/request-service` không có tên file hay thư mục vi phạm.

## Rủi ro và lưu ý

- `go.work.sum` có thể cần cập nhật (`make tidy-all`); chưa chạy.
- Tên database `request` có thể vướng từ dành riêng ở MySQL của môi trường nào đó (chưa kiểm chứng). Nếu vướng, đổi thành `requests` ở toàn series (đổi một lần ở đây, `migrate.sh`, compose, CI).
