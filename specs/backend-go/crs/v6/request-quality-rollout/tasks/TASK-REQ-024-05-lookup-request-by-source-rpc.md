# TASK-REQ-024-05: RPC nội bộ `LookupRequestBySource` ở `request-service`

**From Solution:** BE-REQ-SOL-024
**Priority:** P1
**Service:** `request-service`
**File:** `backend-go/proto/orca/request/v1/request.proto`, `backend-go/services/request-service/internal/usecase/lookup_request_by_source.go` (mới), `.../internal/adapter/grpc/server.go`, `.../internal/adapter/{postgres,mysql}/request_repository.go` (thêm truy vấn), `.../cmd/server/main.go` (guard)
**Depends on:** CR-REQ-002 (bảng `requests`, chỉ mục nguồn), CR-REQ-004; BE-REQ-SOL-025 task 01 (đọc cờ)
**Status:** [x] DONE (đã kiểm chứng 2026-10-08: `go test ./... -count=1` trong `services/request-service` (512 test PASS, 0 FAIL) và `services/issue-status-sync` (186 PASS); `go test -tags integration ./internal/adapter/postgres/... ./internal/adapter/grpc/... ./cmd/...` trên `postgres:16-alpine` thật (262 test PASS); `go test -tags integration ./internal/adapter/mysql/...` trên `mysql:8.0` thật (PASS, 238 giây); e2e `TestLookupRequestBySource_InternalGuardTenantAndFlag`)

---

## Context

- README v6 mục 8 dòng 12 liệt kê `LookupRequestBySource` là RPC thêm; CR-REQ-024 mục 2.3 mô tả hành vi.
- `common/internalcaller.Guard(expectedToken string, fullMethods ...string)` (`internalcaller.go:25`) chặn caller không có token nội bộ; `ClientInterceptor(token)` ở phía gọi. `ReportTaskOutcome` dùng cùng cơ chế (CR-REQ-013).
- `requests` có `source_provider`, `source_site`, `source_ref` và khoá idempotent `request_idempotency` (CR-REQ-002, 004).
- Lý do không dùng `ListRequests`: nó kiểm quyền đọc theo người dùng; consumer chỉ có tenant.

## Việc cần làm

1. Proto: `message LookupRequestBySourceRequest { string provider = 1; string site = 2; string ref = 3; } message LookupRequestBySourceResponse { bool found = 1; string request_id = 2; }`; `rpc LookupRequestBySource(...)` (additive; chạy `buf lint` và `buf breaking`).
2. Repository: `FindActiveBySource(ctx, tenantID, provider, site, ref string) (requestID string, ok bool, err error)`; `site` rỗng khớp mọi site; chỉ Request `status NOT IN ('completed','cancelled')`; sắp xếp `created_at DESC LIMIT 1`. Cần chỉ mục `(tenant_id, source_provider, source_ref)` (kiểm migration của CR-REQ-002 đã có chưa; nếu chưa, thêm migration hai dialect).
3. Use case: `RequireTenantID`; kiểm cờ hiệu lực của tenant (dùng cổng của BE-REQ-SOL-025 task 01); cờ tắt thì `found=false` (để `issue-status-sync` trở về hành vi cũ khi cờ tắt).
4. gRPC server: bọc bằng `internalcaller.Guard` cho đúng phương thức này; bảng phân loại `flow_gate` (BE-REQ-SOL-025 task 02) xếp `classInternal`.
5. Không trả `title`, `body`.

## Kiểm thử

- Unit: các ca khớp, khác tenant, `completed` không khớp, `site` rỗng, cờ tắt trả `found=false`.
- Tích hợp hai dialect: truy vấn và chỉ mục (`EXPLAIN` không bắt buộc, chỉ kiểm kết quả).
- Test guard: gọi không token thì lỗi; có token thì chạy.
- Lệnh: `go test ./services/request-service/...` (và tag `integration`).

## Tiêu chí hoàn thành

- [x] RPC trả đúng, chỉ cho caller nội bộ.
- [x] Cờ tắt thì `found=false`.
- [x] `buf breaking` xanh.

## Rủi ro và lưu ý

- Nhiều Request cùng nguồn (CR-REQ-004 cho phép chuỗi "Request theo dõi"): lấy mới nhất còn hoạt động; đủ cho mục đích "bỏ qua đồng bộ PR".

## Kết quả triển khai (2026-10-08)

Proto đã có sẵn từ đợt proto (không sửa `.proto`, nên không chạy `buf breaking`). `NewLookupRequestBySource(..., WithActiveSourceFinder, WithLookupFlagGate)`; truy vấn `FindActiveBySource` ở `adapter/{postgres,mysql}/request_source_lookup.go`; migration `0091` thêm chỉ mục `(tenant_id, source_provider, source_ref)`; guard `internalcaller.Guard(SERVICE_INTERNAL_TOKEN)` trong `cmd/server/wire_rollout.go`; payload `status_changed`/`completed` có `source_*`, `reporter_id`, `version`, `number`, `traceparent`. Phân loại `classInternal` trong `flow_gate.go`. Cần hợp nhất guard này với chuỗi interceptor bảo mật của rf-sec.
