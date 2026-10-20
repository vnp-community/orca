# TASK-REQ-043-05: Test hợp đồng hai dialect và luồng nhiều Phase sau một lần duyệt

**From Solution:** [BE-REQ-SOL-043](../solutions/BE-REQ-SOL-043-approve-plan-and-run-all-steps.md)
**Priority:** P1
**Service:** `request-service`
**File:** `internal/adapter/contracttest/approval_flow_contract.go` (sửa), `internal/adapter/contracttest/execution_flow_contract.go` (sửa), `internal/usecase/continuous_execution_test.go` (sửa)
**Depends on:** TASK-REQ-043-02, TASK-REQ-043-03
**Status:** [x] DONE (2026-10-20)

---

## Context

- `contracttest` chạy một bộ ca trên Postgres 16 và MySQL 8 (testcontainers). `approval_flow_contract.go` có hàm `rowsOf(rig.env, t, tenantID, "orca.request.approval.decided")` (dòng 217, 265, 341...) để đọc hàng outbox.
- `execution_flow_contract.go:575-607` có `pinContinuous` mô phỏng cái `StartExecution` ghim; luồng nhiều Phase với fake task service đã có (`TestContinuous_ChangeRequest_RunsEveryPhaseAfterOneApproval` ở mức usecase, `continuous_execution_test.go:69`).
- Mục tiêu của task này là chứng minh những điều mà fake không chứng minh được: ghim và quyết định **cùng commit** trên DB thật, và cuộn lại khi handler lỗi.

## Việc cần làm

1. Trong `approval_flow_contract.go`, thêm ca: `DecideApproval` thật (repository thật) duyệt Approval Plan với `ExecutionMode=continuous`: sau commit, `requests.execution_mode == 'continuous'`, Request ở `executing`, đúng một hàng outbox `approval.decided` có `execution_mode="continuous"` và không có khoá đó ở ca duyệt thường.
2. Ca cuộn lại: bơm lỗi vào subject handler (decorator của registry trong rig) và kiểm cột `execution_mode` vẫn `NULL`, Approval vẫn `pending`, không có hàng outbox.
3. Ca ghim sẵn `step_by_step` (gọi `PinExecutionModeIfUnset` trước) rồi duyệt với `continuous`: `REQUEST_EXECUTION_MODE_ALREADY_PINNED`, Approval vẫn `pending`.
4. Ca hai goroutine duyệt cùng Approval (một kèm chế độ, một không): đúng một thắng, cột `execution_mode` khớp người thắng.
5. Trong `execution_flow_contract.go`: kịch bản `change_request` 3 Phase, duyệt Plan bằng `DecideApproval` kèm `continuous` (thay `pinContinuous`), rồi chạy `StartExecution`/`EvaluateExecution` với fake task service: không có Approval `phase` nào được mở, Phase đầu có `phase_starts.start_trigger = auto` do người duyệt, hoàn tất lần lượt hết ba Phase mà không gọi `StartPhase` thủ công.
6. `continuous_execution_test.go`: ca usecase tương ứng với fake (nhanh) để giữ phản hồi sớm khi không có Docker.
7. Cập nhật danh sách cột kỳ vọng nếu có (`expected_columns_*.go`): không có cột mới, chỉ xác nhận không cần đổi.

## Kiểm thử

- `go test ./services/request-service/internal/adapter/contracttest/... -run 'Approval|Execution' -v` trên cả hai dialect (cần Docker); `go test ./services/request-service/internal/usecase/... -run Continuous -v`.

## Tiêu chí hoàn thành

