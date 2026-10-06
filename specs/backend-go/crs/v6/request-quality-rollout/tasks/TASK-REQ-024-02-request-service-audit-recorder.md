# TASK-REQ-024-02: Cổng `AuditRecorder` và bản ghi audit cho mọi quyết định Request

**From Solution:** BE-REQ-SOL-024
**Priority:** P1
**Service:** `request-service`
**File:** `backend-go/services/request-service/internal/domain/audit_action.go` (mới), `.../internal/usecase/audit_recorder.go` (mới), `.../internal/adapter/audit/auth_audit_recorder.go` (mới), các use case của CR-REQ-004 đến 010 (thêm lời gọi), `.../internal/usecase/audit_test.go` (mới)
**Depends on:** TASK-REQ-024-01; CR-REQ-004, 005, 006, 007, 009, 010 đã có use case; BE-REQ-SOL-025 task cờ (cho `request.flow.set`)
**Status:** `[ ] TODO`

---

## Context

- `request-service` chưa tồn tại tại thời điểm viết; tên use case lấy từ CR-REQ-004 đến 010 (`CreateRequest`, `ConfirmRequestType`, `ChangeRequestType`, `ReturnToBacklog`, `ReopenRequest`, `CancelRequest`, `ChooseSolutionOption`, `Approve`, `Reject`, `Cancel`, hết hạn Approval).
- Bảng action và `actor_type`: SOL-024 mục 2.8. `outcome` chỉ `allowed|denied`; thất bại kỹ thuật không audit.
- Cấu trúc service theo `arch/03`: cổng ở `usecase`, adapter ở `adapter/`.

## Việc cần làm

1. `domain/audit_action.go`: hằng `ActionRequestCreate = "request.create"`, ..., `ActionApprovalExpire = "approval.expire"`, `ActionRequestJiraSync` không ở đây (thuộc `issue-status-sync`); kiểu `ActorKind` ánh xạ `user|agent|system`.
2. `usecase/audit_recorder.go`: `type AuditRecorder interface { Record(ctx context.Context, e AuditEvent) }`; `AuditEvent{TenantID, ActorID, ActorKind, Action, TargetType, TargetID, Outcome string; Metadata map[string]string}`. Hàm dựng `Target = TargetType + ":" + TargetID`.
3. `adapter/audit/auth_audit_recorder.go`: bọc `auditclient.AppendDetailed`; `Metadata` marshal JSON; **danh sách cho phép khoá** (`source_provider`, `client_name`, `type`, `type_source`, `from`, `to`, `returned_from_stage`, `option`, `subject_type`, `stage`, `enabled`); khoá khác bị bỏ (chặn rò `title`, `body`).
4. Gọi `Record` ở mỗi use case sau khi giao dịch thành công (ngoài transaction): `actor_type=agent` khi `source_provider=mcp` hoặc `origin.kind=mcp` (CR-REQ-004, 017); lệnh bị từ chối quyền (`REQUEST_APPROVAL_NOT_APPROVER`) ghi `outcome=denied`.
5. Bản `noopAuditRecorder` cho test và khi `AUTH_SERVICE_ADDR` rỗng.

## Kiểm thử

- `audit_test.go`: bảng 11 action, mỗi action đúng một bản ghi với `actor_type` đúng; từ chối quyền thành `denied`; metadata chứa `title` bị bỏ (test đưa `title` và `body` vào metadata, khẳng định không có trong JSON gửi đi).
- Test hợp đồng nhỏ: `go test ./internal/usecase/... -run Audit`.

## Tiêu chí hoàn thành

- [ ] Mỗi action ở SOL-024 mục 2.8 (trừ `request.jira.sync`) tạo đúng một bản ghi.
- [ ] Không bản ghi nào chứa `title`, `body`, nội dung Solution.
- [ ] Lỗi gọi auth-service không làm hỏng use case.

## Rủi ro và lưu ý

- Gọi đồng bộ có thể thêm độ trễ; dùng timeout ngắn (2 giây) cho `Record`.
- Audit sau commit có thể mất khi tiến trình chết giữa chừng (best effort, như phần còn lại của repo).
