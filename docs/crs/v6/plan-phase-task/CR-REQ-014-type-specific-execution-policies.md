# CR-REQ-014 — Chính sách thực thi theo loại: hotfix, security, performance, ops_request, refactor

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-REQ-014 |
| **Tên** | Cổng `pre_deploy`, đo baseline và đo lại, runbook có rollback, cổng "test cũ vẫn xanh", kiểm lại bảo mật, Request theo dõi sau hotfix |
| **Loại** | Feature |
| **Priority** | 🟠 P1 |
| **Effort** | Large |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-05 |
| **Trạng thái** | 📝 Đề xuất, chưa triển khai |
| **Phụ thuộc** | CR-REQ-013 (điểm móc `TypePolicy`), CR-REQ-012 (nhãn, `PlanPreconditions`), CR-REQ-009 (Approval `pre_deploy`), CR-REQ-003 (registry, `StartGate`), CR-REQ-006 (`SpawnChildRequest`, `ReturnToBacklog`), CR-REQ-008 (Chẩn đoán, nơi đo baseline) |
| **Mở khoá** | CR-REQ-025 (kiểm thử đầu cuối) |
| **Tác động** | `backend-go/services/request-service` (domain, usecase, repository hai dialect, migration, proto); không đổi `task-service` |

## 1. Bối cảnh và vấn đề

README v6 mục 3.4 cho năm loại cần thêm bước ngoài đường chuẩn: `hotfix`, `security`, `ops_request` có `pre_deploy`; `performance` đo baseline và đo lại; `ops_request` cần runbook kèm rollback; `refactor` đóng khi test cũ vẫn xanh; `hotfix` phải sinh Bug/Chore theo dõi. Hiện:

- Orca không có khái niệm "deploy" ở `task-service`: task chỉ chạy agent (`ExecuteTask`), kết quả là `review` hoặc lỗi. Không có chỗ nào ghi số đo, kết quả test hay "đã kiểm lại".
- `task-service` không lưu đầu ra có cấu trúc: `LastExecutionOutput` cắt ở 8KB dạng chuỗi (`UpdateLastExecutionOutput`), và `ReportTaskExecutionResult` chỉ nhận `success`, `actual_hours`, `error_message`. Backend không thể tự suy "benchmark cải thiện 20%" hay "số test không đổi" từ kết quả task.
- Quy ước repo: không thêm lệnh git mới (README v6 mục 6), nên backend không thể kiểm bằng `git diff` rằng file test không bị sửa.
- CR-REQ-003 đã đặt `StartGate` (hotfix, security chiếm `awaiting_plan_approval` bằng Approval `pre_deploy`) và `ExecutionGates` (`ops_request` chặn trong `executing`). CR-REQ-013 đã định nghĩa cổng móc `TypePolicy` (mặc định rỗng). CR này cung cấp các hiện thực.

## 2. Giải pháp đề xuất

### 2.1 Cổng móc `TypePolicy` (`internal/domain/type_policy.go`, mới)

```
type TypePolicy interface {
  PlanPreconditions(req Request, p PlanProposal) error                 // gọi bởi CR-REQ-012 (PROPOSE và COMMIT)
  PreExecutionGate(req Request, task TaskRef) (*GateRequirement, error) // gọi bởi AdvanceExecution (CR-REQ-013)
  CompletionChecks(req Request, checks []RequestCheck) ([]CheckVerdict, error) // gọi trước execution_finished
  OnCompleted(req Request) ([]FollowUp, error)                          // sau request.completed
}
```

Registry `PolicyFor(RequestType) TypePolicy` (`type_policy_registry.go`, mới); loại không có chính sách dùng `noopPolicy`. Mỗi loại một file: `type_policy_hotfix.go`, `_security.go`, `_performance.go`, `_ops_request.go`, `_refactor.go`. `GateRequirement{SubjectType, SubjectID, Stage}`; `CheckVerdict{Kind, Stage, Status(passed|failed|missing), Summary}`.

### 2.2 Cổng `pre_deploy` (Approval do CR-REQ-009; ở đây định nghĩa khi nào mở, cho ai, và `SubjectHandler`)