- [x] Ghim cùng commit với quyết định và cuộn lại cùng nhau, chứng minh trên Postgres 16 và MySQL 8. Bằng chứng: `TestPostgres_ApprovalFlowContract` và `TestMySQL_ApprovalFlowContract`, các ca `RunAll*`/`ApproveWithoutMode*` xanh trên cả hai.
- [x] Hàng outbox `approval.decided` có `execution_mode` đúng điều kiện (điều acceptance qua WS không kiểm được). Bằng chứng: `RunAllPinsTheModeInTheDecisionsTransaction` (có khoá) và `ApproveWithoutModeLeavesTheModeUnpinned` (vắng khoá).
- [x] Luồng ba Phase chạy hết sau một lần duyệt, không `StartPhase` thủ công, không Approval `phase`. Bằng chứng: `ExecutionFlow_RunAll_OneDecisionPinsAndRunsEveryPhase` xanh trên Postgres và MySQL; ca usecase nhanh `TestContinuous_ApprovingThePlanWithTheChoiceRunsEveryPhase`.

## Rủi ro và lưu ý

- Bơm lỗi vào handler cần điểm mở trong rig; nếu rig không cho thì thêm decorator trong file test, không đổi mã sản phẩm.
- Chạy MySQL trong CI chậm; giữ số ca cần thiết.

## Chuẩn hoàn thành chung (áp dụng cho mọi task, không thương lượng)

- Mã chạy thật, **không stub, không TODO, không `panic` cho lỗi thường, không bỏ qua lỗi**; mọi nhánh lỗi có mã lỗi và test.
- Chạy `gitnexus_impact` trước khi sửa symbol có sẵn và báo blast radius; chạy `detect_changes` trước khi commit (CLAUDE.md).
- Hai dialect: mọi thay đổi lưu trữ có migration và test hợp đồng trên **Postgres 16 và MySQL 8** (testcontainers).
- Không file nào vượt `max-lines` (tách file khi cần, không thêm disable); không đặt tên `helpers`, `utils`, `common`, `misc`.
- Chạy được trên macOS, Linux, Windows và qua SSH; lệnh git theo baseline 2.25 (`guides/reference/git-compatibility.md`).
- Không ghi nội dung Request, Solution, phản hồi hoặc bí mật vào log, span hay khung sự kiện (C9).
- `go build ./...` và `go test` của gói đã đổi xanh; `go vet` xanh; lint xanh; không làm hỏng test hiện có.

## Ghi chú triển khai (2026-10-20)

- File: `contracttest/approval_flow_run_all_contract.go` (mới, 5 ca: ghim cùng commit và nhấp đúp, không chọn, handler lỗi cuộn lại, đã ghim `step_by_step`, đua hai quyết định 12 vòng) được đăng ký trong `approval_flow_contract.go`; `contracttest/execution_flow_run_all_contract.go` (mới, `flowRunAllFromTheDecision`: `DecideApproval` thật + `TransitionSubjectHandler` + `StartExecution`, ba Phase, không `pinContinuous`, không Approval `phase`, `phase_starts.start_trigger = auto` do người duyệt) đăng ký trong `execution_flow_contract.go`; `usecase/continuous_execution_test.go` thêm ca fake.
- Ngoài danh sách file của task: `ExecutionEnv` thêm trường `Locker` (`contracttest/execution_repository_contract.go`) và hai file nối môi trường `adapter/{postgres,mysql}/execution_integration_test.go` đặt `Locker: NewRequestLocker(base)`, vì `DecideApproval` cần khoá Request; không có thay đổi mã sản phẩm. `expected_columns*.go`: không có cột mới nên không đổi (file đó đang được luồng khác sửa).
- Lệnh: `go test -tags=integration -p 1 ./internal/adapter/postgres/ ./internal/adapter/mysql/ -run 'Execution|Approval' -count=1` xanh; `go test ./internal/usecase/... -run Continuous -v` xanh.
- Sai khác: ca nhiều Phase kiểm `StartedBy` bằng người duyệt thật ở tầng contract (DB thật) nhưng ở ca fake chỉ kiểm `trigger=auto` vì `exActors` của rig cố định `approver-1`.
- Không kiểm chứng: chạy trên stack (task 06 bỏ qua).
