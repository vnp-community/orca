# BE-REQ-SOL-001: Dựng `request-service`: module Go, proto, DB hai dialect, outbox, wiring, CI

> **📋 Proposed.** Chưa triển khai, chưa chạy test nào. Điều kiện tiên quyết của toàn bộ series v6. Không phụ thuộc solution nào.

**CR:** [CR-REQ-001](../../../../../../docs/crs/v6/request-service-foundation/CR-REQ-001-scaffold-request-service.md)
**Service:** `request-service` (mới) · `proto` · `go.work` / `Makefile` / `deploy/*` · `.github/workflows`
**TDD tham chiếu:** [`arch/02`](../../../../tdd/architecture/02-microservices-decomposition.md) (ranh giới service), [`arch/03`](../../../../tdd/architecture/03-clean-architecture-guidelines.md) (layout), [`arch/05`](../../../../tdd/architecture/05-data-architecture.md) (DB-per-service, RLS, outbox), [`arch/08`](../../../../tdd/architecture/08-inter-service-communication.md) (subject, outbox, dedup), [`arch/09`](../../../../tdd/architecture/09-observability-reliability.md) (health, log, trace)

---

## 1. Trạng thái hiện tại (re-verify)

Đã đọc: `backend-go/go.work`, `backend-go/Makefile`, `deploy/postgres-init-databases.sh`, `services/task-service/{cmd/server/main.go, internal/config/config.go, internal/adapter/{postgres,mysql}/outbox.go, deploy/Dockerfile, migrations/*/0005_outbox.up.sql, 0012, 0014}`, `services/notification-service/migrations/*/0002_processed_events.up.sql`, `services/mcp-service/{cmd/server/main.go, internal/adapter/postgres/tenant_tx.go, migrations/postgres/0001_init.up.sql, deploy/Dockerfile}`, `common/{outbox,eventbus,dbcapability,grpcmw,tenant,apperrors,health,internalcaller}`, `proto/{buf.yaml,buf.gen.yaml}`, `deploy/dev/{docker-compose.yml,scripts/migrate.sh,scripts/build-local.sh}`, `.github/workflows/backend-go-{task,mcp}-service.yml`.

Xác nhận đúng: `go.work` có 19 module service, không có `request-service`; không có thư mục `proto/orca/request`; `dbcapability.DetectDialectFromDSN` chấp nhận `postgres://`, `postgresql://`, `mysql://`, `tidb://`; `task-service/cmd/server/main.go` có `switch caps.Dialect` và hàm `toMySQLDriverDSN` (dòng 507 trở đi); `outbox.Store` có hai phương thức `FetchUnpublished`, `MarkPublished`.

### Correction relative to CR-REQ-001

| # | CR nói | Mã thật | Xử lý trong solution này |
|---|--------|---------|--------------------------|
| C1 | `outbox_events` có RLS "như `task.task_sources`" | RLS của `task-service` **không bao giờ chạy**: không nơi nào gọi `set_config('app.tenant_id', ...)` (chính `postgres/share_link.go:17` ghi nhận) và pool kết nối bằng owner. `mcp-service` mới làm RLS thật: `FORCE`, `NULLIF(current_setting(...))`, `withTenantTx`, hai policy `relay_read` và `relay_mark_published` | Theo mẫu `mcp-service` (xem 2.D). Tiêu chí "tenant A không đọc được B" chỉ đạt được theo cách này |
| C2 | `processed_events` chỉ có `event_id` làm PK | README v6 mục 8 điểm 3 yêu cầu mọi bảng có `tenant_id`; `mcp-service` dùng PK `(tenant_id, event_id)` | `processed_events(tenant_id, event_id)`, PK kép |
| C3 | Không nhắc `EnsureStream` | Service khác tạo stream trong `main.go` (`mcp-service` dòng 128: `pub.EnsureStream(ctx, "MCP", []string{"orca.mcp.>"})`). Không có stream thì consumer của CR-REQ-005 không đăng ký được | `pub.EnsureStream(ctx, "REQUEST", []string{"orca.request.>"})` trong `main.go` |
| C4 | Go version không nêu | `go.work` ghi `go 1.26.0`, mọi `go.mod` service ghi `go 1.25.0`, Dockerfile dùng `golang:1.25-bookworm` | `go.mod` của module mới: `go 1.25.0`, giữ Dockerfile `golang:1.25-bookworm` |
| C5 | Dockerfile "bỏ `COPY policy` nếu build lỗi" | Mẫu `mcp-service` vẫn `COPY policy ./policy` ở stage build (cần vì `go.work` tham chiếu cả cây) nhưng không copy `orca-authz` sang runtime | Giữ `COPY policy ./policy` ở stage build, bỏ dòng copy bundle `orca-authz` ở runtime |
| C6 | `docker-compose.yml` mẫu `task-service` | Stack dev chạy binary mount: `./bin/<svc>/orca` vào image chung, migration ở `./bin/<svc>/migrations`; `build-local.sh` dòng 43 có danh sách service riêng; `migrate.sh` dòng 27 có `SERVICES` riêng | Thêm `request-service` vào cả `build-local.sh` lẫn `migrate.sh` (CR chỉ nhắc `migrate.sh`) |

