# BE-CV-SOL-010: Dựng `code-intel-service`: module Go, proto `orca.codeintel.v1`, DB hai dialect, outbox, wiring, CI

> **📋 Proposed.** Chưa triển khai, chưa chạy build/test/migration/`buf` nào. Điều kiện tiên quyết của toàn bộ phía backend v7. Không phụ thuộc solution nào của v7 (trừ cổng G0 cho proto `codeintel_common.proto`, mục 4).

**CR:** [CR-CV-010](../../../../../../docs/crs/v7/code-intel-service-foundation/CR-CV-010-scaffold-code-intel-service.md)
**Service:** `code-intel-service` (mới) · `common/apperrors` (thêm 2 Kind) · `proto/orca/codeintel/v1` (mới) · `go.work` / `Makefile` / `deploy/*` · `.github/workflows`
**TDD tham chiếu:** [`arch/02`](../../../../tdd/architecture/02-microservices-decomposition.md) (ranh giới service, DB riêng), [`arch/03`](../../../../tdd/architecture/03-clean-architecture-guidelines.md) (layout domain/usecase/adapter), [`arch/05`](../../../../tdd/architecture/05-data-architecture.md) (database-per-service, mục "Multi-tenancy", "Migration conventions"), [`arch/07`](../../../../tdd/architecture/07-security-architecture.md) (mục "Multi-tenancy isolation", "Service-to-service transport security"), [`arch/08`](../../../../tdd/architecture/08-inter-service-communication.md) (mục "gRPC conventions", "Event conventions"), [`arch/09`](../../../../tdd/architecture/09-observability-reliability.md) (mục "Health checks")

---

## Hợp đồng áp dụng

| Mục hợp đồng | Nội dung áp dụng ở solution này |
|---|---|
| PQ-03 (8) | Thêm `KindResourceExhausted`, `KindUnavailable` vào `common/apperrors` (thay đổi **duy nhất** được phép ở `common/apperrors`), ánh xạ `codes.ResourceExhausted`, `codes.Unavailable` |
| PQ-07, §2.1 #1 | `codeintel.proto` chứa duy nhất `service CodeIntelService`, mọi RPC thêm về sau vào đây; message ở file `codeintel_<chủ đề>.proto` |
| PQ-23, §6.2 | Tên env `CODEINTEL_*`, container `orca-go-code-intel`, job `migrate-codeintel`, DB/schema `codeintel` |
| §4.1, T0 | Quy ước DB/RLS; bảng `outbox_events`, `processed_events` (migration `0001_init`) |
| §5 | Stream JetStream `CODEINTEL` (`orca.codeintel.>`), bốn subject mà 011–013 phát (hằng ở domain) |
| §6.1, §6.2 | Cờ công tắc env (`CODEINTEL_ENABLED`, `CODEINTEL_QUALITY_GATE_ENABLED`, `CODEINTEL_AI_REVIEW_ENABLED`, `CODEINTEL_TENANT_DEFAULT_*`) được **đọc** ở cấu hình; việc thi hành cờ thuộc SOL-013 |
| §2 (đầu mục) | CI phải gọi `buf lint`/`buf breaking` trực tiếp, không dùng `make proto-lint` (có `\|\| true`) |
| §10 | Danh sách wiring: `go.work`, `Makefile`, hai file `init-databases.sh`, compose dev, `migrate.sh`, `build-local.sh`, `check-opa-bundle-in-images.sh`, workflow |
| H10 | Mọi bảng có `tenant_id`; không FK; hai dialect |

## Lệch giữa CR và hợp đồng

