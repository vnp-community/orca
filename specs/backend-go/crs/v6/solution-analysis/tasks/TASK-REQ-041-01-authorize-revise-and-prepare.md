# TASK-REQ-041-01: `AuthorizeRevise`: approver của cổng Analysis được chỉnh

**From Solution:** [BE-REQ-SOL-041](../solutions/BE-REQ-SOL-041-analysis-revision-by-prompt.md)
**Priority:** P1
**Service:** `request-service`
**File:** `internal/usecase/solution_authorization.go` (sửa), `internal/usecase/generate_solution.go` (sửa `prepare`), `internal/usecase/ports_solution.go` (sửa interface), mọi fake của `SolutionActorAuthorizer`, `cmd/server/wire_*.go` nơi dựng authorizer
**Depends on:** Không
**Status:** [ ] TODO

---

## Context

- `prepare` luôn gọi `AuthorizeGenerate` (reporter hoặc `admin|lead|owner`).
- `ApproverAwareAuthorizer.AuthorizeChoose` là mẫu: liệt kê Approval `pending` rồi `Decider.CanDecide`.
- Cổng Analysis: `solution`, `findings`, `answer`.

## Việc cần làm

1. Thêm `AuthorizeRevise` vào `SolutionActorAuthorizer`; cập nhật `ReporterOrAdminAuthorizer` (= `AuthorizeGenerate`), `ApproverAwareAuthorizer` (SOL-041 mục 2.2) và mọi fake.
2. `ApproverAwareAuthorizer.AuthorizeRevise`: liệt kê Approval `pending` của Request cho từng `subject_type ∈ {solution, findings, answer}` (một truy vấn với `IN` nếu `ApprovalListFilter` hỗ trợ, nếu không ba truy vấn), `Decider.CanDecide` cho từng Approval, qua khi có một cái thành công; lỗi hạ tầng trả nguyên lỗi, không nuốt thành `Forbidden`.
3. `GenerateSolution.prepare`: nếu `!system && req.Status == awaiting_analysis_approval && in.Feedback != ""` thì `AuthorizeRevise`, ngược lại `AuthorizeGenerate`.
4. Nối trong wiring; cổng thiếu thì khởi động lỗi.
5. Cập nhật mô tả lỗi `ErrSolutionForbidden` để nêu cả approver ("only the reporter, an administrator or an approver of the pending decision may ...").

## Kiểm thử

- Bảng quyền: reporter, admin, lead, owner, approver theo user, theo team, theo role, người chỉ đọc, người lạ; × ba kiểu cổng (`solution`, `findings`, `answer`); × (có/không `feedback`, `analyzing`/`awaiting_analysis_approval`).
- `TestPrepare_Revise_ApproverNotReporter_Succeeds`, `_ApproverStartingFirstAnalysis_Forbidden`, `_AuthorizeRunsBeforeCancelPending` (fake ghi thứ tự), `_InfraErrorNotMaskedAsForbidden`.
- `go test ./services/request-service/internal/usecase/... -run 'Authorize|Revise|Prepare' -v`.

## Tiêu chí hoàn thành

- [ ] Approver chỉnh được ở cả ba kiểu cổng; người không có quyền bị `REQUEST_SOLUTION_FORBIDDEN` và không đổi dữ liệu.
- [ ] Approver không khởi động được lần phân tích đầu.
- [ ] Kiểm quyền xảy ra trước khi Approval bị huỷ (test thứ tự).
- [ ] Mọi fake biên dịch; test hiện có xanh.

## Rủi ro và lưu ý

- `CanDecide` có thể áp ràng buộc tách nhiệm; không áp lại cho bước chỉnh (reporter đã được phép từ `Base`).
- Chạy `gitnexus_impact` cho `GenerateSolution.prepare` và `SolutionActorAuthorizer`.

## Chuẩn hoàn thành chung (áp dụng cho mọi task, không thương lượng)

- Mã chạy thật, **không stub, không TODO, không `panic` cho lỗi thường, không bỏ qua lỗi**; mọi nhánh lỗi có mã lỗi và test.
- Chạy `gitnexus_impact` trước khi sửa symbol có sẵn và báo blast radius; chạy `detect_changes` trước khi commit (CLAUDE.md).
- Hai dialect: mọi thay đổi lưu trữ có migration và test hợp đồng trên **Postgres 16 và MySQL 8** (testcontainers).
- Không file nào vượt `max-lines` (tách file khi cần, không thêm disable); không đặt tên `helpers`, `utils`, `common`, `misc`.
- Chạy được trên macOS, Linux, Windows và qua SSH; lệnh git theo baseline 2.25 (`guides/reference/git-compatibility.md`).
- Không ghi nội dung Request, Solution, phản hồi hoặc bí mật vào log, span hay khung sự kiện (C9).
- `go build ./...` và `go test` của gói đã đổi xanh; `go vet` xanh; lint xanh; không làm hỏng test hiện có.
