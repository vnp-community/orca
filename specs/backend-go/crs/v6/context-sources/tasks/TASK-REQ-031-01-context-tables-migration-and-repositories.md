# TASK-REQ-031-01: Migration `context_sources`, `context_packs`, `evidence` và repository hai dialect

**From Solution:** BE-REQ-SOL-031 (mục B, D bước 7)
**Priority:** P1
**Service:** `request-service`
**File:** `backend-go/services/request-service/migrations/postgres/NNNN_context_sources.{up,down}.sql` (mới), `.../migrations/mysql/NNNN_context_sources.{up,down}.sql` (mới), `.../internal/usecase/ports.go` (sửa), `.../internal/adapter/postgres/{context_sources,context_packs,evidence}.go` (mới), `.../internal/adapter/mysql/{context_sources,context_packs,evidence}.go` (mới), `.../internal/adapter/{postgres,mysql}/context_repositories_integration_test.go` (mới)
**Depends on:** BE-REQ-SOL-001 (module, `InTx`, `exec` lấy từ ctx), BE-REQ-SOL-002 (bảng `requests`, `RequestRepository`)
**Status:** `[x] DONE`

---

## Context

- `request-service` chưa tồn tại trên đĩa (đã `ls backend-go/services/request-service`: không có). Mọi đường dẫn trên là "(mới)"; layout lấy từ SOL-001 mục A.
- **Số migration `NNNN`:** CR 001, 002, 004, 006, 007, 009, 010, 034, 035 cùng thêm migration. Bắt buộc chạy `ls backend-go/services/request-service/migrations/postgres backend-go/services/request-service/migrations/mysql` lúc bắt đầu, lấy số kế tiếp **chung cho cả hai dialect** (hai thư mục phải cùng số). Không đặt số theo trí nhớ.
- Mẫu RLS Postgres thật: `mcp-service/migrations/postgres/0001_init.up.sql` (`ENABLE` + `FORCE`, `NULLIF(current_setting('app.tenant_id', true), '')::uuid`, khối `DO $$ ... FOREACH`) và `mcp-service/internal/adapter/postgres/tenant_tx.go` (`withTenantTx`). `task-service` **không** đặt `app.tenant_id` (RLS im lặng), không sao theo nó. Trong `request-service` giao dịch do `InTx` của SOL-001 mục 2.D đặt `set_config`.
- Khác CR-REQ-031 mục 2.1: cột `key` đặt tên `source_key` (từ khoá MySQL); `seq` của `evidence` cấp bằng khoá hàng `requests` (`FOR UPDATE`), **không** CAS `version` (solution mục 1, C1).
- CHECK có hiệu lực ở MySQL 8.0.16 trở lên; chỉ mục `created_at DESC` cần 8.0.

## Việc cần làm

1. Đọc thư mục migrations, chốt `NNNN`. Viết `NNNN_context_sources.up.sql` Postgres đúng SQL ở solution mục B: ba bảng `request.context_sources`, `request.context_packs`, `request.evidence`; ràng buộc `context_sources_transport_shape` (`internal` có `adapter` và không `server_ref`; `mcp` ngược lại); chỉ mục `context_packs_latest (tenant_id, request_id, stage, created_at DESC)`, `context_packs_input (tenant_id, request_id, stage, input_digest)`; `UNIQUE (tenant_id, request_id, seq)` ở `evidence`. Bật `ENABLE` và `FORCE ROW LEVEL SECURITY` cho cả ba, policy `tenant_isolation` có `USING` và `WITH CHECK`.
2. `NNNN_context_sources.down.sql`: `DROP TABLE request.evidence; DROP TABLE request.context_packs; DROP TABLE request.context_sources;` (đúng thứ tự FK).
3. MySQL: cùng tên bảng/cột (không có schema `request`, dùng database của service), `CHAR(36)`, `JSON`, `kind VARCHAR(24)`, `trust VARCHAR(8)`, `body MEDIUMTEXT`, `excerpt TEXT`, `TIMESTAMP(6)`. Không có RLS: không `ENABLE ROW LEVEL SECURITY`; `tenant_id` luôn nằm ở `WHERE`.
4. `ports.go` thêm:

```go
type ContextSourceRepository interface {
    List(ctx context.Context) ([]domain.ContextSource, error)
    Get(ctx context.Context, key string) (domain.ContextSource, error)          // ErrNotFound
    Upsert(ctx context.Context, s domain.ContextSource, expectedVersion int64) (domain.ContextSource, error) // CAS; version 0 = tạo mới
    SetStatus(ctx context.Context, key, status string, expectedVersion int64) (domain.ContextSource, error)
}
type ContextPackRepository interface {
    Insert(ctx context.Context, p domain.ContextPack) error                      // trong InTx cùng evidence
    FindByInputDigest(ctx context.Context, requestID string, stage domain.Stage, digest string, notBefore time.Time) (domain.ContextPack, bool, error)
    Latest(ctx context.Context, requestID string, stage domain.Stage) (domain.ContextPack, bool, error)
}
type EvidenceRepository interface {
    NextSeq(ctx context.Context, requestID string) (int, error)                  // khoá hàng requests FOR UPDATE rồi MAX(seq)+1
    InsertBatch(ctx context.Context, items []domain.Evidence) error
    Get(ctx context.Context, id string) (domain.Evidence, error)
    GetBySeq(ctx context.Context, requestID string, seq int) (domain.Evidence, error)
    MarkUsedBy(ctx context.Context, id string, use domain.EvidenceUse) error    // append vào used_by, idempotent theo (kind,id)
}
```

