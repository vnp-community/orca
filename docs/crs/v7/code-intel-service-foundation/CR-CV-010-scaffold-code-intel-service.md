# CR-CV-010 — Dựng `code-intel-service`: khung service Go, proto `orca.codeintel.v1`, outbox, wiring và CI

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-CV-010 |
| **Tên** | Tạo service Go mới `code-intel-service` (gRPC, DB riêng, hai dialect Postgres + MySQL) theo layout `notification-service`, kèm outbox kiểu `mcp-service` |
| **Loại** | Feature (service mới) |
| **Priority** | 🔴 P0, điều kiện tiên quyết của toàn bộ v7 phía backend |
| **Effort** | Medium (khung service, proto rỗng-có-chủ, wiring 8 file, CI, deploy dev) |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-05 |
| **Trạng thái** | 📝 Đề xuất, chưa triển khai |
| **Phụ thuộc** | Không |
| **Mở khoá** | CR-CV-011, 012, 013 và mọi CR backend còn lại của v7 (020 đến 024, 030 đến 038, 040) |
| **Tác động** | `backend-go/go.work`, `backend-go/Makefile`, `backend-go/deploy/postgres-init-databases.sh`, `deploy/dev/docker/postgres/init-databases.sh`, `deploy/dev/docker-compose.yml`, `deploy/dev/scripts/migrate.sh`, `deploy/dev/scripts/build-local.sh`, `backend-go/ci/check-opa-bundle-in-images.sh`, `.github/workflows/backend-go-code-intel-service.yml` (mới), `backend-go/proto/orca/codeintel/v1` (mới), `backend-go/proto/gen/go/orca/codeintel/v1` (sinh), `backend-go/services/code-intel-service` (mới) |

---

## 1. Bối cảnh và vấn đề

Khảo sát 2026-10-05 (đọc code thật):

1. Không có `code-intel-service`, không có `backend-go/proto/orca/codeintel` (thư mục `backend-go/proto/orca` hiện có 16 package, không có `codeintel`). `backend-go/go.work` liệt kê `common`, `proto`, `cmd/orca-cli` và 19 service, không có service này.
2. D3 ([README v7](../README.md) mục 2) chốt service Go riêng để chuẩn hoá, cache, phân quyền và lưu trạng thái review. Không đặt vào `infra-fleet-service` (giữ vai trò vận chuyển) và không đặt vào `mcp-service` (O2: không đi qua chính sách MCP ở MVP).
3. O1 yêu cầu hai dialect. Mẫu hai dialect gần nhất là `notification-service` (`cmd/server/main.go` dòng 91 đến 159: `secrets.DatabaseCredentialsFromFile` → `dbcapability.DetectDialectFromDSN` → `switch caps.Dialect` tạo `pgxpool` hoặc `sql.Open("mysql", ...)`, đăng ký health `postgres`/`mysql`). `task-service` có thêm outbox relay (`cmd/server/main.go` dòng 391 đến 420: `eventbus.Connect` → `EnsureStream` → `outbox.NewRelay` chạy goroutine, chờ `WaitGroup` khi tắt). `mcp-service` là mẫu RLS có hiệu lực và outbox ghi cùng transaction nhưng chỉ Postgres (`migrations/postgres` không có thư mục `mysql`).
4. Series v6 (CR-REQ-001) đã sao mẫu này nhưng khi đối chiếu code thật có các chỗ lệch; CR này sửa thay vì sao nguyên văn (bảng 1.1).

### 1.1 Điểm đã kiểm lại so với CR-REQ-001 (v6)

