# TASK-REQ-002-06: Bộ test hợp đồng dùng chung hai dialect và test schema

**From Solution:** BE-REQ-SOL-002
**Priority:** P0
**Service:** `request-service`
**File:** `internal/adapter/contracttest/request_repository_contract.go` (mới, package thường, không phải `_test.go`, để hai adapter import), `internal/adapter/postgres/request_repository_contract_test.go`, `internal/adapter/mysql/request_repository_contract_test.go`, `internal/adapter/postgres/schema_contract_test.go`, `internal/adapter/mysql/schema_contract_test.go` (mới)
**Depends on:** TASK-REQ-002-04, TASK-REQ-002-05
**Status:** [ ] TODO

---

## Context

CR-REQ-002 mục 5 yêu cầu một bộ kịch bản chạy cho cả hai dialect qua hàm nhận interface repository, đặt tên `request_repository_contract_test.go`. File `_test.go` không import được từ package khác, nên bộ kịch bản đặt ở package `contracttest` (không phải `helpers`/`testutil`) và mỗi adapter có file `_test.go` mỏng gọi nó. Mẫu testcontainers: `mcp-service/internal/adapter/postgres/*_integration_test.go` (tạo role `NOSUPERUSER NOBYPASSRLS`, build tag `integration`); `common/testutil` có sẵn tiện ích container (đọc trước khi tự viết).

## Việc cần làm

1. `contracttest.RunRequestRepositoryContract(t *testing.T, newEnv func(t *testing.T) Env)` với `type Env struct { Tx usecase.TxRunner; Requests usecase.RequestRepository; History ...; Solutions ...; Links ...; Idempotency ...; CtxForTenant func(tenantID string) context.Context }`. Mỗi kịch bản chạy `t.Run`.
2. Kịch bản (tên con): `CreateGetRoundTrip`; `NumberingConcurrent20` (20 `InTx` song song: `NextNumber` + `Create`, kiểm 20 số liên tiếp từ 1); `NumberingRollbackNoGap`; `ClaimConcurrent` (12 goroutine cùng khoá, đúng một `claimed`); `ClaimLoserSeesWinnerID`; `UpdateCASConflict`; `UpdateNotFound`; `TenantIsolationRead`, `TenantIsolationUpdate`; `ListKeysetStableUnderInsert` (trang 1, chèn hàng mới ở đầu, trang 2 không trùng không sót); `ListFilters` (status, type, project, nguồn); `LinkSelfRejected`; `TypeHistoryOrderedByAt`; `SolutionsCAS`.
3. `schema_contract_test.go` mỗi dialect: truy vấn `information_schema.columns` cho các bảng `requests`, `request_counters`, `request_type_history`, `solutions`, `request_links`, `request_idempotency`; so với bảng mong đợi (tên cột, `is_nullable`) lưu trong test; cùng một bảng mong đợi cho hai dialect (đặt trong `contracttest`, hàm `ExpectedColumns()`).
4. Test hằng/CHECK: mỗi `CHECK ... IN (...)` trong DB phải bằng hằng Go: chèn từng giá trị `AllRequestTypes()`, `AllRequestStatuses()` được; giá trị ngoài tập bị từ chối (đã có ở TASK-REQ-002-01, ở đây khẳng định chiều ngược: mọi hằng Go đều được DB chấp nhận).
5. Postgres: kịch bản `RLSDirectSQL` ở file riêng (role không superuser).
6. Ghi vào PR cách chạy: CI matrix `dialect` đã có (TASK-REQ-001-06).

## Kiểm thử

- `go test -tags=integration ./services/request-service/internal/adapter/postgres/... -run Contract -v`
- `go test -tags=integration ./services/request-service/internal/adapter/mysql/... -run Contract -v`
- Đối chứng: cố ý bỏ `AND tenant_id = ?` trong một truy vấn MySQL thử nghiệm (không commit) và xác nhận `TenantIsolation*` đỏ.

## Tiêu chí hoàn thành

- [ ] Cùng một bộ kịch bản xanh với cả hai dialect.
- [ ] Bảng cột mong đợi khớp `information_schema` cả hai DB.
- [ ] Mọi hằng Go được DB chấp nhận.
- [ ] Test cách ly tenant đỏ khi bỏ điều kiện `tenant_id` (đã thử đối chứng).

## Rủi ro và lưu ý

- Test tích hợp cần Docker; không chạy được trong `go test ./...` thường, đúng ý (build tag).
- Tránh tên `testutil`/`helpers` cho package mới; `contracttest` đặt theo khái niệm.
- Chạy 20 giao dịch song song trên container nhỏ có thể chập chờn (deadlock MySQL ở `request_counters`); cho phép thử lại tối đa 3 lần với lỗi 1213 và ghi vào test.