| # | CR-CV-010 nói | Hợp đồng nói | Xử lý (hợp đồng thắng) |
|---|---|---|---|
| L1 | Không sửa `common` (mục 3: "không sửa `common/grpcmw`"); CR-CV-013 mục 3 nói "không sửa `common/apperrors`" | PQ-03 (8): CR-CV-010 **được thêm** hai Kind vào `common/apperrors` | Task 010-02 làm việc này; SOL-013 dùng Kind mới thay `LimitError` riêng |
| L2 | `CODEINTEL_SNAPSHOT_MAX_BYTES` 8 MiB, `CODEINTEL_SNAPSHOT_TTL` 24h | PQ-14 (2)(3): mặc định 3 MiB (tối đa 8 MiB), TTL 7 ngày (`168h`), giữ 3 commit mỗi `(binding, view)` | `config.go` đặt mặc định theo hợp đồng (`3145728`, `168h`); hạn mức tenant 512 MiB giữ nguyên, thêm 64 MiB mỗi binding (T3) |
| L3 | Chỉ khai báo `CODEINTEL_TENANT_DEFAULT_ENABLED` | §6.2 thêm `CODEINTEL_ENABLED`, `CODEINTEL_QUALITY_GATE_ENABLED`, `CODEINTEL_AI_REVIEW_ENABLED`, `CODEINTEL_TENANT_DEFAULT_QUALITY_GATE_ENABLED` (mặc định `false`) | `config.go` đọc đủ 5 biến |
| L4 | `CODEINTEL_STATUS_TTL` 15s | PQ-24: cache cờ 5 s; trạng thái 15 s giữ nguyên (CR-012) | Thêm `CODEINTEL_FLAG_CACHE_TTL=5s` (mới) cho SOL-013 |
| L5 | Bốn subject outbox | §5 liệt kê bảy subject | CR-010 chỉ khai báo hằng cho bốn subject do 011/012/013 phát; ba subject `quality.*`, `agent_turn.recorded` do CR chủ sở hữu thêm |
| L6 | Health chỉ HTTP | §7.1 G0 viết "`codeintel.proto` rỗng-có-health" | Hiểu là health HTTP `/healthz`, `/readyz`; **không** thêm gRPC health (CR-CV-010 mục 1.1 điểm 4, đã kiểm). Ghi vào câu hỏi mở Q3 để chủ hợp đồng xác nhận |
| L7 | Đường dẫn init DB `backend-go/deploy/postgres-init-databases.sh` | §10 nhắc cả hai file; xác nhận lại: danh sách `DATABASES` của hai file **khác nhau** | Thêm `codeintel` vào cả hai (task 010-08) |

## Phụ thuộc chéo khu vực

| Khu vực | Solution | Quan hệ |
|---|---|---|
| BE | `BE-CV-SOL-020-canonical-graph-model` | Cổng **G0** (§7.1): `codeintel_common.proto` (`WorktreeSelector`, `ResultMeta`, …) và stub sinh. SOL-010 không cần chúng để merge (service rỗng), nhưng G0 gồm cả hai |
| BE | `BE-CV-SOL-011-*`, `BE-CV-SOL-012-*`, `BE-CV-SOL-013-*` | Mở khoá: cần module, outbox, migration `0001`, adapter khung từ SOL-010 |
| BE | `BE-CV-SOL-023-infra-fleet-codeintel-transport` | Song song (G2); không phụ thuộc lẫn nhau |
| BE | `BE-CV-SOL-040-codeintel-channel-foundation` | Cần địa chỉ `CODE_INTEL_SERVICE_ADDR` trỏ tới container `orca-go-code-intel` (thêm biến ở gateway là việc của 040) |
| BE | `BE-CV-SOL-073-settings-flag-and-rollout` | Dùng biến công tắc `CODEINTEL_*` khai báo ở đây |
| BE | `BE-CV-SOL-071-metrics-tracing-and-budgets` | Thêm `/metrics` vào mux HTTP (SOL-010 chỉ có `/healthz`, `/readyz`) |
| AG / FE | — | Không có việc ở agent/frontend |

Thứ tự (§7.2): `010 → 011 → 012 → 013`; đợt 1 cùng `020`, `023`.

---

## 1. Trạng thái hiện tại (Re-verify)

