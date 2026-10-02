# BE-MCP-SOL-001: Dựng `mcp-service` — module Go, proto nội bộ, schema `mcp` có RLS thật

> **🔲 Designed — chưa implement.** Điều kiện tiên quyết của mọi solution v5 còn lại. Không phụ thuộc solution nào.

**CR:** [CR-MCP-001](../../../../../../docs/crs/v5/mcp-service-foundation/CR-MCP-001-scaffold-mcp-service.md)
**Service:** `mcp-service` (mới) · `proto` · `go.work`/`Makefile`/`deploy/*` · `.github/workflows`
**TDD tham chiếu:** [`arch/03`](../../../../tdd/architecture/03-clean-architecture-guidelines.md) (layout), [`arch/05`](../../../../tdd/architecture/05-data-architecture.md) (DB-per-service, RLS, outbox), [`arch/08`](../../../../tdd/architecture/08-inter-service-communication.md) (subject/outbox), [`arch/09`](../../../../tdd/architecture/09-observability-reliability.md) · **Hợp đồng FE:** [CONTRACT](../../CONTRACT-mcp-ui-api.md) §1 `McpServerInfo` (nguồn dữ liệu), §2.3 mã lỗi

---

## 1. Trạng thái hiện tại (re-verify — xem [README](./README.md))

Đã đọc trực tiếp: `go.work`, `Makefile`, `services/usage-service/**` (main.go, config, Dockerfile, migrations 0001/0002, `repository_test.go:130`), `common/{config,eventbus,outbox,tenant,apperrors,grpcmw,health}`, `proto/{buf.yaml,buf.gen.yaml,orca/usage/v1/usage.proto}`, `backend-go/deploy/postgres-init-databases.sh`, `deploy/dev/docker-compose.yml` + `scripts/`. Khuôn `usage-service` là mẫu đúng: `cmd/server/main.go` là composition root duy nhất (NATS → tracing → DSN từ `secrets.DatabaseCredentialsFromFile` → pool → outbox relay → `grpc.NewServer(grpcmw.ChainUnary(logger), grpcmw.StatsHandler())` → HTTP `health.New().Handler()`), `config.Load()` nhúng `commonconfig.Base` (`GRPC_PORT` 9090, `HTTP_PORT` 8080, `DATABASE_DSN`, `OTLP_ENDPOINT`).

### Quyết định khác/thêm so với CR gốc

1. **Postgres-only (tạm thời) — Đã chốt (D2, 2026-10-01), fail-fast nếu DSN không phải Postgres.** TDD `arch/04` coi multi-dialect là chuẩn (mọi service đều có `migrations/mysql`), nhưng `mcp-service` dùng `FOR UPDATE SKIP LOCKED`, `JSONB`, partial index, RLS `FORCE` — không có tương đương MySQL. `main.go` gọi `dbcapability.DetectDialectFromDSN` và trả lỗi rõ ràng nếu `!= DialectPostgres`. `usecase/ports.go` **không** import gì của Postgres ⇒ thêm adapter MySQL sau không phải sửa usecase. Ghi vào README service + backlog. **Đã chốt, không còn chờ xác nhận.** Hỗ trợ đa dialect về sau sẽ cần: `migrations/mysql` + `adapter/mysql` song song, thay `FOR UPDATE SKIP LOCKED`/`JSONB`/partial index/RLS bằng cơ chế tương đương (hoặc kiểm ở tầng ứng dụng), và CI matrix như `backend-go-usage-service.yml` — hiện **không** tạo thư mục `mysql/`.
2. **`0001_init` chỉ tạo bảng có mã dùng ngay** (`tenant_settings`, `processed_events`, `outbox_events`). CR liệt kê đủ 8 bảng, nhưng bảng không có usecase là schema chết; mỗi solution sở hữu bảng tự thêm migration kế tiếp (`0002_sessions` ← SOL-004, `oauth_*`/`grants` ← SOL-005, `tool_policies` ← SOL-012, `approval_requests` ← SOL-013, `external_servers` ← SOL-014). Chỉ **thêm** (expand-only), CI up→down→up chạy trên từng bước.
3. **RLS phải hoạt động thật**, khác `usage-service`: `ENABLE` + `FORCE ROW LEVEL SECURITY`, mọi truy vấn chạy trong `withTenantTx` gọi `set_config('app.tenant_id', $1, true)`; test tích hợp dùng role **không phải superuser** (superuser bypass RLS kể cả `FORCE`).
4. **Proto 001 chỉ khai báo RPC có triển khai thật**: `GetServerInfo`. Các nhóm RPC khác của CR (Session, Grant, Policy, Approval, ExternalServer…) do solution sở hữu thêm vào cùng file — thêm RPC/message là thay đổi additive, `buf breaking` (FILE) vẫn xanh. Không khai báo RPC `Unimplemented` "cho đủ bộ" (tránh README ghi "Real" cho thứ còn stub).

