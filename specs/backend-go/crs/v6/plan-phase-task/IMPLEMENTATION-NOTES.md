# Ghi chú triển khai: plan-phase-task (đợt T1: BE-REQ-SOL-011, `task-service`)

Ngày: 2026-10-07. Phạm vi: TASK-REQ-011-01 đến 07. Các solution 012 đến 014 chưa triển khai (đợt sau); mục dưới đây bổ sung khi họ làm.

## Kết quả kiểm chứng

- `task-service`: `go build ./...`, `go vet ./...` (cả `-tags integration`), `go test ./...` xanh; `gofmt -l` sạch.
- Integration chạy thật: Postgres `postgres:16-alpine` (68 PASS trong lần chạy đầy đủ, 3 lỗi chỉ vì container không khởi động kịp do máy tải, chạy lại riêng đều PASS) và MySQL `mysql:8.0.46` (82 PASS trong lần chạy đầy đủ, 1 lỗi khởi động container, chạy lại PASS). Migration 0015/0016 up, `down 2`, up sạch trên cả hai.
- Build, vet, test xanh ở các module dùng task proto: `proto`, `api-gateway`, `orchestration-service`, `workflow-service`, `project-service`, `mcp-service`.

## Quyết định lệch so với task

1. **Sửa build trước** (3 lỗi độc lập của commit `f7c16b6cc`): `list_execution_states.go` và test dùng `common/apperrors` thay `common/errx`; xoá khai báo trùng `ErrInvalidTaskType` ở `task_type.go`; `server_execution_states.go` dùng `tenant.RequireTenantID` và `apperrors.ToGRPCStatus` như `server.go`; thêm `usecase.ExecutionStateReader` vào `repoAll` ở `main.go`; test gRPC dùng fake riêng thay `usecase.FakeExecutionStateReader` (fake nằm trong file `_test` của package khác); hai test integration `execution_states_test.go` dùng sai tên (`Type`, `EdgeDependsOn`) nên sửa thành `Kind`, `EdgeKindDependsOn`. Các RPC `ListExecutionStates` (thuộc 013) giữ nguyên, chỉ cho biên dịch được.
2. **Xoá stub của 012**: `usecase/create_plan_tree.go` (`CreatePlanTree`, hàm rỗng, không ai dùng), `usecase/create_task_hierarchy.go` (stub của 011-03, thay bằng `validateTaskHierarchy` trong `create_task.go`), `usecase/sync_container_status.go` bản cũ (`DeriveContainerStatusSync` rỗng, thay bằng bản thật). Đã grep: không có người gọi.
3. **Xoá `domain/task_types.go`** (hằng `container/phase/step` không dùng, mâu thuẫn tài liệu).
4. **`IsContainerType` chỉ là `plan|phase`** (bản kiểm toán tính cả `epic`, làm epic mất `task_number`: đã sửa, có test `TestRepository_Create_ContainerHasNoTaskNumber` kiểm epic vẫn có số).
5. **Postgres `GetAncestors` có danh sách cột riêng**, thiếu `request_id`; `subtreeColumnNames` và `prefixedTaskColumns` cũng thiếu: đã bổ sung (lỗi chỉ lộ khi chạy Postgres thật).
6. **Tiến độ trong `SyncContainerStatus`** gọi `RecalculateProgress` một lần ở container trên cùng sau khi leo hết chuỗi, thay vì mỗi cấp (kết quả giống, ít truy vấn hơn).
7. **`SyncOne(containerID)`** thêm vào `SyncContainerStatus` để `ReconcileContainerStatuses` dùng (task nói "hoặc hàm `SyncOne` mới"); cổng đối soát có thêm `TouchContainer` để "chạm" `updated_at` khi không đổi.
8. **`RecoverInterruptedExecutions.Sweep`** (Execute + đối soát) là hàm mới, `RunRecoveryLoop` gọi nó; `Execute` giữ nguyên chữ ký.
9. `UpdateTask` cũng sync sau khi mở khoá task phụ thuộc (`UpdateStatus(open)` của dependents), không chỉ sau `repo.Update`.
10. `ListTasks`: kiểu native (`uuid`) nên chuỗi không phải UUID ở `project_id`/`parent_id`/`request_ids`/`page_token` giờ trả `TASK_LIST_FAILED` thay vì danh sách rỗng (cách so `::text` cũ).