Đã đọc (2026-10-06): `backend-go/go.work`, `backend-go/Makefile` (dòng 7–11 `SERVICES`, dòng 80–81 `proto-lint`), `backend-go/deploy/postgres-init-databases.sh`, `deploy/dev/docker/postgres/init-databases.sh`, `deploy/dev/scripts/{migrate.sh,build-local.sh}`, `deploy/dev/docker-compose.yml` (dòng 40–110 `x-go-common-env`, `x-go-defaults`, `x-migrate-defaults`; dòng 325 `task-service`; dòng 705 `migrate-task`), `backend-go/ci/check-opa-bundle-in-images.sh`, `.github/workflows/{backend-go-task-service.yml,backend-go-mcp-service.yml}`, `services/notification-service/cmd/server/main.go` (dòng 124 `switch caps.Dialect`, dòng 331 `toMySQLDriverDSN`), `services/task-service/cmd/server/main.go` (dòng 148, 400–441, 516), `services/task-service/deploy/Dockerfile`, `services/mcp-service/{cmd/server/main.go,internal/adapter/postgres/{tenant_tx.go,repository.go,tenant_scope_guard_test.go},migrations/postgres/{0001_init,0002_authorization}.up.sql,internal/domain/outbox.go,internal/config/config.go}`, `common/{apperrors/apperrors.go,grpcmw/grpcmw.go,internalcaller/internalcaller.go,outbox/outbox.go,eventbus/eventbus.go,dbcapability/capability.go,health/health.go,config/config.go,policy/evaluator.go,testutil}`, `proto/buf.yaml`.

Xác nhận đúng: `go.work` có 19 module service (không `code-intel-service`); `proto/orca/` có 16 thư mục, không có `codeintel`; `Makefile` `SERVICES` ở dòng 7–11; `proto-lint` kết thúc `|| true` (dòng 81); `dbcapability.DetectDialectFromDSN` chấp nhận `postgres://`, `postgresql://`, `mysql://`, `tidb://`; `grpcmw` chỉ có interceptor unary (`ChainUnary`), không có tenant extraction cho stream; `mcp-service` tự đọc metadata trong `StreamEvents` (`governance_server.go:272`) và dùng `internalcaller.StreamGuard` (`cmd/server/main.go:226`); không file Go nào nhập `grpc_health_v1`; `mcp-service` RLS thật (`FORCE`, `NULLIF`, `set_config`); `outbox.DefaultConfig = {PollInterval 2s, BatchSize 100}`; `x-go-common-env` **đã có** `PROJECT_SERVICE_ADDR`, `INFRA_FLEET_SERVICE_ADDR`, `GIT_GATEWAY_SERVICE_ADDR`, `AUTH_SERVICE_ADDR`, `NATS_URL`.

### Correction relative to CR-CV-010

| # | CR nói | Mã thật | Xử lý |
|---|---|---|---|
| C1 | `notification-service` dialect switch ở "dòng 91 đến 159" | `switch caps.Dialect` ở dòng 124; `toMySQLDriverDSN` dòng 331 (CR ghi 324–330 là chú thích) | Chỉ ảnh hưởng chú thích; layout đúng |
| C2 | `task-service` `eventbus.Connect` "dòng 391 đến 420" | `Connect` ở dòng 400, `EnsureStream` dòng 408, `NewRelay` dòng 411 | Dùng số dòng mới |
| C3 | Thêm `PROJECT_SERVICE_ADDR`, `INFRA_FLEET_SERVICE_ADDR`, `GIT_GATEWAY_SERVICE_ADDR`, `AUTH_SERVICE_ADDR` vào compose | Bốn biến đã nằm trong `x-go-common-env` (`docker-compose.yml` dòng 46–60) | Service mới chỉ `<<: *go-common-env`, **không** thêm lại bốn biến |
| C4 | Postgres `outbox_events` mẫu `mcp-service` | `mcp-service` đọc `ORDER BY created_at` (`repository.go`), cột `created_at DEFAULT now()`; hai sự kiện trong cùng transaction có cùng `created_at` (v6 SOL-001 mục 3 D2 đã gặp) | Hợp đồng T0 **không** có cột `seq`. Hiện service này ghi tối đa một sự kiện mỗi transaction (xem 011) nên giữ đúng T0; ghi nhận ở Q2 |
| C5 | Dockerfile sao `task-service` | Stage build `COPY policy ./policy`; runtime `COPY ... policy/orca-authz` và `COPY services/task-service/migrations /migrations` | Code-intel dùng OPA (SOL-013) nên **giữ** copy bundle; `check-opa-bundle-in-images.sh` thêm vào mảng `svcs` |
| C6 | `common/config` "không có `BoolEnv`" | Đúng: chỉ `StringEnv` xuất; `mcp-service/internal/config/config.go` có `boolEnv` (dòng 54) và `intEnv` (dòng 66) | Viết lại tại `config.go` (không import chéo service) |
| C7 | `reflection.Register` | Có ở `task-service/main.go:441` và `mcp-service/main.go:233` | Giữ, kèm chú thích "chỉ sau mesh" |

