# TASK-REQ-035-06: Migration `security_compliance`, `request_audit_outbox` và bộ giao audit không-được-mất

**From Solution:** BE-REQ-SOL-035 (mục E audit, C4, C5)
**Priority:** P0
**Service:** `request-service`
**File:** `backend-go/services/request-service/migrations/{postgres,mysql}/NNNN_security_compliance.{up,down}.sql` (mới), `.../internal/domain/{audit_event.go,audit_metadata.go}` (mới/sửa: do TASK-REQ-024-02 có thể đã tạo `AuditEvent`), `.../internal/usecase/{audit_outbox.go,audit_recorder.go}` (sửa/mới), `.../internal/adapter/{postgres,mysql}/audit_outbox.go` (mới), `.../internal/adapter/auditdelivery/deliverer.go` (mới), `.../cmd/server/main.go` (sửa: chạy bộ giao), và `_test.go` tương ứng
**Depends on:** TASK-REQ-024-01 (`auditclient.AppendDetailed`, `Entry`), TASK-REQ-024-02 (`AuditRecorder`, `AuditEvent`), TASK-REQ-025-01 (`tenant_settings`), TASK-REQ-035-05 (RLS cho bảng mới)
**Status:** `[ ] TODO`

---

## Context

- `common/auditclient/client.go`: `Append` gửi 6 trường, **nuốt lỗi** (best-effort có chủ ý, để auth-service sập không biến quyết định quyền thành 500). `AppendAuditEntryRequest` (`proto/orca/auth/v1/auth.proto:371-382`) đã có `actor_type`, `target_type`, `target_id`, `metadata_json`; DB `auth.audit_log.actor_type ∈ user|agent|system` (`0013_audit_actor_type.up.sql`), `outcome` chỉ `allowed|denied`.
- TASK-REQ-024-01 định nghĩa `Entry{TenantID, ActorID, ActorType, Action, Target, TargetType, TargetID, Outcome, IPAddress, MetadataJSON string}` và `AppendDetailed(ctx, Entry)`; `MetadataJSON` > 4096 byte bị thay bằng `{"truncated":true}` ở client (đã đặt trong task đó). Task này **không định nghĩa lại**; CR 035 viết `Metadata map[string]any` là cách gọi tiện ở tầng `request-service` (marshal tại `audit_metadata.go`).
- TASK-REQ-024-02: `AuditRecorder.Record(ctx, AuditEvent)` gọi sau commit, không audit lỗi kỹ thuật. Task này thêm đường **bền** cho hành động không-được-mất và các hành động mới.
- `common/outbox` chỉ có vòng relay đẩy lên NATS (`Store.FetchUnpublished/MarkPublished` rồi `eventbus.Publisher.Publish`), không gọi RPC: `request_audit_outbox` cần bộ giao riêng (solution C5). `AppendAuditEntry` không có khoá idempotency ⇒ giao lặp tạo hai dòng audit; giảm thiểu bằng `metadata.audit_id`.
- Hành động không-được-mất (CR 2.5): `request.export`, `request.erase`, `approval.approve` cổng `pre_deploy`, đổi `ai_egress_mode` (`ai.egress.set`). Mọi hành động khác vẫn best-effort qua `Record`.
- `metadata_json` **không bao giờ** chứa `title`, `body`, nội dung Solution/Plan, `comment` Approval.

## Việc cần làm

1. Migration Postgres `NNNN_security_compliance.up.sql` (chốt `NNNN` bằng `ls` hai thư mục; chờ 025-01):

