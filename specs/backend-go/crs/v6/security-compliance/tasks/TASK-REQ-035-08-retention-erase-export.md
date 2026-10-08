# TASK-REQ-035-08: Lưu giữ (`RunRetention`), xoá theo yêu cầu (`EraseRequest`) và xuất (`ExportRequest`, `ExportTenantRequests`)

**From Solution:** BE-REQ-SOL-035 (mục F lưu giữ, C6, C7)
**Priority:** P1
**Service:** `request-service`, `proto`
**File:** `backend-go/services/request-service/internal/domain/{erasable_columns.go,retention_policy.go}` (mới), `.../internal/usecase/{run_request_retention.go,erase_request.go,export_request.go}` (mới), `.../internal/adapter/{postgres,mysql}/retention.go` (mới), `.../internal/adapter/grpcclient/task_content_eraser.go` (mới, bản `NotSupported`), `.../internal/adapter/grpc/server_compliance.go` (mới), `.../cmd/server/main.go` (sửa: job hằng ngày), `.../internal/config/config.go` (sửa: `REQUEST_ERASE_HMAC_KEY`), `backend-go/proto/orca/request/v1/compliance.proto` (mới), và `_test.go` tương ứng
**Depends on:** TASK-REQ-035-01 (`Redact` cho xuất), 035-04 (nhóm `admin`, stream), 035-06 (bảng, `RecordDurable`, cột `erased_at`), BE-REQ-SOL-002 (`requests`), BE-REQ-SOL-007 (`solutions`, `analysis_runs`), 009 (`approvals`), CR 031 (`context_packs`, `evidence`), CR 034 (`ai_trace_blobs`, `ai_usage_ledger`)
**Status:** [x] DONE (đã kiểm chứng 2026-10-08: `cd backend-go && opa test policy/orca-authz && go test ./services/request-service/... && go test -tags integration ./services/request-service/...`)

---

## Context

- Không có chính sách lưu giữ, xoá hay xuất ở service nào của repo (chỉ audit append-only ở `auth.audit_log`); `enterprise-readiness-checklist.md` mục 4 đánh dấu thiếu. Thời hạn mặc định (730/400/30 ngày) là đề xuất chưa có yêu cầu pháp lý (Q3).
- Cột cấu hình đã có từ task 06: `tenant_settings.request_retention_days` (730; 0 = giữ mãi), `ai_trace_retention_days` (30), `ledger_retention_days` (400).
- Ẩn danh hoá thay vì xoá hàng (CR 2.8, quyết định 10): giữ id, trạng thái, loại, mốc thời gian, để thống kê và truy vết còn nguyên. `reporter_id` đổi sang UUID dẫn xuất HMAC (solution C6) vì dùng để cấp quyền reporter.
- Bảng chứa văn bản tự do (solution C7): `requests` (`title`, `body`, `classification_reason`, `return_reason`), `request_type_history.reason`, `solutions.options` (JSON) và `content_ref`, `analysis_runs.raw_output`, `approvals.comment`, `context_packs.body`, `evidence.excerpt` + `title`, `ai_trace_blobs` (xoá hàng), `request_checks` (kiểm khi đọc proto CR-014: cột kết quả lệnh nếu có). `ai_usage_ledger` chỉ có digest ⇒ không chứa nội dung, giữ theo `ledger_retention_days`.
- `task-service` chưa có RPC xoá nội dung task theo `request_id` (CR 011: thêm cột `request_id`, nhưng không có RPC xoá `Description`/`AIContext`): `TaskContentEraser` bản `NotSupported` trả lỗi miền, kết quả `EraseRequest` liệt kê `external_erasure`.
- `FOR UPDATE SKIP LOCKED` cần MySQL ≥ 8.0.1 (CR-DB-002); mẫu hai dialect: `task-service/internal/adapter/{postgres,mysql}/execution_leases.go`.

## Việc cần làm