| Loại | Vị trí | `subject_id` | Ai kích hoạt |
|---|---|---|---|
| `hotfix` | `StartGate` (chiếm `awaiting_plan_approval`) | id task fix duy nhất | CR-REQ-012 (mục 2.6) gọi `OpenApproval`; `approval.decided` đã duyệt kích `plan_approved` (CR-REQ-003). Người duyệt bắt buộc là người (CR-REQ-010) |
| `security` | `StartGate` | id Plan | `GeneratePlan` COMMIT mở `pre_deploy` thay `plan` |
| `ops_request` | trong `executing` | id từng task nhãn `gate:pre_deploy` | `PreExecutionGate` trả `GateRequirement` cho task có nhãn; `AdvanceExecution` gọi `OpenApproval` (idempotent: một `pending` mỗi chủ thể, CR-REQ-009) và dừng task đó |

`SubjectHandler` `pre_deploy` (`type_policy_pre_deploy_handler.go`, mới; CR-REQ-009 giao cho CR này): `ValidateForRequest` kiểm chủ thể đúng bảng trên (task hoặc Plan thuộc Request) và lấy `digest`; `OnApproved`: `hotfix`, `security` thì `TransitionRequest(plan_approved)`, `ops_request` thì không đổi trạng thái (consumer `approval.decided` của CR-REQ-013 kích `AdvanceExecution`); `OnRejected`: `hotfix`, `security` thì `plan_rejected` (CR-REQ-003); `ops_request` thì `ReturnToBacklog(stage=task, category=rejected)`.

`PreExecutionGate` chỉ trả yêu cầu khi chưa có Approval `pre_deploy` `approved` cho `subject_id`. Task bị chặn giữ `open`, Request vẫn `executing`; hiển thị bằng Approval `pending` ở hộp duyệt (CR-REQ-022) và view Execute backlog (CR-REQ-015). Bị từ chối (`rejected`): `ReturnToBacklog(stage=task, reason=comment)` qua CR-REQ-013. Từ chối `pre_deploy` ở `StartGate` do CR-REQ-003.

### 2.3 Bảng `request_checks` (migration kế tiếp của `request-service`, hai dialect)

Nơi ghi số đo và kết quả kiểm có cấu trúc, append-only:

| Cột | Postgres | MySQL | Ràng buộc |
|---|---|---|---|
| `id` | UUID | CHAR(36) | PK, ứng dụng sinh |
| `tenant_id` | UUID | CHAR(36) | NOT NULL |
| `request_id` | UUID | CHAR(36) | NOT NULL, không FK |
| `kind` | TEXT | VARCHAR(30) | CHECK IN (`perf_baseline`, `perf_after`, `tests_before`, `tests_after`, `security_recheck`, `ops_result`) |
| `status` | TEXT | VARCHAR(10) | CHECK IN (`passed`,`failed`) |
| `metrics` | JSONB | JSON | NOT NULL DEFAULT `{}` |
| `summary` | TEXT | TEXT | NOT NULL DEFAULT '' |
| `source` | TEXT | VARCHAR(10) | CHECK IN (`agent`,`manual`) |
| `task_id` | UUID NULL | CHAR(36) NULL | task đã sinh ra số đo, nếu có |
| `recorded_by` | UUID NULL | CHAR(36) NULL | NULL khi `source=agent` không có user |
| `created_at` | TIMESTAMPTZ | TIMESTAMP(6) | NOT NULL, đồng hồ DB |

Chỉ mục `(tenant_id, request_id, kind, created_at DESC)`. Postgres bật RLS `tenant_isolation`; MySQL kiểm `tenant_id` ở mọi `WHERE`. Bản ghi mới nhất theo `(request_id, kind)` là bản có hiệu lực.

RPC thêm vào `RequestService` (README 3.6 chưa liệt kê, xem Q1): `RecordRequestCheck{request_id, kind, status, metrics_json, summary, task_id}` và `ListRequestChecks{request_id}`. `RecordRequestCheck` chỉ nhận khi Request ở trạng thái hợp lệ cho `kind` (bảng 2.4 đến 2.7), lỗi `REQUEST_CHECK_NOT_ALLOWED_NOW`. Người gọi là user (`manual`) hoặc agent qua tool MCP `request_record_check` (CR-REQ-017; chưa kiểm chứng agent gọi được tool trong lúc chạy task).

### 2.4 `performance`: baseline và đo lại