```sql
ALTER TABLE request.requests ADD COLUMN contains_secret_suspected BOOLEAN NOT NULL DEFAULT FALSE,
                             ADD COLUMN erased_at TIMESTAMPTZ, ADD COLUMN erased_by UUID;
CREATE TABLE request.request_audit_outbox (
  id UUID PRIMARY KEY, tenant_id UUID NOT NULL, audit_id UUID NOT NULL UNIQUE,
  action TEXT NOT NULL, actor_id TEXT NOT NULL, actor_type TEXT NOT NULL CHECK (actor_type IN ('user','agent','system')),
  target_type TEXT NOT NULL, target_id TEXT NOT NULL, outcome TEXT NOT NULL CHECK (outcome IN ('allowed','denied')),
  ip_address TEXT NOT NULL DEFAULT '', metadata_json TEXT NOT NULL DEFAULT '{}' CHECK (octet_length(metadata_json) <= 4096),
  attempts INT NOT NULL DEFAULT 0, next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(), delivered_at TIMESTAMPTZ);
CREATE INDEX request_audit_outbox_pending ON request.request_audit_outbox (next_attempt_at) WHERE delivered_at IS NULL;
CREATE TABLE request.request_webhook_nonces (tenant_id UUID NOT NULL, source TEXT NOT NULL, nonce_hash CHAR(64) NOT NULL,
  expires_at TIMESTAMPTZ NOT NULL, PRIMARY KEY (tenant_id, source, nonce_hash));
CREATE INDEX request_webhook_nonces_expiry ON request.request_webhook_nonces (expires_at);
ALTER TABLE request.tenant_settings ADD COLUMN request_retention_days INT NOT NULL DEFAULT 730 CHECK (request_retention_days >= 0),
  ADD COLUMN ai_trace_retention_days INT NOT NULL DEFAULT 30 CHECK (ai_trace_retention_days >= 0),
  ADD COLUMN ledger_retention_days INT NOT NULL DEFAULT 400 CHECK (ledger_retention_days >= 0),
  ADD COLUMN redact_pii_in_prompts BOOLEAN NOT NULL DEFAULT FALSE;
```

   Bật `ENABLE`/`FORCE ROW LEVEL SECURITY` + `tenant_isolation` cho hai bảng mới; riêng `request_audit_outbox` thêm policy `audit_relay` (`app.relay='on'`) cho bộ giao đa tenant. `down.sql` đảo ngược theo thứ tự. MySQL tương ứng (`CHAR(36)`, `TINYINT(1)`, `TEXT` + kiểm 4096 byte ở ứng dụng, không `WHERE ... IS NULL` partial index: dùng chỉ mục thường `(delivered_at, next_attempt_at)`). `0` ở các cột `*_retention_days` nghĩa là giữ mãi.
2. `domain/audit_metadata.go`: `func MarshalAuditMetadata(m map[string]any) (string, error)` từ chối (trả lỗi) khoá thuộc danh sách cấm `{"title","body","content","comment","options","prompt","response","excerpt"}` ở **mọi độ sâu** (duyệt đệ quy) và giá trị chuỗi dài hơn 256 ký tự; thêm `request_id`, `audit_id`, `correlation_id` (nếu ctx có trace id; `common/outbox` không truyền trace context nên mặc định chỉ `request_id`). Hàm thuần có test.
3. `usecase/audit_outbox.go`: `type AuditOutbox interface{ Enqueue(ctx context.Context, e DurableAudit) error }` (ghi trong **cùng giao dịch** `InTx` với thay đổi dữ liệu); `DurableAudit{Action, ActorID, ActorType, TargetType, TargetID, Outcome, IP string; Metadata map[string]any}`; `AuditRecorder.RecordDurable(ctx, e)` gọi `Enqueue` (nếu ctx không trong giao dịch thì tự mở một giao dịch nhỏ). Danh sách `durableActions` (map) = `request.export`, `request.erase`, `approval.approve` khi `subject_type=pre_deploy`, `ai.egress.set`; `Record` chuyển sang `RecordDurable` khi hành động nằm trong danh sách.
4. `auditdelivery/deliverer.go`: `Deliverer{store AuditOutboxStore; client AppendDetailedClient; clock; batch int; backoff}` chạy vòng `Run(ctx)`: mỗi lần `store.Claim(ctx, now, limit)` bằng `SELECT ... FOR UPDATE SKIP LOCKED` (Postgres) và `FOR UPDATE SKIP LOCKED` (MySQL ≥ 8.0.1, theo CR-DB-002) trong giao dịch ngắn, gọi `client.AppendDetailed` **có kiểm lỗi**: `auditclient.AppendDetailed` nuốt lỗi (best-effort) nên không phân biệt thành công, do đó **thêm** vào `common/auditclient` hàm `AppendDetailedStrict(ctx, Entry) error` trả lỗi RPC (additive, cùng gói, thuộc task này; không đổi `Append`/`AppendDetailed`). Thành công ⇒ `delivered_at=now`; lỗi ⇒ `attempts++`, `next_attempt_at = now + min(2^attempts × 5s, 15m)`; `attempts` ≥ 20 ⇒ log lỗi mức error một lần (không xoá, để người vận hành thấy). Dọn bản ghi đã giao quá 7 ngày bằng lô nhỏ.
5. `main.go`: khởi động `Deliverer.Run` trong goroutine có `WaitGroup` như outbox relay (SOL-001 mục C); dừng êm khi tắt.
6. Metric (không nhãn tenant): `request_audit_outbox_pending` (gauge, đo bằng `COUNT` mỗi 30 giây), `request_audit_delivery_failures_total`; alert đề xuất khi `pending` > 100 trong 10 phút (ghi vào task 024-08 alert, không tạo file ở đây).
7. Bảng hành động mới ở `AuditRecorder` (thêm hằng): `request.read.denied`, `request.access.denied`, `request.export`, `request.erase`, `request.retention.run`, `ai.budget.set`, `ai.egress.set` (cùng 12 hành động của CR-REQ-024 mục 2.8). `request.retention.run` ghi `{processed: n, anonymized: n}` (số lượng, không nội dung).
8. Không sửa hoặc xoá `auth.audit_log`: chính sách của bảng đó (INSERT/SELECT ở production, `0001_init.up.sql`) giữ nguyên.

