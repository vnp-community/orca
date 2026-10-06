# TASK-REQ-002-07: Nối `GetRequest` và `ListRequests` vào repository, ánh xạ proto

**From Solution:** BE-REQ-SOL-002
**Priority:** P1
**Service:** `request-service`
**File:** `internal/usecase/{get_request.go,list_requests.go}`, `internal/adapter/grpc/{server.go,request_mapper.go}`, `internal/adapter/grpc/server_test.go`, `cmd/server/main.go`, `README.md` (sửa/mới)
**Depends on:** TASK-REQ-002-04 hoặc 002-05 (một trong hai để chạy; cả hai để hoàn thành), TASK-REQ-001-05
**Status:** [ ] TODO

---

## Context

TASK-REQ-001-05 để `GetRequest`, `ListRequests` trả `Unimplemented`. CR-REQ-001 mục 5 mong test: gọi `GetRequest` id không tồn tại thì `NotFound` (phụ thuộc kho của CR-REQ-002). Tenant và user được `grpcmw.TenantExtractionInterceptor` đưa vào ctx từ metadata. Quyền đọc theo project do gateway kiểm (README v6 mục 6, mục 8 điểm 13 yêu cầu RPC tự kiểm quyền nhưng quyền ghi/đọc ở mức Request chưa chốt: CR-REQ-010).

## Việc cần làm

1. `usecase/get_request.go`: `type GetRequest struct{ repo RequestRepository }`, `Execute(ctx, id string) (domain.Request, error)`: `tenant.RequireTenantID`, id không phải UUID thì `ErrRequestNotFound` (không `InvalidArgument`, tránh phân biệt), gọi `repo.Get`.
2. `usecase/list_requests.go`: `Execute(ctx, ListFilter) (ListResult, error)`: `Normalize`, gọi `repo.List`.
3. `adapter/grpc/request_mapper.go`: `toProtoRequest(domain.Request) *requestv1.Request` và `filterFromProto(*requestv1.ListRequestsRequest) usecase.ListFilter` (trường nguồn 6 đến 8 chưa có trong proto; để `ListFilter` trống ở đó, CR-REQ-004 nối thêm).
4. `adapter/grpc/server.go`: `NewServer(getRequest *usecase.GetRequest, listRequests *usecase.ListRequests)`; `GetRequest` và `ListRequests` gọi use case, lỗi qua `apperrors.ToGRPCStatus`.
5. `main.go`: tạo use case với `repo` đã chọn theo dialect; type assert repo thành các cổng (struct `Repository` của mỗi adapter cài đủ).
6. README: cập nhật bảng real vs stub: `GetRequest`, `ListRequests` là thật.

## Kiểm thử

- `TestGetRequest_NotFound` và `TestGetRequest_OtherTenantNotFound` (gRPC in-process với repo thật qua testcontainers; chạy từng dialect): trả `codes.NotFound` với mã `REQUEST_NOT_FOUND`.
- `TestListRequests_PaginatesAndFilters`: chèn 120 Request, `page_size=50` hai trang đầy và một trang 20, không trùng.
- `TestListRequests_PageSizeCapped`: `page_size=500` bị cắt còn 200.
- `TestMapper_RoundTripFields`: mọi cột ánh xạ đúng, `Type` rỗng thành chuỗi rỗng, `confidence` nil thành trường `optional` không đặt.
- Lệnh: `go test ./services/request-service/internal/usecase/... ./services/request-service/internal/adapter/grpc/...` và bản `-tags=integration`.

## Tiêu chí hoàn thành

- [ ] `GetRequest` id không tồn tại hoặc thuộc tenant khác trả `NotFound`, không phân biệt.
- [ ] `ListRequests` phân trang keyset và lọc `project_id`, `status`, `type`.
- [ ] README ghi đúng RPC thật và RPC còn `Unimplemented`.
- [ ] Không còn `Unimplemented` ở hai RPC này.

## Rủi ro và lưu ý

- `confidence` là `optional double` (TASK-REQ-001-02) để UI phân biệt "chưa có đề xuất" với 0; kiểm mapper dùng `HasConfidence`/con trỏ đúng.
- Chưa có kiểm quyền theo project: không mở `request-service` cho đường vào khác ngoài `api-gateway` (CR-REQ-016).
