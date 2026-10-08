# TASK-REQ-002-02: Domain: Request, loại, trạng thái, nguồn, Solution, liên kết, lỗi

**From Solution:** BE-REQ-SOL-002
**Priority:** P0
**Service:** `request-service`
**File:** `internal/domain/{request.go,request_type.go,request_status.go,request_size_urgency.go,request_source.go,request_type_change.go,solution.go,request_link.go,request_errors.go}` và `*_test.go` (mới)
**Depends on:** TASK-REQ-001-01
**Status:** [x] DONE (đã kiểm chứng 2026-10-07: go test ./internal/domain/... (TestParseRequestType_All, TestParseRequestStatus_All, TestParseSizeUrgencyProvider_All, TestNewRequest_Validation, TestRequestStatus_IsTerminal, TestNewRequestLink_SelfRejected, TestErrorsMapToGRPC))

---

## Context

Mẫu domain thuần stdlib: `task-service/internal/domain/task.go` (kiểu `Status`, `ErrCannotSetInProgress`), `task_source.go`. Lỗi dùng `common/apperrors.New(kind, code, message, cause)`, kind gồm `KindNotFound`, `KindInvalidArgument`, `KindFailedPrecondition`, `KindAlreadyExists`. `tenant.RequireTenantID` trả lỗi `tenant.ErrNoTenant`. Giá trị hợp lệ: README v6 mục 3.2 (11 loại), 3.3 (11 trạng thái), 3.5 (nguồn, kind Solution, trạng thái Solution, lý do link). Domain không import adapter, proto hay `pgx`.

## Việc cần làm

1. `request_type.go`: `type RequestType string`; 11 hằng (`RequestTypeChangeRequest = "change_request"`, `bug`, `hotfix`, `task`, `spike`, `question`, `refactor`, `security`, `performance`, `docs`, `ops_request`); `ParseRequestType(string) (RequestType, error)` (lỗi `REQUEST_INVALID_TYPE`); `AllRequestTypes() []RequestType` theo thứ tự README.
2. `request_status.go`: `type RequestStatus string`; 11 hằng (`new`, `classifying`, `awaiting_type_confirmation`, `analyzing`, `awaiting_analysis_approval`, `planning`, `awaiting_plan_approval`, `executing`, `completed`, `request_backlog`, `cancelled`); `ParseRequestStatus`; `IsTerminal()` đúng cho `completed`, `cancelled`. **Không** khai báo bảng chuyển (CR-REQ-003).
3. `request_size_urgency.go`: `RequestSize` (`S`,`M`,`L`), `Urgency` (`normal`,`urgent`), `ParseSize` (`REQUEST_INVALID_SIZE`), `ParseUrgency`; `ReturnStage` (`classification`,`analysis`,`plan`,`phase`,`task`) và `TypeSource` (`ai`,`human`).
4. `request_source.go`: `SourceProvider` (`jira`,`github`,`gitlab`,`linear`,`mcp`,`manual`,`webhook`), `ParseSourceProvider`; `SourceRef{Provider SourceProvider; Site, Ref, URL string}`. Chuẩn hoá site và ref thuộc CR-REQ-004, chưa làm ở đây.
5. `request.go`: `type Request struct` đủ cột của `requests` (`Type RequestType` rỗng = chưa có; `Confidence *float64`; `Size RequestSize` rỗng = chưa có; `PlanTaskID string` rỗng = chưa có; `ProjectID string` rỗng = không gắn project; `Version int64`; `CreatedAt`, `UpdatedAt time.Time`); `NewRequestInput`; `NewRequest(in) (Request, error)`: kiểm tenant (`REQUEST_TENANT_REQUIRED`), `title` sau `strings.TrimSpace` không rỗng (`REQUEST_TITLE_REQUIRED`), `reporter_id` không rỗng, provider hợp lệ; id `uuid.NewString()`; `Status=new`, `Urgency=normal`, `Version=1`. `HasType()`.
6. `request_type_change.go`: `RequestTypeChange{ID, RequestID, FromType, ToType RequestType; ActorID string; ActorKind ActorKind; Reason string; At time.Time}`; `ActorKind` (`ai`,`user`; thêm `system` ở CR-REQ-006 cho bảng khác, không dùng ở đây).
7. `solution.go`: `Solution{ID, RequestID, Kind, Status, OptionsJSON []byte, ChosenOption *int, ContentRef, GenerationRunID string, CreatedAt, UpdatedAt, Version}`; `SolutionKind` (`solution`,`diagnosis`,`findings`,`answer`), `SolutionStatus` (`draft`,`proposed`,`approved`,`rejected`,`superseded`). CR-REQ-007 sở hữu nội dung `options`.
8. `request_link.go`: `RequestLink{ParentRequestID, ChildRequestID string; Reason LinkReason; CreatedBy string; CreatedAt}`; `LinkReason` 4 giá trị; `NewRequestLink` trả `REQUEST_LINK_SELF` khi cha trùng con.
9. `request_errors.go`: constructor cho các mã ở SOL-002 mục B kèm `Kind` (bảng CR-REQ-002 mục 2.2): `ErrRequestNotFound(id)`, `ErrRequestVersionConflict(id, expected)`, `ErrSourceAlreadyExists(existingRequestID)` (mang `request_id` hiện có, ví dụ trong `Message` hoặc trường chi tiết theo cách `apperrors` cho phép; đọc `apperrors.go` để chọn), v.v.
10. Hằng giá trị phải là nguồn duy nhất; test hợp đồng ở TASK-REQ-002-06 so chúng với `CHECK` của migration.