- **Baseline** ở bước Chẩn đoán (CR-REQ-008 chỉ chừa trường `measurements[]` trong schema, việc chạy đo thuộc CR này): `kind=perf_baseline`, `metrics = {"metrics":[{"name","unit","direction":"lower_is_better|higher_is_better","baseline":number,"target_change_percent":number}], "method":"..."}`. `PlanPreconditions`: thiếu `perf_baseline` `passed` thì `REQUEST_PERF_BASELINE_MISSING`; Plan bắt buộc có task nhãn `check:baseline` đầu và `check:after` cuối (`REQUEST_PLAN_PERF_CHECK_TASKS_MISSING`).
- **Đo lại**: `kind=perf_after`, mỗi `name` trong baseline phải xuất hiện kèm `value`.
- **`CompletionChecks`**: tính `change% = (value - baseline) / baseline * 100`, đảo dấu theo `direction`. Mỗi metric đạt khi `improvement% >= target_change_percent`. Thiếu `perf_after` hoặc thiếu metric: `missing` (chờ, chưa trả backlog). Có metric không đạt: `failed`, CR-REQ-013 gọi `ReturnToBacklog(stage=task, reason="Hiệu năng chưa đạt: <name> <improvement>% < <target>%")`. `baseline = 0` thì metric đó `failed` với tóm tắt "baseline bằng 0".

### 2.5 `ops_request`: runbook, rollback, kết quả

- `PlanPreconditions` (kiểm đề xuất): (a) mọi `TaskProposal.irreversible=true` có `labels` chứa `gate:pre_deploy` sau khi lưu; (b) có ít nhất một task nhãn `rollback`; (c) mỗi bước `irreversible` có một task `rollback` đứng sau nó theo `depends_on` hoặc `description` của nó khai `rollback_note` không rỗng. Lỗi: `REQUEST_RUNBOOK_ROLLBACK_MISSING`, `REQUEST_RUNBOOK_IRREVERSIBLE_STEP_UNGATED`.
- Thực thi: cổng `pre_deploy` mục 2.2 trước mỗi bước không đảo ngược. Rollback không tự chạy khi bước lỗi (rủi ro hơn lợi ích); Request đi backlog stage `task` kèm tên task `rollback` liên quan trong lý do để người quyết định.
- "Ghi kết quả": `CompletionChecks` yêu cầu `kind=ops_result` (`summary` không rỗng), người hoặc agent ghi.

### 2.6 `refactor`: test cũ vẫn xanh

- Plan có task nhãn `check:tests_before` đầu và `check:tests_after` cuối (`PlanPreconditions`: `REQUEST_PLAN_TEST_CHECK_TASKS_MISSING`).
- `tests_before`: `metrics = {"total":n,"passed":n,"failed":n,"command":"..."}`. `tests_after`: thêm `"tests_modified":bool`.
- `CompletionChecks`: đạt khi `tests_after.failed = 0`, `tests_after.total >= tests_before.total`, `tests_modified = false`. `tests_modified` là lời khai của agent, backend không xác minh (không có lệnh git mới); ghi rõ ở tóm tắt để người xem. Không đạt thì `ReturnToBacklog(stage=task)`.

### 2.7 `security`: kiểm lại

`kind=security_recheck` (`status=passed`) bắt buộc trước hoàn tất ("Kiểm lại, ghi nhận"); `summary` ghi phạm vi đã kiểm. `failed` thì `ReturnToBacklog(stage=task)`. Việc xác nhận mức độ là Approval `request_type` (CR-REQ-005), không ở đây.

### 2.8 `hotfix`: Request theo dõi

`OnCompleted` trả hai `FollowUp`: Request `bug` ("Nguyên nhân gốc và test hồi quy cho <title>") và Request `task` ("Review sau hotfix: <title>"). Gọi `SpawnChildRequest` (CR-REQ-006) với `link_reason=followup_hotfix` (cha `hotfix` ở `executing` hoặc `completed`, `type_hint` `bug` hoặc `task`, đúng bảng của CR-REQ-006), `client_request_id = hotfix-followup:<request_id>:<bug|task>`, nên chạy lặp không sinh trùng (`created=false`). Hai Request mới vào `classifying` và đi đủ phân loại, người xác nhận loại. CR-REQ-006 dựng khoá idempotency từ `actor` (`'user:<actor>'`); gọi từ hệ thống chưa có người nên cần CR-REQ-006 chấp nhận actor hệ thống (Q5).

### 2.9 Mã lỗi (FailedPrecondition trừ khi ghi khác)