## 2. Giải pháp

### A. Cây thư mục `backend-go/services/mcp-service/`

```
go.mod                      # module github.com/stablyai/orca-go/services/mcp-service ; go 1.25.0
cmd/server/main.go          # composition root (khuôn usage-service, Postgres-only)
internal/config/config.go
internal/domain/            # stdlib only
    tenant_settings.go      # TenantSettings, KillSwitch
    scope.go                # ScopeDescriptor + Catalog() (orca:read|write|exec|admin + Risk)
    outbox.go               # OutboxRecord (khuôn usage-service/internal/domain/outbox.go)
    errors.go               # apperrors constructors: ErrTenantSettingsInvalid ...
internal/usecase/
    ports.go                # TenantSettingsRepository, OutboxWriter, Clock
    get_server_info.go      # GetServerInfo.Execute(ctx) -> ServerInfo
internal/adapter/grpc/server.go
internal/adapter/postgres/{repository.go,tenant_tx.go,repository_integration_test.go}
migrations/postgres/0001_init.{up,down}.sql
deploy/Dockerfile           # copy usage-service/deploy/Dockerfile, đổi tên binary
README.md                   # "real vs stub" (xem §F)
```

`internal/config/config.go`:

```go
type Config struct {
    commonconfig.Base
    NATSURL                 string // NATS_URL, mặc định nats://localhost:4222
    DatabaseCredentialsFile string // DATABASE_CREDENTIALS_FILE (Vault Agent), rơi về DATABASE_DSN
    // TenantDefaultEnabled: giá trị `enabled` khi tạo lười hàng tenant_settings cho tenant mới.
    // Đã chốt (D6): mặc định TRUE; admin vẫn tắt được. Chỉ ảnh hưởng `enabled`, không đổi mặc định
    // rủi ro/hard-deny/scope/kill switch (BE-012, BE-015). Công tắc tổng vẫn là MCP_ENABLED của gateway.
    TenantDefaultEnabled bool   // MCP_TENANT_DEFAULT_ENABLED, mặc định true
    DefaultMaxTokenDays  int    // MCP_DEFAULT_MAX_TOKEN_DAYS, mặc định 90 (trần PAT, T4)
}
```

Tên env `MCP_*` theo T8; env của gateway (BE-002) khác env của service này, không dùng chung tên trừ `MCP_SESSION_IDLE_TTL` (BE-004).

### B. Domain & usecase tối thiểu (thật, không stub)

```go
// internal/domain/tenant_settings.go
type KillSwitch struct{ Active bool; Reason string; At *time.Time }
type TenantSettings struct {
    TenantID           string
    Enabled            bool
    DCREnabled         bool
    MaxTokenDays       int
    ApprovalTTLSeconds int
    KillSwitch         KillSwitch
    UpdatedAt          time.Time
}

// internal/usecase/get_server_info.go — một usecase, một type, Execute(ctx, in)
type GetServerInfo struct{ repo TenantSettingsRepository; cfg Defaults }
func (uc *GetServerInfo) Execute(ctx context.Context) (ServerInfo, error) {
    tenantID, err := tenant.RequireTenantID(ctx) // KHÔNG nhận tenant từ request (arch/05, C2)
    if err != nil { return ServerInfo{}, err }
    s, found, err := uc.repo.GetTenantSettings(ctx, tenantID)
    if err != nil { return ServerInfo{}, err }
    if !found { s = uc.cfg.DefaultsFor(tenantID) } // enabled = MCP_TENANT_DEFAULT_ENABLED (hàng được tạo lười khi tenant ghi lần đầu hoặc lần đầu `GetServerInfo`; sau đó thay đổi env không đổi tenant đã có hàng)
    return ServerInfo{Settings: s, Scopes: domain.ScopeCatalog()}, nil
}
```

`domain.ScopeCatalog()` là **nguồn duy nhất** cho `McpServerInfo.scopesSupported` (CONTRACT §1): bốn scope `orca:read`(risk `read`), `orca:write`(`write_reversible`), `orca:exec`(`exec`), `orca:admin`(`admin`), nhãn/mô tả tiếng Anh. SOL-006 mở rộng additive.

