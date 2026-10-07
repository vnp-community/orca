# TASK-REQ-004-03: Proto `CreateRequest`, `SourceHints`, bộ lọc nguồn của `ListRequests`

**From Solution:** BE-REQ-SOL-004
**Priority:** P0
**Service:** `proto`
**File:** `proto/orca/request/v1/request.proto` (sửa), `proto/gen/go/orca/request/v1/*` (sinh lại)
**Depends on:** TASK-REQ-001-02
**Status:** [x] DONE

---

## Context

TASK-REQ-001-02 khai báo `Request` (trường 1 đến 23) và `ListRequestsRequest` (1 đến 5). CR-REQ-004 mục 2.1 thêm message tạo và ba trường lọc nguồn (6 đến 8); `source_hints` vào `Request` là trường 24 (số dành sẵn). `reporter_id` không có trên dây: lấy từ metadata `x-orca-user-id` qua `tenant.UserID`.

## Việc cần làm

1. Thêm `message RequestSource { string provider = 1; string ref = 2; string url = 3; string site = 4; }`, `message SourceHints { string issue_type = 1; repeated string labels = 2; string priority = 3; string type_hint = 4; }`.
2. Thêm `CreateRequestRequest { string project_id = 1; string title = 2; string body = 3; RequestSource source = 4; SourceHints hints = 5; string client_request_id = 6; }` và `CreateRequestResponse { Request request = 1; bool created = 2; }`; thêm `rpc CreateRequest(CreateRequestRequest) returns (CreateRequestResponse);`.
3. `Request` thêm `SourceHints source_hints = 24;`.
4. `ListRequestsRequest` thêm `string source_provider = 6; string source_site = 7; string source_ref = 8;`.
5. `make proto-gen`; cập nhật `request_mapper.go` (TASK-REQ-002-07): `toProtoRequest` điền `source_hints`; `filterFromProto` điền ba bộ lọc.

## Kiểm thử

- `buf lint`, `buf breaking` (các trường là cộng thêm).
- `TestMapper_SourceHintsRoundTrip`, `TestFilterFromProto_SourceFields`.
- Lệnh: `go build ./... && go test ./services/request-service/internal/adapter/grpc/...`.

## Tiêu chí hoàn thành

- [x] `buf breaking` xanh.
- [x] Số trường đúng như bước 1 đến 4.
- [x] Mapper và test cập nhật.

## Rủi ro và lưu ý

- `type_hint` ở `SourceHints` do CR-REQ-006 dùng; thêm ngay để khỏi sửa proto lần nữa. Client thông thường (UI, MCP) có được gửi `type_hint` hay không: quyết định ở TASK-REQ-004-04 (chỉ chấp nhận khi `link_reason` có từ `SpawnChildRequest`; `CreateRequest` công khai bỏ qua trường này).
