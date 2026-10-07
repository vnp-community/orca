# TASK-REQ-025-01: Bảng `tenant_settings` và RPC `GetRequestFlowSettings`, `SetRequestFlowSettings`

**From Solution:** BE-REQ-SOL-025
**Priority:** P0
**Service:** `request-service`
**File:** `backend-go/services/request-service/migrations/postgres/00NN_tenant_settings.up.sql`, `.down.sql` (mới), `.../migrations/mysql/00NN_tenant_settings.up.sql`, `.down.sql` (mới), `.../internal/domain/flow_settings.go` (mới), `.../internal/usecase/flow_settings.go` (mới), `.../internal/adapter/{postgres,mysql}/tenant_settings.go` (mới), `.../internal/adapter/grpc/server.go`, `backend-go/proto/orca/request/v1/request.proto`
**Depends on:** CR-REQ-001, CR-REQ-002 (số migration tiếp theo, mẫu RLS)
**Status:** `[x] DONE`

---

## Context

- Mẫu cờ hai tầng: `mcp-service/internal/domain/tenant_settings.go` (`TenantSettings`, `Validate`) và `api-gateway/internal/config/config_mcp.go` (`MCP_ENABLED`, `MCP_TENANT_DEFAULT_ENABLED`).
- CR-REQ-001 khai báo biến `REQUEST_FLOW_ENABLED` (mặc định `false`) và log; chưa có đọc theo tenant. Số `00NN` chốt khi triển khai theo migration cuối của CR-REQ-002 và 004 đến 013.
- `tenant.Role(ctx)` (hoặc tên tương đương trong `common/tenant`) mang `admin|user`; đọc `common/tenant` trước khi dùng.
- Thiết kế bảng và hiệu lực: SOL-025 mục 2.2. Chỉ khai báo trong proto `GetRequestFlowSettings`, `SetRequestFlowSettings` (README v6 mục 8 dòng 12).

## Việc cần làm

1. Migration Postgres: bảng `request.tenant_settings(tenant_id TEXT PRIMARY KEY, request_flow_enabled BOOLEAN NOT NULL DEFAULT false, updated_by TEXT NOT NULL DEFAULT '', updated_at TIMESTAMPTZ NOT NULL DEFAULT now())`, `ENABLE`/`FORCE ROW LEVEL SECURITY`, chính sách `tenant_isolation` như bảng khác. MySQL: `tenant_id VARCHAR(255) PRIMARY KEY`, `request_flow_enabled TINYINT(1) NOT NULL DEFAULT 0`, `updated_at TIMESTAMP(6)`. `down` xoá bảng.
2. Domain `FlowSettings{TenantID string; Enabled bool}`; port `FlowSettingsRepository{Get(ctx, tenantID) (FlowSettings, bool, error); Upsert(ctx, tenantID string, enabled bool, updatedBy string) error}`.
3. Use case `EffectiveFlow(ctx) (bool, error)`: `cfg.FlowEnabled && row.Enabled`; thiếu dòng, lỗi đọc thì `false` (và log lỗi). `GetRequestFlowSettings` trả `{enabled: effective}`. `SetRequestFlowSettings`: chỉ `role=admin` (`REQUEST_APPROVAL_FORBIDDEN` hoặc mã riêng `REQUEST_FLOW_ADMIN_ONLY`, chốt khi viết), upsert, gọi `AuditRecorder` (`request.flow.set`, BE-REQ-SOL-024 task 02).
4. Proto: `message GetRequestFlowSettingsResponse { bool enabled = 1; }`, `message SetRequestFlowSettingsRequest { bool enabled = 1; }`, thêm hai RPC. `buf lint`, `buf breaking`.
5. Adapter Postgres dùng `withTenantTx` (theo CR-REQ-002); MySQL lọc `tenant_id` ở mọi truy vấn.

## Kiểm thử

- Unit: bảng `EffectiveFlow` (env tắt, tenant bật; env bật, thiếu dòng; env bật, tenant bật; lỗi đọc); `Set` từ chối không phải admin; ghi audit.
- Tích hợp hai dialect: upsert, đọc, mặc định thiếu dòng, cô lập tenant (RLS thật cho Postgres, dùng role không phải superuser).
- Migration up/down/up trên hai dialect.
- Lệnh: `go test ./services/request-service/...` và `-tags=integration`.

## Tiêu chí hoàn thành

- [x] Hai RPC hoạt động; mặc định `enabled=false`.
- [x] Biến tổng tắt thì `Get` trả `false` dù tenant bật.
- [x] Postgres và MySQL xanh.

## Rủi ro và lưu ý

- Làm task này sớm (CR-016, 017 đã trỏ tới `request.flowStatus`).
- Không chạm `task-service`: cờ không ẩn `type plan|phase`.
