# TASK-REQ-039-04: task-service: nhận `base_ref`, tạo worktree thực thi từ ref đã chọn hoặc nhánh mặc định

**From Solution:** [BE-REQ-SOL-039](../solutions/BE-REQ-SOL-039-request-worktree-base-branch.md)
**Priority:** P1
**Service:** `task-service, request-service`
**File:** `proto/orca/task/v1/task.proto` (sửa) + mã sinh, `services/task-service/internal/{usecase/execute_task.go,usecase/ports.go,adapter/grpc/server.go,adapter/grpcclient/worktree_provisioner.go}` (sửa); `services/request-service/internal/{usecase/ports_execution.go,usecase/advance_execution.go,adapter/grpcclient/task_client.go}` (sửa) và mọi fake
**Depends on:** TASK-REQ-039-02
**Status:** [x] DONE (2026-10-19)

---

## Context

- Hai đường `request-service → task-service`: `Execute` và `ExecuteContract`; cả hai phải mang nhánh.
- `WorktreeProvisioner.EnsureWorktree` không gửi `BaseRef`; agent rẽ từ HEAD checkout chính.
- Một Plan dùng chung một worktree: chỉ lần tạo đầu cần nhánh.

## Việc cần làm

1. `TaskServiceExecuteRequest.base_ref` (số trường trống kế tiếp, kiểm bằng `buf`/`protoc`); `ExecuteTaskInput.BaseRef`.
2. `TaskExecutor.Execute(ctx, taskID, executionRequestID string, opts ExecuteOptions) error`; `ExecuteOptions{BaseRef string}`; `ContractRun.BaseRef`. `AdvanceExecution.dispatch` truyền `req.BaseRef` cho cả hai đường. Cập nhật `TaskClient` và **mọi fake/test** dùng cổng này.
3. `WorktreeProvisioner.EnsureWorktree(ctx, tenantID, task, baseRef)`. Nếu `baseRef==""` thì `git.BaseRefDefault{RepoId}`; lỗi thì `TASK_EXECUTE_WORKTREE_FAILED` kèm nguyên nhân, không rơi về HEAD. Gửi `CreateWorktreeRequest.BaseRef`.
4. Nhánh dùng lại worktree (`task.WorktreeID`, `findIssueWorktree`) bỏ qua `baseRef`.
5. Task chạy lẻ (không thuộc Request, `RequestID` rỗng) cũng đi qua `BaseRefDefault` để quy tắc đúng ở mọi đường vào.
6. Giữ nguyên `Branch: task/<id>` và `LinkedIssue*`.

## Kiểm thử

- `TestEnsureWorktree_UsesBaseRef`, `_EmptyBaseRefUsesRepoDefault`, `_DefaultLookupFails_ErrorNoHeadFallback`, `_ReuseExistingIgnoresBaseRef`, `_AdoptsIssueWorktree`.
- `TestAdvanceExecution_Dispatch_PassesBaseRef_BothPaths`.
- Hợp đồng execution flow (`execution_flow_contract.go`): kịch bản Request có `base_ref`; fake task service ghi nhận `BaseRef` nhận được.
- Git thật: checkout chính ở nhánh thứ ba; worktree task rẽ từ nhánh đã chọn / nhánh mặc định, không từ nhánh thứ ba.
- `go test ./services/task-service/... ./services/request-service/internal/usecase/... -run 'BaseRef|Worktree|Dispatch' -v`.

## Tiêu chí hoàn thành

- [x] Cả hai đường dispatch mang `base_ref`.
- [x] Không còn đường nào tạo worktree task từ HEAD hiện tại.
- [x] Mọi fake và adapter biên dịch với chữ ký mới; test cũ xanh.

## Rủi ro và lưu ý

- Đổi chữ ký cổng có blast radius cao; liệt kê mọi nơi dùng bằng `gitnexus_impact` và ghi vào PR.
- Đổi mặc định của task từ HEAD sang nhánh mặc định: ghi chú phát hành.

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

### Tệp đã đổi

