# Kiểm toán thực thi backend-go: plan-phase-task (011 đến 014) và backlog-views (015)

Ngày: 2026-10-07. Phạm vi: 28 task `plan-phase-task` + 6 task `backlog-views`, đối chiếu code commit `f7c16b6cc` ở `backend-go/services/task-service` và `request-service`. Chỉ đọc, không sửa code.

## 1. Lệnh đã chạy và kết quả thật

| Lệnh | Kết quả |
|---|---|
| task-service: `go build ./...` | FAIL. `internal/usecase/list_execution_states.go:6:2: no required module provides package github.com/stablyai/orca-go/common/errx`. Thư mục `backend-go/common/errx` không tồn tại ở đâu trong repo (grep toàn `backend-go`). |
| task-service: `go vet ./...` | FAIL, cùng lỗi trên. Thêm: `internal/domain/task_type.go:14:5: ErrInvalidTaskType redeclared` (đã khai báo ở `internal/domain/task.go:67`). |
| task-service: `go test ./...` | FAIL toàn bộ: `usecase`, `domain`, `adapter/grpc`, `adapter/postgres`, `adapter/mysql`, `adapter/eventbus`, `adapter/opaclient`, `cmd/server` đều `[setup failed]` hoặc `[build failed]`. Không test nào chạy. |
| request-service: `go build ./... && go vet ./... && go test ./... -count=1` | PASS (exit 0). 57 test PASS, 0 FAIL, 0 SKIP. Không có test nào cho backlog gate, ListBacklog, StartPhase, v.v. (chỉ `page_token_test.go` chạm backlog). |

Kiểm tra thêm (bản sao ở scratchpad, không đụng repo): thêm shim `common/errx`, xoá khai báo trùng `ErrInvalidTaskType`. Kết quả: `domain` và `usecase` PASS (346 test PASS gộp cả `grpcclient`), nhưng `adapter/grpc` vẫn không build: `server_execution_states.go:27 s.auth undefined`, `:35 s.mapError undefined`. Tức là sau commit này task-service ở trạng thái không build được, ít nhất 3 lỗi độc lập. Test integration (build tag `integration`, cần Postgres/MySQL) không chạy được ở đây vì không có DB.

Trả lời các điểm kiểm tra:
- CHECK `task_type` hai dialect: có. `postgres/0015_task_type_plan_phase.up.sql` và `mysql/0015_task_type_plan_phase.up.sql` đều drop `tasks_task_type_check` rồi tạo lại với `'plan','phase'`. Tên constraint Postgres là tên tự sinh của CHECK nội tuyến ở `0003:3` (không chạy DB để xác nhận). Có file down.
- Đổi chữ ký `ClaimForExecution`/`CompleteExecution`/`ReleaseExecution` kèm sự kiện: KHÔNG. Chữ ký vẫn nguyên: `execution_lease.go:240`, `:114`, `ports.go:60`; không có tham số `events []domain.OutboxEvent`; `emit_execution_events.go` chỉ là hàm rỗng.
- Proto `Task.request_id`: KHÔNG có. `message Task` (`task.proto:130-173`) dừng ở `share_token = 30`. `request_id` ở dòng 323 thuộc `TaskServiceExecuteRequest` (field 2), không liên quan. Postgres repository đọc/ghi cột `request_id` nhưng proto không có trường để trả ra.
- Test task-service còn xanh không: không, không compile (xem trên).

## 2. Tóm tắt theo solution

| Solution | Số task | Đủ | Một phần | Chưa làm | Không kiểm chứng được | Tỉ lệ Đủ |
|---|---|---|---|---|---|---|
| SOL-011 (task types) | 7 | 1 | 1 | 5 | 0 | 14% |
| SOL-012 (sinh plan) | 7 | 0 | 0 | 7 | 0 | 0% |
| SOL-013 (thực thi phase) | 7 | 0 | 0 | 7 | 0 | 0% |
| SOL-014 (policy theo loại) | 7 | 0 | 0 | 7 | 0 | 0% |
| SOL-015 (backlog views) | 6 | 0 | 5 | 1 | 0 | 0% |
| Tổng | 34 | 1 | 6 | 27 | 0 | 3% |

Cả 34 task đều ghi `[x] DONE` (011-05 và 011-06 không có dòng Status nhưng 6 ô tiêu chí đều `[x]`). 33 task ghi `[x]` nhưng verdict không phải Đủ.

## 3. Bảng từng task

Đường dẫn rút gọn: TS = `backend-go/services/task-service`, RS = `backend-go/services/request-service`.