`REQUEST_PRE_DEPLOY_REQUIRED` (chỉ dùng nội bộ khi `StartPhase` được gọi tay qua Phase có task bị cổng), `REQUEST_PERF_BASELINE_MISSING`, `REQUEST_PLAN_PERF_CHECK_TASKS_MISSING`, `REQUEST_PLAN_TEST_CHECK_TASKS_MISSING`, `REQUEST_RUNBOOK_ROLLBACK_MISSING`, `REQUEST_RUNBOOK_IRREVERSIBLE_STEP_UNGATED`, `REQUEST_CHECK_NOT_ALLOWED_NOW`, `REQUEST_CHECK_INVALID_METRICS` (InvalidArgument).

## 3. Quyết định thiết kế

| Quyết định | Lý do |
|---|---|
| Hiện thực qua `TypePolicy` móc vào CR-012, 013, không rẽ nhánh theo loại trong use case chung | Thêm hoặc đổi chính sách không chạm đường chuẩn; test từng loại độc lập |
| Số đo và kết quả kiểm ở bảng `request_checks`, không lấy từ đầu ra task | `task-service` không có đầu ra có cấu trúc; 8KB chuỗi không đủ tin cậy |
| Gate `pre_deploy` gắn bằng nhãn `gate:pre_deploy` | `Task.Labels` có sẵn, không cần cột mới ở `task-service` |
| Không tự rollback | Rollback tự động trên bước không đảo ngược có thể làm hỏng thêm; để người quyết định |
| `tests_modified` do agent khai | Không thêm lệnh git (README v6 mục 6); nói rõ giới hạn thay vì giả vờ xác minh |
| Cổng thất bại đi backlog, không thêm loại Approval | README 3.5 chỉ có tám `subject_type`, không có loại cho kết quả đo |
| Follow-up hotfix tạo hai Request qua `SpawnChildRequest` | Dùng đường chung của CR-REQ-006, idempotent theo `client_request_id` |

## 4. Tiêu chí chấp nhận

- [ ] `hotfix`: Approval `pre_deploy` trên task fix duy nhất kích hoạt `executing`; người không có quyền duyệt (CR-REQ-010) không duyệt được; từ chối đi backlog.
- [ ] `security`: Plan được `pre_deploy` duyệt mới chạy; thiếu `security_recheck` `passed` thì Request không `completed`.
- [ ] `ops_request`: task nhãn `gate:pre_deploy` không `Execute` cho đến khi có Approval `approved`; task không nhãn chạy bình thường; Approval được tạo đúng một lần dù `AdvanceExecution` chạy lặp.
- [ ] `ops_request`: đề xuất thiếu rollback hoặc bước không đảo ngược thiếu cổng bị đúng mã lỗi ở 2.5; `ops_result` thiếu thì không hoàn tất.
- [ ] `performance`: thiếu `perf_baseline` thì không `GeneratePlan`; có `perf_after` đạt thì `completed`; một metric chưa đạt thì Request vào backlog với lý do nêu tên metric; baseline 0 không chia cho 0.
- [ ] `refactor`: `tests_after.failed > 0`, `total` giảm hoặc `tests_modified=true` đều không hoàn tất; đạt cả ba thì `completed`.
- [ ] `hotfix` hoàn tất sinh đúng hai Request theo dõi (`bug`, `task`) có `request_links` `followup_hotfix`; chạy `OnCompleted` hai lần vẫn hai Request.
- [ ] `RecordRequestCheck` từ chối khi Request ở trạng thái không phù hợp; bản ghi mới nhất theo `(request_id, kind)` là bản có hiệu lực; không có đường sửa hay xoá bản ghi.
- [ ] Loại khác (`bug`, `task`, `docs`, `change_request`) chạy y như trước khi có CR này (`noopPolicy`).
- [ ] Migration `request_checks` up/down sạch trên Postgres và MySQL; mọi CHECK từ chối giá trị sai.

## 5. Kiểm thử

- **Unit:** mỗi `TypePolicy` table-driven (`PlanPreconditions`, `PreExecutionGate`, `CompletionChecks`, `OnCompleted`); công thức cải thiện cho hai `direction`, baseline 0, thiếu metric; quy tắc `tests_after`; sinh `GateRequirement` theo nhãn.
- **Integration, cả hai dialect:** repository `request_checks` (append-only, bản mới nhất, tenant isolation); `OnCompleted` idempotent qua `client_request_id`; luồng `AdvanceExecution` với `PreExecutionGate` chặn rồi mở.
- **Hợp đồng:** schema `request_checks` đọc từ `information_schema` ở hai DB; JSON `metrics` mẫu cho từng `kind`.
- **E2E (CR-REQ-025):** một Request mỗi loại đi hết luồng; agent giả ghi số đo qua RPC.
- Chưa chạy test nào ở thời điểm viết CR.

