# TASK-REQ-042-03: Chuyển relay của `task-service`, `workflow-service`, `orchestration-service`, `git-gateway-service`

**From Solution:** [BE-REQ-SOL-042](../solutions/BE-REQ-SOL-042-prompt-catalog-shared-agent-relay-and-stdin.md)
**Priority:** P1
**Service:** `task-service, workflow-service, orchestration-service, git-gateway-service`
**File:** `services/task-service/internal/adapter/grpcclient/{simple_executor,aidecompose_relay}.go`, `services/workflow-service/internal/adapter/infrafleetclient/agent_step_executor.go`, `services/orchestration-service/internal/adapter/infrafleetclient/worker_dispatcher.go`, `services/git-gateway-service/internal/adapter/grpcclient/relay_executor.go` (chỉ `Complete`)
**Depends on:** TASK-REQ-042-01
**Status:** [x] DONE (2026-10-20) — workflow-service không biên dịch ở HEAD (mã `step_approvals` của luồng khác) nên phần workflow chỉ kiểm bằng overlay + test đơn vị

---

## Context

- task-service: `runAgentPrompt` gửi `agentExecPromptParams` (có `Model`, `InitFile`, `Env`, hợp đồng) qua `RelayByDevServer`/`Relay`; `AICompleter.Complete` cho tách task.
- workflow-service: thêm `AccountID`; hồ sơ môi trường qua `BuildAgentEnv` (giữ ở service).
- orchestration-service: prompt là `task.TaskTitle`; bộ phân tích kết quả riêng.
- git-gateway: `relay()` là bộ gửi chung cho mọi lệnh git/fs; chỉ `Complete` đổi.

## Việc cần làm

1. **Đọc đầy đủ** `worker_dispatcher.go` và `agent_step_executor.go` trước khi sửa; ghi vào PR hành vi riêng nào phải giữ.
2. Mỗi nơi: thay struct tham số, lời gọi `Relay`/`RelayByDevServer`, phân tích kết quả bằng `agentrelay`; giữ nguyên logic nghiệp vụ ở lại service (nội suy `{{outputs.*}}` của task-service, `BuildAgentEnv` của workflow-service, xử lý `TimedOut`/`ExitCode` của orchestration).
3. Xoá các struct và hàm trùng lặp.
4. git-gateway: `Complete` gọi `agentrelay.Complete` (chỉ `prompt`, như hiện nay); các hàm `git.*`, `fs.*` không đổi.
5. Cập nhật ghi chú đầu file nói "field name UNVERIFIED" thành đúng thực tế sau khi gom.

## Kiểm thử

- Test hiện có của từng file xanh; thêm test so JSON với bản chụp (task 01) cho task-service (hợp đồng, thường), workflow-service (có `accountId`), orchestration (ba trường).
- Acceptance `request`, `matrix`; UI native-execute với claude giả; kịch bản workflow nếu có trong `suite_workflow_dag.py`.
- `go test` của bốn service.

## Tiêu chí hoàn thành

- [x] Không còn nơi nào ngoài `agentrelay` dựng tham số `agent.execPrompt`/`ai.complete` (task, workflow, orchestration, git-gateway `Complete`; grep `"agent.execPrompt"`/`"ai.complete"` trong mã không-test của `services/` và `common/` chỉ còn comment và infra-fleet).
- [x] Logic riêng của từng service được giữ và có test.
- [x] Acceptance xanh. Acceptance `--ai-stub platform devserver request matrix exits ai plan analysis task notfound aistub request_base_ref analysis_revision` (2026-10-20, sau khi triển khai lại request/task/git-gateway/orchestration): 121 đạt, 0 lỗi, 3 chặn (AIS-xx cần AI thật, MCP-01/02 có sẵn), 2 bỏ qua; chạy lại tập nhỏ hơn sau lần triển khai cuối: 102 đạt, 0 lỗi. UI `qa-bug-task-native-flow` 3/3; `--real native-execute` (Claude thật, repo tạm) 2/2 đạt trong 11,8 phút.

