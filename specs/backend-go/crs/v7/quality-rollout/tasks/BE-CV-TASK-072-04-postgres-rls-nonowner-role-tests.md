# BE-CV-TASK-072-04: Test RLS thật trên Postgres bằng vai trò không phải chủ sở hữu

**From Solution:** BE-CV-SOL-072
**Priority:** P0
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/adapter/postgres/rls_nonowner_integration_test.go` (mới, tag `integration`), `.../postgres/tenant_scope_guard_test.go` (mới)
**Depends on:** BE-CV-SOL-011-data-model-and-migrations, BE-CV-SOL-011-repositories-and-maintenance (`withTenantTx`, `withMaintenanceTx`, `withRelayTx`)
**Status:** `[ ] TODO`

---

## Context

- Mẫu có sẵn: `mcp-service/internal/adapter/postgres/session_repository_integration_test.go:36` (`CREATE ROLE … NOSUPERUSER NOBYPASSRLS`) và `tenant_scope_guard_test.go` (AST: mỗi method exported dùng `withTenantTx`/`withRelayTx`).
- Dev compose dùng superuser `orca` ⇒ RLS vô hiệu ở dev (hợp đồng §4.1); testcontainers tự tạo vai trò.
- Bảng T1–T15; outbox có policy `app.relay='on'`, bảo trì `app.maintenance`.

## Việc cần làm

1. Harness: chạy migration bằng chủ sở hữu, kết nối kiểm thử bằng vai trò app `NOSUPERUSER NOBYPASSRLS`.
2. Với **mỗi bảng** T1–T15 (bảng dữ liệu): chèn dòng tenant A và B (UUID hợp lệ); không `set_config` ⇒ 0 dòng; A không thấy B; `WITH CHECK` chặn chèn sai tenant; truy vấn không `WHERE` không rò.
3. Outbox: chỉ `withRelayTx` đọc/cập nhật xuyên tenant; `withTenantTx` thì không.
4. Kiểm `ENABLE` + `FORCE` qua `pg_class.relrowsecurity/relforcerowsecurity` cho mọi bảng nghiệp vụ.
5. `TestEveryRepositoryMethodRunsInsideAScopedTx` (AST) cho mọi file repository.

## Kiểm thử

- `go test -tags=integration ./internal/adapter/postgres/... -run 'RLS|ScopedTx'` (cần Docker).

## Tiêu chí hoàn thành

- [ ] Mọi bảng có kiểm 0 dòng khi thiếu `set_config`.
- [ ] Bảng mới thêm mà không có RLS làm test đỏ (danh sách bảng đọc từ `information_schema`).

## Rủi ro và lưu ý

- Không chạy bằng superuser; nếu pool app của service ở dev là superuser, ghi README yêu cầu vai trò `NOSUPERUSER NOBYPASSRLS` (README v7 §8 điểm 16).