## Kiểm thử

- `audit_metadata_test.go`: khoá cấm ở độ sâu 3 bị từ chối; chuỗi 300 ký tự bị từ chối; `request_id` luôn có.
- `property_audit_test.go` (`TestAuditMetadataNeverContainsContent`): với một Request chứa `title`/`body` đặc trưng (`"SECRET-TITLE-123"`), chạy mọi use case ghi audit với `AuditRecorder` giả và khẳng định không bản ghi nào chứa các chuỗi đó (duyệt mọi bản ghi, mọi trường).
- `audit_outbox_integration_test.go` (hai dialect): (a) `TestEnqueueInSameTx` (rollback giao dịch ⇒ không có dòng outbox); (b) `TestClaimSkipLocked` (hai bộ giao, mỗi dòng giao đúng một lần); (c) `TestDeliverRetriesWithBackoff` (fake client lỗi hai lần rồi thành công: `attempts=2`, `delivered_at` đặt); (d) `TestExportNotLostWhenAuthDown` (auth giả luôn lỗi ⇒ dòng còn `delivered_at IS NULL`; khi phục hồi được giao); (e) cách ly tenant; (f) `request.export` và `request.erase` luôn có dòng outbox cùng giao dịch với thay đổi.
- `common/auditclient/client_test.go`: `AppendDetailedStrict` trả lỗi RPC, `AppendDetailed` vẫn nuốt.
- Migration: up/down/up hai dialect; giá trị mặc định `730/30/400`.
- Lệnh: `cd backend-go && go test ./common/auditclient/... ./services/request-service/internal/domain/... ./services/request-service/internal/usecase/... ./services/request-service/internal/adapter/auditdelivery/... && go test -tags=integration ./services/request-service/internal/adapter/...`. Chưa chạy.

## Tiêu chí hoàn thành

- [ ] `request.export` và `request.erase` không mất khi `auth-service` tạm sập (bản ghi nằm trong `request_audit_outbox` và được giao sau).
- [ ] Mọi hành động ở CR-REQ-024 2.8 và CR 035 2.5 tạo đúng một bản ghi `AppendDetailed` với `actor_type` đúng; không có `title`/`body` trong `metadata_json` (test thuộc tính).
- [ ] Giao lặp không làm hỏng (có `audit_id` trong metadata).
- [ ] Migration up/down/up sạch hai dialect; RLS bật cho bảng mới.
- [ ] Không đổi hành vi `Append`/`AppendDetailed` hiện có.

## Rủi ro và lưu ý

- `AppendAuditEntry` không idempotent: giao lặp có thể tạo dòng audit trùng; người đọc khử trùng bằng `metadata.audit_id`.
- `AppendDetailedStrict` là thêm vào `common/auditclient` (dùng chung): additive, nhưng chạy `gitnexus_impact` trên `auditclient.Client` trước khi sửa.
- Audit vẫn best-effort cho hành động thường: sập `auth-service` có thể mất bản ghi `request.access.denied` (chấp nhận, CR mục 2.5).
- `ALTER TABLE ... ADD COLUMN` MySQL không `IF NOT EXISTS`; chạy một lần; xung đột tên cột với migration của CR 034 (khác tên) cần thứ tự merge rõ.
