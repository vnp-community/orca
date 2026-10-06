# BE-CV-TASK-011-08: Repository Postgres: `graph_snapshots` (tỉa, hạn mức) và `reindex_jobs` (`active_key`, đếm hạn mức)

**From Solution:** BE-CV-SOL-011-repositories-and-maintenance
**Priority:** P0
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/adapter/postgres/{graph_snapshot,reindex_job}_repository.go` (+ `_integration_test.go`) (mới)
**Depends on:** BE-CV-TASK-011-02, 011-04, 011-06
**Status:** [ ] TODO

---

## Context

Hợp đồng T3, T7, PQ-14, PQ-15, PQ-16. `graph_snapshots.payload` là `jsonb`: bỏ NUL trước khi ghi (domain, 011-04). Đồng hồ DB cho `expires_at`.

## Việc cần làm

1. `graph_snapshot.Put(ctx, s, limits)`: trong một `withTenantTx`: (a) chặn `payload_bytes > limits.MaxPayloadBytes` → `CODEINTEL_PAYLOAD_TOO_LARGE` không ghi; (b) `INSERT … ON CONFLICT (tenant_id, repo_binding_id, view, head_commit, params_hash) DO UPDATE SET schema_version, etag, total_count, payload, payload_bytes, truncated, tool_versions, created_at = now(), expires_at = now() + make_interval(secs => $ttl)`; (c) tỉa: giữ `limits.KeepCommits` (3) `head_commit` mới nhất của `(binding, view)` — dòng vừa ghi luôn được giữ; (d) nếu `SUM(payload_bytes)` của binding > `BindingQuotaBytes`, xoá cũ nhất (không xoá dòng mới nhất mỗi view).
2. `Get(ctx, key, schemaVersion)`: chỉ trả dòng `expires_at > now()` và đúng `schema_version`; không có → `CODEINTEL_NOT_FOUND` (use case coi là miss).
3. `DeleteByBinding(ctx, bindingID, exceptHeadCommit)`; `TenantBytes(ctx)` (`COALESCE(SUM(payload_bytes), 0)`).
4. `reindex_job.Create`: `INSERT` với `active_key = repo_binding_id` khi `queued`; lỗi `23505` có `ConstraintName == 'uq_reindex_active_key'` → `CODEINTEL_REINDEX_IN_PROGRESS`; lỗi khác (PK) → `CODEINTEL_ALREADY_EXISTS`; ghi `events` (`reindex.started`) cùng tx.
5. `Get`, `ListByBinding` (≤ 100, `ORDER BY created_at DESC`), `UpdateProgress` (`percent *int` → NULL khi nil; cập nhật `updated_at`, `status='running'`, `started_at` nếu null), `Finish` (kiểm `CanTransition`, `active_key = NULL`, `finished_at = now()`, ghi `events`), `CountActiveByDevServer` (`JOIN codeintel.repo_bindings b ON b.id = j.repo_binding_id WHERE j.status IN ('queued','running') AND b.dev_server_id = $1`), `CountActiveByTenant`, `LastSucceededFinishedAt` (MAX `finished_at` của job `succeeded` theo binding).
6. `message` đã được che bí mật ở use case (SOL-013); repository chỉ cắt 500 ký tự.

## Kiểm thử

- Integration Postgres: `Put` thay thế cùng khoá; tỉa giữ 3 commit; hạn mức binding; mới nhất mỗi view còn; `Get` không trả dòng hết hạn (đồng hồ DB: tạo dòng với TTL 1 s rồi chờ hoặc `UPDATE expires_at`); payload quá trần; NUL.
- Hai `Create` đồng thời cùng binding (20 vòng): đúng một thành công; sau `Finish` tạo lại được; `UpdateProgress` với `nil` giữ `percent` NULL; `CountActive*`.
- Lệnh: `go test -tags=integration ./services/code-intel-service/internal/adapter/postgres/... -run 'Snapshot|Reindex'`.

## Tiêu chí hoàn thành

- [ ] Các tiêu chí snapshot/reindex ở SOL mục 4 đạt trên Postgres.
- [ ] Phân biệt vi phạm `active_key` với PK bằng tên ràng buộc.

## Rủi ro và lưu ý

- Câu tỉa phải một câu hoặc một tx ngắn, tránh giữ khoá lâu trên bảng lớn.
- `percent` NULL phải đi tới UI là `null` (PQ-16).