| # | CR-REQ-001 giả định | Code thật | Hệ quả ở CR này |
|---|---|---|---|
| 1 | "RLS như `task.task_sources`" cô lập tenant | `task-service` không bao giờ `SET app.tenant_id` (chú thích ở `task-service/internal/adapter/postgres/share_link.go` dòng 10 đến 24: grep ra 0 lời gọi `set_config`), RLS ở đó không có tác dụng | Dùng mẫu `mcp-service`: `set_config('app.tenant_id', $1, true)` trong transaction (`mcp-service/internal/adapter/postgres/tenant_tx.go`) + `FORCE ROW LEVEL SECURITY` + `NULLIF(current_setting(...), '')::uuid` (`mcp-service/migrations/postgres/0001_init.up.sql` dòng 44 đến 57) |
| 2 | Kết nối bằng role app, RLS chặn tenant | Compose dev kết nối bằng `orca`, là superuser của image `postgres:16-alpine` (`POSTGRES_USER: orca`, `deploy/dev/docker-compose.yml` dòng 129); `mcp-service/README.md` dòng 78 đến 90 ghi rõ RLS không có hiệu lực ở dev, production phải dùng role `NOSUPERUSER NOBYPASSRLS` | Tiêu chí "tenant A không đọc được tenant B" chỉ kiểm bằng integration test với role không phải superuser (mục 4, 5); lọc `tenant_id` ở tầng ứng dụng vẫn bắt buộc |
| 3 | `TxRunner.InTx(ctx, fn(ctx))`, "mẫu `task-service`" | `task-service/internal/usecase/ports.go` dòng 416 đến 425: `RunInTx(ctx, fn(ctx, taskRepo, edgeRepo))`, không phải executor-trong-context | Dùng mẫu `mcp-service`: phương thức ghi của repository nhận `events []OutboxRecord` và ghi outbox cùng transaction (`mcp-service/internal/usecase/external_server_ports.go` dòng 44 đến 55) |
| 4 | "đăng ký gRPC health" | Không service nào đăng ký `grpc_health_v1` (grep `grpc_health_v1`, `grpc/health` trong `common`, `services` ra rỗng); chỉ có HTTP `/healthz`, `/readyz` (`common/health`) và `reflection.Register` | Health chỉ qua HTTP; không thêm gRPC health |
| 5 | `grpcmw.ChainUnary` đủ | `ChainUnary` chỉ gồm interceptor unary (`common/grpcmw/grpcmw.go`); không có interceptor stream nào trong `common`, `services` ngoài `internalcaller.StreamGuard`. RPC stream không được tách `x-orca-tenant-id` khỏi metadata. `mcp-service` xác nhận: "StreamEvents has no stream interceptor in this service, so it reads the identity metadata itself" (`mcp-service/internal/adapter/grpc/governance_server.go` dòng 272 đến 285) | Thêm interceptor stream cục bộ (mục 2.4) vì `CodeIntelService.StreamCodeIntelEvents` là server-streaming (README v7 mục 3.6) |
| 6 | `make proto-lint` chặn PR | Target `proto-lint` kết thúc bằng `|| true` (`backend-go/Makefile`), workflow `backend-go-mcp-service.yml` ghi chú đúng điều đó và gọi `buf lint`, `buf breaking` trực tiếp | Workflow mới gọi `buf` trực tiếp, không dùng `make proto-lint` |
| 7 | Wiring: `go.work`, Makefile, một script init DB, compose, `migrate.sh`, CI | Còn thiếu: `deploy/dev/docker/postgres/init-databases.sh` (file thật được compose dev mount, `docker-compose.yml` dòng 135; khác `backend-go/deploy/postgres-init-databases.sh` của `backend-go/docker-compose.yml` dòng 23), `deploy/dev/scripts/build-local.sh` (mảng `ALL_SERVICES`, dòng 37 đến 44) và `backend-go/ci/check-opa-bundle-in-images.sh` (mảng `svcs`) | Mục 2.8 liệt kê đủ |
| 8 | Dockerfile sao từ `task-service` | Dev compose không build Dockerfile: mount binary vào image distroless (`docker-compose.yml` dòng 75 đến 79, `./bin/<svc>/orca:/app/bin/orca:ro`); migration dev chỉ lấy `migrations/postgres` (`build-local.sh` dòng 108 đến 113) | MySQL chỉ được kiểm bằng CI; Dockerfile phục vụ prod |
| 9 | Tên DB | Tên DB không có dấu gạch (`scmintegration`, `issuetracking`, `aiprovider`) và `migrate.sh` đặt tên theo khoá riêng (`infra`, `scm`) | DB và schema tên `codeintel`; khoá `migrate.sh` là `codeintel` |

## 2. Giải pháp đề xuất

### 2.1 Module và layout

Module `github.com/stablyai/orca-go/services/code-intel-service` (mới), `go 1.25.0` như mọi module khác (`go.work` ghi `go 1.26.0`, các `go.mod` ghi `1.25.0`, CI dùng Go 1.25; giữ 1.25.0).