## 2. Giải pháp

### A. Cây thư mục `backend-go/services/request-service/` (mới)

```
go.mod                          # module github.com/stablyai/orca-go/services/request-service ; go 1.25.0
README.md                       # bảng "real vs stub" (xem F)
cmd/server/main.go              # composition root; toMySQLDriverDSN viết lại tại đây
cmd/server/mysql_dsn.go         # toMySQLDriverDSN + test (tách file để main.go gọn)
internal/config/config.go
internal/domain/outbox_event.go # OutboxEvent{ID, TenantID, Subject, OccurredAt, Version, Payload []byte}
internal/domain/outbox_subjects.go  # hằng subject, ví dụ SubjectRequestCreated = "orca.request.request.created"
internal/usecase/ports.go       # TxRunner, OutboxWriter, (CR-REQ-002 thêm repo)
internal/adapter/grpc/server.go # RequestServiceServer nhúng Unimplemented, ApprovalServiceServer nhúng Unimplemented
internal/adapter/postgres/{repository.go,tx.go,outbox.go}
internal/adapter/mysql/{repository.go,tx.go,outbox.go}
internal/adapter/eventbus/      # (rỗng ở CR này; consumer thêm ở CR-REQ-005, 013)
internal/adapter/grpcclient/    # (rỗng; client task-service, infra-fleet, issuetracking thêm ở CR sau)
migrations/postgres/0001_init.{up,down}.sql
migrations/mysql/0001_init.{up,down}.sql
deploy/Dockerfile
```

Thư mục rỗng không commit được vào git: `eventbus/` và `grpcclient/` chỉ tạo khi có file đầu tiên (CR sau). Cấm `helpers`, `utils`, `common`, `misc`; không thêm `max-lines` disable (AGENTS.md).

### B. Cấu hình `internal/config/config.go`

```go
type Config struct {
    commonconfig.Base                       // GRPC_PORT 9090, HTTP_PORT 8080, DATABASE_DSN, OTLP_ENDPOINT
    DatabaseCredentialsFile string          // DATABASE_CREDENTIALS_FILE, ưu tiên hơn DATABASE_DSN
    NATSURL                 string          // NATS_URL, rỗng thì không chạy relay (log cảnh báo như task-service)
    TaskServiceAddr         string          // TASK_SERVICE_ADDR (chỉ đọc, chưa dial)
    AIProviderServiceAddr   string          // AI_PROVIDER_SERVICE_ADDR (chỉ đọc, chưa dial)
    ProjectServiceAddr      string          // PROJECT_SERVICE_ADDR (chỉ đọc, chưa dial)
    RequestFlowEnabled      bool            // REQUEST_FLOW_ENABLED, mặc định false; CR-REQ-025 mới là chủ cờ
}
```

`config.Load()` theo khuôn `task-service/internal/config/config.go`. Biến địa chỉ khai báo nhưng không dial: giữ test được mà không cần service khác.

### C. `main.go`

Thứ tự (theo `task-service/cmd/server/main.go`): logger, `apperrors.SetLogger`, tracing, DSN (file Vault trước, env sau), `DetectDialectFromDSN`, `switch caps.Dialect`:

```go
type store interface {
    usecase.TxRunner
    usecase.OutboxWriter
    outbox.Store
}
var repo store
switch caps.Dialect {
case dbcapability.DialectPostgres:
    pool, _ := pgxpool.New(ctx, dsn)
    repo = requestpg.New(pool)
    healthSrv.Register("postgres", func() error { return pool.Ping(ctx) })
case dbcapability.DialectMySQL:
    driverDSN, _ := toMySQLDriverDSN(dsn)
    db, _ := sql.Open("mysql", driverDSN)
    repo = requestmysql.New(db)
    healthSrv.Register("mysql", func() error { return db.PingContext(ctx) })
default:
    return fmt.Errorf("unsupported database dialect: %s", caps.Dialect)
}
```