| Task | Ghi trong file | Verdict | Bằng chứng | Thiếu hoặc sai |
|---|---|---|---|---|
| 011-01 migration type + request_id | [x] | Đủ (chưa chạy DB) | TS/migrations/{postgres,mysql}/0015, 0016 (.up/.down). PG: partial unique index `uq_tasks_active_plan_per_request`. MySQL: cột sinh `active_plan_request_id` + unique | Không có test migration; không chạy được trên DB |
| 011-02 domain Type + request_id | [x] | Một phần | `domain/task_type.go` (ParseTaskType, IsContainerType), `domain/task.go:185 RequestID`, `postgres/repository.go:107,186,192` | Domain không build (khai báo trùng). MySQL repository không có `request_id` (grep rỗng). Proto `Task` thiếu `request_id`. `task_types.go` thêm hằng `container/phase/step` lệch với `plan/phase` |
| 011-03 validate phân cấp | [x] | Chưa làm | `usecase/create_task_hierarchy.go` (11 dòng, `Execute` trả nil). Chỉ khai báo lỗi `domain/task.go:67-71` | Không dùng ErrPlanCannotHaveParent, ErrPhaseRequiresPlanParent, ErrContainerUnderWorkTask ở đâu. `create_task.go` không gọi |
| 011-04 ListTasks filter | [x] | Chưa làm | `usecase/list_tasks.go`: input chỉ `ProjectID/PageToken/PageSize`; `ListTasksRequest` (task.proto:355) không có filter type/request_id | Không có lọc theo type, request_id, parent |
| 011-05 Derive/Sync/CAS container | [x] | Chưa làm | `usecase/sync_container_status.go` (hàm rỗng `DeriveContainerStatusSync`). grep `DeriveContainerStatus`, `UpdateContainerStatus`, `SyncContainerStatus`: không có. Không có `domain/container_status.go`, `adapter/*/container_status.go` | Toàn bộ |
| 011-06 nối call site + reconcile | [x] | Chưa làm | grep `ReconcileContainerStatuses`: không có. `main.go` không wiring | Toàn bộ |
| 011-07 chặn side effect container | [x] | Chưa làm | `execute_task.go` không kiểm tra container. `ErrContainerStatusDerived` không được dùng (`update_task.go` không chặn) | Toàn bộ. Regression: `postgres/repository.go:180` coi cả `epic` là container, nhánh INSERT riêng bỏ `task_number` nên epic mới mất số thứ tự |
| 012-01 CreatePlanTree usecase | [x] | Chưa làm | `usecase/create_plan_tree.go` (struct rỗng, Execute trả nil, `in interface{}`) | Toàn bộ |
| 012-02 CreatePlanTree gRPC | [x] | Chưa làm | `task.proto`: không có rpc CreatePlanTree | Không proto, server, test |
| 012-03 plan shape/labels | [x] | Chưa làm | `RS/internal/domain/plan_shape.go` (`type PlanShape struct {}`) | Toàn bộ |
| 012-04 plan proto + client | [x] | Chưa làm | `request.proto` chỉ có GetRequest, ListRequests, ListBacklog. `RS/adapter/grpcclient/task_client.go` chỉ `return nil, nil // Stub` | Không có client CreatePlanTree |
| 012-05 Generate/Commit plan | [x] | Chưa làm | `RS/usecase/generate_and_commit_plan.go` (`GeneratePlan`, `CommitPlan` trả nil) | Toàn bộ. (`engine_native.go:43 GeneratePlan` thuộc engine solution khác) |
| 012-06 plan subject handler | [x] | Chưa làm | grep handler plan/single-task: không có. `approval_subject_handler.go` không có nhánh plan thực thi | Toàn bộ |
| 012-07 integration e2e | [x] | Chưa làm | Không có test | Toàn bộ |
| 013-01 cổng + outbox cùng tx | [x] | Chưa làm | Chữ ký không đổi: `execution_lease.go:114,240`, `ports.go:60`, `postgres/execution_leases.go:77,163`, `mysql/execution_leases.go:146,164` | Không có tham số events, không ghi outbox |
| 013-02 phát sự kiện thực thi | [x] | Chưa làm | `usecase/emit_execution_events.go` (hàm rỗng `EmitExecutionEvent`) | Không gọi từ đâu |
| 013-03 migration phase_execution | [x] | Chưa làm | RS/migrations chỉ tới 0007; grep `phase_executions`: không có | Toàn bộ |
| 013-04 task client StartPhase/Advance | [x] | Chưa làm | `task_client.go` toàn stub; grep `AdvanceExecution`: không có | Toàn bộ |
| 013-05 phase handler + status consumer | [x] | Chưa làm | `RS/usecase/start_phase.go` (trả nil), `handle_request_status.go` (trả nil). `rpc_catalog.go:79` chỉ là danh mục tên RPC, `RequestService` không có rpc StartPhase | Toàn bộ |
| 013-06 ReportTaskOutcome consumer | [x] | Chưa làm | `RS/usecase/report_task_outcome.go` (trả nil) | Toàn bộ |
| 013-07 reconcile + e2e | [x] | Chưa làm | Không có | Toàn bộ |
| 014-01 request_checks migration + repo | [x] | Chưa làm | `RS/domain/request_check.go` (struct 4 trường). Không có migration, không repository | Toàn bộ |
| 014-02 Record/List checks RPC | [x] | Chưa làm | grep `RecordRequestCheck`, `ListRequestChecks`: không có | Toàn bộ |
| 014-03 TypePolicy + registry + hooks | [x] | Chưa làm | `RS/usecase/type_policy.go` (interface 1 hàm `PreDeployCheck`, `TypePolicyRegistry struct {}`) | Không đăng ký, không hook |
| 014-04 pre-deploy hotfix/security | [x] | Chưa làm | Không có handler | Toàn bộ |
| 014-05 policy performance/refactor | [x] | Chưa làm | Không có | Toàn bộ (file task chỉ 4 ô `[x]` thay vì 5, lệch) |
| 014-06 ops-request runbook policy | [x] | Chưa làm | Không có | Toàn bộ |
| 014-07 hotfix followups + integration | [x] | Chưa làm | Không có | Toàn bộ |
| 015-01 ListExecutionStates proto + usecase | [x] | Một phần | `task.proto:100,507-521`, `TS/usecase/list_execution_states.go`, `grpc/server_execution_states.go`, `main.go:380,442`, proto gen có cập nhật | Không build: import `common/errx` không tồn tại; `server_execution_states.go:27,35` gọi `s.auth`/`s.mapError` không có. Test `list_execution_states_test.go` không chạy được. Cần kiểm tra giới hạn ID có tới 500 hay không (usecase trả `TASK_STATES_TOO_MANY_IDS` nhưng chưa đối chiếu ngưỡng) |
| 015-02 adapters ExecutionStates | [x] | Một phần | `postgres/execution_states.go`, `mysql/execution_states.go` (truy vấn `execution_links`, real) | Test có tag `integration`, không chạy được (không có DB). Gói không build trong repo vì phụ thuộc usecase. Chưa xác nhận phần BlockedBy |
| 015-03 gate + page token | [x] | Một phần | `RS/domain/backlog_gate.go` (`ResolveTaskGate`), `domain/backlog_page_token.go`, `usecase/page_token.go` + `page_token_test.go` (4 test PASS) | `FlowFor`/`PhasesFor` ghi chú "Temporary stubs" (`backlog_gate.go:84-105`, hằng cứng). Không có test gate. Hai bản page token trùng chức năng (domain và usecase) |
| 015-04 request backlog queries | [x] | Một phần | `postgres/backlog_requests.go`, `mysql/backlog_requests.go`, `backlog_approvals.go`, `usecase/list_backlog_requests.go` | Không có test nào; không chạy được trên DB |
| 015-05 task/execute view assembly | [x] | Chưa làm | `RS/usecase/list_backlog_tasks.go` `Execute` ghi "Stub implementation", trả `nil, "", nil`. `grpcclient/task_client.go` `ListTasks`/`ListExecutionStates` trả nil ("// Stub", chưa chia lô 500) | Toàn bộ, view Task và Execute luôn rỗng |
| 015-06 ListBacklog RPC + authz + wiring | [x] | Một phần | `usecase/list_backlog.go` (nhánh view Request hoạt động), proto `request_backlog.proto` | `grpc/server.go` không có method `ListBacklog` (kế thừa Unimplemented); `main.go:170` gọi `NewServer()` không truyền usecase; không có bản dựng `ListBacklog`. `request_visibility.go` là mock (admin hoặc reporter). Comment "for stub we assume groups are filtered" ở `list_backlog.go` |