### C. Proto `backend-go/proto/orca/mcp/v1/mcp.proto` (bề mặt nội bộ gateway ↔ mcp-service, không phải wire MCP)

```proto
syntax = "proto3";
package orca.mcp.v1;
option go_package = "github.com/stablyai/orca-go/proto/gen/go/orca/mcp/v1;mcpv1";
import "google/protobuf/timestamp.proto";

// McpService giữ trạng thái/quyết định của lớp MCP (T7). tenant_id/user_id/role đi qua
// gRPC metadata (common/tenant), KHÔNG có field tenant trong request.
service McpService {
  rpc GetServerInfo(GetServerInfoRequest) returns (GetServerInfoResponse);
  // Các nhóm RPC khác được thêm bởi: BE-004 (Session), BE-005/006 (Grant/Client/Token),
  // BE-012 (Policy), BE-013 (Approval/Audit/KillSwitch), BE-014 (ExternalServer).
}

message GetServerInfoRequest {}
message GetServerInfoResponse {
  bool enabled = 1;
  bool dcr_enabled = 2;
  int32 max_token_days = 3;
  int32 approval_ttl_seconds = 4;
  KillSwitch kill_switch = 5;
  repeated ScopeDescriptor scopes = 6;
}
message KillSwitch { bool active = 1; string reason = 2; google.protobuf.Timestamp at = 3; }
message ScopeDescriptor { string id = 1; string label = 2; string description = 3; string risk = 4; }
```

Sinh stub: `make proto-gen` (`cd proto && buf generate`, plugin `protoc-gen-go` + `protoc-gen-go-grpc`, `paths=source_relative`) → `proto/gen/go/orca/mcp/v1/`. `buf lint` (STANDARD) + `buf breaking` (FILE) — lưu ý Makefile `proto-lint` hiện kết thúc bằng `|| true` nên **không chặn**; CI mới phải gọi `buf breaking` trực tiếp (§E).

### D. Migration `migrations/postgres/0001_init.up.sql`

```sql
CREATE SCHEMA IF NOT EXISTS mcp;

CREATE TABLE mcp.tenant_settings (
    tenant_id             UUID PRIMARY KEY,
    enabled               BOOLEAN NOT NULL DEFAULT false,      -- chỉ là lưới an toàn của DDL; usecase luôn INSERT tường minh enabled = MCP_TENANT_DEFAULT_ENABLED (D6)
    dcr_enabled           BOOLEAN NOT NULL DEFAULT false,      -- BE-005 quyết định mặc định hiệu lực
    max_token_days        INT     NOT NULL DEFAULT 90 CHECK (max_token_days BETWEEN 1 AND 90),
    approval_ttl_seconds  INT     NOT NULL DEFAULT 600 CHECK (approval_ttl_seconds BETWEEN 30 AND 86400),
    kill_switch_active    BOOLEAN NOT NULL DEFAULT false,
    kill_switch_reason    TEXT    NOT NULL DEFAULT '',
    kill_switch_at        TIMESTAMPTZ,
    updated_by            UUID,                                  -- logical FK user (auth-service), KHÔNG FK xuyên DB
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE mcp.processed_events (          -- dedupe consumer NATS (arch/08), khuôn notification-service
    tenant_id    UUID NOT NULL,
    event_id     UUID NOT NULL,
    processed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, event_id)
);

CREATE TABLE mcp.outbox_events (             -- khuôn usage.outbox_events (0002_outbox)
    id UUID PRIMARY KEY, tenant_id UUID NOT NULL, subject TEXT NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL, version INT NOT NULL, payload JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(), published_at TIMESTAMPTZ
);
CREATE INDEX idx_mcp_outbox_unpublished ON mcp.outbox_events (created_at) WHERE published_at IS NULL;

-- RLS thật (khác usage-service): FORCE để chủ bảng cũng bị áp dụng.
DO $$ DECLARE t text; BEGIN
  FOREACH t IN ARRAY ARRAY['tenant_settings','processed_events','outbox_events'] LOOP
    EXECUTE format('ALTER TABLE mcp.%I ENABLE ROW LEVEL SECURITY', t);
    EXECUTE format('ALTER TABLE mcp.%I FORCE ROW LEVEL SECURITY', t);
    EXECUTE format($p$CREATE POLICY tenant_isolation ON mcp.%I
        USING (tenant_id = current_setting('app.tenant_id', true)::uuid)
        WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::uuid)$p$, t);
  END LOOP;
END $$;
```