```
services/code-intel-service/                        (mới)
├── cmd/server/
│   ├── main.go                  # composition root, dialect switch, relay, shutdown
│   └── mysql_driver_dsn.go      # toMySQLDriverDSN (viết lại, không import chéo service)
├── internal/config/config.go    # nhúng commonconfig.Base
├── internal/domain/             # entity, lỗi, OutboxRecord (CR-CV-011 thêm file)
├── internal/usecase/ports.go    # cổng repo, OutboxRecord, Clock
├── internal/adapter/grpc/       # server.go (CodeIntelService), tenant_stream_interceptor.go
├── internal/adapter/postgres/   # pgx: repository.go, tenant_tx.go, outbox.go
├── internal/adapter/mysql/      # database/sql: repository.go, outbox.go
├── internal/adapter/eventbus/   # nối outbox.Relay (không có consumer ở CR này)
├── internal/adapter/grpcclient/ # dial.go (CR-CV-012, 021 thêm client project/infra-fleet)
├── migrations/postgres/         # golang-migrate
├── migrations/mysql/
├── deploy/Dockerfile            # sao services/task-service/deploy/Dockerfile
└── README.md                    # bảng "thật / chưa làm" của từng RPC
```

Không dùng tên `helpers`, `utils`, `common`, `misc` (AGENTS.md); không thêm `max-lines` disable. `toMySQLDriverDSN` đã được sao nguyên văn giữa các service (chú thích của `notification-service/cmd/server/main.go` dòng 324 đến 330); lần này sao tiếp vào `mysql_driver_dsn.go` chứ không đưa lên `common` để không đổi blast radius của `common`. Xem Q2.

### 2.2 Cấu hình (`internal/config/config.go`)

Nhúng `commonconfig.Base` (`GRPC_PORT` 9090, `HTTP_PORT` 8080, `DATABASE_DSN`, `OTLP_ENDPOINT`, đọc bởi `commonconfig.LoadBase("code-intel-service")`). `common/config` chỉ xuất `StringEnv`; không có `BoolEnv`/`DurationEnv` (hàm `intEnv` không xuất), nên `config.go` tự viết bộ phân tích bool, duration, int giống `mcp-service/internal/config/config.go` (`boolEnv`).

| Biến | Dùng cho | Mặc định |
|------|----------|----------|
| `DATABASE_CREDENTIALS_FILE` | `secrets.DatabaseCredentialsFromFile`; không có file thì dùng `DATABASE_DSN` (`common/secrets/vault.go` dòng 56 đến 67; cả hai rỗng thì lỗi) | `/vault/secrets/database-credentials` |
| `NATS_URL` | `eventbus.Connect` | `nats://localhost:4222`; lỗi kết nối chỉ cảnh báo và tắt relay như `task-service` |
| `PROJECT_SERVICE_ADDR`, `INFRA_FLEET_SERVICE_ADDR`, `GIT_GATEWAY_SERVICE_ADDR`, `AUTH_SERVICE_ADDR` | khai báo ở CR này; dial thật ở CR-CV-012, 013, 021 | `project-service:9090`, `infra-fleet-service:9090`, `git-gateway-service:9090`, `auth-service:9090` |
| `OPA_BUNDLE_PATH` | bó Rego (CR-CV-013); ở CR này chỉ đọc và log | `/policy/orca-authz` |
| `CODEINTEL_TENANT_DEFAULT_ENABLED` | giá trị `code_intel_enabled` của dòng `tenant_settings` tạo lười (O8; mẫu `MCP_TENANT_DEFAULT_ENABLED`) | `false` |
| `CODEINTEL_INTERNAL_CALLER_TOKEN` | bí mật dùng chung cho `internalcaller.Guard` (CR-CV-013) | rỗng, log WARN; rỗng nghĩa là mọi RPC bị chặn (fail closed, `common/internalcaller/internalcaller.go` dòng 21 đến 25) |
| `CODEINTEL_SNAPSHOT_MAX_BYTES`, `CODEINTEL_SNAPSHOT_TTL`, `CODEINTEL_SNAPSHOT_TENANT_QUOTA_BYTES`, `CODEINTEL_MAINTENANCE_INTERVAL` | giới hạn và dọn snapshot (CR-CV-011) | `8388608`, `24h`, `536870912`, `10m` |
| `CODEINTEL_ORPHAN_RETENTION`, `CODEINTEL_REINDEX_STALE_AFTER`, `CODEINTEL_BINDING_IDLE_RETENTION` | dọn dữ liệu mồ côi, job mồ côi, binding rảnh (CR-CV-011 mục 2.5) | `168h`, `45m`, `2160h` |
| `CODEINTEL_STATUS_TTL`, `CODEINTEL_STATUS_TIMEOUT` | cache và hạn chót thăm dò `codeintel.status` (CR-CV-012 mục 2.5) | `15s`, `10s` |
| `CODEINTEL_*` (hạn mức, CR-CV-013) | xem CR-CV-013 mục 2.6 | theo CR đó |