- Proto: `backend-go/proto/orca/task/v1/task.proto` (`TaskServiceExecuteRequest.base_ref = 7`, số trống kế tiếp), mã sinh bằng `protoc` (`task.pb.go`, `task_grpc.pb.go`; dòng phiên bản của `protoc` trong tiêu đề tệp sinh đổi theo công cụ cục bộ, không có thay đổi nào khác).
- task-service: `usecase/execute_task.go` (`ExecuteTaskInput.BaseRef`, truyền vào `EnsureWorktree`), `usecase/ports.go` (chữ ký `WorktreeProvisioner.EnsureWorktree(ctx, tenantID, task, baseRef)`), `adapter/grpc/server.go` (ánh xạ `base_ref`), `adapter/grpcclient/worktree_provisioner.go`. `simple_executor.go` không phải sửa: nó không gọi `EnsureWorktree` (chỉ có một câu chú thích nhắc tên), cuộc gọi duy nhất nằm ở `usecase/execute_task.go`.
- request-service: `usecase/ports_execution.go` (`ExecuteOptions{BaseRef}`, `TaskClient.Execute(..., opts)`, `ContractRun.BaseRef`), `usecase/advance_execution.go` (`dispatch` gán `req.BaseRef` cho cả hai đường), `adapter/grpcclient/task_client.go` (`Execute`, `ExecuteContract`, `UnavailableTaskClient`), `adapter/contracttest/execution_fake_task_service.go` (ghi `ExecuteCall{TaskID, BaseRef, Contract}`), `adapter/contracttest/execution_flow_contract.go` (đăng ký kịch bản mới, thêm trường `advance`, `gofmt` một dòng trống thừa có sẵn), `adapter/contracttest/execution_flow_base_ref_contract.go` (mới).
- Test mới: task-service `adapter/grpcclient/worktree_base_ref_test.go`, `adapter/grpcclient/worktree_real_git_test.go`, `adapter/grpc/server_execute_base_ref_test.go`, `usecase/execute_task_base_ref_test.go`; request-service `usecase/advance_execution_base_ref_test.go`. Fake cũ cập nhật chữ ký: `usecase/fakes_test.go`, `adapter/grpc/server_test.go` (thêm `newTestServerWithWorktrees`), `worktree_provisioner_test.go` (fake git-gateway có `BaseRefDefault`, mặc định trả `main`), `worktree_issue_reuse_test.go`; request-service `execution_fakes_test.go`, `contract_run_planner_test.go`, `adapter/grpc/server_execution_test.go`, `adapter/grpcclient/task_client_test.go`.

### Blast radius (tìm bằng grep và `go vet`, không có GitNexus)

- `usecase.TaskClient.Execute` (request-service): bên gọi duy nhất `AdvanceExecution.dispatch`; bên cài: `grpcclient.TaskClient`, `UnavailableTaskClient`, fake `exTasks` (và `fixTaskFake`, `contractTasks` nhúng nó), `fakeTasks` của `adapter/grpc`.
- `usecase.ContractTaskClient.ExecuteContract`: chữ ký không đổi, `ContractRun` thêm trường.
- `usecase.WorktreeProvisioner.EnsureWorktree` (task-service): bên gọi duy nhất `ExecuteTask.Execute`; bên cài: `grpcclient.WorktreeProvisioner`, `fakeWorktreeProvisioner`, `stubWorktreeProvisioner`.
- `TaskServiceExecuteRequest` còn được api-gateway dựng (`httpgateway/task_routes.go`, `wscompat/channels_automation_task.go`); trường mới tùy chọn nên không phải sửa, nhưng hành vi các đường đó đổi (xem Lưu ý phát hành).

### Hành vi

