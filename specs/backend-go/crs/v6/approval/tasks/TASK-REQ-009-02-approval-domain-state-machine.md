# TASK-REQ-009-02: Domain `Approval`, máy trạng thái và lỗi miền

**From Solution:** [BE-REQ-SOL-009](../solutions/BE-REQ-SOL-009-generic-approval-domain-and-api.md) mục B
**Priority:** P0
**Service/Area:** `request-service` / domain
**File:** `backend-go/services/request-service/internal/domain/approval.go` (mới), `approval_subject.go` (mới), `approval_test.go` (mới)
**Depends on:** CR-REQ-001 (module Go)
**Status:** `[x] DONE`

## Context

- Không có mã nào để sửa: `request-service` chưa tồn tại. Mẫu phong cách domain thuần: `mcp-service/internal/domain/approval.go` (có `EffectiveStatus` cho hết hạn lười) và `common/apperrors`.
- Trạng thái Approval của repo MCP là `denied`; ở đây là `rejected` (README 3.5). Không copy tên.
- Domain chỉ import stdlib (arch/03).

## Việc cần làm

1. `approval_subject.go`: `type SubjectType string` với tám hằng (`SubjectRequestType`, `SubjectSolution`, `SubjectFindings`, `SubjectAnswer`, `SubjectPlan`, `SubjectPhase`, `SubjectTaskList`, `SubjectPreDeploy`), `func (s SubjectType) Valid() bool`, `var AllSubjectTypes = []SubjectType{...}`.
2. `approval.go`: `type ApprovalStatus string` (`pending|approved|rejected|cancelled|expired`), struct `Approval` đủ trường như solution mục B, `func (s ApprovalStatus) Terminal() bool`.
3. Phương thức `Approve(by, comment string, now time.Time) error`, `Reject(by, comment string, now time.Time) error` (comment sau `TrimSpace` phải không rỗng, tối đa 2000 ký tự), `Cancel(by, reason string, now time.Time) error`, `Expire(now time.Time) error`. Mỗi phương thức: chỉ từ `pending`, nếu không trả `ErrApprovalNotPending`; đặt `DecidedBy`, `DecidedAt`, `Comment`, tăng `Version`, cập nhật `UpdatedAt`.
4. `func (a Approval) EffectiveStatus(now time.Time) ApprovalStatus`: `pending` mà `DueAt != nil && !now.Before(*DueAt)` trả `expired` (dùng để trả `REQUEST_APPROVAL_EXPIRED` mà không ghi).
5. Lỗi miền: `ErrApprovalNotPending`, `ErrApprovalCommentRequired`, `ErrApprovalCommentTooLong`, `ErrApprovalSubjectTypeInvalid`. Ánh xạ sang mã `REQUEST_APPROVAL_*` ở lớp usecase (không đặt mã gRPC trong domain).
6. `func ValidateApprovalComment(s string) (string, error)` dùng chung bởi Reject và Cancel.

## Kiểm thử

- `approval_test.go`: bảng đủ 5x4 cặp (trạng thái, hành động): chỉ 4 cặp từ `pending` hợp lệ; `Reject` thiếu comment; comment 2001 ký tự; `EffectiveStatus` ở biên `now == DueAt`; `Version` tăng đúng 1 mỗi lần chuyển.
- Lệnh: `cd backend-go/services/request-service && go test ./internal/domain/...`.

## Tiêu chí hoàn thành

- [x] Không import ngoài stdlib trong `internal/domain`.
- [x] Mọi chuyển trạng thái bất hợp lệ trả `ErrApprovalNotPending`.
- [x] `AllSubjectTypes` khớp CHECK của migration (test so sánh với danh sách hằng chép trong test, và task 06 kiểm lại với SQL).
- [x] `go vet` và lint sạch; tên file theo khái niệm, không `utils`/`helpers`.

## Rủi ro và lưu ý

- Thêm `subject_type` sau này (CR bổ sung) là thay đổi ở ba nơi: hằng, CHECK, handler; ghi vào doc comment của `AllSubjectTypes`.