Sau đó: `eventbus.Connect` (lỗi thì log cảnh báo, không thoát), `pub.EnsureStream(ctx, "REQUEST", []string{"orca.request.>"})`, `outbox.NewRelay(repo, pub, outbox.DefaultConfig, logger)` chạy goroutine với `WaitGroup`, `grpc.NewServer(grpcmw.ChainUnary(logger), grpcmw.StatsHandler())`, đăng ký gRPC health, `RequestService`, `ApprovalService`, HTTP `health.New().Handler()` (`/healthz`, `/readyz`). Tắt êm: `GracefulStop`, `outboxRelayWG.Wait()`.

DSN scheme lạ: `DetectDialectFromDSN` trả lỗi, `main` thoát mã khác 0 với thông báo nêu dialect không hỗ trợ.

### D. Giao dịch và RLS (khác CR, xem C1)

Port (đặt ở `usecase/ports.go`):

```go
type TxRunner interface {
    // InTx chạy fn trong một giao dịch. Nếu ctx đã mang giao dịch (lồng nhau) thì fn tham gia
    // giao dịch đó, không mở giao dịch mới: CreateRequest gọi TransitionRequest trong cùng commit.
    InTx(ctx context.Context, fn func(ctx context.Context) error) error
}
type OutboxWriter interface {
    InsertOutboxEvent(ctx context.Context, ev domain.OutboxEvent) error // dùng giao dịch của ctx nếu có
}
```

**Executor lấy từ ctx**, không truyền repo theo callback. Lý do: `task-service` dùng `RunInTx(fn(ctx, tasks, edges))` hand-off repo có phạm vi giao dịch, mô hình này không mở rộng được khi một use case chạm 5 repo và gọi use case khác (task-service tự ghi nhận không lồng được `RunInTx`, `usecase/add_edge.go:63-66`). CR-REQ-002 viết "mẫu task-service", nhưng mẫu thật khác; solution này chọn executor trong ctx và nêu rõ ở đây.

Postgres (`adapter/postgres/tx.go`):

```go
type txKey struct{}
func (r *Repository) InTx(ctx context.Context, fn func(context.Context) error) error {
    if _, ok := ctx.Value(txKey{}).(pgx.Tx); ok { return fn(ctx) } // tham gia
    tenantID, err := tenant.RequireTenantID(ctx); if err != nil { return err }
    tx, err := r.pool.Begin(ctx); if err != nil { return err }
    defer func() { _ = tx.Rollback(ctx) }()
    if _, err := tx.Exec(ctx, `SELECT set_config('app.tenant_id', $1, true)`, tenantID); err != nil { return err }
    if err := fn(context.WithValue(ctx, txKey{}, tx)); err != nil { return err }
    return tx.Commit(ctx)
}
// exec trả tx của ctx; nếu không có thì bọc từng câu lệnh trong giao dịch ngắn có set_config.
```

MySQL (`adapter/mysql/tx.go`): cùng hình dạng với `*sql.Tx`, không có `set_config` (không có RLS, `SupportsRLS=false`); mọi truy vấn ghi `tenant_id = ?` ở `WHERE`.

`outbox.Store` của Postgres chạy trong `withRelayTx` (`set_config('app.relay','on',true)`), chỉ `FetchUnpublished` và `MarkPublished` được dùng.

### E. Migration `0001_init` (hai dialect)

Postgres (`migrations/postgres/0001_init.up.sql`), schema `request`:

```sql
CREATE SCHEMA IF NOT EXISTS request;
CREATE TABLE request.outbox_events (
    id UUID PRIMARY KEY, tenant_id UUID NOT NULL, subject TEXT NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL, version INT NOT NULL, payload JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(), published_at TIMESTAMPTZ,
    seq BIGINT GENERATED ALWAYS AS IDENTITY);   -- thứ tự chèn trong cùng giao dịch (xem D2 ở mục 3)
CREATE INDEX idx_request_outbox_unpublished ON request.outbox_events (created_at, seq) WHERE published_at IS NULL;
CREATE TABLE request.processed_events (
    tenant_id UUID NOT NULL, event_id UUID NOT NULL, subject TEXT NOT NULL,
    processed_at TIMESTAMPTZ NOT NULL DEFAULT now(), PRIMARY KEY (tenant_id, event_id));
CREATE INDEX idx_request_processed_events_at ON request.processed_events (processed_at);
-- RLS thật: ENABLE + FORCE, NULLIF cho kết nối pool còn app.tenant_id = '' (khuôn mcp-service 0001)
-- policy tenant_isolation USING + WITH CHECK; relay_read (SELECT) và relay_mark_published (UPDATE) theo app.relay
```