## Điều chưa kiểm chứng

- TiDB (generated column `STORED` với `CASE`, `FOR UPDATE SKIP LOCKED`); chỉ MySQL 8.0.46 đã chạy.
- `EXPLAIN` của truy vấn theo `request_ids` (không bắt buộc).
- `ExecuteTask` end-to-end với repository thật (các điểm gọi sync được kiểm bằng fake CAS; vòng đời lá-phase-plan được kiểm bằng repository thật qua `UpdateStatus`/`CompleteExecution`/`Sync`).
- `TestAddEdge_ConcurrentCycleRace_ExactlyOneSucceeds` (có từ trước, không thuộc 011) lỗi một lần ("2 successes") khi máy tải nặng rồi PASS 4/4 lần chạy lại: cửa sổ đua của cycle check vẫn mở khi chưa có cạnh nào để khoá. Không do thay đổi này (chỉ thêm một `Get` sau khi ghi cạnh), nhưng chưa điều tra sâu.
- `DeleteTask` một con của phase không kích hoạt sync (ngoài phạm vi task); đối soát không bắt được vì xoá không bump `updated_at` của ai. Nên thêm khi làm CR-REQ-006/013.

## Giới hạn đã biết của đối soát

Container chỉ được xét lại khi `updated_at` nhỏ hơn `updated_at` mới nhất của con. `UpdateTask` ghi lại toàn hàng (kể cả `status`) từ bản đọc trước: một lần sửa tiêu đề container đua với ghi trạng thái suy ra có thể đè trạng thái rồi bump `updated_at`, che lỗi cho tới khi con đổi lần kế. Không ảnh hưởng tới plan/phase mà người dùng không sửa.

## Cho đợt sau (012 đến 015)

- `request-service` gọi `CreateTask` với `task_type=plan|phase`, `request_id` (gRPC); `ListTasks` mặc định ẩn plan/phase, cần `task_types=[plan]`/`[phase]` rõ ràng và tối đa 100 `request_ids`.
- BE-REQ-SOL-013: `ClaimForExecution`/`CompleteExecution`/`ReleaseExecution` giữ chữ ký; chỗ gọi `syncContainerParent` cạnh các lời gọi này ở `execute_task.go`/`report_execution_result.go`, tránh sửa trùng khối khi thêm sự kiện cho task lá.
- Gateway (CR-REQ-016) hiện chỉ chuyển `title/parent_id/project_id/creator_id` cho `CreateTask` và `project_id/page_token/page_size` cho `ListTasks`; chưa chuyển `task_type`, `request_id`, `task_types`, `request_ids`, `parent_id`.

## Câu hỏi mở còn lại (từ solution)

- Container `done` mở lại khi thêm con mới: hiện cho phép và log Warn (`TestSync_ReopensDoneContainerWhenNewChildUnfinished`).
- Huỷ Plan không cascade `cancelled` xuống con ở `task-service`.
- Chưa có trần `page_size` cho `ListTasks`.


---

# Đợt 2, phần `task-service` của SOL-012 và SOL-013 (TASK-REQ-012-01, 012-02, 013-01, 013-02)

Ngày: 2026-10-08. Nhánh `rf/task-a`. Migration: không cần (dải 0017..0019 chưa dùng). Chi tiết quyết định nằm ở mục "Ghi chú triển khai" trong từng file task.

## Kết quả kiểm chứng

- `task-service`: `go build ./...`, `go vet ./...` (cả `-tags integration`), `go test ./...` xanh; `gofmt -l` sạch.
- Integration thật Postgres 16 (toàn bộ gói `adapter/postgres`, PASS 497s) và MySQL 8 (toàn bộ gói `adapter/mysql`, PASS 2470s).
- Test mới: usecase `create_plan_tree_test.go` (17), `task_run_events_test.go` (12), `server_plan_tree_test.go` (3), integration mỗi dialect 11 test.