`down.sql`: `DROP SCHEMA mcp CASCADE`. Lưu ý: `outbox.Relay` đọc *mọi* tenant ⇒ `FetchUnpublished/MarkPublished` chạy bằng kết nối riêng với role `mcp_relay` có `BYPASSRLS` **hoặc** policy thứ hai `USING (current_setting('app.relay', true) = 'on')` — chọn policy thứ hai (không cần role đặc quyền): `relay` đặt `set_config('app.relay','on',true)` chỉ trong hai hàm Store.

`internal/adapter/postgres/tenant_tx.go`:

```go
// withTenantTx: mọi truy vấn domain phải đi qua đây để RLS có hiệu lực.
func (r *Repository) withTenantTx(ctx context.Context, tenantID string, fn func(pgx.Tx) error) error {
    tx, err := r.pool.Begin(ctx); if err != nil { return err }
    defer tx.Rollback(ctx) //nolint:errcheck
    if _, err := tx.Exec(ctx, `SELECT set_config('app.tenant_id', $1, true)`, tenantID); err != nil { return err }
    if err := fn(tx); err != nil { return err }
    return tx.Commit(ctx)
}
```

Truy vấn vẫn `WHERE tenant_id = $1` tường minh (phòng thủ nhiều lớp, như `usage-service`), RLS là lớp thứ hai **có hiệu lực**.

### E. Wiring repo-level (đủ để `make build/vet/test/lint` và `dev-up` chạy)

| File | Thay đổi |
|---|---|
| `backend-go/go.work` | thêm `./services/mcp-service` (giữ thứ tự chữ cái, sau `issue-tracking-service`… trước `notification-service`) |
| `backend-go/Makefile` | thêm `mcp-service` vào `SERVICES :=` |
| `backend-go/deploy/postgres-init-databases.sh` | thêm `mcp` vào `DATABASES`. **Chỉ chạy lần đầu volume** ⇒ môi trường đã có volume phải `CREATE DATABASE mcp OWNER orca;` thủ công (ghi vào README + rollout) |
| `deploy/dev/docker-compose.yml` | khối `mcp-service:` theo khuôn `usage-service` (`<<: *go-defaults`, `container_name: orca-go-mcp`, `./bin/mcp-service/orca:/app/bin/orca:ro`, `DATABASE_DSN: postgresql://orca:${POSTGRES_PASSWORD}@postgres:5432/mcp?sslmode=disable`, `MCP_TENANT_DEFAULT_ENABLED`, `depends_on` postgres+nats); thêm `MCP_SERVICE_ADDR: mcp-service:9090` vào khối anchor `go-common-env` (cạnh `USAGE_SERVICE_ADDR`, dòng ~70) |
| `deploy/dev/scripts/build-local.sh`, `migrate.sh` | thêm `mcp-service` / `mcp` vào danh sách (`migrate.sh:27` có `SERVICES="auth tenant … usage credential issuetracking scm"` — cách ánh xạ tên→thư mục migration **(chưa xác minh)**, đọc script khi làm) |
| `.github/workflows/backend-go-mcp-service.yml` | khuôn `backend-go-usage-service.yml` nhưng **không matrix dialect**: `go build`, `go vet`, `go test ./...`, `go test -tags=integration ./internal/adapter/postgres/...`, bước `buf lint` + `buf breaking --against '.git#branch=main'` (chạy **thật**, không `|| true`); `paths:` gồm `backend-go/common/**`, `backend-go/proto/orca/mcp/**`, `backend-go/services/mcp-service/**` |
| `deploy/Dockerfile` | sao `usage-service/deploy/Dockerfile` (`golang:1.25-bookworm`, distroless, `COPY services/mcp-service/migrations /migrations`) |

`main.go` khác `usage-service` ở: (i) không có nhánh MySQL; (ii) `pub.EnsureStream(ctx, "MCP", []string{"orca.mcp.>"})` + `outbox.NewRelay(store, pub, outbox.DefaultConfig, logger)`; (iii) `healthSrv.Register("postgres", …)`; (iv) đăng ký `mcpv1.RegisterMcpServiceServer`. **Lưu ý ranh giới subject:** subject tạm thời của T5 nằm ở `orca.ephemeral.mcp.>` và **không** khớp `orca.mcp.>`, nên không bị stream `MCP` bắt nhầm (xem BE-004).

### F. `services/mcp-service/README.md` — bảng "real vs stub"