1. `erasable_columns.go`: `type ErasableColumn struct{ Table, Column string; Mode EraseMode }` với `EraseMode` ∈ `ClearText` (`''`), `ClearJSON` (`'{}'`), `Null`, `DeleteRows`; biến `ErasableColumns []ErasableColumn` liệt kê đúng danh sách ở Context; `var ExemptTextColumns = map[string]string{"outbox_events.payload": "không chứa nội dung (đã kiểm ở payload golden)", ...}` kèm lý do.
2. `retention_policy.go`: `RetentionPlan{TenantID string; RequestCutoff, TraceCutoff, LedgerCutoff time.Time}` từ `tenant_settings` và `now` (0 ngày ⇒ không cutoff cho nhóm đó); hàm thuần `PlanFor(settings, now)` có test.
3. Repository (`retention.go`, hai dialect): 
   - `ClaimExpiredRequests(ctx, cutoff time.Time, limit int) ([]string, error)` chọn `id` của Request `status IN ('completed','cancelled')`, `updated_at < cutoff`, `erased_at IS NULL`, `ORDER BY updated_at LIMIT ? FOR UPDATE SKIP LOCKED` trong giao dịch giữ khoá tới hết `AnonymizeRequests`;
   - `AnonymizeRequests(ctx, ids []string, reporterKey []byte, at time.Time) (int, error)`: trong **một** giao dịch cho lô, `UPDATE` từng bảng theo `ErasableColumns` (`WHERE tenant_id=? AND request_id IN (...)`), đổi `requests.title='[erased]'`, `body=''`, `classification_reason=''`, `return_reason=''`, `reporter_id=derive(tenant, reporter_id)`, `erased_at=at`; Request đã `erased_at` được bỏ qua (idempotent); không đổi `version` ngoài +1 do CAS thông thường (dùng `UPDATE ... SET version=version+1`);
   - `PurgeTraceBlobs(ctx, cutoff, limit)`, `PurgeLedger(ctx, cutoff, limit)` (xoá hàng; lô 200).
   Mọi truy vấn có `tenant_id`; job lặp theo tenant (liệt kê tenant có hàng quá hạn qua truy vấn đa tenant nằm trong danh sách cho phép của test quét SQL, task 05).
