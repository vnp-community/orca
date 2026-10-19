# TASK-REQ-042-03: Chuyển relay của `task-service`, `workflow-service`, `orchestration-service`, `git-gateway-service`

**From Solution:** [BE-REQ-SOL-042](../solutions/BE-REQ-SOL-042-prompt-catalog-shared-agent-relay-and-stdin.md)
**Priority:** P1
**Service:** `task-service, workflow-service, orchestration-service, git-gateway-service`
**File:** `services/task-service/internal/adapter/grpcclient/{simple_executor,aidecompose_relay}.go`, `services/workflow-service/internal/adapter/infrafleetclient/agent_step_executor.go`, `services/orchestration-service/internal/adapter/infrafleetclient/worker_dispatcher.go`, `services/git-gateway-service/internal/adapter/grpcclient/relay_executor.go` (chỉ `Complete`)
**Depends on:** TASK-REQ-042-01
**Status:** [ ] TODO

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

- [ ] Không còn nơi nào ngoài `agentrelay` dựng tham số `agent.execPrompt`/`ai.complete`.
- [ ] Logic riêng của từng service được giữ và có test.
- [ ] Acceptance xanh.

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