## Quyết định lệch so với task

1. `ErrTxConflict` + retry tx trong `CreatePlanTree` (deadlock MySQL ở chỉ mục duy nhất khi 8 goroutine cùng request: thấy thật 7/8 lỗi trước khi sửa).
2. `UpdateContainerStatus` tham gia tx của `RunInTx` (trước đó mở tx riêng).
3. Bỏ `plan_task_type` khỏi proto; không hỗ trợ `single_task` qua RPC này.
4. Revert trước khi có link phát event best-effort, không nguyên tử (xem 013-02).
5. Test bắt được lỗi thật: Phase bị `normalizeProposalType` ép thành `task`; đã sửa.

## Chưa kiểm chứng / lưu ý

- `task.proto` đã sửa song song bởi agent khác: cần hợp nhất tay, sinh lại bằng `buf generate --path orca/task`. Mã sinh trong worktree này chỉ là bản cục bộ.
- Consumer `api-gateway`, `notification-service` chưa chạy lại; `selectEngine` coi task có `depends_on` là phức tạp (Engine 2) nên task trong Plan có phụ thuộc đi orchestration (hành vi có sẵn, cần quyết định ở CR-013-04).
- Phase/Plan mới tạo ở `open`; chưa gọi `SyncContainerStatus` sau `CreatePlanTree` (task `blocked` chưa phản ánh lên Phase tới lần sync kế).
- Chu kỳ đối soát request-service (013-07) chưa làm.


---

# Đợt 3, phần request-service (exec): CR-REQ-013 (03 đến 07), CR-REQ-014, CR-REQ-015 (03 đến 06)

Ngày: 2026-10-08. Nhánh `rf/exec` (tách trước khi có Solution thật). Migration `0050_phase_execution`, `0051_request_checks`. Không sửa `.proto`, không sửa module dùng chung (`common/*`, `proto/*`, `task-service`): chỉ `backend-go/services/request-service` và tài liệu. CR-REQ-012 phần request-service (GeneratePlan, CommitPlan, PlanGenerator) KHÔNG làm đợt này.

## Kết quả kiểm chứng (2026-10-08)

- `request-service`: `gofmt -l`, `go build`, `go vet` (cả `-tags integration`), `go test ./...` PASS.
- Integration thật: `go test -tags integration ./internal/adapter/postgres ./internal/adapter/mysql ./internal/adapter/eventbus ./cmd/server` PASS với `postgres:16-alpine` (vai trò ứng dụng NOSUPERUSER NOBYPASSRLS), `mysql:8.0`, `nats:2.10-alpine`. Gồm `RunExecutionRepositoryContract` (24 kịch bản mỗi dialect), `RunExecutionFlowContract` (7 kịch bản E2E mỗi dialect: repository thật, `grpcclient.TaskClient` thật nói chuyện với task-service giả qua bufconn, use case thật), migration 0050/0051 up/down/up, RLS (chính sách `relay_scan` chỉ SELECT), NATS (consumer bền TASK, REQUEST).
- `task-service`: `ListExecutionStates` (015-01/02) chạy lại: unit, gRPC, integration Postgres+MySQL PASS.
- Test `TestCreate_ClaimLoser_ReturnsWinner` (có từ trước, intake) báo race khi chạy `-race` toàn gói `usecase`; không do thay đổi này. Test của đợt này chạy `-race` sạch.

## Kiến trúc đã làm

