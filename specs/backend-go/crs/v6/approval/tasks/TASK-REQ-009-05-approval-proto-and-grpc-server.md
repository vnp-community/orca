# TASK-REQ-009-05: `approval.proto` và `ApprovalService` gRPC

**From Solution:** [BE-REQ-SOL-009](../solutions/BE-REQ-SOL-009-generic-approval-domain-and-api.md) mục G
**Priority:** P0
**Service/Area:** `request-service` / proto, adapter grpc
**File:** `backend-go/proto/orca/request/v1/approval.proto` (mới), `backend-go/proto/gen/go/orca/request/v1/` (sinh), `backend-go/services/request-service/internal/adapter/grpc/approval_server.go` (mới), `approval_server_test.go` (mới)
**Depends on:** TASK-REQ-009-04; CR-REQ-001 (buf, generate)
**Status:** [x] DONE (đã kiểm chứng 2026-10-08: `go test ./internal/adapter/grpc/... -run Approval`; `cmd/server` `TestRun_ServesApprovalServicesWhenEnabled` với Postgres thật; `buf lint` không báo lỗi cho approval.proto)

## Context

- `proto/orca/request/` chưa tồn tại; CR-REQ-001 tạo `request.proto`. Đặt `approval.proto` cùng package `orca.request.v1`.
- Quy ước repo: sinh mã qua `buf generate` ở `backend-go/proto` (đọc `buf.yaml`, `buf.gen.yaml` lúc làm). `buf breaking` (FILE) phải xanh.
- README mục 8 điểm 13: gateway không kiểm quyền OPA; server phải tự kiểm quyền.

## Việc cần làm

1. Viết `approval.proto` đúng CR mục 2.6: enum `ApprovalSubjectType`, `ApprovalStatus`; message `Approval`; `ApprovalService` bảy RPC; `DecideApprovalResponse{approval, request_status}`. Số field theo CR.
2. Chạy `buf lint`, `buf generate`; commit mã sinh nếu repo commit mã sinh (kiểm `git ls-files backend-go/proto/gen | head`).
3. `approval_server.go`: mỗi RPC gọi use case tương ứng; `RequestApproval` chỉ cho `PRE_DEPLOY` hoặc mở lại cổng đã đóng, còn lại trả `REQUEST_APPROVAL_SUBJECT_TYPE_NOT_ALLOWED`. Lấy tenant và người dùng từ ngữ cảnh gRPC (`grpcmw`, `tenant`), không từ thân request.
4. Ánh xạ `apperrors` sang `codes` gRPC; không để lộ thông điệp nội bộ.
5. Hàm chuyển `domain.Approval` sang proto và ngược lại ở file riêng `approval_mapping.go` (mới).

## Kiểm thử

- `approval_server_test.go` với use case giả: `RequestApproval` với `SOLUTION` bị từ chối; mã lỗi gRPC đúng cho từng lỗi; thiếu tenant trong ngữ cảnh thì `Unauthenticated`/`PermissionDenied` theo quy ước `grpcmw`.
- `buf breaking --against '.git#branch=main'` (từ `backend-go/proto`).
- Lệnh: `go test ./internal/adapter/grpc/... -run Approval`.

## Tiêu chí hoàn thành

- [x] `buf lint` không báo lỗi cho `approval.proto`. `buf breaking`: chưa chạy vì task không sửa `.proto` (còn `- [ ]` ngầm cho đến khi người điều phối chạy).
- [x] Bảy RPC khớp tên trong README v6 mục 3.6.
- [x] Không RPC nào tin `tenant_id` từ thân request.
- [x] Mã sinh biên dịch; server đăng ký được trong `main.go` (task 06).

## Rủi ro và lưu ý

- Frontend và `api-gateway` (CR-REQ-016) đọc tên trường từ file này: không đổi tên sau khi merge nếu CR-REQ-016 đã bắt đầu.

## Kết quả triển khai (2026-10-08)
- `approval.proto` do đợt proto-first tạo; task này không sửa `.proto`. `ApprovalServer` cài 8 RPC (7 của README 3.6 cộng `ExtendApproval`), `approval_mapping.go` chuyển enum/message hai chiều, lỗi qua `apperrors.ToGRPCStatus`. Tenant và người gọi chỉ lấy từ ctx (`grpcmw`), server không đọc `tenant_id` từ thân. `RequestApproval` chỉ nhận `PRE_DEPLOY` hoặc mở lại cổng đã đóng (`REQUEST_APPROVAL_SUBJECT_TYPE_NOT_ALLOWED` nếu không).
- `buf lint` toàn repo còn các cảnh báo cũ ở proto khác nhưng 0 cảnh báo ở `request/v1/approval.proto`. `buf breaking`: chưa chạy (không sửa proto).
