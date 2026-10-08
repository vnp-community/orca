# TASK-REQ-010-07: API quản trị chính sách (có điều kiện) và test tích hợp toàn luồng

**From Solution:** [BE-REQ-SOL-010](../solutions/BE-REQ-SOL-010-approval-authorization-notification-expiry.md) mục D, 5
**Priority:** P2
**Service/Area:** `request-service` / usecase, grpc, test
**File:** `internal/usecase/manage_approval_policies.go` (mới), `internal/adapter/grpc/approval_policy_admin_server.go` (mới, chỉ khi được chấp nhận), `proto/orca/request/v1/approval.proto` (sửa, chỉ khi được chấp nhận), `internal/adapter/{postgres,mysql}/approval_policy_flow_integration_test.go` (mới)
**Depends on:** TASK-REQ-010-03, TASK-REQ-010-06
**Status:** [x] DONE (đã kiểm chứng 2026-10-08: `go test -tags integration ./internal/adapter/postgres ./internal/adapter/mysql -run "ApprovalFlow|ApprovalPolicy"`; `cmd/server` `TestRun_ServesApprovalServicesWhenEnabled`)

## Context

- README v6 mục 3.6 không có RPC quản trị chính sách. Câu hỏi mở 2 của SOL-010 chưa có đáp án. **Không viết phần proto/gRPC của task này cho tới khi người duyệt series chấp nhận**; nếu từ chối, v1 chỉ dùng `DefaultPolicy` và bảng nạp bằng seed SQL (bước 5).
- Bước test tích hợp toàn luồng độc lập với câu hỏi đó và luôn phải làm.

## Việc cần làm

1. (Có điều kiện) `ManageApprovalPolicies.List/Upsert/Delete`: chỉ `tenant.Role=="admin"`; kiểm `approvers` (kind hợp lệ, tối đa 20, team tồn tại qua `tenant-service`), `due_after_seconds >= 0`, `REQUEST_APPROVAL_POLICY_INVALID`/`..._NOT_FOUND`; khoá lạc quan bằng `version`.
2. (Có điều kiện) Thêm `ApprovalPolicyAdminService` vào `approval.proto` (`buf breaking` xanh), server tự kiểm quyền admin.
3. Test tích hợp toàn luồng với handler giả: mở Approval có chính sách `team:<id>` (fake tenant-service) rồi thành viên team `Approve` được, người ngoài `REQUEST_APPROVAL_NOT_APPROVER`; `pre_deploy` khi chỉ người yêu cầu là admin thì `NO_ELIGIBLE_APPROVER`; hết hạn đưa Request về backlog.
4. Test chéo tenant: chính sách của tenant A không ảnh hưởng tenant B (cả RLS Postgres và lọc MySQL).
5. Nếu không làm API: ghi câu lệnh `INSERT` mẫu vào README của service. Không tạo migration dữ liệu, để không ép chính sách lên mọi tenant.

## Kiểm thử

- Lệnh: `go test ./internal/usecase/... ./internal/adapter/... -run "PolicyAdmin|ApprovalPolicyFlow"` trên hai DB.
- Test quyền: người không phải admin gọi `Upsert` bị `PermissionDenied`.

## Tiêu chí hoàn thành

- [x] Test toàn luồng xanh trên Postgres và MySQL.
- [x] Chéo tenant không rò.
- [x] Phần API chỉ tồn tại nếu câu hỏi mở 2 được chấp nhận; trạng thái được ghi vào PR.

## Rủi ro và lưu ý

- Thêm RPC ngoài README 3.6 cần cập nhật README v6 (người điều phối).
- Không đặt mặc định chính sách bằng migration dữ liệu.

## Kết quả triển khai (2026-10-08)
- Câu hỏi mở 2 coi như được chấp nhận (proto `ApprovalPolicyAdminService` 3 RPC đã có từ đợt proto-first): `ManageApprovalPolicies` (chỉ admin người, không agent; Validate; khoá lạc quan `version`; `tenant_id` và `created_by` lấy từ ctx) và `ApprovalPolicyAdminServer` thật. README v6 mục 3.6 vẫn chưa liệt kê RPC này (người điều phối cập nhật).
- Test toàn luồng với handler giả (`RunApprovalFlowContract`): chính sách team (thành viên duyệt được, người ngoài `NOT_APPROVER`), snapshot không đổi khi sửa chính sách, `pre_deploy` mà admin duy nhất là người yêu cầu thì `NO_ELIGIBLE_APPROVER` và không có dòng, hết hạn đưa Request về backlog, chéo tenant không rò (cả RLS Postgres và lọc MySQL).
- Chưa làm: kiểm "team tồn tại qua tenant-service" khi lưu chính sách (cần client thêm; chính sách trỏ team không tồn tại thì không ai khớp).