5. Adapter Postgres và MySQL: mọi hàm lấy `tenantID` bằng `tenant.RequireTenantID(ctx)`, mọi câu SQL có `tenant_id = $1`. `Upsert` dùng `INSERT ... ON CONFLICT (tenant_id, source_key) DO UPDATE ... WHERE context_sources.version = $n` (Postgres) và `INSERT ... ON DUPLICATE KEY UPDATE` kèm `WHERE version` đổi thành hai bước `UPDATE ... WHERE version=?` rồi kiểm `RowsAffected` (MySQL); 0 dòng bị ảnh hưởng thì `ErrVersionConflict`.
6. `NextSeq`: `SELECT 1 FROM requests WHERE tenant_id=? AND id=? FOR UPDATE` (không dòng: `ErrNotFound`), rồi `SELECT COALESCE(MAX(seq),0)+1 FROM evidence WHERE tenant_id=? AND request_id=?`. Hàm phải chạy trong `InTx` (không có giao dịch trong ctx thì trả lỗi lập trình `ErrTxRequired`, không tự mở giao dịch vì khoá phải sống đến hết lần chèn).
7. JSON: `scopes`, `redaction`, `enabled_for`, `items`, `missing`, `used_by` encode bằng `encoding/json`; `scopes` rỗng giữ `[]` không `null` (nhớ khởi tạo slice rỗng).
8. Không FK chéo service: `context_sources.server_ref` chỉ là UUID, không FK; FK nội bộ duy nhất là `evidence.context_pack_id` và `context_packs.request_id`.

## Kiểm thử

- `context_repositories_integration_test.go` (build tag `integration`, chạy cả hai dialect theo mẫu `repository_contract` của TASK-REQ-002-06):
  - `TestContextSource_UpsertVersionConflict`: hai `Upsert` cùng `expectedVersion`, một thành công, một `ErrVersionConflict`.
  - `TestContextSource_CheckRejectsUnknownKindTransportTrust`: INSERT thô với `kind='x'`, `transport='ssh'`, `trust='top'` đều lỗi.
  - `TestContextSource_TransportShape`: `internal` không `adapter`, `mcp` không `server_ref` bị từ chối.
  - `TestEvidence_NextSeqConcurrent`: hai goroutine, hai `InTx`, cùng Request, mỗi bên cấp rồi chèn một dòng; kết quả `seq` là `{1,2}`, không `UNIQUE` vi phạm.
  - `TestEvidence_TenantIsolation`: tenant B không `Get`, `GetBySeq`, `FindByInputDigest` được dữ liệu tenant A; riêng Postgres, truy vấn không đặt `app.tenant_id` trả 0 dòng.
  - `TestContextPack_FindByInputDigest_RespectsNotBefore`.
  - `TestMigration_UpDownUp`: up, down, up sạch ở cả hai dialect.
- Lệnh: `cd backend-go && go test ./services/request-service/internal/adapter/... && go test -tags=integration ./services/request-service/internal/adapter/...` (cần Docker; chưa chạy).

## Tiêu chí hoàn thành

- [x] Hai thư mục migration có cùng số `NNNN`, up/down/up sạch trên Postgres 14+ và MySQL 8.0.16+.
- [x] RLS Postgres bật `FORCE`; test "quên `set_config`" trả 0 dòng.
- [x] Không câu SQL MySQL nào thiếu `tenant_id` (sẽ được test quét SQL của BE-REQ-SOL-035 task 05 kiểm).
- [x] `NextSeq` không trùng dưới đua; `used_by` cập nhật idempotent.
- [x] Không file tên `helpers`, `utils`, `common`; không `max-lines` disable.

## Rủi ro và lưu ý

- Đổi tên `key` thành `source_key` lệch với CR; ghi vào PR để người duyệt cập nhật CR.
- `FOR UPDATE` trên `requests` có thể chờ giao dịch chuyển trạng thái đang giữ hàng; timeout giao dịch mặc định của service phải ngắn (xem SOL-001), tránh chờ vô hạn.
- MySQL không đảm bảo `CHECK` trước 8.0.16 (bỏ qua im lặng); CI phải ghim phiên bản.
- Các bảng mới phải được thêm vào danh sách bảng của test quét SQL và test cách ly của BE-REQ-SOL-035.