DSN không phải `postgres://`, `postgresql://`, `mysql://`, `tidb://` làm service thoát với lỗi `detecting database dialect` (`dbcapability.DetectDialectFromDSN`, `common/dbcapability/capability.go`).

### 2.3 `main.go`

Thứ tự khởi động (theo `task-service/cmd/server/main.go` và `notification-service/cmd/server/main.go`):

1. `signal.NotifyContext` (SIGINT, SIGTERM) → `config.Load()` → `logging.New(cfg.ServiceName, version)` → `tracing.Init`.
2. `secrets.DatabaseCredentialsFromFile` → `DetectDialectFromDSN` → `health.New()`.
3. `switch caps.Dialect`: Postgres tạo `pgxpool` + `codeintelpostgres.New(pool)`, đăng ký health `postgres` (ping 2 giây); MySQL tạo `sql.Open("mysql", toMySQLDriverDSN(dsn))` + `codeintelmysql.New(db)`, health `mysql`; mặc định lỗi `unsupported database dialect`. `repo` có kiểu giao diện gộp `usecase.OutboxStore` + (các cổng do CR-CV-011 thêm) + `outbox.Store`.
4. `eventbus.Connect(ctx, cfg.NATSURL)`; thành công thì `pub.EnsureStream(ctx, "CODEINTEL", []string{"orca.codeintel.>"})` rồi `outbox.NewRelay(repo, pub, outbox.DefaultConfig, logger)` chạy goroutine; chờ bằng `WaitGroup` khi tắt. Lỗi bus: log cảnh báo, không thoát (như `task-service` dòng 402).
5. `grpc.NewServer(grpcmw.ChainUnary(logger), grpcmw.StatsHandler(), grpc.ChainStreamInterceptor(tenantStreamInterceptor()))`, đăng ký `codeintelv1.RegisterCodeIntelServiceServer` (nhúng `UnimplementedCodeIntelServiceServer`), `reflection.Register` (chú thích sẵn có: chỉ để sau mesh).
6. HTTP `health.Handler()` (`/healthz`, `/readyz`); thêm `/metrics` ở CR-CV-071.
7. Chặn tới khi `ctx.Done()` hoặc lỗi từ `errCh`; `GracefulStop`, `httpServer.Shutdown` (10 giây), `WaitGroup.Wait()`.

### 2.4 Interceptor stream lấy tenant

`TenantExtractionInterceptor` chỉ bọc unary. `internal/adapter/grpc/tenant_stream_interceptor.go` (mới) cài `grpc.StreamServerInterceptor`: đọc đúng bốn khoá metadata của `common/grpcmw` (`MetadataTenantID`, `MetadataUserID`, `MetadataRole`, `MetadataClientIP`), gọi `tenant.With*`, bọc `grpc.ServerStream` để `Context()` trả ctx đã gắn. Không tự bịa khoá mới. Một test kiểm: stream không có metadata tenant thì handler thấy `tenant.RequireTenantID` lỗi (không trả dữ liệu). Chưa kiểm chứng `notification-service.StreamNotifications` (gọi `RequireTenantID` trên ctx stream, `server.go` dòng 159) hoạt động ở runtime; có thể nó bị lỗi cùng nguyên nhân.

### 2.5 Proto `proto/orca/codeintel/v1/codeintel.proto` (mới)

`package orca.codeintel.v1;`, `go_package = "github.com/stablyai/orca-go/proto/gen/go/orca/codeintel/v1;codeintelv1"` (theo `notification.proto`). Stub sinh vào `proto/gen/go/orca/codeintel/v1` và được commit (các package khác đều commit stub, `git ls-files proto/gen/go/orca/notification`).

