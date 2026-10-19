# TASK-REQ-041-01: `AuthorizeRevise`: approver của cổng Analysis được chỉnh

**From Solution:** [BE-REQ-SOL-041](../solutions/BE-REQ-SOL-041-analysis-revision-by-prompt.md)
**Priority:** P1
**Service:** `request-service`
**File:** `internal/usecase/solution_authorization.go` (sửa), `internal/usecase/generate_solution.go` (sửa `prepare`), `internal/usecase/ports_solution.go` (sửa interface), mọi fake của `SolutionActorAuthorizer`, `cmd/server/wire_*.go` nơi dựng authorizer
**Depends on:** Không
**Status:** [x] DONE (2026-10-19)

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

- [x] Approver chỉnh được ở cả ba kiểu cổng; người không có quyền bị `REQUEST_SOLUTION_FORBIDDEN` và không đổi dữ liệu.
- [x] Approver không khởi động được lần phân tích đầu.
- [x] Kiểm quyền xảy ra trước khi Approval bị huỷ (test thứ tự).
- [x] Mọi fake biên dịch; test hiện có xanh.

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

---

## Ghi chú triển khai

**Ngày:** 2026-10-19.

### File đã đổi (`backend-go/services/request-service/`)

- `internal/usecase/ports_solution.go`: `SolutionActorAuthorizer` thêm `AuthorizeRevise`.
- `internal/usecase/solution_authorization.go`: `ReporterOrAdminAuthorizer.AuthorizeRevise` (= quyền reporter/admin/lead/owner); `ApproverAwareAuthorizer.AuthorizeRevise` lọc Approval `pending` theo ba kiểu `solution`, `findings`, `answer` (ba truy vấn `List`, vì `ApprovalListFilter` chỉ có một `SubjectType`, không có `IN`), `CanDecide` từng Approval; hàm dùng chung `orPendingApprover` (cũng phục vụ `AuthorizeChoose`); `NewApproverAwareAuthorizer` báo lỗi khi thiếu repository hoặc decider; `isApproverRefusal`.
- `internal/usecase/generate_solution.go`: `prepare` gọi `authorizeCaller` mới: `awaiting_analysis_approval` + `feedback != ""` thì `AuthorizeRevise`, còn lại `AuthorizeGenerate`. Lời gọi hệ thống không đổi. Kiểm quyền vẫn nằm trong `prepare`, trước `InTx` (và trước `CancelPending`).
- `internal/domain/solution_errors.go`: thông điệp `ErrSolutionForbidden` nêu cả approver (mã lỗi không đổi).
- `cmd/server/wire_solution.go`: dựng authorizer bằng `NewApproverAwareAuthorizer`, lỗi thì `wireSolution` trả lỗi (khởi động thất bại). `wire_plan.go` và `contracttest/solution_contract.go` vẫn dùng struct literal, vẫn biên dịch, không đổi.
- Test: `internal/usecase/solution_fakes_test.go` (`allowAll.AuthorizeRevise`), `internal/usecase/solution_revise_authorization_test.go` (mới), `cmd/server/wire_solution_test.go` (stores giả cho authorizer + test khởi động lỗi khi thiếu repository).

### Blast radius (grep, GitNexus không dùng được)

- `SolutionActorAuthorizer`: người dùng interface là `GenerateSolution` (`generate_solution.go`), `ChooseSolutionOption` (`choose_solution_option.go`), `NewPlanActorAuthorizer` (`plan_ports.go`, chỉ gọi `AuthorizeGenerate`). Cài đặt: `ReporterOrAdminAuthorizer`, `ApproverAwareAuthorizer`, fake `allowAll` (test). Dựng: `wire_solution.go`, `wire_plan.go`, `contracttest/solution_contract.go`.
- `GenerateSolution.prepare`: gọi từ `start`, và `start` từ `execute`/`PrepareForSystem`. Đường hệ thống bỏ qua kiểm quyền (có test).
- Rủi ro: thấp-vừa. Không đổi proto, schema, API.

### Lệnh đã chạy và kết quả

- `go build ./...` (trong `backend-go/services/request-service`): xanh.
- `go vet ./...` (cùng thư mục): xanh. `go test ./... -run XXX_NONE`: mọi gói biên dịch.
- `go test ./internal/usecase/ -run 'Authorize|Revise|Prepare' -v`: PASS (bảng 3 kiểu cổng x 9 người gọi; `TestPrepare_Revise_ApproverNotReporter_Succeeds` cho change_request, spike, question; `_ApproverStartingFirstAnalysis_Forbidden`; `_AuthorizeRunsBeforeCancelPending` (fake ghi thứ tự: `authorize_revise@outside` rồi `cancel_pending@tx`); `_InfraErrorNotMaskedAsForbidden`; lỗi hạ tầng ở list từng kiểu, ở snapshot approver và ở thư mục team).
- `go test ./internal/usecase/... ./cmd/...`: PASS toàn bộ.

### Sai khác và lưu ý

- `AuthorizeChoose` nay dùng chung `orPendingApprover`, nên lỗi hạ tầng của `CanDecide` (ví dụ thư mục team không khả dụng) được trả nguyên thay vì bị nuốt thành `Forbidden` như trước. Chỉ các lỗi `PermissionDenied`/`Unauthenticated` tính là "không có quyền". Đây là hệ quả của yêu cầu "không nuốt lỗi hạ tầng"; test cũ vẫn xanh.
- Dùng ba truy vấn `List` (không có `IN`); dừng ở lần khớp đầu tiên.
- Reporter vẫn được phép qua `Base`; ràng buộc tách nhiệm của `CanDecide` không áp lại cho người chỉnh vì họ đã qua `Base`; một approver đồng thời là requester của Approval (`SelfApprovalAllowed=false`) bị `CanDecide` từ chối nên rơi về `Forbidden` nếu không phải reporter/admin.

### Chưa kiểm chứng

- Chưa chạy test hợp đồng testcontainers hai dialect và e2e (không có thay đổi lưu trữ trong task này).
- `go test ./internal/domain/...` hiện đỏ ở `TestValidateCatalog_AllMethodsClassified` và `TestCatalogFieldsExistInRequestMessages` do `SetRequestBaseRef` chưa vào danh mục RPC: thuộc việc của task REQ-039/040 (agent khác), không liên quan task này.
- `golangci-lint` không chạy được sạch trong môi trường (lỗi typecheck từ `go-retryablehttp` ngoài phạm vi); `gofmt -l` và `go vet` sạch cho các file đã đổi.