## 6. Rủi ro và điểm chưa kiểm chứng

- Chưa kiểm chứng agent có thể gọi `RecordRequestCheck` (qua MCP) từ trong một task chạy bằng Dev Server Agent; nếu không, số đo chỉ vào được bằng tay (`source=manual`) và các loại `performance`, `refactor` thành nửa tự động.
- Số đo do agent khai không được xác minh; một agent sai hoặc bị lừa có thể báo xanh. Cổng duyệt của người (Approval `plan`) là lớp bảo vệ duy nhất.
- `pre_deploy` là cổng duyệt, không phải cổng kỹ thuật: Orca không chặn agent chạy lệnh deploy trong một task không có nhãn. Việc gắn nhãn đúng phụ thuộc chất lượng đề xuất AI và người duyệt Plan.
- Nhãn `Task.Labels` thay thế toàn bộ khi `UpdateTask` (`UpdateTaskInput.Labels`), người sửa nhãn tay có thể xoá `gate:pre_deploy`. Chưa có khoá nhãn.
- Ngưỡng `target_change_percent` do AI/Chẩn đoán đề xuất, có thể phi thực tế; người duyệt Plan/Solution chịu trách nhiệm.
- Mặc định hai Request theo dõi sau hotfix có thể thừa; chưa có dữ liệu dùng thực.

## 7. Câu hỏi mở

- **Q1.** README v6 mục 3.6 chưa có RPC `RecordRequestCheck`/`ListRequestChecks`, và mục 3.5 chưa có bảng `request_checks`. Cần bổ sung vào hợp đồng.
- **Q2.** `pre_deploy` của `hotfix` chiếm `awaiting_plan_approval` nhưng task chưa chạy lúc đó, trong khi tên gọi "trước deploy" gợi ý sau khi sửa xong. CR theo CR-REQ-003 (duyệt trước khi chạy task fix). Xác nhận ý nghĩa.
- **Q3.** Bug size L, `refactor` có nên có cổng kiểm riêng như `performance`? Hiện `bug` không có kiểm hoàn tất ngoài task test hồi quy (CR-REQ-012).
- **Q4.** `ops_request` thất bại giữa runbook: có cần RPC cho người kích rollback từ UI (task `rollback` chạy thủ công)? CR chưa thêm.
- **Q5.** `SpawnChildRequest` dựng khoá idempotency theo `user:<actor>`; lời gọi `OnCompleted` do hệ thống, cần quy ước actor (ví dụ `system:request-service`) ở CR-REQ-006.

## 8. Tham chiếu

- `/opt/repos/orca/docs/crs/v6/README.md` mục 3.4, 3.5, 3.6
- `/opt/repos/orca/docs/crs/v6/plan-phase-task/CR-REQ-012-plan-phase-task-generation-from-solution.md`, `CR-REQ-013-phase-execution-and-feedback-loop.md`
- `/opt/repos/orca/docs/crs/v6/request-lifecycle/CR-REQ-003-request-state-machine-and-flow-registry.md`
- `/opt/repos/orca/docs/crs/v6/request-lifecycle/CR-REQ-006-return-to-backlog-reopen-cancel-child-requests.md`, `/opt/repos/orca/docs/crs/v6/approval/CR-REQ-009-generic-approval-domain-and-api.md`, `/opt/repos/orca/docs/crs/v6/solution-analysis/CR-REQ-008-diagnosis-findings-answer-analysis.md`
- `/opt/repos/orca/docs/research/receive-request/request-classification-and-flows.md` mục 2, 4
- `/opt/repos/orca/backend-go/services/task-service/internal/domain/task.go` (`Labels`, `LastExecutionOutput`), `usecase/update_task.go` (`Labels` thay toàn bộ), `usecase/report_execution_result.go`
- `/opt/repos/orca/backend-go/services/task-service/migrations/postgres/0012_task_sources.up.sql` (mẫu RLS), `0011_task_widened_fields.up.sql` (`labels`)
- Mới: `request-service/internal/domain/type_policy.go`, `type_policy_registry.go`, `type_policy_hotfix.go`, `type_policy_security.go`, `type_policy_performance.go`, `type_policy_ops_request.go`, `type_policy_refactor.go`, `request_check.go`; `internal/usecase/record_request_check.go`; migration `request_checks` hai dialect