Chưa kiểm chứng: `buf lint STANDARD` có chấp nhận `service` rỗng hay không (chưa chạy `buf`); tên stream `CODEINTEL` có trùng stream đang chạy ở môi trường thật; `go.work.sum`/`go.mod` đồng bộ; `golang-migrate` MySQL với database `codeintel`.

## 2. Giải pháp

### A. Cây thư mục `backend-go/services/code-intel-service/` (mới)

```
go.mod                                   # module github.com/stablyai/orca-go/services/code-intel-service ; go 1.25.0
README.md                                # bảng "thật / chưa làm" từng RPC; mục "Yêu cầu RLS" (NOSUPERUSER NOBYPASSRLS)
cmd/server/main.go                       # composition root
cmd/server/mysql_driver_dsn.go           # toMySQLDriverDSN (sao, không import chéo service) + _test
internal/config/config.go                # nhúng commonconfig.Base; boolEnv/intEnv/durationEnv tự viết
internal/domain/outbox_record.go         # OutboxRecord{ID, Subject, OccurredAt, Version, PayloadJSON}
internal/domain/outbox_subjects.go       # hằng 4 subject (L5)
internal/usecase/ports.go                # OutboxStore (cổng ghi), Clock
internal/adapter/grpc/server.go          # CodeIntelServiceServer nhúng Unimplemented
internal/adapter/grpc/tenant_stream_interceptor.go
internal/adapter/postgres/{repository.go,tenant_tx.go,outbox.go}
internal/adapter/mysql/{repository.go,outbox.go}
internal/adapter/eventbus/outbox_relay.go  # nối outbox.Relay; chưa consumer
migrations/postgres/0001_init.{up,down}.sql
migrations/mysql/0001_init.{up,down}.sql
deploy/Dockerfile
```

Không tạo thư mục rỗng (`grpcclient/` do SOL-012 tạo file đầu). Cấm `helpers`, `utils`, `common`, `misc`; không `max-lines` disable (AGENTS.md).

### B. `common/apperrors`: hai Kind mới (PQ-03 (8))

```go
// thêm cuối khối const Kind, sau KindDeadlineExceeded (giữ giá trị iota cũ)
KindResourceExhausted // hạn mức/tốc độ: codes.ResourceExhausted
KindUnavailable       // phụ thuộc hạ nguồn không với tới: codes.Unavailable
```

`ToGRPCStatus` thêm hai `case` (`apperrors.go` hiện có 8 case, mặc định `codes.Unknown` cho Kind lạ). Thêm vào cuối để không đổi giá trị số của Kind cũ. Task 010-02 **phải** chạy `gitnexus_impact` trên `ToGRPCStatus` và `Kind` trước khi sửa (chú thích mã nguồn nêu 463 điểm gọi; blast radius lớn nhưng thay đổi thuần cộng). Test: bảng Kind → code, kiểm Kind cũ không đổi.

### C. Cấu hình `internal/config/config.go`

