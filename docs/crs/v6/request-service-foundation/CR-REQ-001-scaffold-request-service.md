# CR-REQ-001 — Dựng `request-service`: nơi sở hữu Request, Solution và Approval

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-REQ-001 |
| **Tên** | Tạo service Go mới `request-service` (gRPC, DB riêng, hai dialect) theo layout của `notification-service` |
| **Loại** | Feature (service mới) |
| **Priority** | 🔴 P0, điều kiện tiên quyết của toàn bộ v6 |
| **Effort** | Medium (khung service, proto, wiring, CI, deploy dev) |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-05 |
| **Trạng thái** | 📝 Đề xuất, chưa triển khai |
| **Phụ thuộc** | Không |
| **Mở khoá** | CR-REQ-002 và mọi CR còn lại của v6 |
| **Tác động** | `backend-go/go.work`, `backend-go/Makefile`, `backend-go/deploy/postgres-init-databases.sh`, `deploy/dev/docker-compose.yml`, `deploy/dev/scripts/migrate.sh`, `.github/workflows` (workflow mới), `backend-go/proto/orca/request/v1` (mới), `backend-go/services/request-service` (mới) |

---

## 1. Bối cảnh và vấn đề

1. Khảo sát 2026-10-05: không có bảng, entity, RPC nào cho Request, Solution, Approval trong `backend-go` (README v6 mục 1). `go.work` liệt kê 19 module service, không có `request-service`.
2. D1 (README v6) chốt tách service riêng. Không nhét vào `task-service` vì `task-service` sở hữu cây Task và thực thi; Request có vòng đời, cổng duyệt và nhịp thay đổi khác.
3. `mcp-service` là service mới gần nhất nhưng Postgres-only (CR-MCP-001, D2). Series này yêu cầu cả Postgres và MySQL, nên mẫu đúng là `notification-service` và `task-service`, đều có `adapter/postgres`, `adapter/mysql`, `migrations/postgres`, `migrations/mysql`.

## 2. Giải pháp đề xuất

### 2.1 Module và layout

Module `github.com/stablyai/orca-go/services/request-service` (mới). Sao cấu trúc `notification-service`:

```
services/request-service/                         (mới)
├── cmd/server/main.go                 # composition root, dialect switch như task-service
├── internal/config/config.go          # nhúng commonconfig.Base
├── internal/domain/                   # entity thuần, lỗi (CR-REQ-002 thêm file)
├── internal/usecase/ports.go          # cổng repo, TxRunner, OutboxWriter
├── internal/adapter/grpc/             # server RequestService, ApprovalService
├── internal/adapter/postgres/         # pgx
├── internal/adapter/mysql/            # database/sql
├── internal/adapter/eventbus/         # publisher (outbox relay), consumer (CR sau)
├── internal/adapter/grpcclient/       # task-service, ai-provider-service (CR sau)
├── migrations/postgres/               # golang-migrate
├── migrations/mysql/
└── deploy/Dockerfile                  # sao services/task-service/deploy/Dockerfile
```

Cấm tên `helpers`, `utils`, `common`, `misc` (AGENTS.md). Không thêm `max-lines` disable.

### 2.2 Cấu hình (`internal/config/config.go`)

Nhúng `commonconfig.Base` (`GRPC_PORT` mặc định 9090, `HTTP_PORT` 8080, `DATABASE_DSN`, `OTLP_ENDPOINT`). Trường riêng:

| Biến | Dùng cho | Mặc định |
|------|----------|----------|
| `DATABASE_CREDENTIALS_FILE` | `secrets.DatabaseCredentialsFromFile`, ưu tiên hơn `DATABASE_DSN` | rỗng |
| `NATS_URL` | `eventbus.Connect` | rỗng: tắt relay, ghi log cảnh báo như `task-service` |
| `TASK_SERVICE_ADDR`, `AI_PROVIDER_SERVICE_ADDR`, `PROJECT_SERVICE_ADDR` | dial ở CR sau; ở CR này chỉ khai báo, không dial | rỗng |
| `REQUEST_FLOW_ENABLED` | cờ `request_flow_enabled` (CR-REQ-025); ở CR này chỉ đọc và log | `false` |

DSN không phải `postgres://`, `postgresql://`, `mysql://`, `tidb://` làm service thoát ngay (lỗi từ `dbcapability.DetectDialectFromDSN`).

### 2.3 `main.go`

Thứ tự khởi động (theo `task-service/cmd/server/main.go`): logger → tracing → DSN từ file/env → `DetectDialectFromDSN` → `switch caps.Dialect` tạo `repo` (pgxpool hoặc `sql.Open("mysql", ...)`) và đăng ký health `postgres`/`mysql` → `eventbus.Connect` → `outbox.NewRelay(repo, pub, outbox.DefaultConfig, logger)` chạy goroutine, chờ bằng `WaitGroup` khi tắt → `grpc.NewServer(grpcmw.ChainUnary(logger))` → đăng ký gRPC health + `RequestService`, `ApprovalService` → HTTP `health.Handler()` (`/healthz`, `/readyz`). Dịch DSN MySQL sang driver DSN giống `toMySQLDriverDSN` của `task-service` (đọc hàm đó, không import chéo service; viết lại trong `cmd/server`).