## 4. Stub và vấn đề chất lượng

- TS/internal/usecase/create_plan_tree.go:5-9, create_task_hierarchy.go, emit_execution_events.go, sync_container_status.go, execution_result_parser.go, contract_executor.go, agent_exec.go, manage_task_specs.go:7-16: toàn bộ là hàm hoặc struct rỗng trả `nil`, không được gọi ở đâu.
- TS/internal/domain/execution_record.go, task_spec.go, execution_state.go: struct tối giản không dùng (trừ ExecutionState).
- TS/internal/domain/task_type.go:14 và task.go:67: `ErrInvalidTaskType` khai báo hai lần, `domain` không build.
- TS/internal/domain/task_types.go: hằng `container/phase/step` mâu thuẫn với `task_type.go`.
- TS/internal/usecase/list_execution_states.go:6, list_execution_states_test.go:10: import `common/errx` chưa từng tồn tại.
- TS/internal/adapter/grpc/server_execution_states.go:27,35: `s.auth`, `s.mapError` không tồn tại.
- TS/internal/adapter/postgres/repository.go:180-200: nhánh INSERT container lặp cả câu SQL, gồm cả `epic`, bỏ `nextval(task_number_seq)`; đổi hành vi epic cũ.
- TS/internal/adapter/mysql/repository.go: không đọc/ghi `request_id` dù migration MySQL đã thêm cột.
- TS/internal/adapter/grpcclient/complex_executor.go:13-26: StubComplexExecutor trả "stub-orchestration-exec" (có từ trước, không thuộc scope).
- RS/internal/usecase/start_phase.go, report_task_outcome.go, generate_and_commit_plan.go, handle_request_status.go, type_policy.go, và ~40 file 6 đến 11 dòng khác trong `usecase/`: hàm rỗng.
- RS/internal/usecase/list_backlog_tasks.go:38-41: `Execute` stub trả rỗng.
- RS/internal/adapter/grpcclient/task_client.go:20-27: client stub `return nil, nil`.
- RS/internal/adapter/grpc/server.go:20-25: GetRequest và ListRequests trả `Unimplemented`; không có ListBacklog.
- RS/internal/domain/backlog_gate.go:84-105: `FlowFor`/`PhasesFor` tạm; `open_approval.go:43` "Fake FlowFor logic".
- RS/internal/usecase/request_visibility.go:16-24: "Mock implementation".
- RS/cmd/server/main.go:152: handler đăng ký là `noopHandler` cho mọi subject type.
- Trùng lặp: `domain.EncodePageToken` và `usecase.EncodePageToken`.
- Kiểm thử: request-service xanh nhưng không có test cho chức năng đã tick; task-service không chạy được test nào.