| Biến | Mặc định | Ghi chú |
|---|---|---|
| `GRPC_PORT`, `HTTP_PORT`, `DATABASE_DSN`, `OTLP_ENDPOINT` | 9090, 8080, —, — | từ `commonconfig.LoadBase("code-intel-service")` |
| `DATABASE_CREDENTIALS_FILE` | `/vault/secrets/database-credentials` | `secrets.DatabaseCredentialsFromFile` |
| `NATS_URL` | `nats://localhost:4222` | lỗi kết nối: cảnh báo, tắt relay |
| `PROJECT_SERVICE_ADDR`, `INFRA_FLEET_SERVICE_ADDR`, `GIT_GATEWAY_SERVICE_ADDR`, `AUTH_SERVICE_ADDR` | `<tên>:9090` | khai báo; SOL-012/013 dial |
| `OPA_BUNDLE_PATH` | `/policy/orca-authz` | SOL-013 nạp; ở đây chỉ đọc và log |
| `CODEINTEL_ENABLED`, `CODEINTEL_QUALITY_GATE_ENABLED`, `CODEINTEL_AI_REVIEW_ENABLED` | `false` | công tắc env (PQ-24) |
| `CODEINTEL_TENANT_DEFAULT_ENABLED`, `CODEINTEL_TENANT_DEFAULT_QUALITY_GATE_ENABLED` | `false` | giá trị cột khi tạo lười `tenant_settings` |
| `CODEINTEL_INTERNAL_CALLER_TOKEN` | rỗng | rỗng = chặn mọi RPC (fail closed), log WARN |
| `CODEINTEL_SNAPSHOT_MAX_BYTES` | `3145728` | tối đa `8388608`; vượt tối đa → lỗi cấu hình |
| `CODEINTEL_SNAPSHOT_TTL` | `168h` | PQ-14 (3) |
| `CODEINTEL_SNAPSHOT_TENANT_QUOTA_BYTES`, `CODEINTEL_SNAPSHOT_BINDING_QUOTA_BYTES` | `536870912`, `67108864` | T3 |
| `CODEINTEL_MAINTENANCE_INTERVAL` | `10m` | SOL-011 |
| `CODEINTEL_ORPHAN_RETENTION`, `CODEINTEL_REINDEX_STALE_AFTER`, `CODEINTEL_BINDING_IDLE_RETENTION` | `168h`, `45m`, `2160h` | SOL-011 |
| `CODEINTEL_STATUS_TTL`, `CODEINTEL_STATUS_TIMEOUT` | `15s`, `10s` | SOL-012 |
| `CODEINTEL_FLAG_CACHE_TTL` (mới) | `5s` | PQ-24, SOL-013 |

Giá trị sai (bool/duration/int không phân tích được) làm `Load()` trả lỗi (không bỏ qua thầm). Các biến hạn mức của SOL-013 khai báo ở SOL-013 (cùng struct, file `config_limits.go`).

### D. `main.go`

Thứ tự: `signal.NotifyContext` → `config.Load` → `logging.New` → `apperrors.SetLogger` → `tracing.Init` → `secrets.DatabaseCredentialsFromFile` → `DetectDialectFromDSN` → `health.New()` → `switch caps.Dialect` (Postgres `pgxpool` + health `postgres` ping 2 s; MySQL `sql.Open("mysql", toMySQLDriverDSN(dsn))` + health `mysql`; mặc định `unsupported database dialect`) → `eventbus.Connect` (lỗi: cảnh báo, không thoát) → `pub.EnsureStream(ctx, "CODEINTEL", []string{"orca.codeintel.>"})` → `outbox.NewRelay(repo, pub, outbox.DefaultConfig, logger)` chạy goroutine có `WaitGroup` → `grpc.NewServer(grpcmw.ChainUnary(logger), grpcmw.StatsHandler(), grpc.ChainStreamInterceptor(tenantStreamInterceptor()))` → `RegisterCodeIntelServiceServer` → `reflection.Register` → HTTP `health.Handler()` → chờ `ctx.Done()`/lỗi → `GracefulStop`, `httpServer.Shutdown(10s)`, `WaitGroup.Wait()`.

```go
// repo: kiểu gộp, mỗi dialect hiện thực đủ.
type store interface {
    usecase.OutboxStore // ghi outbox trong cùng transaction nghiệp vụ (SOL-011 mở rộng)
    outbox.Store        // FetchUnpublished, MarkPublished
}
```

Giữ chỗ để SOL-013 chèn `internalcaller.Guard`/`StreamGuard` (xem 013): `main.go` chỉ khai báo biến `serverOpts []grpc.ServerOption` và hàm `buildServerOptions(cfg, logger)` để 013 không phải sửa lại thứ tự khởi động.

### E. Interceptor stream lấy tenant

```go
// internal/adapter/grpc/tenant_stream_interceptor.go (mới)
func tenantStreamInterceptor() grpc.StreamServerInterceptor // đọc 4 khoá grpcmw.Metadata{TenantID,UserID,Role,ClientIP}, gọi tenant.With*, bọc ServerStream.Context()
```