CR này khai báo `service CodeIntelService {}` **không có RPC nào**, vì mọi RPC trong README v7 mục 3.6 đều cần message do CR khác sở hữu (không khai báo RPC chưa có message để `buf breaking` không khoá message nửa vời, xem CR-REQ-001 mục 2.4). Quy tắc cho các CR sau: message đặt ở file riêng `codeintel_<chủ đề>.proto` cùng package, RPC thêm vào `service` trong `codeintel.proto`.

| RPC (README v7 3.6) | CR sở hữu message | File proto đề xuất |
|---|---|---|
| `BindRepo`, `ListRepoBindings`, `GetIndexStatus` | CR-CV-012 | `codeintel_binding.proto` |
| `RequestReindex`, `GetReindexJob` | CR-CV-004 (agent) + CR-CV-013 (hạn mức) + CR-CV-021 (điều phối); chốt khi viết CR-CV-021 | `codeintel_reindex.proto` |
| `GetStructure`, `GetSubgraph`, `GetImpact`, `GetSymbol`, `GetRouteMap` | CR-CV-020, 021 | `codeintel_graph.proto` |
| `GetArchitecture`, `ListDataFlows`, `GetDataFlow`, `GetErd`, `GetStorageMap` | CR-CV-031, 033, 034, 035 | `codeintel_views.proto` |
| `GetChangeOverlay`, `GetReadingOrder`, `ListFindings`, `DismissFinding`, `GetContractDiff` | CR-CV-036, 037, 038 | `codeintel_review.proto` |
| `GetReviewState`, `SaveReviewState`, `GetC4Overrides`, `SaveC4Overrides` | CR-CV-011 (kiểu lưu), CR-CV-052/033 (nội dung) | `codeintel_review_state.proto` |
| `StreamCodeIntelEvents` (service này phát cho gateway) | CR-CV-024 | `codeintel_events.proto` |

Chạy `buf lint` và `buf breaking` (`proto/buf.yaml`: `STANDARD` và `FILE`); chưa chạy `buf` với `service` rỗng, xem mục 6.

### 2.6 Hai bảng hạ tầng (migration `0001_init`)

Postgres: `CREATE SCHEMA IF NOT EXISTS codeintel;` MySQL: database `codeintel`, bảng không tiền tố (theo `task-service` và `mcp-service`). Id do ứng dụng sinh.

| Bảng | Cột | Ghi chú |
|------|-----|---------|
| `outbox_events` | `id` (UUID / `CHAR(36)`) PK, `tenant_id`, `subject` TEXT, `occurred_at`, `version` INT, `payload` (JSONB / JSON), `created_at` mặc định đồng hồ DB, `published_at` NULL | Sao `task/migrations/*/0005_outbox.up.sql`. Postgres: chỉ mục riêng phần `(created_at) WHERE published_at IS NULL`; MySQL: chỉ mục `(published_at, created_at)` |
| `processed_events` | `tenant_id`, `event_id`, `subject`, `processed_at`; PK `(tenant_id, event_id)` | Dedup consumer (CR-CV-012 nghe `orca.project.worktree.deleted`, CR-CV-024). Dạng khoá kép theo `mcp-service` (`notification-service` chỉ khoá `event_id`); chỉ mục `processed_at` để dọn |

Postgres: cả hai bảng `ENABLE` + `FORCE ROW LEVEL SECURITY`, chính sách `tenant_isolation` (`USING` và `WITH CHECK` theo `NULLIF(current_setting('app.tenant_id', true), '')::uuid`); thêm hai chính sách chỉ cho relay trên `outbox_events` (`FOR SELECT` và `FOR UPDATE` với `current_setting('app.relay', true) = 'on'`), đúng mẫu `mcp-service/migrations/postgres/0001_init.up.sql` dòng 59 đến 65. Down: bỏ chính sách rồi `DROP TABLE`.

Bảng nghiệp vụ nằm ở CR-CV-011 (`0002_code_intel_core`).

### 2.7 Outbox

Adapter mỗi dialect hiện thực `outbox.Store` (`FetchUnpublished`, `MarkPublished`, `common/outbox/outbox.go`). Postgres chạy hai phương thức này trong `withRelayTx` (đặt `app.relay='on'`, mẫu `mcp-service/internal/adapter/postgres/repository.go` dòng 97 đến 134). Mọi phương thức ghi của repository nhận `events []domain.OutboxRecord` và chèn vào `outbox_events` trong cùng transaction với thay đổi nghiệp vụ; use case dựng `OutboxRecord` (`ID`, `Subject`, `OccurredAt`, `Version`, `PayloadJSON`, mẫu `mcp-service/internal/domain/outbox.go`). Subject theo quy ước `orca.<service>.<entity>.<event>`: bốn subject của README v7 mục 3.8 là `orca.codeintel.index.changed`, `orca.codeintel.reindex.started`, `orca.codeintel.reindex.finished`, `orca.codeintel.review.saved`; stream JetStream `CODEINTEL` bao `orca.codeintel.>`.