- `EvaluateExecution.Run(req)` là hàm trạng thái dùng chung ("evaluateExecutionState"): đọc `ListTasks` một lần, tự đặt `done` cho task `review` (cờ), trả backlog khi hết lần thử / quá cửa sổ dispatch / mọi task của container bị huỷ, ghi `phase_done` và phát `phase.completed` đúng một lần, mở Approval Phase kế (chỉ khi flow có cổng `phase`), hoàn tất qua `CompletionChecks` rồi `execution_finished`, rồi `OnCompleted`, hoặc `AdvanceExecution`. Consumer outcome, `StartExecution`, `ResumeAfterGate` và `ReconcileExecutingRequests` đều gọi nó, nên mất sự kiện chỉ tốn độ trễ.
- `ReportTaskOutcome`: giao dịch ngắn (`MarkProcessed` + ghi `task_run_outcomes` + outbox `phase.completed`), phản ứng sau commit; phản ứng lỗi chỉ log (sự kiện đã tiêu thụ), đối soát chạy lại sau `REQUEST_RECONCILE_QUIET`.
- Consumer bền: `request-service-task-outcome` (TASK `orca.task.task.statuschanged`), `request-service-status-start` (`status_changed` `to=executing`), `request-service-execution-gate` (`approval.decided` đã duyệt `pre_deploy`).
- `TaskClient` thật (`grpcclient/task_client.go`): theo `next_page_token`, chia `request_ids` 100 và `task_ids` 500, ánh xạ mã `TASK_EXECUTE_*` sang lỗi domain, chuyển tenant và người thực thi qua metadata; `UnavailableTaskClient` khi thiếu `TASK_SERVICE_ADDR`. Thay stub cũ.
- `TaskExecutionGuard` thật cho `ReturnToBacklog`/`CancelRequest` (fail closed khi task-service lỗi).
- `PhaseArtifacts`, `PreDeployArtifacts` gắn vào `approvalSubjectArtifacts` (hotfix/security/ops_request); phase/pre_deploy qua `TransitionSubjectHandler` có sẵn (từ chối: `ReturnToBacklog` stage theo `ApprovalReturnStage`, category `rejected`).
- 015: `ListBacklog` THẬT (ba view), `ResolveTaskGate` viết lại theo `FlowFor` (bản cũ sai: nhiều nhánh chết), `ListByStatus` keyset khai triển, mọi truy vấn đi qua `scoped` (bản cũ dùng `exec` trần nên RLS ẩn hết hàng), `ListGateApprovals` có thêm `decided_by`/`decided_at`.

## Quyết định lệch so với task