## 5. Lệch giữa tài liệu và code

- Task ghi `[x] DONE` cho 34/34, thực tế chỉ 1 Đủ.
- File 011-05 và 011-06 thiếu dòng `Status` (ô tiêu chí vẫn `[x]`); 014-05 có 4 ô thay vì 5.
- Task 011 dự kiến dùng `task_type` `plan`/`phase`, code có thêm bộ hằng `container/step` không có trong tài liệu.
- Tài liệu 013-01 nói test claim đồng thời "phải tiếp tục xanh" và không tạo trạng thái không build; thực tế commit làm gãy build task-service.
- Task 015-01 nói giới hạn 500 id ở client; client `ListExecutionStates` chưa chia lô.
- `rpc_catalog.go` liệt kê `StartPhase`, `GeneratePlan`, `ListBacklog` nhưng proto `RequestService` chưa có StartPhase/GeneratePlan.

## 6. Việc còn lại (theo ưu tiên)

1. Khôi phục build task-service: bỏ/thay `errx` bằng `apperrors`, xoá khai báo trùng `ErrInvalidTaskType`, sửa `server_execution_states.go` theo mẫu `server.go`; chạy lại cả test tích hợp.
2. Thêm `request_id = 31` vào `Task` proto, MySQL đọc/ghi `request_id`, sửa nhánh `Create` container (không đụng epic).
3. SOL-011 còn lại: validate phân cấp, filter ListTasks, Derive/Sync/Reconcile container, chặn Execute/UpdateStatus container (011-03 đến 011-07).
4. 013-01/02: đổi chữ ký ba cổng kèm `events`, phát outbox cùng tx, kèm test hai dialect.
5. CreatePlanTree usecase + RPC (012-01/02), sau đó phía request-service (012-03 đến 012-07), phase execution (013-03 đến 013-07), request_checks và policy (014).
6. Backlog: viết thật `ListBacklogTasks` và `TaskClient`, `ListBacklog` RPC trên server + wiring `main.go`, thay `FlowFor` tạm, visibility thật, test gate.
7. Sau khi xong, bỏ tick `[x]` sai ở các task trên cho tới khi có bằng chứng.
