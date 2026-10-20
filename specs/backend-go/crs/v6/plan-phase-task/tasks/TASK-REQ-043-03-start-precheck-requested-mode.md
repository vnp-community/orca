# TASK-REQ-043-03: `ExecutionStartPrecheck` kiểm quyền thực thi theo chế độ được yêu cầu

**From Solution:** [BE-REQ-SOL-043](../solutions/BE-REQ-SOL-043-approve-plan-and-run-all-steps.md)
**Priority:** P1
**Service:** `request-service`
**File:** `internal/usecase/approval_execution_precheck.go` (sửa), `internal/usecase/approval_execution_precheck_test.go` (sửa), `cmd/server/wire_execution.go` (kiểm, không đổi nếu chữ ký đủ)
**Depends on:** TASK-REQ-043-01
**Status:** [x] DONE (2026-10-20)

---

## Context

- `Check(ctx, a, approverID)` (`approval_execution_precheck.go:21-53`) chỉ kiểm khi chế độ phân giải là `continuous` (`:34-39`): `p.Modes.Mode(ctx, req)` đọc cài đặt dự án nên dự án `step_by_step` không bao giờ bị kiểm.
- Với lựa chọn "chạy hết" ở dự án `step_by_step`, người duyệt sẽ là người thực thi cho cả run (`execution_actors.go:35-73`), nên phải kiểm quyền execute ngay lúc duyệt.
- Người không có quyền duyệt phải nhận lỗi duyệt thường, không phải gợi ý quyền task (`:41-46`); test `TestStartPrecheck_SomeoneWhoMayNotDecide...` giữ nguyên.
- `wire_execution.go:146-151` nối precheck sau khi `Access` được tạo; chữ ký `Check` đổi không đụng việc nối.

## Việc cần làm

1. `Check(ctx, a, approverID string, requested domain.ExecutionMode) error`.
2. Thay `switch` lọc subject/stage bằng `domain.ExecutionModeChoiceApplies(a)`; hành vi lọc không đổi.
3. Chế độ hiệu lực = `requested` nếu khác rỗng, ngược lại `p.Modes.Mode(ctx, req)`. Chỉ `continuous` mới tiếp tục tới `CanDecide` rồi `Access.CheckReady`.
4. Giữ nguyên cách đổi `REQUEST_EXECUTE_FORBIDDEN` thành `ErrExecutionActorNotAllowed` và việc lỗi tạm của task-service không bị đổi thành từ chối.
5. Cập nhật chỗ gọi trong `decide_approval.go` (task 02) và mọi test gọi `Check`.

## Kiểm thử

- Ca mới: dự án `step_by_step` + `requested=continuous` + người duyệt thiếu quyền → `REQUEST_EXECUTION_ACTOR_NOT_ALLOWED`; cùng ca nhưng người duyệt có quyền → nil; `requested=""` ở dự án `step_by_step` → nil và `CheckReady` không được gọi; `requested=continuous` kèm Approval `phase` hoặc stage khác → nil (không kiểm); người không được duyệt + `requested=continuous` → nil (nhường cho `CanDecide` trong transaction).
- Test hiện có (`TestStartPrecheck_*`) cập nhật chữ ký, kỳ vọng không đổi.
- `go test ./services/request-service/internal/usecase/... -run StartPrecheck -v`.

## Tiêu chí hoàn thành

- [x] Lựa chọn "chạy hết" ở dự án `step_by_step` bị từ chối trước khi ghi quyết định khi người duyệt thiếu quyền execute. Bằng chứng: `TestStartPrecheck_RequestedContinuousInAStepByStepProjectChecksTheApprover`, `TestDecideApproval_PrecheckSeesTheRequestedMode` (không ghi gì, Approval vẫn `pending`).
- [x] Không còn `switch` trùng lặp giữa precheck và `ExecutionModeChoiceApplies`. Bằng chứng: `Check` dùng `domain.ExecutionModeChoiceApplies(a)`.
- [x] Mọi test precheck cũ xanh với kỳ vọng cũ. Bằng chứng: tám `TestStartPrecheck_*` cũ chỉ đổi chữ ký (thêm đối số `""`), kỳ vọng không đổi.

## Rủi ro và lưu ý

- Lời gọi `CheckReady` đi tới task-service trước transaction; lỗi tạm không được biến thành từ chối (đã có test, giữ).
- Chạy `gitnexus_impact` cho `ExecutionStartPrecheck.Check`.

## Chuẩn hoàn thành chung (áp dụng cho mọi task, không thương lượng)

- Mã chạy thật, **không stub, không TODO, không `panic` cho lỗi thường, không bỏ qua lỗi**; mọi nhánh lỗi có mã lỗi và test.
- Chạy `gitnexus_impact` trước khi sửa symbol có sẵn và báo blast radius; chạy `detect_changes` trước khi commit (CLAUDE.md).
- Hai dialect: mọi thay đổi lưu trữ có migration và test hợp đồng trên **Postgres 16 và MySQL 8** (testcontainers).
- Không file nào vượt `max-lines` (tách file khi cần, không thêm disable); không đặt tên `helpers`, `utils`, `common`, `misc`.
- Chạy được trên macOS, Linux, Windows và qua SSH; lệnh git theo baseline 2.25 (`guides/reference/git-compatibility.md`).
- Không ghi nội dung Request, Solution, phản hồi hoặc bí mật vào log, span hay khung sự kiện (C9).
- `go build ./...` và `go test` của gói đã đổi xanh; `go vet` xanh; lint xanh; không làm hỏng test hiện có.

## Ghi chú triển khai (2026-10-20)

- Mã: `usecase/approval_execution_precheck.go`: `Check(ctx, a, approverID, requested)`; chế độ hiệu lực = `requested` nếu khác rỗng, ngược lại `Modes.Mode`. `wire_execution.go` không đổi (chữ ký của `Check` không ảnh hưởng việc nối).
- Test thêm: `TestStartPrecheck_RequestedContinuousPassesForAnApproverWhoMayExecute`, `...RequestedModeIsIgnoredForApprovalsThatDoNotStartARun` (phase, solution, request_type, stage khác), `...RequestedContinuousFromSomeoneWhoMayNotDecideLeavesTheRefusalToTheDecision`; ca `requested=""` ở dự án `step_by_step` trả nil khi không có quyền (chứng tỏ `CheckReady` không được gọi).
- Lệnh: `go test ./internal/usecase/... -run StartPrecheck -v` (12 test xanh).
- Blast radius (grep): `ExecutionStartPrecheck.Check` chỉ có một chỗ gọi (`decide_approval.go`) và test của chính nó.