4. `derive(tenant, reporterID, key)` = 16 byte đầu của `HMAC-SHA256(key, tenant + "|" + reporterID)` định dạng UUID (đặt bit phiên bản 4 để hợp lệ). Khoá `REQUEST_ERASE_HMAC_KEY` (nguồn Vault/secrets như các khoá khác của repo; kiểm `common/secrets`): rỗng ⇒ `RunRetention` và `EraseRequest` từ chối với `FailedPrecondition` `REQUEST_ERASE_KEY_MISSING` (fail closed).
5. `run_request_retention.go`: `RunRetention.Execute(ctx)` mỗi lần: với từng tenant, `PlanFor`, lặp `Claim` → `Anonymize` đến khi không còn hoặc đạt 5 lô; `PurgeTraceBlobs`, `PurgeLedger`; ghi `RecordDurable(request.retention.run, system, {processed, anonymized, blobs_purged, ledger_purged})` (số lượng, không nội dung); chạy hằng ngày bằng ticker trong `main.go` (giờ cấu hình `REQUEST_RETENTION_RUN_AT`, mặc định 03:00 UTC) và khi khởi động nếu bỏ lỡ; `SKIP LOCKED` làm hai bản sao không xử lý trùng.
6. `erase_request.go`: `EraseRequest.Execute(ctx, requestID, reason)`: `admin` (đã kiểm ở interceptor; vẫn kiểm `tenant.Role` lần nữa), Request không ở `executing` (`FailedPrecondition` `REQUEST_ERASE_NOT_ALLOWED`), `reason` bắt buộc, độ dài ≤ 500; `Anonymize([id])` ngay; gọi `TaskContentEraser.Erase(ctx, requestID)`: bản `NotSupported` trả `ErrErasureUnsupported` ⇒ ghi vào phản hồi `external_erasure=[{system:"task-service", status:"unsupported"}]`, **không** làm hỏng thao tác; `RecordDurable(request.erase, ...)` cùng giao dịch với ẩn danh hoá (task 06). Phản hồi: `{erased_at, external_erasure[], not_covered_note}` với `not_covered_note` liệt kê bản sao ngoài hệ thống (Jira, git, nhà cung cấp LLM, sao lưu DB).
7. `export_request.go`: `ExportRequest.Execute(ctx, requestID)` dựng gói JSON `{request, type_history, solutions, approvals, links, ledger_summary, provenance}` (Request đã `erased_at` ⇒ gói chỉ có khung), **toàn bộ chuỗi** đi qua `secretscan.Redact`, cắt `ExportMaxBytes = 5 << 20`: vượt ⇒ `ResourceExhausted` `REQUEST_EXPORT_TOO_LARGE` (không cắt im lặng); `ExportTenantRequests` (server-streaming, mỗi `ExportChunk` một dòng NDJSON ≤ 64 KB; phân trang keyset `(created_at, id)` lô 100; kiểm `ctx.Done()`; tối đa 1 luồng đồng thời mỗi tenant bằng semaphore trong bộ nhớ). Cả hai `RecordDurable(request.export, {request_id|count, bytes})`.
8. `compliance.proto` (`package orca.request.v1`): `rpc EraseRequest`, `rpc ExportRequest`, `rpc ExportTenantRequests(...) returns (stream ExportChunk)`; message gồm `request_id`, `reason`, `external_erasure[]`, `bytes`; thêm vào `Catalog` nhóm `admin` (task 04) và lớp rate `write`.
9. Kiểm tra `TestEveryTextColumnDeclaredErasableOrExempt` (xem Kiểm thử) để cột mới của CR khác không lọt khỏi việc xoá.

## Kiểm thử

- `retention_policy_test.go`: `PlanFor` với `0` (giữ mãi) không cutoff; 730 ngày đúng mốc; múi giờ UTC.
- `run_request_retention_test.go` (repo giả): chỉ Request `completed|cancelled` quá hạn bị ẩn danh; Request `executing` hoặc chưa quá hạn không chạm; chạy lại không đổi (idempotent); khoá HMAC rỗng ⇒ từ chối; hai bản sao (hai goroutine, repo giả có khoá) không xử lý trùng.
- `retention_integration_test.go` (hai dialect): `TestAnonymizeClearsAllErasableColumns` (tạo dữ liệu mọi bảng liệt kê rồi kiểm toàn bộ cột về giá trị rỗng, `reporter_id` đổi, `erased_at` đặt, `id/status/type/created_at` giữ nguyên); `TestSkipLockedNoDoubleProcess`; `TestEveryTextColumnDeclaredErasableOrExempt` (quét `information_schema.columns` kiểu `text|character varying|json|jsonb|longtext|mediumtext` của mọi bảng `request`, đòi có trong `ErasableColumns` hoặc `ExemptTextColumns`); cách ly tenant.
- `erase_request_test.go`: `executing` bị từ chối; `reason` rỗng bị từ chối; `TaskContentEraser` không hỗ trợ ⇒ thao tác thành công kèm `external_erasure`; audit `request.erase` có trong `request_audit_outbox` cùng giao dịch (rollback ⇒ không có).
- `export_request_test.go`: gói không chứa chuỗi bí mật mẫu (`ghp_...`) sau `Redact`; vượt 5 MB ⇒ `REQUEST_EXPORT_TOO_LARGE`; Request đã xoá ⇒ không còn nội dung; `ExportTenantRequests` phân trang ổn định qua hai lần gọi, huỷ ngữ cảnh dừng sớm.
- `server_compliance_test.go`: người không phải admin ⇒ `PermissionDenied` cho cả ba RPC; `agent` ⇒ `PermissionDenied` kể cả với admin phía sau.
- Lệnh: `cd backend-go && go test ./services/request-service/internal/domain/... ./services/request-service/internal/usecase/... ./services/request-service/internal/adapter/grpc/... && go test -tags=integration ./services/request-service/internal/adapter/...`; `buf lint && buf breaking`. Chưa chạy.