## Rủi ro và lưu ý

- Blast radius cao ở `SimpleExecutor`; tách thay đổi theo service, mỗi service một commit.

## Chuẩn hoàn thành chung (áp dụng cho mọi task, không thương lượng)

- Mã chạy thật, **không stub, không TODO, không `panic` cho lỗi thường, không bỏ qua lỗi**; mọi nhánh lỗi có mã lỗi và test.
- Chạy `gitnexus_impact` trước khi sửa symbol có sẵn và báo blast radius; chạy `detect_changes` trước khi commit (CLAUDE.md).
- Hai dialect: mọi thay đổi lưu trữ có migration và test hợp đồng trên **Postgres 16 và MySQL 8** (testcontainers).
- Không file nào vượt `max-lines` (tách file khi cần, không thêm disable); không đặt tên `helpers`, `utils`, `common`, `misc`.
- Chạy được trên macOS, Linux, Windows và qua SSH; lệnh git theo baseline 2.25 (`guides/reference/git-compatibility.md`).
- Không ghi nội dung Request, Solution, phản hồi hoặc bí mật vào log, span hay khung sự kiện (C9).
- `go build ./...` và `go test` của gói đã đổi xanh; `go vet` xanh; lint xanh; không làm hỏng test hiện có.

---

## Ghi chú triển khai (2026-10-20)

### Blast radius (GitNexus không dùng được; grep/go list)

- `SimpleExecutor` (task-service): chỉ `cmd/server/main.go:260` gọi `NewSimpleExecutor` (chữ ký không đổi); `runAgentPrompt` do `Execute` và `ExecuteWithContract` dùng. Kiểu `agentExecPromptParams/Result`, `resultBlockParam`, `agentParsedResult`, `contractRunTimeoutMs`, `contractMaxOutputBytes` chỉ dùng trong package `grpcclient` của task-service (cộng test). `AICompleter`: `cmd/server/main.go:229`, chữ ký không đổi.
- `AgentExecutor` (workflow-service): `cmd/server/main.go:236`, chữ ký không đổi. `relay()`, `execResult`, `toStepResult` còn dùng cho shell/git/notification nên giữ.
- `WorkerDispatcher` (orchestration): `cmd/server/main.go:190`, chữ ký không đổi.
- git-gateway `RelayExecutor.Complete`: gọi từ `GenerateCommitMessage`, `GeneratePullRequestFields` qua `usecase.AICompleter`; `relay()`, `relayStream()` và mọi hàm `git.*`/`fs.*` không đổi.
- Không phải sửa `cmd/` của service nào.

### Hành vi riêng ở lại service (đã đọc đủ `simple_executor.go`, `agent_step_executor.go`, `worker_dispatcher.go`)

- task-service: nội suy `{{outputs.<id>.stdout|*}}` (`buildExecutePrompt`/`interpolateOutputs`, chưa đụng, chuyển ở task 06/07); trust `full` cố định; env `ORCA_TASK_ID/ORCA_PROJECT_ID` + `BuildAgentEnv` + `ORCA_PROJECT_NAME` + `initFile` + model từ hồ sơ (best-effort); tuỳ chọn contract (`contractRunOptions.execParams` chọn `agentrelay.Contract` hoặc `Plain`, `knowledgeRoot` chỉ khi `useKnowledgeHub` và có repo root); không đặt deadline RPC (`agentrelay.WithoutCallDeadline()`); metadata tenant do `runAgentPrompt` gắn một lần vì cùng ctx còn dùng cho luồng output (relayer KHÔNG có `WithMetadata`, tránh hai giá trị tenant); `relayError` (Transient) cho lỗi relay, còn kết quả không đọc được (`ErrMalformedResult`) và tham số bị agentrelay từ chối không là `relayError`; khối kết quả/phân loại lỗi chạy (`ParseExecutionResult`, `ClassifyRunFailureChecked`), `stdoutScanLimit`, `TailUTF8`.
- workflow-service: `accountId` từ `ProviderResolver`, model (`cfg.Model` bị `preferredModel` ghi đè), `BuildAgentEnv`/`BuildProjectContext`/`ORCA_PROJECT_NAME` chỉ khi có `UserID`, kiểm `connectionId` rỗng trước khi gọi, `toStepResult` dùng chung với shell, `WithMetadata(withTenantMetadata)`.
- orchestration-service: prompt là `task.TaskTitle`, `stepId` là `handle`; `TimedOut` và `ExitCode` nil/khác 0 là lỗi; không metadata tenant, không deadline.
- git-gateway: chỉ `ai.complete` với `prompt`; thêm `complete_relay_client.go` (bọc client cho agentrelay) để giữ hai hành vi mà `relay()` vẫn cho mọi lệnh git/fs: dev server rơi sau khi dispatch thành `DEV_SERVER_UNREACHABLE`; lỗi JSON-RPC `{error:{code,message}}` trong câu trả lời "ok" thành lỗi chứ không thành nội dung rỗng. Giữ nhánh `RelayByDevServer` khi ctx có dev server id.