MySQL: `CHAR(36)`, `TIMESTAMP(6)`, `JSON`, `ENGINE=InnoDB`, `seq BIGINT NOT NULL AUTO_INCREMENT` + `UNIQUE KEY (seq)`, chỉ mục `(published_at, created_at, seq)` thay cho partial index, không có schema (database riêng tên `request`), không RLS. Down: Postgres `DROP SCHEMA IF EXISTS request CASCADE`; MySQL `DROP TABLE` ngược thứ tự.

Tên bảng outbox là `outbox_events` (README v6 mục 8 điểm 3), không phải `outbox`.

### F. Proto `proto/orca/request/v1/{request.proto,approval.proto}`

Chỉ khai báo RPC có message thật (như `BE-MCP-SOL-001` §Quyết định 4):

```proto
service RequestService {
  rpc GetRequest(GetRequestRequest) returns (GetRequestResponse);
  rpc ListRequests(ListRequestsRequest) returns (ListRequestsResponse);
}
message GetRequestRequest { string id = 1; }
message GetRequestResponse { Request request = 1; }
message ListRequestsRequest {
  string project_id = 1; repeated string status = 2; repeated string type = 3;
  int32 page_size = 4; string page_token = 5;   // 6..8 dành cho CR-REQ-004 (source_provider, source_site, source_ref)
}
message ListRequestsResponse { repeated Request requests = 1; string next_page_token = 2; }
```

Message `Request` (id, project_id, number, title, body, các cột nguồn, type, type_source, size, urgency, confidence, classification_reason, status, returned_from_stage, return_reason, plan_task_id, reporter_id, created_at, updated_at, version) do TASK-REQ-001-02 định nghĩa; trường 24 trở đi dành cho `source_hints` (CR-REQ-004) và `returned_category` (CR-REQ-006). `approval.proto` ở CR này chỉ có `service ApprovalService {}` rỗng để `RegisterApprovalServiceServer` có chỗ gọi; RPC do CR-REQ-009 thêm. Nếu `buf lint` từ chối service rỗng thì bỏ file và dời đăng ký sang CR-REQ-009.

`status` và `type` trong `ListRequestsRequest` là `repeated` ngay từ đầu (CR ghi số ít) vì `ListFilter` của CR-REQ-002 nhận mảng và view backlog cần nhiều trạng thái; đổi sang số ít sau này mới phá `buf breaking`.

### G. Đăng ký repo, deploy, CI

| Việc | Chỗ sửa |
|------|---------|
| `./services/request-service` | `backend-go/go.work` (giữa `project-service` và `scm-integration-service`) |
| `request-service` vào `SERVICES` | `backend-go/Makefile` dòng 7 đến 11 |
| `request` vào `DATABASES` | `backend-go/deploy/postgres-init-databases.sh` |
| `request-service` và `migrate-request` | `deploy/dev/docker-compose.yml` (mẫu `task-service` dòng 325, `migrate-task` dòng 705) |
| `request` vào `SERVICES` | `deploy/dev/scripts/migrate.sh` dòng 27 |
| `request-service` vào danh sách build | `deploy/dev/scripts/build-local.sh` dòng 43 |
| Workflow | `.github/workflows/backend-go-request-service.yml` (mới) từ `backend-go-task-service.yml`, `matrix.dialect: [postgres, mysql]`, thêm đường dẫn `backend-go/proto/orca/request/**` |

## 3. Quyết định thiết kế

| Quyết định | Lý do |
|------------|-------|
| Hai dialect ngay | Sửa sau tốn hơn; README v6 mục 6 |
| Executor trong ctx, `InTx` tham gia khi lồng | `CreateRequest` gọi `TransitionRequest` cùng commit (CR-REQ-004); không cần bộ repo theo callback |
| RLS thật theo `mcp-service` | RLS kiểu `task-service` không chạy, tiêu chí cách ly tenant sẽ chỉ đạt giả |
| `processed_events` có `tenant_id` | Quy ước README v6 mục 8; cùng khuôn `mcp-service` |
| Proto chỉ khai báo RPC có message | Tránh khoá message nửa vời bằng `buf breaking` |
| Cờ `REQUEST_FLOW_ENABLED` chỉ đọc và log | Chủ cờ là CR-REQ-025 |
| D2: cột `seq` và `ORDER BY created_at, seq` cho outbox | `now()` Postgres cố định theo giao dịch nên hai sự kiện ghi trong một giao dịch (`created` rồi `status_changed`) cùng `created_at`; relay sắp theo `created_at` sẽ phát sai thứ tự và consumer CR-REQ-005 có thể thấy `classifying` trước `created`. `task-service` không gặp vì ghi một sự kiện mỗi giao dịch. Cột `seq` là điểm lệch so với mẫu `task-service`/`mcp-service` |