### 2.4 Proto `proto/orca/request/v1/` (mới)

Hai file: `request.proto` (service `RequestService`) và `approval.proto` (service `ApprovalService`). `package orca.request.v1;`, `go_package = "github.com/stablyai/orca-go/proto/gen/go/orca/request/v1;requestv1"`.

Ở CR này khai báo đầy đủ **tên RPC** theo README v6 mục 3.6 nhưng chỉ ba RPC có message thật để service chạy được: `GetRequest(GetRequestRequest{id}) returns (GetRequestResponse{Request})`, `ListRequests(ListRequestsRequest{project_id, status, type, page_size, page_token})`. Các RPC còn lại: server trả `codes.Unimplemented` qua nhúng `UnimplementedRequestServiceServer`; message được CR sở hữu từng RPC định nghĩa (CR-REQ-004 `CreateRequest`, CR-REQ-005 `ClassifyRequest`/`ConfirmRequestType`/`ChangeRequestType`, CR-REQ-006 `ReturnToBacklog`/`ReopenRequest`/`CancelRequest`/`SpawnChildRequest`, CR-REQ-009 toàn bộ `ApprovalService`). Không khai báo RPC chưa có message để `buf breaking` không bị phá ở CR sau.

Chạy `buf lint` và `buf breaking` (cấu hình trong `proto/buf.yaml`), sinh stub vào `proto/gen/go/orca/request/v1`.

### 2.5 Hai bảng hạ tầng (migration `0001_init`)

Postgres, schema `request`; MySQL, database riêng không tiền tố (theo `task-service`).

| Bảng | Cột | Ghi chú |
|------|-----|---------|
| `outbox_events` | `id UUID/CHAR(36) PK`, `tenant_id`, `subject TEXT`, `occurred_at`, `version INT`, `payload JSONB/JSON`, `created_at`, `published_at NULL` | Sao `task/migrations/*/0005_outbox.up.sql`. Postgres: chỉ mục riêng phần `WHERE published_at IS NULL`. MySQL: chỉ mục `(published_at, created_at)` |
| `processed_events` | `event_id UUID/CHAR(36) PK`, `subject`, `processed_at` | Dedup consumer (CR-REQ-013), sao `notification-service/migrations/*/0002_processed_events.up.sql`; chỉ mục `processed_at` để dọn |

Bảng nghiệp vụ nằm ở CR-REQ-002 (`0002_request_core`). `outbox_events` có RLS trên Postgres như `task.task_sources` (`tenant_isolation` theo `app.tenant_id`).

### 2.6 Outbox

Adapter implement `outbox.Store` (`FetchUnpublished`, `MarkPublished`) cho cả hai dialect, kèm `InsertOutboxEvent` chạy trong cùng transaction với ghi nghiệp vụ (cổng `OutboxWriter`, `TxRunner` ở `usecase/ports.go`). Subject theo quy ước `orca.<service>.<entity>.<event>`; xem Câu hỏi mở Q1.

### 2.7 Đăng ký repo và triển khai

| Việc | Chỗ sửa |
|------|---------|
| Thêm `./services/request-service` | `backend-go/go.work` (giữ thứ tự chữ cái, giữa `project-service` và `scm-integration-service`) |
| Thêm vào `SERVICES` | `backend-go/Makefile` dòng 7-11 |
| Thêm database `request` | `backend-go/deploy/postgres-init-databases.sh` (biến `DATABASES`) |
| Service `request-service` và `migrate-request` | `deploy/dev/docker-compose.yml` (mẫu `task-service` dòng 325, `migrate-task` dòng 705) |
| Thêm `request` vào `SERVICES` | `deploy/dev/scripts/migrate.sh` |
| Workflow CI | `.github/workflows/backend-go-request-service.yml` (mới), sao `backend-go-task-service.yml` với `matrix.dialect: [postgres, mysql]` |
| `README.md` của service | Ghi mục "real vs stub": ở CR này RPC nào là thật, RPC nào `Unimplemented` |

## 3. Quyết định thiết kế

| Quyết định | Lý do |
|------------|-------|
| Hai dialect ngay từ đầu | Sửa sau tốn hơn (SQL đặc thù Postgres thấm vào repo); README v6 mục 6 |
| Không dial service khác ở CR này | Giữ CR nhỏ, test được mà không cần `task-service`; dial thêm ở CR dùng nó |
| Khai báo chỉ RPC có message | Tránh message nửa vời bị khoá bởi `buf breaking` |
| Tách `request.proto` và `approval.proto` | Hai service, hai chủ sở hữu CR (001 và 009) |
| Cờ `request_flow_enabled` chỉ đọc | Cờ thật thuộc CR-REQ-025; tránh hai nguồn sự thật |