### File đổi

- task-service `internal/adapter/grpcclient/`: `simple_executor.go` (xoá `agentExecPromptParams/Result`, dựng bằng `agentrelay.Run`), `simple_executor_contract.go` (xoá `resultBlockParam`, `agentParsedResult`, hằng số contract, `apply`; thêm `execParams`, `execPromptError`, `changesJSON`), `aidecompose_relay.go`; test: `simple_executor_test.go`, `simple_executor_contract_test.go` (đổi sang `agentrelay.ExecParams`), mới `simple_executor_wire_test.go`.
- workflow-service `internal/adapter/infrafleetclient/`: `agent_step_executor.go` (+ `agentRunOutcome`), `agent_step_executor_test.go`, mới `agent_step_wire_test.go`. Ghi chú "field name UNVERIFIED" bị xoá cùng struct: agent đọc `initFile` thật ở `agent/src/relay/agent-print-mode-exec.ts:63`.
- orchestration-service `internal/adapter/infrafleetclient/`: `worker_dispatcher.go`, mới `worker_dispatcher_wire_test.go`.
- git-gateway-service `internal/adapter/grpcclient/`: `relay_executor.go` (chỉ `Complete`, struct/constructor thêm trường `complete`), mới `complete_relay_client.go`, `relay_executor_complete_test.go`.

### Chứng minh JSON không đổi

- task-service: `task_plain` khớp golden từng byte (sau chuẩn hoá khoá) qua `SimpleExecutor.Execute` thật. `task_profile`, `task_contract`, `task_contract_knowledge`: env/initFile/model của golden là dữ liệu mẫu nên không thể tái tạo qua resolver; test so (1) mọi khoá còn lại với golden, (2) toàn bộ JSON với bản chép nguyên văn struct cũ (`legacyTaskParams` + `legacyContract`) điền cùng giá trị. `ai.complete`: khớp `complete_git`.
- workflow-service: `workflow_bare` khớp golden; `workflow_profile` so khoá còn lại với golden và toàn bộ với bản chép struct cũ.
- orchestration: khớp golden `orchestration` và bản chép struct cũ.
- git-gateway: `Complete` khớp golden `complete_git`.
- Đột biến: bỏ `WithoutCallDeadline()` làm `TestTaskWire_TheRelayCallCarriesNoDeadline...` fail, rồi hoàn lại.

### Thay đổi hành vi có chủ đích (cần người duyệt)