Giữ lại bản ghi đã publish 7 ngày rồi xoá (cùng cửa sổ "~7 ngày" mà chú thích `notification-service/migrations/postgres/0002_processed_events.up.sql` nêu cho `processed_events`); việc xoá do công việc dọn của CR-CV-011. Chưa kiểm chứng service nào hiện đang xoá outbox đã publish.

### 2.8 Đăng ký repo và triển khai

| Việc | Chỗ sửa |
|------|---------|
| Thêm `./services/code-intel-service` | `backend-go/go.work`, giữ thứ tự chữ cái (giữa `automation-service` và `credential-broker-service`) |
| Thêm `code-intel-service` vào `SERVICES` | `backend-go/Makefile` dòng 7 đến 11 (đủ cho `build`, `vet`, `test`, `lint`, `tidy-all`) |
| Thêm database `codeintel` | `backend-go/deploy/postgres-init-databases.sh` **và** `deploy/dev/docker/postgres/init-databases.sh` (biến `DATABASES`; hai file có danh sách khác nhau, thêm vào cả hai) |
| Service `code-intel-service` và `migrate-codeintel` | `deploy/dev/docker-compose.yml`. Mẫu `task-service` (dòng 325) cho `volumes: ./bin/code-intel-service/orca:/app/bin/orca:ro` và `./policy/orca-authz:/policy/orca-authz:ro`; `container_name: orca-go-code-intel`; `DATABASE_DSN: postgresql://orca:${POSTGRES_PASSWORD}@postgres:5432/codeintel?sslmode=disable`; `CODEINTEL_TENANT_DEFAULT_ENABLED: ${CODEINTEL_TENANT_DEFAULT_ENABLED:-false}`; `CODEINTEL_INTERNAL_CALLER_TOKEN`; `OPA_BUNDLE_PATH`; `mem_limit: 256m` (mặc định `go-defaults` là 128m, dòng 101; snapshot tới 8 MiB nhân số truy vấn đồng thời có thể vượt 128m, chưa đo); `depends_on` postgres healthy, `project-service`, `infra-fleet-service` started. `migrate-codeintel` theo mẫu `migrate-task` (dòng 705) với `./bin/code-intel-service/migrations` |
| Thêm `codeintel` vào `SERVICES` | `deploy/dev/scripts/migrate.sh` dòng 26 (script tự `CREATE DATABASE` nếu thiếu qua `ENSURE_DB_SQL`) |
| Thêm `code-intel-service` vào `ALL_SERVICES` | `deploy/dev/scripts/build-local.sh` dòng 37 đến 44 (build binary và làm phẳng `migrations/postgres` vào `bin/<svc>/migrations`) |
| Thêm `code-intel-service` vào `svcs` | `backend-go/ci/check-opa-bundle-in-images.sh` (Dockerfile có `COPY policy`; CR-CV-013 dùng OPA) |
| Địa chỉ cho gateway | `CODE_INTEL_SERVICE_ADDR` trong `x-go-common-env` do CR-CV-040 thêm, không làm ở đây |
| Workflow CI | `.github/workflows/backend-go-code-intel-service.yml` (mới), sao `backend-go-task-service.yml` (ma trận `dialect: [postgres, mysql]`, cài `migrate` với `-tags`) cộng hai bước `buf lint --path orca/codeintel` và `buf breaking` của `backend-go-mcp-service.yml` (chỉ chạy `breaking` khi file đã có trên `origin/main`); `paths` gồm `backend-go/common/**`, `backend-go/proto/orca/codeintel/**`, `backend-go/proto/gen/go/orca/codeintel/**`, `backend-go/services/code-intel-service/**`, `backend-go/go.work` |
| `README.md` của service | Ghi mục "thật / chưa làm" cho từng RPC, mục "Yêu cầu RLS" (role `NOSUPERUSER NOBYPASSRLS`, như `mcp-service/README.md`) |

## 3. Quyết định thiết kế