1. Bảng đối soát dùng thêm `execution_reconcile_state` (lease + `last_run_at`) thay `FOR UPDATE SKIP LOCKED`/`GET_LOCK`: cùng SQL hai dialect, không giữ giao dịch qua lời gọi gRPC. Quét xuyên tenant qua chính sách `relay_scan` (Postgres) giống sweeper Approval.
2. `task_run_outcomes` thêm cột `once` + khoá duy nhất `(tenant_id, task_id, once)` để Phase/Plan hoàn tất đúng một lần, và `event_id` duy nhất (task nói không duy nhất): giao lặp không đếm lỗi hai lần.
3. `phase.started` không có trường `dispatched` (phát cùng giao dịch `phase_starts`, trước khi dispatch); danh sách task nằm ở phản hồi `StartPhase`.
4. Lỗi dispatch tạm thời ghi `task_run_outcomes` `failed` với `cause=dispatch_error` (không tính lần thử); cửa sổ 15 phút tính từ lần lỗi đầu kể từ lần `started` gần nhất.
5. Trả backlog khi mọi task của Plan (không Phase) bị huỷ dùng `stage=task`, vì `StageForStatus(executing)` không cho `plan`. `OnRejected` của `phase`/`pre_deploy` dùng `ReturnRequestToBacklog` có sẵn (không phải trigger `plan_rejected`), kết quả cùng trạng thái.
6. Chống agent tự khai: `RecordRequestCheck` không có `source` trong payload (suy từ `actor_type`: agent hoặc manual; `orca_verified` chỉ dành cho kiểm tra do Orca đo, CR-REQ-029); `performance`/`refactor` bỏ qua `status` agent gửi và Orca tự tính từ số liệu; `security_recheck`/`ops_result` do agent khai `passed` chưa đủ (verdict `missing` tới khi người ghi), `failed` luôn được tin. Lệch nhẹ so với CR (agent qua MCP được ghi); đổi ở `humanConfirmedVerdict` nếu chủ CR muốn khác.
7. `ListRequestChecks`: proto không có cờ `effective`; trả cũ→mới, bản cuối mỗi `kind` là bản hiệu lực (domain có `MarkEffective`).
8. Follow-up hotfix tạo Request với người báo cáo là người báo cáo của hotfix (cột `reporter_id` là UUID, actor hệ thống không có); khoá idempotency `hotfix-followup:<request>:<bug|task>`. Lỗi vĩnh viễn log và bỏ, lỗi tạm thời trả về nhưng không làm Request `completed` thụt trạng thái; nếu chết giữa chừng, không có đường tự thử lại (Request đã `completed`): tạo tay bằng `SpawnChildRequest` cùng `client_request_id`.
9. Quyền xem Request (015-06): `MemberRequestVisibility` theo CR-REQ-035 2.3 tạm thời (admin, người báo cáo, thành viên project qua `ListMembers` của project-service; thiếu `PROJECT_SERVICE_ADDR` thì chỉ báo cáo viên và admin). Lọc xảy ra trước khi gọi task-service; thiếu chính sách là lỗi, không phải "thấy hết". Thay bằng bộ kiểm quyền của CR-REQ-035 khi hợp nhất.
10. `StartPhase` kiểm quyền bằng `ApprovalStartPhaseAuthorizer`: báo cáo viên, admin, hoặc người có thể duyệt Approval Phase.
11. RPC nội bộ `ReportTaskOutcome` được bảo vệ bằng `common/internalcaller.Guard(SERVICE_INTERNAL_TOKEN)` (token trống thì từ chối hết).
12. `TaskClient` không có `ListExecutionRecords` (chưa dùng; CR-REQ-029 thêm).
13. Không tạo `request_execution.proto`/`request_check.proto`/`request_backlog.proto` mới: proto của `plan.proto`, `request_backlog.proto` đã có.

## Chỗ cần nối khi hợp nhất vào `feat/request-flow-backend` (đã có Solution, Approval thật)

- `cmd/server/main.go`: (a) trước `wireRequestLifecycle` thêm `execTasks, err := dialExecutionTasks(cfg, log)`; (b) `wireRequestLifecycle(stores, registry, execTasks.guard)` (thêm tham số); (c) `wireApproval(..., dirs, execTasks.subjectArtifacts())` (thêm tham số cuối; `approvalSubjectArtifacts(extra...)` gộp map: rf-sol phải đưa artifacts Solution/Plan vào cùng map, không ghi đè phase/pre_deploy); (d) sau `approval.BindConfirm` gọi `wireExecution(...)` và `.WithExecution(execution.Server)` trên server; (e) `grpc.ChainUnaryInterceptor(internalcaller.Guard(...))`.
- `cmd/server/store_wiring.go`: trường `execution executionStores` và hai dòng khởi tạo; `wire_approval.go`: trường `Open`, `Authorizer` trong `approvalWiring`.
- `internal/adapter/grpc/server.go`: trường `execution ExecutionUseCases` (file `server_execution.go` có `WithExecution`).
- `usecase/ports.go`: `BacklogRequestFilter.RequestID`, `BacklogRequestReader.ListByStatus`. `domain/request_check.go` và `domain/backlog_gate.go` được viết lại; xoá stub `usecase/type_policy.go`, `verify_execution.go` (CR-REQ-029 viết `VerifyExecution` thật).
- Khi CR-REQ-012 hợp nhất: gọi `PolicyFor(type).PlanPreconditions` ở `GeneratePlan`/`CommitPlan`; `CommitPlan` mở `pre_deploy` theo `FlowDefinition.StartGate`; ghi nhãn `gate:pre_deploy`/`rollback`/`check:*` theo hằng `domain.PolicyLabel*`; `ExecutionStarter` dùng `Request.PlanTaskID` nếu muốn thay `ListTasks`.
- Migration `0050`, `0051` nằm trong dải 0050..0059 đã cấp.