- `baseRef` rỗng: `BaseRefDefault{RepoId}` (gửi kèm tenant và user như `CreateWorktree`); lỗi RPC hoặc `ref` rỗng là lỗi tường minh, `ExecuteTask` bọc thành `TASK_EXECUTE_WORKTREE_FAILED` kèm nguyên nhân (trước khi ghi trạng thái), request-service coi mã này là lỗi dispatch tạm thời và thử lại trong `DispatchRetryWindow`. Không có đường rơi về HEAD.
- Nhánh dùng lại (`task.WorktreeID` còn tồn tại, `findIssueWorktree`) bỏ qua `baseRef`. Worktree đã bị xoá thì tạo lại và dùng `baseRef` (đúng ý: Request vẫn khoá ref).
- Worktree dùng chung của Plan: `AdvanceExecution` luôn truyền `req.BaseRef`; các task sau được `SetWorktree(shared)` nên task-service chỉ dùng lại worktree và bỏ qua ref, nên chỉ lần tạo worktree đầu của Plan tiêu thụ ref. Truyền đều ở mọi lần dispatch (thay vì chỉ lần đầu) để worktree dùng chung bị xoá giữa chừng vẫn được tạo lại đúng ref.
- `Request` được tải mới trong từng lần `Execute`/`Run` (mọi bên gọi `Evaluate.Run`/`Advance.Execute` đều `Requests.Get` ngay trước khi chạy, dưới khoá Request), không có bản sao lưu giữ giữa hai lần chạy.
- Phụ thuộc vào luật khoá của task 03 (Request `executing` được tính là đã có worktree nên `base_ref` bị khoá): không cài ở đây.

### Lệnh đã chạy

- `cd backend-go/services/task-service && go build ./... && go vet ./... && go test ./... -race -count=1`: xanh.
- `cd backend-go/services/request-service && go build ./... && go vet ./... && go test ./... -race -count=1`: xanh (tất cả gói).
- `go vet -tags=integration ./internal/adapter/...` và `DOCKER_HOST=unix:///Users/datbeohbbh/.colima/default/docker.sock TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE=/var/run/docker.sock go test -tags=integration ./internal/adapter/postgres/ ./internal/adapter/mysql/ -run ExecutionFlow -count=1 -p 1 -v`: Postgres 16 và MySQL 8 đều xanh, gồm kịch bản mới `ExecutionFlow_BaseRef_ReachesTaskServiceOnBothDispatchPaths` (5 ca: plain/contract có ref, contract với ref đặt sau khi tạo Request, plain/contract không ref; mỗi ca có 2 task để thấy cả lần dispatch thứ hai trong Plan).
- Đột biến thử: bỏ khối tra `BaseRefDefault` trong provisioner thì 4 test (2 test git thật, 2 test đơn vị) đỏ; đã hoàn nguyên.
- Test git thật (`worktree_real_git_test.go`): kho gốc có `main` và `release/x`, bản clone checkout nhánh thứ ba `feature/third` (ba commit khác nhau); fake git-gateway chạy `git worktree add` và `git symbolic-ref refs/remotes/origin/HEAD` thật (bỏ điểm bắt đầu khi ref rỗng, đúng hành vi HEAD của git, nên hồi quy sẽ lộ ra). Chứng minh: ref rõ ràng thì HEAD worktree == SHA của `release/x`; rỗng thì == SHA của `main`, không phải nhánh thứ ba (cả task thuộc Request và task chạy lẻ); mất `origin/HEAD` thì lỗi, không tạo thư mục worktree và không tạo nhánh `task/<id>`.

### Lưu ý phát hành

- Mọi đường chạy task (kể cả task lẻ từ UI/api-gateway và automation, không có `base_ref`) giờ rẽ từ nhánh mặc định của kho (`BaseRefDefault`), không còn từ HEAD đang checkout ở checkout chính. Nếu git-gateway không xác định được nhánh mặc định thì lần chạy lỗi `TASK_EXECUTE_WORKTREE_FAILED` thay vì lặng lẽ rẽ từ HEAD.
- `BaseRefDefault` thật (`localgit.Executor`) trả tên nhánh cục bộ (`main`), nên worktree rẽ từ nhánh cục bộ đó chứ không phải `origin/main` mới nhất (`CreateWorktree` của git-gateway không fetch); đây là hành vi sẵn có của git-gateway, ngoài phạm vi.

### Chưa kiểm chứng

- Chưa chạy với task-service, git-gateway và agent thật (không chạy e2e theo quy tắc phiên); đường `relay` của git-gateway cho `BaseRefDefault`/`CreateWorktree` chỉ được kiểm gián tiếp qua fake.
- `golangci-lint` không chạy được; chỉ `gofmt` và `go vet`.
- api-gateway không biên dịch trên cây hiện tại (việc của người khác) nên không build lại; trường proto mới là tùy chọn nên không ảnh hưởng.