| Quyết định | Lý do |
|------------|-------|
| Hai dialect ngay từ đầu (O1) | Sửa sau tốn hơn; mặc định của README v7 mục 2 |
| Layout `notification-service`, outbox và RLS theo `mcp-service` | `notification-service` có hai adapter nhưng không có outbox; `task-service` có relay nhưng RLS vô hiệu và `TxRunner` khác mẫu; `mcp-service` đúng ở hai điểm sau nhưng chỉ Postgres |
| `service CodeIntelService {}` rỗng | Không khoá message của CR khác bằng `buf breaking` |
| Ghi outbox qua tham số `events` của phương thức repository | Tránh `TxRunner` nhiều kiểu; một transaction cho cả thay đổi và sự kiện |
| Interceptor stream cục bộ, không sửa `common/grpcmw` | `ChainUnary` có 16 điểm gọi trực tiếp (chú thích ở `grpcmw.StatsHandler` ghi blast radius CRITICAL); không động vào |
| Không dial service khác ở CR này | Chạy được và test được độc lập |
| DB/schema `codeintel` | Khớp README v7 mục 3.5 và quy ước tên DB không dấu gạch |

## 4. Tiêu chí chấp nhận

- [ ] `make build`, `make vet`, `make test`, `make lint` xanh với module mới trong `go.work`.
- [ ] `buf lint` xanh với package `orca.codeintel.v1` (kể cả `service` rỗng); stub sinh vào `proto/gen/go/orca/codeintel/v1`; `buf breaking` xanh so với `main` (bỏ qua khi file chưa có trên `main`).
- [ ] Với `DATABASE_DSN` Postgres, service khởi động, `/healthz` và `/readyz` trả 200; với DSN MySQL cũng vậy; `grpcurl` thấy `orca.codeintel.v1.CodeIntelService` qua reflection.
- [ ] DSN scheme lạ làm tiến trình thoát với lỗi nêu dialect không hỗ trợ.
- [ ] Migration `0001_init` chạy `up` rồi `down` sạch trên Postgres và MySQL bằng golang-migrate; `up` lại sau `down` không lỗi.
- [ ] Ghi một dòng `outbox_events` cùng transaction rồi relay publish được lên NATS; rollback thì không có dòng.
- [ ] Postgres, role `NOSUPERUSER NOBYPASSRLS`: tenant A không đọc, không ghi được dòng `outbox_events` và `processed_events` của tenant B; chỉ phiên có `app.relay='on'` đọc được chéo tenant và chỉ `SELECT`/`UPDATE`, không `INSERT`. MySQL: test repo chứng minh mọi truy vấn lọc `tenant_id`.
- [ ] Test AST (mẫu `mcp-service/.../tenant_scope_guard_test.go`) bảo đảm mọi phương thức xuất của repository Postgres chạy trong `withTenantTx` hoặc `withRelayTx`; bản MySQL kiểm mọi phương thức xuất lấy tenant qua `tenant.RequireTenantID`.
- [ ] Stream gRPC thử nghiệm: có metadata tenant thì handler đọc được tenant, không có thì `RequireTenantID` lỗi.
- [ ] Gọi RPC chưa có (nếu CR sau đã thêm) trả `Unimplemented`, không panic.
- [ ] Workflow CI chạy hai dialect và xanh; compose dev dựng được `code-intel-service` và `migrate-codeintel` chạy xong.
- [ ] Tên file, thư mục không có `helpers`, `utils`, `common`, `misc`; không có `max-lines` disable mới.

## 5. Kiểm thử

- **Unit:** `config` (mặc định, phân tích bool/duration sai, DSN rỗng); `toMySQLDriverDSN` (hai dạng đầu vào `mysql://...@tcp(...)` và `mysql://user:pass@host/db`, tự thêm `parseTime=true`); interceptor stream; AST guard của hai adapter.
- **Integration (`-tags=integration`, testcontainers `common/testutil/{postgres,mysql,nats}.go`), chạy hai dialect:** migration up/down; `FetchUnpublished` theo thứ tự `created_at`; `MarkPublished`; hai relay đồng thời không mất sự kiện (at-least-once, giao lặp được chấp nhận); RLS với role không phải superuser (Postgres).
- **Hợp đồng:** `buf breaking`; test gRPC in-process kiểm reflection.
- **Chưa chạy bất kỳ test nào ở thời điểm viết CR.**