## 4. Phụ thuộc và thứ tự

Không phụ thuộc. Mở khoá CR-REQ-002 và mọi CR còn lại. Thứ tự task: 01 (module, config, workspace) trước 02 (proto), 03 (migration), 04 (adapter), rồi 05 (main, gRPC), 06 (deploy, CI). 02 và 03 song song được. Chi tiết ở [tasks/README](../tasks/README.md).

## 5. Kiểm thử

- **Unit:** `config.Load` (mặc định, ghi đè env); `toMySQLDriverDSN` (`mysql://`, `tidb://`, scheme lạ, thiếu host); `main` thoát khi DSN lạ; server gRPC trả `Unimplemented` cho RPC chưa viết.
- **Integration (`-tags=integration`, testcontainers), chạy từng dialect:** migration up, down, up; `InsertOutboxEvent` trong `InTx` rồi rollback thì không còn dòng; `FetchUnpublished` theo `created_at`; `MarkPublished`; hai relay đồng thời không mất sự kiện; Postgres với role `NOSUPERUSER NOBYPASSRLS`: tenant A không đọc được dòng của B, kể cả bằng SQL trực tiếp; MySQL: test chứng minh truy vấn lọc `tenant_id`.
- **Hợp đồng:** `buf lint`, `buf breaking --against '.git#branch=main,subdir=backend-go/proto'`. Lưu ý Makefile `proto-lint` kết thúc bằng `|| true`, nên CI mới phải gọi `buf breaking` trực tiếp.
- **Chưa chạy bất kỳ test nào.**

## 6. Rủi ro và điểm chưa kiểm chứng

- Chưa kiểm chứng `golang-migrate` MySQL driver với database tên `request`; tên có thể trùng từ dành riêng ở môi trường nào đó, nếu vướng đổi `requests` toàn series.
- `go.work.sum` và `go.mod` phải đồng bộ (`make tidy-all`); chưa chạy.
- Role ứng dụng kết nối Postgres trong môi trường dev có phải owner hay `NOBYPASSRLS` chưa kiểm; nếu là owner thì `FORCE` vẫn áp dụng, nếu là superuser thì RLS bị bỏ qua. Cần xác nhận với vận hành.
- Stack dev mount binary `./bin/request-service/orca`: script đóng gói (`build-local.sh`) phải sinh cả `migrations`; chưa kiểm chứng.
- Tên tool `buf` trên CI: Makefile hiện bỏ qua lỗi lint; PR có thể xanh dù proto sai.

## 7. Câu hỏi mở

- **Q1.** Dạng subject: README v6 mục 3.7 ghi `request.created`, repo dùng `orca.<service>.<entity>.<event>`. Mục 8 điểm 4 chốt `orca.request.<entity>.<event>`; solution theo đó.
- **Q2.** `common/internalcaller` cho `ReportTaskOutcome` (CR-REQ-013): chưa chốt, không thuộc solution này.
- **Q3.** Có cần `approval.proto` rỗng ở CR này (xem F), hay dời sang CR-REQ-009 hoàn toàn.

## 8. Tham chiếu

- `/opt/repos/orca/docs/crs/v6/README.md` mục 6 và 8
- `/opt/repos/orca/backend-go/services/task-service/cmd/server/main.go`
- `/opt/repos/orca/backend-go/services/task-service/internal/config/config.go`
- `/opt/repos/orca/backend-go/services/mcp-service/internal/adapter/postgres/tenant_tx.go`
- `/opt/repos/orca/backend-go/services/mcp-service/migrations/postgres/0001_init.up.sql`
- `/opt/repos/orca/backend-go/services/task-service/migrations/postgres/0005_outbox.up.sql` và bản `mysql`
- `/opt/repos/orca/backend-go/services/notification-service/migrations/postgres/0002_processed_events.up.sql`
- `/opt/repos/orca/backend-go/common/outbox/outbox.go`, `common/dbcapability/capability.go`, `common/eventbus/eventbus.go`, `common/grpcmw/grpcmw.go`, `common/health/health.go`
- `/opt/repos/orca/backend-go/go.work`, `backend-go/Makefile`, `backend-go/deploy/postgres-init-databases.sh`
- `/opt/repos/orca/deploy/dev/docker-compose.yml`, `deploy/dev/scripts/{migrate.sh,build-local.sh}`
- `/opt/repos/orca/.github/workflows/backend-go-task-service.yml`, `backend-go-mcp-service.yml`
- `/opt/repos/orca/specs/backend-go/crs/v5/mcp-service-foundation/solutions/BE-MCP-SOL-001-scaffold-mcp-service.md` (mẫu)