## Kiểm thử

- `TestParseRequestType_All` (11 giá trị hợp lệ, chuỗi rỗng và `"foo"` lỗi `REQUEST_INVALID_TYPE`), tương tự `Status`, `Size`, `Urgency`, `SourceProvider`.
- `TestNewRequest_Validation` (title toàn khoảng trắng, thiếu tenant, thiếu reporter, provider lạ; hợp lệ cho mặc định `Status=new`, `Version=1`).
- `TestRequestStatus_IsTerminal`.
- `TestNewRequestLink_SelfRejected`.
- `TestErrorsMapToGRPC`: mỗi lỗi qua `apperrors.ToGRPCStatus` ra đúng `codes` (`NotFound`, `InvalidArgument`, `FailedPrecondition`, `AlreadyExists`).
- Lệnh: `go test ./services/request-service/internal/domain/...`.

## Tiêu chí hoàn thành

- [x] Domain chỉ import stdlib, `uuid`, `common/apperrors`, `common/tenant`.
- [x] 11 loại, 11 trạng thái đúng chính tả README v6.
- [x] Mọi mã lỗi ở SOL-002 mục B có constructor và test ánh xạ gRPC.
- [x] Không có tên file `helpers`, `utils`, `common`, `misc`.

## Rủi ro và lưu ý

- `Type` rỗng thay vì con trỏ: repository phải ánh xạ `NULL` sang rỗng; viết chung một hàm quét nullable ở mỗi adapter.
- CR-REQ-003 sẽ thêm trigger, bảng chuyển và `FlowFor`; đừng đặt chúng vào `request_status.go`.

## Ghi chú triển khai

`NewRequest` nay từ chối `reporter_id` rỗng (mã `REQUEST_REPORTER_REQUIRED`) và `source_provider` rỗng/lạ, bỏ các khối `if` rỗng và comment nháp; thời gian cắt về micro giây. `ActorKindAI` đổi thành `ActorKindAgent = "agent"` cho khớp CHECK `actor_kind`. Tiêu chí 'domain chỉ import stdlib, uuid, apperrors, tenant' đúng cho các file của task này (request*.go); package `domain` còn file của feature khác import `grpc` (`rpc_catalog.go`) và `x/text`.