## 4. Tiêu chí chấp nhận

- [ ] `make build`, `make vet`, `make test`, `make lint` xanh với module mới trong `go.work`.
- [ ] `buf lint` và `buf breaking` xanh; stub sinh vào `proto/gen/go/orca/request/v1`.
- [ ] Với `DATABASE_DSN` Postgres, service khởi động, `/healthz`, `/readyz` và gRPC health trả OK; với DSN MySQL cũng vậy.
- [ ] DSN scheme lạ làm tiến trình thoát với lỗi nêu rõ dialect không hỗ trợ.
- [ ] Migration `0001_init` chạy `up` rồi `down` sạch trên Postgres và MySQL bằng golang-migrate.
- [ ] Ghi một dòng `outbox_events` trong transaction rồi relay publish được lên NATS; hủy transaction thì không có dòng.
- [ ] Postgres: tenant A không đọc được dòng `outbox_events` của tenant B (RLS). MySQL: test repo chứng minh mọi truy vấn lọc `tenant_id`.
- [ ] Gọi RPC chưa triển khai trả `Unimplemented`, không panic.
- [ ] Workflow CI chạy hai dialect và xanh.
- [ ] Tên file, thư mục không có `helpers`, `utils`, `common`, `misc`; không có `max-lines` disable mới.

## 5. Kiểm thử

- **Unit:** `config` (đọc env, mặc định); `main` helper chuyển DSN MySQL; use case ghi outbox với repo giả.
- **Integration (`-tags=integration`, testcontainers), chạy hai dialect:** migration up/down; `FetchUnpublished` theo thứ tự `created_at`; `MarkPublished`; hai relay đồng thời không mất sự kiện (at-least-once, giao lặp được chấp nhận); RLS (Postgres).
- **Hợp đồng:** `buf breaking` so với `main`; test gRPC khởi tạo server in-process và gọi `GetRequest` với id không tồn tại, mong `NotFound` (phụ thuộc CR-REQ-002 để có kho; trước đó mong `Unimplemented`).
- Chưa chạy bất kỳ test nào ở thời điểm viết CR.

## 6. Rủi ro và điểm chưa kiểm chứng

- Chưa kiểm chứng `golang-migrate` MySQL driver trong CI cho database tên `request`; workflow task-service dùng cùng cách nên rủi ro thấp.
- `go.work.sum` và `go.mod` của module mới phải đồng bộ (`make tidy-all`); chưa chạy.
- Tên database `request` có thể trùng tên dành riêng ở một số môi trường MySQL; chưa kiểm chứng. Nếu vướng, đổi thành `requests` ở toàn series.
- Dockerfile sao từ task-service có `COPY policy`; request-service chưa dùng OPA, bỏ dòng này nếu build lỗi (xem CR-REQ-010 cho quyền duyệt).

## 7. Câu hỏi mở

- **Q1.** README v6 mục 3.7 ghi `orca.request.*` với sự kiện `request.created`; quy ước repo cho subject đầy đủ `orca.request.request.created` (xem `orca.task.task.statuschanged`, `orca.mcp.session.closed`). Cần chốt dạng subject; hai CR này giả định dạng đầy đủ `orca.request.<entity>.<event>`.
- **Q2.** README v6 mục 3.5 ghi bảng `outbox`; repo dùng `outbox_events`. Cần sửa README.
- **Q3.** Mô hình xác thực service-to-service (`common/internalcaller`) cho gọi `ReportTaskOutcome` chưa được chốt ở README; xem CR-REQ-013.

## 8. Tham chiếu

- `/opt/repos/orca/docs/crs/v6/README.md`
- `/opt/repos/orca/docs/crs/v5/mcp-service-foundation/CR-MCP-001-scaffold-mcp-service.md`
- `/opt/repos/orca/backend-go/services/task-service/cmd/server/main.go` (dialect switch, relay)
- `/opt/repos/orca/backend-go/services/notification-service/migrations/postgres/0002_processed_events.up.sql` và bản `mysql`
- `/opt/repos/orca/backend-go/services/task-service/migrations/postgres/0005_outbox.up.sql` và bản `mysql`
- `/opt/repos/orca/backend-go/common/outbox/outbox.go`, `common/dbcapability/capability.go`, `common/eventbus/eventbus.go`, `common/grpcmw/grpcmw.go`, `common/health/health.go`
- `/opt/repos/orca/backend-go/go.work`, `backend-go/Makefile`, `backend-go/deploy/postgres-init-databases.sh`
- `/opt/repos/orca/deploy/dev/docker-compose.yml`, `deploy/dev/scripts/migrate.sh`
- `/opt/repos/orca/.github/workflows/backend-go-task-service.yml`
- `/opt/repos/orca/docs/crs/v4/multi-database/CR-DB-002-dialect-capability-layer-foundation.md`