## Tiêu chí hoàn thành

- [x] `RunRetention` ẩn danh hoá đúng Request quá hạn, không chạm Request chưa kết thúc; chạy hai bản sao không xử lý trùng.
- [x] `EraseRequest` ẩn danh hoá ngay (không khi `executing`), có audit không-được-mất, báo bản sao ngoài hệ thống.
- [x] `ExportRequest` không chứa chuỗi bí mật mẫu; vượt 5 MB trả lỗi rõ.
- [x] Mọi cột văn bản tự do của schema `request` nằm trong `ErasableColumns` hoặc danh sách miễn trừ (test quét).
- [x] Khoá HMAC thiếu ⇒ từ chối (fail closed).

## Ví dụ tham khảo

Gói `ExportRequest` (cấu trúc, mọi chuỗi đã qua `Redact`):

```json
{"request": {"id": "...", "number": 42, "type": "bug", "status": "completed", "title": "...", "body": "..."},
 "type_history": [], "solutions": [], "approvals": [], "links": [],
 "ledger_summary": {"calls": 7, "input_tokens": 41000, "output_tokens": 6200, "cost_usd_est": "0.31"},
 "provenance": [{"step": "solution", "prompt_id": "solution", "prompt_version": "1.0.0", "model": "..."}],
 "exported_at": "2026-10-06T10:00:00Z", "patterns_version": "ss/1"}
```

## Rủi ro và lưu ý

- Ẩn danh hoá không xoá bản sao ở Jira, git, nhà cung cấp LLM, sao lưu DB: không hứa "xoá hoàn toàn" với khách (CR mục 6).
- `task-service` chưa có RPC xoá nội dung: `Description`/`AIContext` của task có `request_id` vẫn còn; cần CR riêng (Q4).
- Mặc định 730/400/30 ngày chưa có yêu cầu pháp lý; đổi bằng `tenant_settings`, không cần migration.
- `reporter_id` đổi sang UUID dẫn xuất làm người báo cáo mất quyền `reporter` trên Request đã xoá: chủ ý.
- Lô `Anonymize` dài giữ khoá lâu: giới hạn lô 200 và giao dịch ngắn; đo trên dữ liệu thật chưa có.

## Ghi chú triển khai (2026-10-08)

- `EraseRequest`, `ExportRequest`, `ExportTenantRequests` là handler gRPC thật (`server_compliance.go`, nối trong `main.go`), quyền admin kiểm hai lần (interceptor và use case, agent bị từ chối).
- `ErasableColumns` 18 cột/9 bảng + `ExemptTextColumns` (kèm lý do) + test quét `information_schema` hai dialect. Xoá cả `title` trong payload outbox: CHƯA (payload `request.created` có `title`; ghi trong IMPLEMENTATION-NOTES).
- Chưa làm: xoá `ai_trace_blobs`/`ai_usage_ledger` (bảng của CR-034 chưa có trong cây này; cột phải đăng ký vào `ErasableColumns`, test sẽ đỏ cho tới khi đăng ký); `TaskContentEraser` bản `Unsupported` (task-service chưa có RPC), kết quả báo `external_erasure: unsupported`.
- Xuất tenant: stream NDJSON, phân phần thân dài để mỗi dòng không quá 64 KB, một luồng mỗi tenant, hủy ngữ cảnh dừng sớm và vẫn ghi audit `complete=false`. `ledger_summary` và `provenance` chưa có trong gói (bảng chưa tồn tại).
- Job lưu giữ: hằng ngày theo `REQUEST_RETENTION_RUN_AT` (UTC) và 30 giây sau khi khởi động; cần `REQUEST_ERASE_HMAC_KEY`, thiếu thì từ chối.
