# TASK-REQ-015-03: Domain `backlog_gate` (cổng của task) và `page_token` keyset

**From Solution:** BE-REQ-SOL-015
**Priority:** P1
**Service:** `request-service`
**File:** `backend-go/services/request-service/internal/domain/backlog_gate.go` (mới), `internal/domain/backlog_page_token.go` (mới), `internal/domain/backlog_gate_test.go`, `backlog_page_token_test.go` (mới)
**Depends on:** CR-REQ-003 (`FlowFor`, `PhasesFor`, `Size`), CR-REQ-009 (kiểu `Approval`, `subject_type`)
**Status:** `[x] DONE`

---

## Context

- `request-service` chưa tồn tại ngày 2026-10-06; đọc `FlowDefinition`, `FlowFor`, `PhasesFor` thật khi CR-REQ-003 merge; tên dưới đây theo CR.
- Hàm thuần, không I/O, để test table-driven toàn bộ 11 loại. Đây là nơi duy nhất mã hoá bảng cổng của solution 2.4.
- Task làm việc: `task_type` ∈ {`task`,`bug`,`feature`}. Container cổng: cha là `phase` thì Phase; cha là `plan` thì Plan; không cha thì không container (hotfix).
- "Bản mới nhất theo `created_at`" của `(subject_type, subject_id)` quyết định; `pending` và `rejected` là chưa duyệt.
- Phân trang theo Request, base64 `(updated_at, id)`.

## Việc cần làm

1. `backlog_gate.go`:
   ```go
   type GateStatus string // approved, pending, rejected, none
   type TaskGateInput struct { Request Request; Task TaskView; Container *TaskView /*phase|plan|nil*/; Plan *TaskView; Approvals ApprovalIndex }
   func ResolveTaskGate(in TaskGateInput) GateResolution // {Approved bool; Status GateStatus; WaitingForPhaseSplit bool}
   ```
   Quy tắc: dưới `phase` với `FlowFor(type).ExecutionGates` có `phase`: cần `phase` `approved` trên Phase và `plan` `approved` trên Plan; dưới `phase` mà không có `phase` gate: `plan` `approved` trên Plan cha của Phase; dưới `plan` với `PhasesFor(size)` sai: `plan` hoặc `task_list` (theo `Plan.Kind`) `approved`, `security` dùng `pre_deploy` trên Plan; dưới `plan` với `PhasesFor` đúng: `WaitingForPhaseSplit=true`, không bao giờ `Approved`; hotfix (không cha): `pre_deploy` `approved` trên task.
2. `ApprovalIndex`: map `(subjectType, subjectID) → latest Approval` dựng từ danh sách `[]Approval` (chọn `created_at` lớn nhất, hoà thì id lớn hơn); hàm `NewApprovalIndex([]Approval)`.
3. `GateStatus` của nhóm: `approved` khi cổng cần thiết đều `approved`; nếu có `pending` thì `pending`; nếu bản mới nhất `rejected` thì `rejected`; không có gì thì `none`.
4. `backlog_page_token.go`: `EncodePageToken(updatedAt time.Time, id string) string` (base64 URL không padding của JSON `{"t":RFC3339Nano,"i":id}`) và `DecodePageToken(s string) (time.Time, string, error)` trả `ErrBadPageToken` (map thành `REQUEST_BACKLOG_BAD_PAGE_TOKEN`).
5. `TaskView`: kiểu domain nhỏ (`ID`, `ParentID`, `Type`, `Status`, `RequestID`, `Title`, `EstimatedHours`, `AssigneeID`) ánh xạ từ `TaskClient`; định nghĩa ở domain để hàm thuần không import grpcclient.
6. Không tạo file tên `helpers`/`utils`.

## Kiểm thử

- `TestResolveTaskGate_AllTypes_Table`: dùng `FlowFor` thật cho 11 loại và size S/M/L; mỗi dòng bảng solution 2.4 có ít nhất một ca chưa duyệt, một ca đã duyệt.
- Ca riêng: `change_request` Plan duyệt nhưng Phase chưa duyệt (chưa); bug size L chưa chia Phase (`WaitingForPhaseSplit`); `task`/`docs` dưới Plan vỏ `task_list`; hotfix `pre_deploy`; Approval đòi duyệt lại (`pending` mới sau `approved` cũ) thành chưa duyệt; `rejected` mới nhất.
- `TestApprovalIndex_LatestWins_TieBreak`.
- `TestPageToken_RoundTrip`, `_BadBase64`, `_BadJSON`, `_EmptyID`.
- `cd /opt/repos/orca/backend-go && go test ./services/request-service/internal/domain/... -run 'TaskGate|ApprovalIndex|PageToken' -v`.

## Tiêu chí hoàn thành

- [x] Mọi điều kiện README v6 3.8 và bảng solution 2.4 có test.
- [x] Hàm không import ngoài stdlib và `common/apperrors`.
- [x] `page_token` hỏng trả `ErrBadPageToken`, không panic.
- [x] Không file tên `helpers`/`utils`/`common`/`misc`.

## Rủi ro và lưu ý

- Bảng cổng 2.4 là mở rộng của CR (README mục 8 điều 7 chấp nhận nhưng chưa xác nhận từng dòng): xác nhận với chủ CR-REQ-015 trước khi merge.
- Task lồng sâu hơn một cấp bị bỏ ở bản đầu (Q4 của CR); hàm trả `Skipped` để hiển thị sau.
- Giả định `approvals` giữ nhiều bản ghi mỗi chủ thể; nếu CR-REQ-009 chỉ giữ một dòng và cập nhật tại chỗ, `ApprovalIndex` vẫn đúng nhưng đơn giản hơn.
