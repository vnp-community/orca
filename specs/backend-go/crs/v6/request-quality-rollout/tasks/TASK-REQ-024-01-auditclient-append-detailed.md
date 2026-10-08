# TASK-REQ-024-01: `common/auditclient.AppendDetailed` mang `actor_type`, `target_type`, `target_id`, `metadata_json`

**From Solution:** BE-REQ-SOL-024
**Priority:** P1
**Service:** `common`
**File:** `backend-go/common/auditclient/client.go`, `backend-go/common/auditclient/client_test.go`
**Depends on:** None
**Status:** [x] DONE (kiểm toán 2026-10-07: đủ code và test đơn vị)

---

## Context

- `client.go`: `Client{auth authv1.AuthServiceClient}`, `New`, `Append(ctx, tenantID, actorID, action, target, outcome, ip)` gọi `AppendAuditEntry` và **nuốt lỗi** (best effort có chủ ý).
- `proto/orca/auth/v1/auth.proto:371-382`: `AppendAuditEntryRequest` đã có `actor_type = 7`, `target_type = 8`, `target_id = 9`, `metadata_json = 10` ("Additive (BE-MCP-SOL-013). Empty actor_type means 'user'").
- DB `auth.audit_log` có `CHECK (actor_type IN ('user','agent','system'))` (`auth-service/migrations/postgres/0013_audit_actor_type.up.sql`); `outcome` chỉ `allowed|denied` (`auth-service/internal/domain/audit.go`).
- Nhiều service dùng `Append` (annotation, infra-fleet, project, task). Không đổi chữ ký của nó.
- Chạy `gitnexus_impact` trên `Append` trước khi sửa (quy tắc dự án).

## Việc cần làm

1. Thêm kiểu `Entry struct { TenantID, ActorID, ActorType, Action, Target, TargetType, TargetID, Outcome, IPAddress, MetadataJSON string }`.
2. Thêm `func (c *Client) AppendDetailed(ctx context.Context, e Entry)` gọi `AppendAuditEntry` với đủ 10 trường; nuốt lỗi như `Append`.
3. Viết lại `Append` gọi `AppendDetailed(Entry{...})` (hành vi không đổi: `ActorType` rỗng).
4. Kiểm phía client: `ActorType` ngoài `user|agent|system` thì để rỗng (và ghi log mức debug), không gửi giá trị làm RPC bị từ chối; `MetadataJSON` > 4096 byte thì thay bằng `{"truncated":true}`.
5. Doc comment ngắn ghi lý do tách (không mở rộng chữ ký `Append` để khỏi vỡ các service gọi).

## Kiểm thử

- `client_test.go`: fake `AuthServiceClient` ghi lại request; `AppendDetailed` gửi đủ trường; `Append` gửi như cũ (`ActorType` rỗng); `ActorType` lạ bị bỏ; metadata quá dài bị thay; lỗi RPC không panic.
- Lệnh: `cd backend-go/common && go test ./auditclient/...`; rồi `cd backend-go && go build ./...` hoặc `make build` (kiểm các service gọi `Append` vẫn biên dịch).

## Tiêu chí hoàn thành

- [x] `AppendDetailed` có test; `Append` không đổi hành vi.
- [x] Tất cả module trong `go.work` biên dịch.

## Rủi ro và lưu ý

- Không đưa `title`, `body` vào `MetadataJSON` ở nơi gọi (kiểm ở task 02).
- Nếu `auth-service` thêm kiểm `metadata_json` nghiêm hơn, test tích hợp ở auth-service mới bắt được; chưa chạy.