## 6. Rủi ro và điểm chưa kiểm chứng

- Chưa chạy `buf lint` với `service` không có RPC; nếu `STANDARD` phàn nàn, thêm RPC `GetIndexStatus` sớm từ CR-CV-012 và đẩy CR-CV-012 lên trước khi merge.
- `golang-migrate` MySQL driver cho database `codeintel`: workflow `task-service` dùng cùng cách, rủi ro thấp; chưa chạy.
- `go.work.sum` và `go.mod` module mới phải đồng bộ (`make tidy-all`); chưa chạy.
- Dev compose chỉ Postgres; MySQL không có đường chạy dev, chỉ CI. Nếu cần thử MySQL tay, phải tự dựng.
- `mem_limit: 256m` chưa đo; xem CR-CV-071.
- RLS trong dev không có hiệu lực (superuser); mọi bảo đảm cô lập tenant ở dev chỉ dựa vào lọc ở tầng ứng dụng.
- Chưa kiểm chứng tác động của interceptor stream tới `grpcmw.LoggingInterceptor` (chỉ unary): log request của RPC stream sẽ không có; chấp nhận, ghi log tại handler.

## 7. Câu hỏi mở

- **Q1.** Có muốn một RPC smoke có message ngay ở CR này (ví dụ `GetIndexStatus` tối giản) thay vì `service` rỗng? Mặc định: không.
- **Q2.** `toMySQLDriverDSN` đã có ba bản sao; có nên chuyển lên `common` (đổi blast radius) hay tiếp tục sao? Mặc định: sao.
- **Q3.** Subject: README v7 mục 3.8 dùng `orca.codeintel.<entity>.<event>` với `index.changed`; nhất quán với quy ước repo. Xác nhận `review.saved` có nghĩa "đã lưu trạng thái review" và entity là `review`.
- **Q4.** Tên stream JetStream `CODEINTEL` chưa trùng stream nào (đã có `TASK`, `PROJECT`, `INFRAFLEET`, và `orca.infra.>`); chưa liệt kê hết stream đang chạy ở môi trường thật.

## 8. Tham chiếu

- `/opt/repos/orca/docs/crs/v7/README.md` mục 2, 3.5, 3.6, 3.8
- `/opt/repos/orca/docs/crs/v6/request-service-foundation/CR-REQ-001-scaffold-request-service.md` và `README.md` (mục "Điểm lệch")
- `/opt/repos/orca/backend-go/services/notification-service/cmd/server/main.go`, `internal/config/config.go`
- `/opt/repos/orca/backend-go/services/task-service/cmd/server/main.go`, `internal/usecase/ports.go`, `internal/adapter/postgres/share_link.go`, `migrations/*/0005_outbox.up.sql`
- `/opt/repos/orca/backend-go/services/mcp-service/migrations/postgres/0001_init.up.sql`, `internal/adapter/postgres/tenant_tx.go`, `repository.go`, `tenant_scope_guard_test.go`, `README.md` (mục "RLS requirements"), `internal/domain/outbox.go`
- `/opt/repos/orca/backend-go/common/{outbox/outbox.go,dbcapability/capability.go,grpcmw/grpcmw.go,health/health.go,config/config.go,secrets/vault.go,internalcaller/internalcaller.go,testutil}`
- `/opt/repos/orca/backend-go/go.work`, `backend-go/Makefile`, `backend-go/deploy/postgres-init-databases.sh`, `backend-go/ci/check-opa-bundle-in-images.sh`
- `/opt/repos/orca/deploy/dev/docker-compose.yml`, `deploy/dev/docker/postgres/init-databases.sh`, `deploy/dev/scripts/migrate.sh`, `deploy/dev/scripts/build-local.sh`
- `/opt/repos/orca/.github/workflows/backend-go-task-service.yml`, `backend-go-mcp-service.yml`
- `/opt/repos/orca/backend-go/proto/buf.yaml`, `proto/orca/notification/v1/notification.proto`
- `/opt/repos/orca/guides/STYLEGUIDE.md`, `guides/reference/git-compatibility.md` (không có lệnh git mới ở CR này)
- CR liên quan: [CR-CV-011](./CR-CV-011-code-intel-data-model-and-repositories.md), [CR-CV-012](./CR-CV-012-project-worktree-to-repo-binding.md), [CR-CV-013](./CR-CV-013-authorization-audit-and-quotas.md)
