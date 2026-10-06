# BE-CV-TASK-072-05: Cô lập tenant ở tầng ứng dụng (hai dialect), cache, stream, relay

**From Solution:** BE-CV-SOL-072
**Priority:** P0
**Service:** `code-intel-service`
**File:** `.../internal/adapter/{postgres,mysql}/tenant_isolation_integration_test.go` (mới, tag `integration`), `.../internal/adapter/mysql/tenant_where_scan_test.go` (mới), `.../internal/usecase/tenant_isolation_test.go` (mới)
**Depends on:** BE-CV-SOL-011-*, BE-CV-SOL-022 (cache), BE-CV-SOL-024 (stream), BE-CV-SOL-023, BE-CV-TASK-072-04
**Status:** `[ ] TODO`

---

## Context

- Hợp đồng §8.3 mục 4: mọi truy vấn DB có `tenant_id` và có test cô lập; MySQL không RLS ⇒ WHERE ở ứng dụng là chốt duy nhất (mẫu: `usage-service/.../repository_test.go`).
- Relay: `RelayByDevServer` đã kiểm tenant sở hữu dev server (`relay_by_dev_server.go`); service chỉ gọi với `dev_server_id` từ `repo_bindings` của tenant gọi.

## Việc cần làm

1. Test chéo tenant `Get/List/Update/Delete` cho mọi repository ở **cả hai dialect** (ma trận `dialect`): trả rỗng / `CODEINTEL_NOT_FOUND` (id con) hoặc `NOT_AUTHORIZED` (dự án).
2. `tenant_where_scan_test.go`: quét tĩnh `adapter/mysql` (và postgres) — mọi chuỗi SQL chạm bảng nghiệp vụ chứa `tenant_id`; ngoại lệ liệt kê tường minh (bảo trì quét theo `expires_at`, có lý do).
3. Cache: hai tenant cùng `(view, head_commit, params_hash)` không chung dòng; singleflight có `tenant_id` trong khoá; test đua.
4. `StreamCodeIntelEvents`: A không nhận sự kiện B.
5. Relay: A đoán `worktreeId` của B ⇒ `CODEINTEL_NOT_AUTHORIZED`, agent giả không bị gọi.
6. `tenant.RequireTenantID`: ctx không tenant ⇒ lỗi, không panic, cho mọi use case (quét bằng reflection/danh sách).

## Kiểm thử

- `go test -tags=integration ./internal/adapter/...`; `go test ./internal/usecase/... -run Tenant`.

## Tiêu chí hoàn thành

- [ ] Xanh trên Postgres và MySQL.
- [ ] Quét tĩnh bắt được truy vấn thiếu `tenant_id` (thử tay).

## Rủi ro và lưu ý

- Quét chuỗi SQL có thể báo nhầm với SQL tạo bằng ghép chuỗi; ưu tiên hằng số tên đặt rõ.