Không bịa khoá mới. Test: có metadata → `tenant.RequireTenantID` thành công; thiếu → lỗi, handler không trả dữ liệu. Chưa kiểm chứng `notification-service.StreamNotifications` (CR nêu) có cùng lỗi ở runtime.

### F. Proto `proto/orca/codeintel/v1/codeintel.proto` (mới)

`package orca.codeintel.v1;`, `go_package = "github.com/stablyai/orca-go/proto/gen/go/orca/codeintel/v1;codeintelv1"`; `service CodeIntelService {}` không RPC (mục 2.1 #1: "chỉ chứa `service`"; "RPC chưa có message thì không khai báo"). Stub sinh vào `proto/gen/go/orca/codeintel/v1` và được commit (các package khác đều commit stub: `git ls-files backend-go/proto/gen/go/orca` có `.pb.go`). Nếu `buf lint` từ chối service rỗng (chưa chạy): thêm RPC `GetIndexStatus` sớm (đẩy SOL-012 lên trước) hoặc bỏ đăng ký rồi để SOL-012 thêm; quyết định ở task 010-01 (bước thăm dò).

### G. Migration `0001_init` (hợp đồng T0)

Postgres (schema `codeintel`): `outbox_events` (`id UUID PK`, `tenant_id`, `subject TEXT`, `occurred_at TIMESTAMPTZ`, `version INT`, `payload JSONB`, `created_at TIMESTAMPTZ DEFAULT now()`, `published_at TIMESTAMPTZ NULL`; chỉ mục từng phần `(created_at) WHERE published_at IS NULL`), `processed_events` (PK `(tenant_id, event_id)`, chỉ mục `processed_at`); `ENABLE` + `FORCE ROW LEVEL SECURITY`; `tenant_isolation` (`USING` + `WITH CHECK` theo `NULLIF(current_setting('app.tenant_id', true), '')::uuid`); `relay_read` (`FOR SELECT`), `relay_mark_published` (`FOR UPDATE`) theo `app.relay='on'`. Chính sách bảo trì (`app.maintenance`) cho hai bảng này thêm ở `0002` (SOL-011) để `0001` đúng như CR-010. MySQL: `CHAR(36)`, `TIMESTAMP(6)`, `JSON`, chỉ mục `(published_at, created_at)`, không RLS.

### H. Adapter outbox

Mỗi dialect hiện thực `outbox.Store`. Postgres: `FetchUnpublished`, `MarkPublished` chạy trong `withRelayTx` (`set_config('app.relay','on',true)`), mẫu `mcp-service/internal/adapter/postgres/tenant_tx.go`; `withTenantTx(ctx, tenantID, fn)` đặt `app.tenant_id`. Hàm nội bộ `insertOutboxEvents(ctx, tx, tenantID, events []domain.OutboxRecord)` dùng chung cho mọi phương thức ghi của SOL-011. MySQL: cùng hình dạng dùng `*sql.Tx`, mọi truy vấn lọc `tenant_id` (riêng hai phương thức relay là xuyên tenant có chủ ý; ghi chú rõ).

Test AST (mẫu `tenant_scope_guard_test.go`): quét **mọi** `*_repository.go` và `repository.go` trong adapter, mọi phương thức xuất phải dùng `withTenantTx|withRelayTx|withMaintenanceTx` (Postgres) hoặc `tenant.RequireTenantID` (MySQL). Bản `mcp-service` chỉ đọc `repository.go`; SOL-011 thêm nhiều file nên guard phải quét theo glob.

### I. Wiring và CI (hợp đồng §10)

| Việc | Chỗ sửa |
|---|---|
| `./services/code-intel-service` | `backend-go/go.work` (giữa `automation-service` và `credential-broker-service`) |
| `code-intel-service` vào `SERVICES` | `backend-go/Makefile` dòng 7–11 (đủ cho `build`, `vet`, `test`, `lint`, `tidy-all`) |
| `codeintel` vào `DATABASES` | `backend-go/deploy/postgres-init-databases.sh` **và** `deploy/dev/docker/postgres/init-databases.sh` |
| `code-intel-service` + `migrate-codeintel` | `deploy/dev/docker-compose.yml`: mẫu `task-service` (dòng 325) và `migrate-task` (dòng 705); `container_name: orca-go-code-intel`; `mem_limit: 256m` (mặc định 128m, snapshot ≤ 3 MiB; chưa đo); mount `./policy/orca-authz:/policy/orca-authz:ro`; `DATABASE_DSN: postgresql://orca:${POSTGRES_PASSWORD}@postgres:5432/codeintel?sslmode=disable`; thêm `CODEINTEL_*` công tắc, `OPA_BUNDLE_PATH`; **không** thêm lại 4 biến địa chỉ (C3) |
| `codeintel` vào `SERVICES` | `deploy/dev/scripts/migrate.sh` dòng 27 (chuỗi `SERVICES`) |
| `code-intel-service` vào `ALL_SERVICES` | `deploy/dev/scripts/build-local.sh` (mảng ở dòng 37–44) |
| `code-intel-service` vào `svcs` | `backend-go/ci/check-opa-bundle-in-images.sh` |
| `CODE_INTEL_SERVICE_ADDR` | **không** ở đây (SOL-040) |
| Workflow | `.github/workflows/backend-go-code-intel-service.yml` (mới): sao `backend-go-task-service.yml` (ma trận `dialect: [postgres, mysql]`) + bước `buf lint --path orca/codeintel` và `buf breaking --path orca/codeintel --against '../../.git#branch=origin/main,subdir=backend-go/proto'` (mẫu `backend-go-mcp-service.yml` dòng 26–36; chỉ `breaking` khi file đã có trên `origin/main`); `paths`: `backend-go/common/**`, `backend-go/proto/orca/codeintel/**`, `backend-go/proto/gen/go/orca/codeintel/**`, `backend-go/services/code-intel-service/**`, `backend-go/go.work` |

## 3. Quyết định thiết kế

| Quyết định | Lý do |
|---|---|
| Hai dialect ngay | O1; mẫu `notification-service` |
| Outbox qua tham số `events` của phương thức repository (không `TxRunner`) | `task-service` dùng `RunInTx(fn(ctx, repos…))` không lồng được; `mcp-service` ghi sự kiện trong cùng transaction với phương thức; giữ một transaction cho thay đổi + sự kiện |
| RLS thật (mẫu `mcp-service`) | RLS kiểu `task-service` không chạy (không `set_config`) |
| Interceptor stream cục bộ, không sửa `common/grpcmw` | `ChainUnary` có 16 điểm gọi trực tiếp (chú thích `StatsHandler`) |
| `common/apperrors` chỉ thêm 2 Kind ở cuối | PQ-03 (8); giữ nguyên giá trị Kind cũ |
| Không dial service khác ở SOL này | Chạy/test độc lập |
| Workflow gọi `buf` trực tiếp | `make proto-lint` có `\|\| true` |

## 4. Tiêu chí chấp nhận

- [ ] `make build vet test lint` xanh với module mới trong `go.work`; `make tidy-all` không đổi gì thêm.
- [ ] `common/apperrors`: hai Kind mới ánh xạ đúng `codes`; Kind cũ không đổi; test hiện có xanh.
- [ ] `buf lint` xanh cho `orca.codeintel.v1`; stub sinh vào `proto/gen/go/orca/codeintel/v1`; `buf breaking` xanh so với `main` (bỏ qua khi file chưa có trên `main`).
- [ ] Postgres và MySQL: service khởi động, `/healthz`, `/readyz` trả 200; `grpcurl` thấy `orca.codeintel.v1.CodeIntelService` qua reflection; DSN scheme lạ → thoát với lỗi `detecting database dialect`.
- [ ] Migration `0001_init` up → down → up sạch ở hai dialect.
- [ ] Ghi `outbox_events` cùng transaction rồi relay publish lên NATS; rollback → không có dòng; hai relay đồng thời không mất sự kiện.
- [ ] Postgres role `NOSUPERUSER NOBYPASSRLS`: tenant A không đọc/ghi được dòng tenant B; chỉ `app.relay='on'` đọc chéo tenant (`SELECT`/`UPDATE`, không `INSERT`). MySQL: test chứng minh mọi truy vấn lọc `tenant_id`.
- [ ] Test AST quét mọi file repository ở cả hai adapter.
- [ ] Stream thử: có metadata tenant → đọc được; không → `RequireTenantID` lỗi.
- [ ] `docker compose` dựng được `code-intel-service` và `migrate-codeintel` chạy xong; workflow CI chạy hai dialect.
- [ ] Không file/thư mục tên `helpers|utils|common|misc`; không `max-lines` disable mới.

## 5. Kiểm thử

- **Unit:** `config` (mặc định, giá trị sai, vượt `SNAPSHOT_MAX_BYTES` tối đa, DSN rỗng); `toMySQLDriverDSN` (dạng `mysql://…@tcp(…)`, `mysql://user:pass@host/db`, `tidb://`, scheme lạ, thiếu host, tự thêm `parseTime=true`); interceptor stream; AST guard; `apperrors` bảng ánh xạ.
- **Integration** (`-tags=integration`, `common/testutil.{StartPostgres,StartMySQL,StartNATS}`), mỗi dialect: migration up/down/up; `FetchUnpublished` theo `created_at`; `MarkPublished`; hai relay đồng thời; RLS role không superuser (Postgres).
- **Hợp đồng:** `buf lint`, `buf breaking`; test gRPC in-process kiểm reflection.
- **Chưa chạy bất kỳ test nào.**

## 6. Rủi ro và điểm chưa kiểm chứng

- `buf lint STANDARD` với `service` rỗng chưa chạy (task 010-01 kiểm trước khi viết tiếp).
- `go.mod` module mới: phiên bản `golang.org/x/time` thêm ở SOL-013; `go.work.sum` đồng bộ chưa chạy.
- Dev compose chỉ Postgres; MySQL chỉ kiểm bằng CI.
- RLS không có hiệu lực ở dev (role `orca` là superuser, `docker-compose.yml` `POSTGRES_USER: orca`); mọi bảo đảm cô lập ở dev dựa vào lọc `tenant_id` tầng ứng dụng.
- `mem_limit: 256m` chưa đo (SOL-071).
- Sự kiện cùng transaction có cùng `created_at` (C4); hiện không xảy ra, nhưng nếu SOL sau ghi hai sự kiện/transaction cần cột thứ tự (Q2).
- Thêm Kind vào `common/apperrors` chạm mọi service khi build; rủi ro thấp (cộng) nhưng phải chạy `make build` toàn workspace.

## 7. Câu hỏi mở

- **Q1.** Smoke RPC có message ngay ở SOL này thay vì service rỗng? Mặc định: không.
- **Q2.** Có thêm cột `seq` (như v6 SOL-001 D2) vào `outbox_events` không? Hợp đồng T0 không có; mặc định: không, giữ theo hợp đồng; chủ hợp đồng quyết nếu 011–013 ghi nhiều sự kiện/transaction.
- **Q3.** "`codeintel.proto` rỗng-có-health" ở §7.1 G0: xác nhận chỉ là HTTP health.
- **Q4.** Tên stream `CODEINTEL`: chưa liệt kê các stream đang chạy (`TASK`, `PROJECT`, `INFRAFLEET`, `MCP`, …).

## 8. Tham chiếu

- `/opt/repos/orca/specs/backend-go/crs/v7/CONTRACT-codeintel-proto-and-data-map.md` (PQ-03, PQ-07, PQ-14, PQ-23, PQ-24; §2, §4.1–4.3, §5, §6, §7, §10)
- `/opt/repos/orca/docs/crs/v7/code-intel-service-foundation/CR-CV-010-scaffold-code-intel-service.md`
- `/opt/repos/orca/specs/backend-go/crs/v6/request-service-foundation/solutions/BE-REQ-SOL-001-scaffold-request-service.md` (mẫu định dạng)
- `/opt/repos/orca/backend-go/services/{notification-service,task-service,mcp-service}/…` (đã liệt kê ở mục 1), `backend-go/common/…`, `deploy/dev/…`, `.github/workflows/…`