1. task-service `Truncated`: kiểu cũ `bool`, agent gửi `{stdout,stderr}` => unmarshal lỗi khi output bị cắt; kiểu mới `*agentrelay.Truncated` sửa lỗi. Test `TestTaskWire_ATruncatedRunDecodes` (kèm khẳng định struct cũ thật sự lỗi).
2. task-service bản ghi thực thi, cột `Changes`: trước lưu nguyên JSON thô của agent, nay lưu JSON của `agentrelay.Changes` (cùng sự kiện, thứ tự khoá chuẩn; `headBefore/headAfter` null thành vắng mặt). Test cũ dùng payload giả `{"files":["a.go"]}` đã đổi sang dạng thật của `agent-worktree-change-snapshot.ts`. Người đọc (`channels_task_execution.go`) chỉ đọc `changedFiles`.
3. workflow-service: một lần chạy agent bị agent giết do timeout (`exitCode` null, `timedOut: true`) trước đây thành `completed` exit 0 (vì `toStepResult` coi vắng exit code là thành công); nay là `failed` với `error: "agent.execPrompt timed out before the agent finished"`. Có test.
4. Kết quả rỗng (`resultJson` = "") của `agent.execPrompt`/`ai.complete` trước đây im lặng thành công/nội dung rỗng; nay là `ErrMalformedResult` (task-service, workflow, orchestration, git-gateway). Agent thật luôn trả đối tượng.
5. git-gateway `Complete` không còn bị `relay()` đổi tên thành `ai.completeViaHiddenTarget` khi ctx có hidden target (agent không có phương thức đó; trước đây sẽ lỗi). Có test.
6. Thông điệp lỗi đổi tiền tố (ví dụ `agentrelay: relay agent.execPrompt: ...`); lỗi deadline thành `context.DeadlineExceeded` trần. Không test nào so chuỗi này.

### Lệnh và kết quả

```
cd backend-go/services/task-service          && go build ./... && go vet ./... && go test ./... -race -count=1   # OK
   go test ./internal/adapter/grpcclient/ -race -run RealGit -v                                                   # 3 test TASK-REQ-039-04: PASS
   DOCKER_HOST=unix:///Users/datbeohbbh/.colima/default/docker.sock TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE=/var/run/docker.sock \
   go test -tags=integration ./internal/adapter/grpcclient/ -run 'NATS|Partial' -v                                # PASS
cd backend-go/services/orchestration-service && go build ./... && go vet ./... && go test ./... -race -count=1   # OK
cd backend-go/services/git-gateway-service   && go build ./... && go vet ./... && go test ./... -race -count=1   # OK
cd backend-go/services/workflow-service      && go build ./...                                                   # FAIL tại HEAD, không phải do thay đổi này
```

workflow-service không biên dịch tại HEAD: `internal/usecase/step_approvals.go` dùng `StepApprovalRepository` chưa được định nghĩa, và `internal/adapter/grpc/step_approvals.go` dùng các kiểu `workflowv1.StepApproval*` chưa có trong proto (việc dở dang của người khác, không sửa). Đã kiểm bằng overlay (`scratchpad/wf-overlay.json`, làm rỗng `internal/usecase/step_approvals.go` và `_test.go`): `go vet -overlay ... ./internal/adapter/infrafleetclient/ ./internal/domain/ ./internal/usecase/` OK; `go test -overlay ... ./... -race` xanh cho `infrafleetclient`, `providerresolver`, `serverresolver`, `stepexecutors`, `taskclient`, `tddagentclient`, `domain`, `usecase`; còn `cmd/server`, `adapter/grpc`, `adapter/mysql`, `adapter/postgres` không biên dịch vì cùng nguyên nhân (không liên quan).

### Chưa kiểm chứng

- Acceptance `request`, `matrix`, UI native-execute với claude giả và kịch bản `suite_workflow_dag.py` (quy tắc của đợt này cấm chạy e2e): chưa chạy nên tiêu chí "Acceptance xanh" để trống và Status là PARTIAL.
- `golangci-lint` không chạy được. `gitnexus_impact`/`detect_changes` không có; blast radius ghi ở trên bằng grep.
- Chưa gọi infra-fleet hay agent thật; mọi kiểm thử dùng client giả.
- `cmd/server` của workflow-service chưa biên dịch được để xác nhận nối dây (chữ ký constructor không đổi nên không cần sửa).
