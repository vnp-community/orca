# TASK-V2-025: Danh tính thực thi khi người duyệt khác người commit Plan; xem lại kết quả chạy của task; dev server "bận"

**From Solution:** (lỗi tiềm ẩn #1 và #2 của [TASK-V2-024](TASK-V2-024-execution-path-identity-and-engine-selection.md); F3 và F4 của [TASK-FE-E2E-011](../../../../../frontend/bugs/e2e-acceptance-v1/tasks/TASK-FE-E2E-011-agent-session-monitoring-in-ui.md)) · **Resolves:** `REQUEST_EXECUTE_FORBIDDEN` khi người duyệt khác người commit, `EXECUTION-RESULT-NOT-VISIBLE` (tab Result ẩn), `DEVSERVER-NO-BUSY-STATE`
**Priority:** P1 (Q1), P2 (Q2, Q3) · **Service/Area:** request-service, task-service, api-gateway, infra-fleet-service, proto
**Files:**
- Q1 request-service: `internal/usecase/{execution_access.go (mới),execution_actors.go,start_phase.go,ports_execution.go}`, `internal/domain/backlog_gate.go` (`TaskView.OwnerID`), `internal/adapter/grpcclient/task_client.go` (`CanExecute`, `GrantExecute`), `cmd/server/wire_execution.go`
- Q1 test: `internal/usecase/{execution_access_test.go (mới),execution_fakes_test.go}`, `internal/adapter/grpcclient/task_access_test.go (mới)`, `internal/adapter/grpc/server_execution_test.go`, `internal/adapter/contracttest/{execution_fake_task_grants.go (mới),execution_fake_task_service.go,execution_flow_contract.go}`, `e2e/{kit_test.go,flow_driver_test.go}`
- Q2 proto: `backend-go/proto/orca/task/v1/task.proto` (+ `task.pb.go`, `task_grpc.pb.go`): RPC `GetTaskExecutionOutput`
- Q2 task-service: `internal/usecase/get_task_execution_output.go (+ test)`, `internal/adapter/grpc/{server.go,server_execution_record.go}`, `cmd/server/main.go`, `internal/domain/stored_output.go (+ test)`, `internal/adapter/{postgres,mysql}/repository.go` + `last_execution_output_test.go` (tích hợp)
- Q2 api-gateway: `internal/adapter/wscompat/{channels_task_execution.go (+ test),channels_automation_task.go}`, `testdata/channel_inventory.json`, `internal/adapter/mcpserver/tools/excluded_channels.yaml`
- Q3 proto: `backend-go/proto/orca/infrafleet/v1/infrafleet.proto` (+ `infrafleet.pb.go`): `DevServer.active_executions`
- Q3 infra-fleet: `internal/adapter/devserveragent/{session.go,client.go,exec_timeouts.go,active_executions_test.go (mới)}`, `internal/usecase/{ports.go,record_dev_server_handshake.go,broadcast_dev_server_status.go}` (+ test), `internal/domain/dev_server_status_event.go`, `internal/adapter/eventbus/dev_server_status_publisher.go`, `internal/adapter/grpc/{server.go,server_capability.go,server_activity_test.go}`, `cmd/server/main.go`
- Q3 api-gateway: `internal/adapter/wscompat/{channels.go,channels_devserver_status.go,channels_test.go,channels_agent_test.go}`
- Tài liệu: `docs/crs/v6/plan-phase-task/CR-REQ-013-phase-execution-and-feedback-loop.md` (mục 2.4 bước 6, mục 6), `specs/backend-go/crs/v6/gateway-and-mcp/CONTRACT-request-ui-api.md` (mục 2.8, 2.9)
**Depends on:** TASK-V2-024
**Status:** `[x]` XONG và đã kiểm trên stack thật (2026-10-10): tab Result có "Agent output", dev server `busy`/`healthy`, thực thi khi người duyệt là admin (chưa kiểm bằng hai người dùng khác nhau thật).

---

## Q1. Người duyệt hoặc bắt đầu khác người commit Plan

### Gốc rễ (đã tái hiện)

`CommitPlan` truyền `CreatorID = người gọi`, nên chỉ người đó là chủ mọi task của cây và có grant `owner` (`apply_tree`). `ApprovalExecutionActors.ActorFor` chọn người bắt đầu Phase hoặc người duyệt Plan. Khi khác người, mọi lời gọi `Execute`, `UpdateTask`, `GetSubtree` thay mặt họ bị task-service từ chối: `REQUEST_EXECUTE_FORBIDDEN`. Lượt UI live (một admin làm hết) không lộ.

Tái hiện: kịch bản hợp đồng mới `ExecutionFlow_ApproverAndStarterAreNotTheCommitter` (A commit, B duyệt, C và D bắt đầu hai Phase; không ai được cấp quyền sẵn) **đỏ** `REQUEST_EXECUTE_FORBIDDEN` khi tắt sửa, xanh khi bật; fake task-service giờ lọc quyền thật (chủ hoặc grant, `apply_tree` kế thừa, bảng quyền của `task_grant.rego`), kể cả `Execute` và `UpdateTask` (trước đó chỉ `GetSubtree` lọc).

### Nghiệp vụ theo CR (đã đọc CR-REQ-013 mục 2.4 bước 6 và mục 6)

Người được thực thi là người request-service đã cho phép: người duyệt Plan (qua chính sách duyệt), người bắt đầu Phase (qua `StartPhaseAuthorizer`: chủ Request, admin hoặc người duyệt Phase), người báo cáo (chủ Request) khi chưa ai duyệt. CR ghi quyền `execute` là "Grant kế thừa theo cây từ người tạo Plan" nhưng không có chỗ nào cấp grant đó; mục 6 ghi nhận rủi ro và để mở.

### Quyết định và lý do

Chọn: **cấp grant mức `user`, `apply_tree`, trên gốc cây của Request, dưới danh nghĩa chủ của gốc cây, ngay trước khi dùng một danh tính** (`ExecutionAccess.Ensure`), chỉ khi `ResolvePermission(execute)` từ chối. Không chọn "thực thi bằng danh tính người commit":
- mất dấu người thật trong audit của task-service (mọi lần chạy đều ghi tên người commit), và `StartedBy` trong `phase_starts` thành vô nghĩa;
- người commit có thể đã nghỉ hoặc đổi quyền; còn grant `user` trên cây của Request thì thu hồi được bằng `RevokeGrant` hiện có.

Mức `user` (`read`, `write`, `execute`; không có `manage`/`admin`): đủ cho `Execute`, `UpdateTask(worktree/status)` và `GetSubtree`, không cho người đó cấp quyền tiếp. Không mở quyền cho ai ngoài người request-service đã xác nhận: grant chỉ được ghi cho đúng danh tính mà `ActorFor` hoặc người gọi `StartPhase` (sau `CanStart`) đưa ra. Không nới thành "mọi người dùng thành chủ cây".

Chi tiết:
- Gốc cây = task không có cha và chưa `cancelled` của Request (Plan, hoặc task sửa đơn lẻ của hotfix vốn không có Plan). Một lần `ListTasks` (không lọc user) cho cả việc tìm gốc lẫn `OwnerID`.
- Người ghi grant = `OwnerID` của gốc. Gốc không có chủ, hoặc chủ trùng người cần mà vẫn bị từ chối, hoặc task-service từ chối `Grant` (`PermissionDenied`): `REQUEST_EXECUTE_FORBIDDEN` (không đoán chủ khác, không đổi danh tính).
- Idempotent: hỏi `ResolvePermission` trước; có quyền rồi thì không ghi. Xác nhận được nhớ 2 phút trong bộ nhớ (khoá tenant + gốc + người) để reconcile và consumer không hỏi task-service mỗi vòng (mỗi lần hỏi, task-service còn ghi một dòng audit). Hết 2 phút hỏi lại, nên grant bị thu hồi thì lỗi lộ ra ở `Execute` rồi tự cấp lại.
- Race hai bên cùng cấp: `task_grants` không có ràng buộc duy nhất, nên tệ nhất là hai dòng trùng (vô hại; `RevokeGrant` xoá cả hai theo khoá `(task, subject, level)`).
- Vị trí gọi: (1) `ApprovalExecutionActors.ActorFor` (mọi đường nền: consumer, reconcile, `autoComplete`, `completePhases`, `dispatch`); (2) `StartPhase`, **sau** `CanStart` và kiểm Phase đã duyệt, **trước** `checkPredecessors` và claim `phase_starts`.
- Audit: task-service ghi `task.grant_received` (`granted_by` = chủ của gốc, vì lời gọi mang danh tính chủ) và một dòng audit `ResolvePermission`; request-service ghi log `granted execute access on the plan tree to the execution actor` kèm `actor`, `granted_as`, `root_id`. Thiếu: sự kiện không nói "request-service làm thay" (cần trường mới ở proto `Grant`, chưa làm).

### Claim `phase_starts` mồ côi (lỗi tiềm ẩn #2): đã xử lý

Trước: người có quyền bắt đầu theo request-service nhưng không có quyền trong task-service để lại claim, thành actor mặc định của Phase, và mọi lần giao sau lặp lại `REQUEST_EXECUTE_FORBIDDEN`. Nay `Ensure` chạy trước claim: bắt đầu được thì có quyền (claim và quyền cùng tồn tại), không được thì dừng ở `REQUEST_EXECUTE_FORBIDDEN` trước khi ghi claim. Test: `TestStartPhase_UngrantableStarterLeavesNoClaimBehind`.

### Test Q1

- Đơn vị `execution_access_test.go` (12): cấp cho người duyệt khác người commit (ghi dưới danh nghĩa chủ, trên gốc); không cấp khi đã có quyền; nhớ xác nhận 2 phút rồi hỏi lại không cấp trùng; gốc không chủ bị từ chối; task-service từ chối grant; sự cố task-service không bị đọc thành `FORBIDDEN`; gốc hotfix không Plan; Plan đã huỷ bị bỏ qua; `ActorFor` làm người được chọn chạy được; `StartPhase` cấp trước claim; `StartPhase` không để claim khi không cấp được.
- Client `task_access_test.go` (5): `CanExecute` hỏi action `execute`; "không có grant" là câu trả lời; sự cố và NotFound; `GrantExecute` ghi mức `user`, `apply_tree`, không hết hạn, dưới danh nghĩa chủ (không rò user của ngữ cảnh); từ chối.
- Hợp đồng `ExecutionFlow_ApproverAndStarterAreNotTheCommitter` (Postgres và MySQL): hai Phase, hai người bắt đầu khác nhau, người duyệt khác người commit; kiểm `ExecutedBy` là từng người bắt đầu, grant chỉ ở mức `USER` (không `OWNER`/`ADMIN`), người ngoài vẫn không chạy được. Đỏ khi tắt sửa (đã chạy đỏ trước khi ghi nhận).
- e2e Go (binary thật + NATS + task stub, hai dialect): bỏ `AllowTreeReaders` khỏi `kit_test.go` (trước đó che lỗi này: reporter, admin, admin2 được đọc sẵn); mọi kịch bản vẫn xanh nhờ grant tự cấp; `requireNoGrantBeyondExecute` kiểm mọi grant sinh ra là mức `USER`.

## Q2. Xem lại kết quả của task thuộc Request (tab Result)

### Hiện trạng và vì sao không chọn phương án (a)

- Gateway chưa đăng ký `execution.get`: tab Result ẩn.
- `ExecutionRecord` chỉ được tạo ở đường hợp đồng của task-service, và đường đó chỉ chạy khi request-service gửi `result_nonce`. **Không bật `result_nonce`**: một lượt chạy hợp đồng chỉ sang `review` khi agent in khối `ORCA_RESULT_BEGIN/END <nonce>` hợp lệ với `status=done`; mọi trường hợp khác trả task về trước đó. request-service hiện không gửi prompt nào chỉ dẫn agent in khối đó (CR-REQ-029 chưa nối vào luồng giao task), nên gửi nonce sẽ làm mọi lượt chạy của Request thất bại. Đây là một CR riêng, không phải việc tối thiểu.
- `tasks.last_execution_output` có sẵn (stdout của lượt chạy thành công gần nhất, 8 KiB) nhưng không có trong proto.

### Quyết định: RPC mới `GetTaskExecutionOutput`, không thêm trường vào `Task`

Thêm `last_execution_output` vào `Task` bị loại vì: `GetTask` ở task-service **không kiểm grant** (xem `usecase/get_task.go`) và route REST `GET /v1/tasks/{id}` trả nguyên proto, nên stdout chưa che bí mật sẽ rò cho mọi người trong tenant; `ListTasks` thêm tới 8 KiB mỗi task; `task.get` được UI poll liên tục. Thay vào đó:

- Proto (chỉ cộng): `rpc GetTaskExecutionOutput(GetTaskExecutionOutputRequest{task_id}) returns (GetTaskExecutionOutputResponse{found, output, redacted, truncated})`. `buf breaking` xanh, `buf lint` vẫn 307 dòng như trước.
- task-service: cần quyền `read` trên task nếu có user (cùng quy tắc `ListExecutionRecords`: lời gọi sibling không có user thì chỉ theo tenant); che bí mật bằng `common/secretscan.Redact` **trước** khi cắt 8 KiB cuối (để bí mật bị cắt giữa chừng không lộ nửa chừng), cắt ở ranh giới ký tự.
- Gateway `execution.get` (hình dạng đầy đủ ở CONTRACT mục 2.8): `ListExecutionRecords` (bản ghi mới nhất) trước; không có thì `GetTaskExecutionOutput`; không có gì thì `TASK_NOT_FOUND: ...` (UI đọc là `legacy`). Lỗi task-service không lộ nội dung (`TASK_FORBIDDEN`, `TASK_NOT_FOUND`, `TASK_UNAVAILABLE`). task-service cũ (`Unimplemented`) vẫn trả lời được.
- Che bí mật ở gateway thêm một lớp cho `stdoutTail`, `summary`, `outputs` của bản ghi hợp đồng (task-service lưu `stdout_tail` thô).

### Lỗi thật tìm thấy trên đường: `UpdateLastExecutionOutput` mất output khi có ký tự lạ

`UpdateLastExecutionOutput` cắt `output[:8192]` theo byte: cắt giữa một ký tự nhiều byte, hoặc stdout có byte NUL, là UTF-8 sai, và Postgres từ chối (`invalid byte sequence for encoding "UTF8"`); lời gọi bỏ qua lỗi (`_ =`), nên một lượt chạy dài có tiếng Việt, Nhật... không để lại output nào. Sửa: `domain.BoundStoredOutput` (bỏ NUL và byte sai, cắt ở ranh giới ký tự) dùng ở cả hai repo. Test tích hợp `TestRepository_UpdateLastExecutionOutput_StoresHostileStdout` chạy đỏ trên Postgres khi bỏ sửa.

### Hình dạng dữ liệu cho frontend (agent P, N)

`execution.get {taskId, latestOnly}` trả (camelCase, đúng thứ `parseExecutionResult` đọc):

```
// task chạy bằng engine 1 không có khối kết quả (mọi task của Request hiện nay):
{ taskId, parseStatus: "missing", filesChanged: [], checksRun: [], stdoutTail: "<stdout cuối, đã che bí mật>",
  source: "last_execution_output", redacted?: true, truncated?: true }
// lượt chạy hợp đồng:
{ taskId, attempt, parseStatus: "ok"|"invalid", status?, summary?, filesChanged, checksRun, failureClass?, stdoutTail?, outputs?, source: "execution_record" }
```

Đề xuất cho frontend: `ExecutionResultPanel` với `parseStatus !== 'ok'` hiện cảnh báo đỏ "kết quả không đúng định dạng" kèm `stdoutTail`; với `source === "last_execution_output"` nên hiện trung tính "không có kết quả có cấu trúc, đây là output của agent" (cần thêm `source` vào `parseExecutionResult` và kiểu `ExecutionResult`). Bỏ `execution.get` khỏi danh sách kênh cho phép chưa đăng ký của hợp đồng kênh (TASK-FE-E2E-001), vì gateway đã đăng ký. `useExecutionResult` đã xử lý `not_found` là `legacy`.

## Q3. Dev server "bận" và dữ liệu handshake

### Nguồn sự thật đã chọn

Thử hai nguồn. (1) Task `in_progress` theo dev server: task-service **không lưu dev server** của lượt giao (cột và liên kết thực thi không có dev server id), nên không đếm được từ DB. (2) Phiên sống của infra-fleet: `devserveragent.session` biết chính xác những cuộc gọi đang chờ. Chọn (2): đếm các cuộc gọi `agent.execPrompt` (chạy task) và `ai.complete` (gọi model: phân tích, sinh Plan) chưa có phản hồi trên phiên sống của dev server. Một cuộc gọi lỗi, hết giờ hoặc rớt phiên được trừ ngay (defer), nên không kẹt "bận" mãi. Đây là bộ nhớ trong của replica giữ phiên, đúng nơi công việc chạy; sau khi infra-fleet khởi động lại, số này bắt đầu từ 0 (các run cũ đã mất cùng phiên).

### Thay đổi

- `DevServer.active_executions` (proto, chỉ cộng, số 15): chỉ `ListDevServers` và `ListDevServersForUser` điền (qua `Server.WithDevServerActivity`); các RPC khác trả 0.
- Gateway `devServer.list`/`listForUser`: thêm `activeExecutions`, `busy`; về 0 và `false` khi không `connected` (kể cả khi kiểm kết nối lỗi, vì khi đó status báo `disconnected`).
- Sự kiện: `infra-fleet` publish `orca.infra.devserver.statusChanged` (JetStream stream `INFRA`, publish trực tiếp, không outbox: ảnh chụp trạng thái, mất một cái thì cái sau thay) khi phiên nối, rớt, hoặc một lượt chạy bắt đầu/kết thúc. `BroadcastDevServerStatus` gộp một loạt thay đổi thành một ảnh chụp **của trạng thái lúc gửi**, một worker nên đúng thứ tự. Gateway thêm kênh push `devServer.subscribeStatus` đẩy `devServer.statusChanged` `{devServerId, connected, activeExecutions, busy}` (lọc theo tenant; bus `nil` thì kênh đóng ngay, như `agent.subscribeStatus`).
- **Lỗi sẵn có ở cấu hình stream**: `main.go` gọi `EnsureStream("INFRA", ["orca.infra.>"])` rồi sau đó `EnsureStream("INFRA", ["orca.infra.agent.>"])` (`CreateOrUpdate`, nên lần sau thu hẹp lại): mọi subject `orca.infra.*` ngoài `agent.*` (kể cả `terminal_session.agent_completed` của thông báo đẩy) không có stream nhận. Đã đổi lần sau thành `orca.infra.>`.
- `platform`/`arch`/`nodeVersion` luôn null: có **hai** nguyên nhân. (a) Gateway `toDevServerView` không chép các trường này dù proto đã có: sửa. (b) infra-fleet chỉ ghi handshake ở đường SSH (`EstablishConnection`) và bulk provision, nên dev server nối bằng WebSocket có cột rỗng và `health_status` mãi `pending`: thêm `RecordDevServerHandshake`, chạy trong hook `WithOnSessionAttached` đã có, ghi các cột qua `UpdateProvisionResult`; `pending` thành `healthy`, còn `degraded`/`unhealthy` (do poll sức khoẻ) được giữ nguyên khi nối lại. Không đụng migration.
- Không làm: `lastConnectedAt` (không có cột), `lastError`.

### Test Q3

- `active_executions_test.go`: đếm đúng hai lượt đang chạy và về 0 khi xong; probe ngắn (`ports.scan`) không tính; lượt hết giờ không để lại số; callback hoạt động bắn khi nối, bắt đầu, kết thúc (chạy `-race`).
- `record_dev_server_handshake_test.go`, `broadcast_dev_server_status_test.go` (gộp loạt thay đổi, ngắt kết nối thì 0 lượt, dev server không tenant bị bỏ qua, publish lỗi không dừng, `Run` dừng theo context; `-race`), `server_activity_test.go`.
- Gateway: `TestDevServerListChannel_ReportsBusyAndHandshakeFacts` (bận, null khi chưa biết, không bận khi ngắt), kênh `devServer.subscribeStatus` đóng khi bus `nil`.

## Blast radius (codegraph, tên chính xác; symbol mới chưa có trong chỉ mục nên một phần đọc tay)

- `ApprovalExecutionActors` / `ActorFor`: 7 symbol; `autoComplete`, `completePhases`, `dispatch` của `EvaluateExecution` (đều từ `Run`, vào từ consumer trạng thái, consumer duyệt, reconcile, `ReportTaskOutcome`) và `refsWithEdges`. Thêm hai lời gọi task-service (`ListTasks`, `ResolvePermission`) mỗi lần chưa có xác nhận trong 2 phút; có ghi nhận ở audit của task-service.
- `StartPhase.Execute`: 1 caller (handler gRPC `StartPhase`) và consumer trạng thái (`Internal`); thêm một lời gọi trước claim.
- `usecase.TaskClient` thêm 2 phương thức: cài đặt thật `grpcclient.TaskClient` và `UnavailableTaskClient`; fake trong test (`exTasks`, `fakeTasks`) đã cập nhật.
- `toDevServerView` / `attachConnectionStatus`: 2 caller (`registerDevServerChannels`, `registerDevServerAccessControlChannels`); chỉ thêm trường, hai trường mới và ba trường nay có giá trị thay vì null.
- `ListDevServers` (RPC): consumer ngoài gateway là `project-service` `dev_server_hostname_resolver` (đọc host), không ảnh hưởng trường mới.
- `UpdateLastExecutionOutput`: 1 caller (`SimpleExecutor.Execute`); hành vi đổi duy nhất: không còn lỗi âm thầm với output có ký tự lạ.
- Proto: chỉ cộng (RPC mới, trường `active_executions = 15`); `buf breaking` xanh.
- Hành vi đổi cho người dùng: người duyệt/bắt đầu khác người commit nay thực thi được và thấy cây Plan (được grant `user`); nút "Result" xuất hiện và có nội dung; thẻ dev server có platform/arch/Node, `busy`.

## Kết quả kiểm (2026-10-10, máy dev, Colima)

Môi trường: `DOCKER_HOST=unix:///Users/datbeohbbh/.colima/default/docker.sock TESTCONTAINERS_RYUK_DISABLED=true`, `migrate` và `buf` trong PATH.

- request-service: `go build ./... && go vet ./... && go test ./...` PASS; `go vet -tags integration/e2e` sạch; `go test -tags integration ./...` PASS (Postgres 77 giây, MySQL 124 giây; gồm `ExecutionFlow_ApproverAndStarterAreNotTheCommitter`); `E2E_DIALECT=postgres go test -tags e2e ./e2e/...` PASS (44 giây), `E2E_DIALECT=mysql` PASS (52 giây), không còn `AllowTreeReaders` trong kit.
- task-service: `go build ./... && go vet ./... && go test ./...` PASS; `go vet -tags integration` sạch; `go test -tags integration -timeout 25m ./...` PASS (Postgres 152 giây, MySQL 727 giây; gồm `TestRepository_UpdateLastExecutionOutput_StoresHostileStdout` ở cả hai).
- api-gateway: `go build ./... && go test ./...` PASS (tệp vàng `channel_inventory.json` thêm đúng `devServer.subscribeStatus` và `execution.get`; parity MCP: hai ngoại lệ trong `excluded_channels.yaml`).
- infra-fleet-service: `go build ./... && go vet ./... && go test ./...` PASS trừ hai lỗi có sẵn (`backendrelaysshprovisioner`, `ephemeralsshconn`: `bind: invalid argument` của unix socket); `go test -race` PASS cho `devserveragent` và các test mới của `usecase` (broadcast, handshake); chạy `-race` cả gói `usecase` thì `TestAgentOutputClassifier_StartupTimeout_KillsAndMarksError` báo data race có sẵn (test fake `scan_workspace_ports_test.go`), không liên quan.
- proto: `buf generate` (đưa header về đúng phiên bản đã commit; hai tệp chỉ đổi header được hoàn lại), `buf lint` 307 dòng (không tăng), `buf breaking --against '../../.git#branch=main,subdir=backend-go/proto'` xanh.

## Việc cần làm

- [x] Q1: `ExecutionAccess` (grant `user` dưới danh nghĩa chủ gốc cây, trước khi dùng danh tính); nối vào `ActorFor` và `StartPhase`; claim mồ côi hết; fake lọc quyền thật; đơn vị, hợp đồng hai dialect, e2e hai dialect; CR-REQ-013 cập nhật.
- [x] Q2: RPC `GetTaskExecutionOutput`, kênh `execution.get`, che bí mật, sửa mất output với ký tự lạ; CONTRACT mục 2.8; tệp vàng; test.
- [~] Q2: không làm `result_nonce` (cần CR đưa prompt hợp đồng vào luồng giao task); lượt chạy **lỗi** vẫn không có output ở `execution.get`.
- [x] Q3: `activeExecutions`/`busy`, `devServer.subscribeStatus`, handshake được ghi, `platform/arch/nodeVersion` chép đúng; sửa stream `INFRA`; test; CONTRACT mục 2.9.
- [ ] Redeploy `request-service`, `task-service`, `api-gateway`, `infra-fleet-service` (người điều phối); xem dưới.
- [ ] Frontend (agent P/N): dùng `busy`/`activeExecutions` và `devServer.statusChanged`; `source` trong tab Result; bỏ `execution.get` khỏi danh sách kênh chưa đăng ký.
- [ ] Commit.

## Chưa kiểm chứng (nói thẳng)

- Chưa chạy trên stack thật: grant dưới danh nghĩa chủ qua task-service thật (`Grant` yêu cầu `manage`; đã đọc `usecase/grant.go` và `resolve_permission.go`, test fake mô phỏng), `ResolvePermission` thật với OPA, đếm `busy` với agent thật khi chạy task dài, sự kiện `devServer.statusChanged` qua NATS thật (chỉ test đơn vị; publisher không có test với NATS).
- Fake task-service mô phỏng quyền theo chủ và grant `user`, `apply_tree`; grant theo team, công ty, hết hạn không có trong fake.
- `grpc metadata` `x-user-id` của task-service vẫn tin người gọi (không có danh tính service-to-service, ghi ở CR-REQ-013 mục 6). Việc request-service ghi grant "dưới danh nghĩa chủ" dựa vào đó, giống mọi lời gọi thay mặt người dùng hiện có (`Execute`, `UpdateTask`).
- Grant không tự thu hồi khi Request xong hoặc huỷ.
- `busy` là bộ nhớ trong của replica giữ phiên: nếu chạy nhiều replica infra-fleet cho cùng một dev server (không phải cấu hình hiện nay), mỗi replica chỉ thấy phiên của mình.
- `execution.get` chỉ có output của lượt chạy thành công gần nhất; nếu agent P đổi nghĩa `stdout` (stream-json) thì `last_execution_output` đi theo.

## Cần redeploy

`request-service`, `task-service`, `api-gateway`, `infra-fleet-service` (mã sinh proto đổi ở `task` và `infrafleet`; không migration, không biến môi trường mới). Thứ tự: `task-service` và `infra-fleet-service` trước, rồi `api-gateway` (gọi RPC mới; task-service cũ vẫn chạy được vì gateway chấp nhận `Unimplemented`), `request-service` bất kỳ lúc nào. NATS: stream `INFRA` sẽ được `infra-fleet-service` cập nhật thành `orca.infra.>` khi khởi động.
