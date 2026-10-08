# TASK-REQ-005-07: Proto và handler gRPC: `ClassifyRequest`, `ConfirmRequestType`, `ChangeRequestType`, `ListRequestTypeHistory`

**From Solution:** BE-REQ-SOL-005
**Priority:** P0
**Service:** `proto`, `request-service`
**File:** `proto/orca/request/v1/request.proto` (sửa), `proto/gen/go/orca/request/v1/*` (sinh lại), `internal/adapter/grpc/server.go`, `internal/adapter/grpc/request_mapper.go`, `internal/adapter/grpc/type_change_mapper.go`, `cmd/server/main.go` (sửa)
**Depends on:** TASK-REQ-005-04, 005-05, 005-06
**Status:** [x] DONE (đã kiểm chứng 2026-10-08: `go test -tags integration -race ./internal/adapter/postgres ./internal/adapter/mysql -run RPC (13+ kịch bản gRPC, máy trạng thái thật); go test ./internal/adapter/grpc; buf lint/breaking --path orca/request/v1/request.proto`)

---

## Context

CR-REQ-005 mục 2.1 định nghĩa message. Proto hiện có `Request` (trường 1 đến 24). `Request.classification_attempts = 26` (số 25 dành cho `returned_category` của CR-REQ-006). `ListRequestTypeHistory` chưa có trong README v6 mục 3.6; README mục 8 điểm 12 đã thêm nó vào danh sách RPC thiếu. Tên kênh WS do CR-REQ-016 chốt. Quyền ghi theo project do gateway; `request-service` tự kiểm tenant và user (mục 8 điểm 13).

## Việc cần làm

1. `request.proto`: thêm bốn RPC và message: `ClassifyRequestRequest{request_id}`, `ClassifyRequestResponse{request}`, `ConfirmRequestTypeRequest{request_id, type, size, urgency, reason, expected_version}`, `ConfirmRequestTypeResponse{request}`, `ChangeRequestTypeRequest{request_id, new_type, size, urgency, reason, expected_version}`, `ChangeRequestTypeResponse{request}`, `ListRequestTypeHistoryRequest{request_id}`, `ListRequestTypeHistoryResponse{repeated RequestTypeChange changes}`, `RequestTypeChange{from_type, to_type, actor_id, actor_kind, reason, at}`. `Request` thêm `int32 classification_attempts = 26;`.
2. `make proto-gen`; kiểm `buf lint`, `buf breaking` (chỉ cộng thêm).
3. `server.go`: bốn handler; `actor_id` từ `tenant.UserID(ctx)`, `ActorKind=user`; lỗi qua `apperrors.ToGRPCStatus`. `ClassifyRequest` gọi `ProposeRequestClassification.ClassifyNow`.
4. `type_change_mapper.go`: `toProtoTypeChange`.
5. `main.go`: lắp use case và consumer (TASK-REQ-005-03) và cổng no-op (TASK-REQ-005-04).
6. README service: bảng real vs stub (các RPC này là thật; `ApprovalRecorder`, `ApprovalCanceller`, `ExecutionGuard` là no-op tới CR-REQ-009, 011).
7. Chú thích trong proto: `ConfirmRequestType` và `ChangeRequestType` là lệnh của người dùng; không có đường tool AI bỏ qua bước xác nhận.

## Kiểm thử

- `TestServer_ConfirmRequestType_RoundTrip` và `_ChangeRequestType_RoundTrip` (gRPC in-process với repo thật, từng dialect).
- `TestServer_ClassifyRequest_NotClassifiable` (`FailedPrecondition`, `REQUEST_NOT_CLASSIFIABLE`).
- `TestServer_ListRequestTypeHistory_Order`.
- `TestServer_ActorFromMetadata` (không user thì `REQUEST_REPORTER_REQUIRED` hoặc mã phù hợp).
- Hợp đồng: `buf breaking`; test tên `type`, `size`, `urgency` trong proto khớp domain.
- Lệnh: `go test ./services/request-service/internal/adapter/grpc/...`.

## Tiêu chí hoàn thành

- [x] Bốn RPC thật, không còn `Unimplemented`.
- [x] `buf lint` và `buf breaking` xanh.
- [x] README real vs stub cập nhật.
- [x] Danh tính người dùng lấy từ metadata, không từ body.

## Rủi ro và lưu ý

- Gateway (CR-REQ-016) chỉ chuyển được các RPC này khi có kênh WS tương ứng; ở đây chỉ làm phía service.
- Frontend phải biết `expected_version` từ `Request.version`; ghi chú trong proto.

## Ghi chú triển khai

- Bốn handler ở `adapter/grpc/server_classification.go`: `ClassifyRequest` trả `run_id` ngay (quyết định D3, proto đã có `run_id`), `ConfirmRequestType`, `ChangeRequestType`, `ListRequestTypeHistory` (`actor_kind` `agent` hiển thị `ai`). Actor từ metadata (`REQUEST_REPORTER_REQUIRED` khi thiếu), không từ body.
- Không có `type_change_mapper.go` riêng: `toProtoTypeChange` nằm cùng tệp handler.
- Kịch bản gRPC: `ClassifyRequest_RunIDAndResult`, `_NotClassifiable`, `ConfirmChange_RoundTripAndHistory`, `ConfirmChange_ActorFromMetadata` (hai dialect).