## Điều chưa kiểm chứng

- Task-service thật: E2E dùng task-service giả (bufconn); cây Plan thật, `ExecuteTask` thật, giả định một worktree gắn nhiều task (project-service) và danh tính service-to-service chưa chạy.
- Hiệu năng/`EXPLAIN` của quét `ListQuietExecuting` và keyset backlog; TiDB; MySQL < 8.0.16 (CHECK bị bỏ qua).
- Giao dịch chứa lời gọi gRPC: `PhaseArtifacts.Describe` / `PreDeployArtifacts.Describe` chạy trong giao dịch mở Approval (giữ khoá Request) và gọi task-service một lần; `TaskExecutionGuard` gọi task-service trong giao dịch trả backlog. Task-service chậm làm giữ khoá lâu (đã có timeout 30 giây).
- Thông báo hàng loạt từ `UpdateTask(done)` tự động (câu hỏi mở CR-013).
- Hai replica thật cùng NATS; chỉ kiểm lease bằng goroutine trên một DB.

## Câu hỏi mở

- `REQUEST_AUTO_COMPLETE_TASKS` mặc định bật; `MAX_PARALLEL_TASKS>1` chỉ đúng khi project-service chấp nhận một worktree cho nhiều task (hiện task thứ hai chờ sự kiện `started` của task đầu).
- Bảng cổng 015 mục 2.4 vẫn chờ chủ CR xác nhận; task `review` (cờ tắt) không nằm trong EXECUTE; ops_request: cổng `pre_deploy` từng task chưa hiện ở `gate_status` của nhóm.
- Có cần cờ cấu hình tắt follow-up hotfix (`REQUEST_HOTFIX_FOLLOWUPS`)?

### Hợp nhất vào feat/request-flow-backend (2026-10-08)
- Hợp nhất sau rf/sec: không cần `internalcaller.Guard` riêng cho `ReportTaskOutcome`/`RecordRequestCheck` (catalog sec đánh dấu `internal`; xác nhận bằng `cmd/server` integration). `wireApproval` nhận cả `owned` (rf/sol) lẫn `artifacts` (rf/exec); `approvalWiring` giữ cả `open` và `Open/Authorizer`.
- Trùng tên đã xử lý: `domain.FollowUp` (exec) giữ nguyên, của rf/sol đổi `FindingFollowUp`; `usecase.ApprovalOpener` (exec, `Execute`) giữ nguyên, của rf/sol đổi `SolutionApprovalOpener` (`Open`); `containsString` trùng thì dùng một bản.
- Migration không trùng số: exec 0050/0051 (đứng trước 0060 của art, vẫn áp được lên DB mới). Trùng tên policy `relay_scan` trên `requests` giữa exec 0050 và roll 0092: policy của roll đổi thành `relay_metrics_scan`. Test migration định vị theo tên tệp (`MigrationScriptIndex`), không đếm từ cuối.
- Cột văn bản mới của exec đã vào catalog xoá: `task_run_outcomes.error_message`, `request_checks.summary/metrics` (erasable); các cột enum/mã nguyên nhân/lease owner exempt.
- Còn lại: `listBacklog` của exec dùng `where` gom biến; thêm `backlog_requests.go:ListByStatus` vào allow-list quét tenant (WHERE luôn bắt đầu bằng tenant_id). RPC có thực thể con chưa đăng ký resolver authz (impact, plan_task, phase, task...) bị từ chối với non-admin cho tới khi wave tương ứng đăng ký ở `wire_entity_resolvers.go`.
- Test e2e (`-tags e2e`) còn 16 kịch bản `02_human_confirms_type` + `TestE20_FlagLifecycle` + `TestMetricsEndpointServesRequestSeries` + 2 test phủ bao fail; cùng tập fail với HEAD trước khi hợp nhất rf/sec (`awaiting_information` sau khi xác nhận loại vì thiếu type_fields từ rf/art), không do sec/exec.