| Hạng mục | Trạng thái |
|---|---|
| gRPC `GetServerInfo` | **Real** (đọc `mcp.tenant_settings`) |
| Outbox relay → `orca.mcp.>` | **Real** (chưa có event nào được ghi cho tới BE-004) |
| Session / OAuth client / grant / policy / approval / external server | **Chưa có** (SOL-004/005/006/012/013/014) — không khai báo RPC |
| MySQL | **Không hỗ trợ** (fail-fast) |

## Hợp đồng với frontend

Gián tiếp: `GetServerInfo` là nguồn của `McpServerInfo.{enabled,dcrEnabled,maxTokenDays,killSwitch,scopesSupported}` mà kênh `mcp.server.info` (BE-003) trả cho FE-MCP-SOL-001/002. Kiểu `McpScopeDescriptor.risk` ∈ `McpRisk` ⇒ `domain.Risk` là chuỗi đúng 5 giá trị của CONTRACT §1. Mã lỗi usecase dùng tiền tố `MCP_*` trong CONTRACT §2.3 (`apperrors.New(KindNotFound, "MCP_NOT_FOUND", …)`; không phân biệt "không tồn tại"/"không có quyền").

## Sửa TDD kèm theo

- **T7** — `00-service-catalog.md` thêm hàng `mcp-service`; `arch/02` bảng + đồ thị + số đếm; `arch/03,04,05,09,10` đổi "17 services" (bảng README v5 §3). Thực hiện bởi BE-MCP-SOL-015.
- **T8** — `arch/09` ghi tiền tố `orca_mcp_*`/`MCP_*`.
- Ghi chú thêm: `arch/05` bổ sung quy tắc "RLS chỉ tính khi `FORCE` + `set_config` + role không superuser" (phát hiện từ `usage-service`).

## Kiểm thử

```bash
cd /opt/repos/orca/backend-go
make proto-gen && cd proto && buf lint && buf breaking --against '.git#branch=main'
cd services/mcp-service && go build ./... && go vet ./... && go test ./...
go test -tags=integration ./internal/adapter/postgres/... -v     # testcontainers (common/testutil.StartPostgres)
make -C .. build vet lint                                          # cần mcp-service nằm trong SERVICES
```

Integration (`//go:build integration`): (1) migration up→down→up sạch; (2) **RLS**: tạo role `mcp_app NOSUPERUSER`, `GRANT` trên schema, mở pool bằng role đó; ghi `tenant_settings` cho tenant A; với `app.tenant_id`=B `SELECT` trả 0 hàng; `INSERT` hàng tenant A khi `app.tenant_id`=B bị `WITH CHECK` từ chối; (3) `GetServerInfo` khi chưa có hàng ⇒ dùng mặc định `MCP_TENANT_DEFAULT_ENABLED`; (4) kết nối DSN `mysql://…` ⇒ `main` thoát với lỗi rõ ràng (unit test hàm `requirePostgres(dsn)`). Unit: `GetServerInfo` với repo giả (tenant thiếu trong ctx ⇒ lỗi `Unauthenticated`).

## Rủi ro & phụ thuộc

- Quên `withTenantTx` ở một truy vấn ⇒ với `FORCE RLS` kết quả rỗng (an toàn, nhưng khó debug): lint bằng test "mọi method repository gọi `withTenantTx`" (reflection/grep CI nhỏ).
- Relay đọc xuyên tenant: policy `app.relay` giới hạn trong hai hàm Store; review kỹ.
- Chia sẻ cluster Postgres với service khác (câu hỏi mở của CR): dev dùng chung instance (một DB `mcp`), staging/prod theo `arch/10` — không đổi thiết kế.
- *Impact (`gitnexus`)*: module mới, không sửa symbol Go hiện có; chỉ sửa `go.work`, `Makefile`, script/compose ⇒ không cần `impact` cho code. Nếu thêm helper vào `common/*` thì chạy trước khi sửa: `gitnexus impact common/outbox --direction upstream` (**chưa chạy**, rủi ro dự kiến MEDIUM: nhiều service import).

## Không thuộc phạm vi

Giao thức MCP (BE-003), session (BE-004), OAuth/PAT (BE-005/006), policy (BE-012), adapter MySQL, Helm/ArgoCD (repo này không có chart — **chưa xác minh** vị trí `orca-go-common-chart`).

## Liên quan

[CR-001](../../../../../../docs/crs/v5/mcp-service-foundation/CR-MCP-001-scaffold-mcp-service.md) · [BE-MCP-SOL-002](./BE-MCP-SOL-002-gateway-mcp-endpoint-wiring.md) · `backend-go/services/usage-service/` (khuôn) · `backend-go/common/outbox/outbox.go`
