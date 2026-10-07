# TASK-REQ-025-04: Kịch bản e2e E02 đến E19 (các luồng, chéo, quyền) và test ma trận loại

**From Solution:** BE-REQ-SOL-025
**Priority:** P0
**Service:** `request-service`
**File:** `backend-go/services/request-service/e2e/request_flow_test.go`, `.../e2e/scenarios_flows.go` (mới), `.../e2e/scenarios_cross_cutting.go` (mới), `.../e2e/type_matrix_test.go` (mới)
**Depends on:** TASK-REQ-025-03; CR-REQ-006 (backlog, đổi loại), 010 (quyền), 014 (loại đặc thù), BE-REQ-SOL-024 task 02 (audit)
**Status:** `[x] DONE`

---

## Context

- Bảng E01 đến E20 ở CR-REQ-025 mục 2.3 (đủ: E01 đến E03; rút gọn: E04 đến E09; hotfix E10; spike, question E11, E12; chéo E13 đến E20). E17 (MCP) và E18 (Jira) cần stack thật nên nằm ở T2 (task 07), không ở T1; E20 ở task 05.
- `FlowFor(type)` ở CR-REQ-003 (`internal/domain/...`, tên file chốt khi triển khai) là nguồn bảng luồng; mỗi Scenario của task 03 khai `Expect` đối chiếu.
- Callback `ReportTaskOutcome` giao lặp và đảo thứ tự: E16 (idempotent, CR-REQ-013).

## Việc cần làm

1. `scenarios_flows.go`: E02 (`bug` size L có Phase), E03 (`refactor` size L), E04 (`bug` size S không Phase), E05 (`task` có `task_list`), E06 (`docs`), E07 (`performance`, Chẩn đoán baseline rồi Plan), E08 (`security`, xác nhận mức độ bởi người và `pre_deploy`), E09 (`ops_request` runbook, rollback, `pre_deploy`), E10 (`hotfix`, người xác nhận loại, một task, `pre_deploy`, Request theo dõi `followup_hotfix`), E11 (`spike`, Findings, Request con `spawned_by_spike`, không Task), E12 (`question`, Answer, không worktree).
2. `scenarios_cross_cutting.go`: E13 (từ chối Plan thành `request_backlog` `returned_from_stage=plan`, mở lại, phân loại lại), E14 (`bug` đổi sang `change_request` giữa chừng; Chẩn đoán, Task giữ làm tham chiếu; `request_type_history` đủ), E15 (task lỗi thì backlog `returned_from_stage=task`), E16 (callback lặp, đảo thứ tự), E19 (người không đủ quyền nhận `REQUEST_APPROVAL_NOT_APPROVER`, audit `denied`).
3. Mỗi kịch bản khẳng định đủ: chuỗi trạng thái, `subject_type` các Approval, `Solution.kind`, cây Plan/Phase/Task, sự kiện outbox (tên subject), dòng audit.
4. `type_matrix_test.go`: `TestEveryRequestTypeHasScenario`: lấy 11 loại từ registry (`AllTypes()` hoặc tên tương đương của CR-REQ-003), khẳng định mỗi loại xuất hiện trong ít nhất một Scenario thành công; thêm loại thứ 12 mà thiếu kịch bản thì đỏ.
5. Mỗi nhóm luồng (đủ, rút gọn, hotfix, spike hoặc question) có ít nhất một kịch bản xanh (test đếm theo nhóm).

## Kiểm thử

- `E2E_DIALECT=postgres go test -tags=e2e ./e2e/...` và `mysql`.
- Kiểm riêng hai dialect nằm trong bộ kịch bản: khoá lặp `request_idempotency` (tạo hai lần cùng `client_request_id`), hai `Approve` đồng thời (một thành công, một `REQUEST_APPROVAL_ALREADY_DECIDED`), claim outbox `SKIP LOCKED` hai worker, thứ tự phân trang `ListRequests`.

## Tiêu chí hoàn thành

- [x] E02 đến E16 và E19 xanh trên hai dialect.
- [x] `TestEveryRequestTypeHasScenario` xanh và đỏ khi giả lập thêm loại.
- [x] Mỗi nhóm luồng có kịch bản xanh.

## Rủi ro và lưu ý

- `performance`, `ops_request`, `spike`, `question` cần năng lực agent chưa có: T1 chỉ kiểm khung luồng với fake, ghi rõ trong tên test để không ai hiểu nhầm là kiểm chất lượng.
- File dài: tách theo nhóm (không dùng `max-lines` disable).
