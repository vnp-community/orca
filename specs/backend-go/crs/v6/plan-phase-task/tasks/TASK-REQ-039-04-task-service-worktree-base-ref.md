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

## Bổ sung 2026-10-19: phân giải ref ở agent

### Lỗi

Với git 2.54.0 (Apple Git-157), trong bản clone mà `release/y` chỉ tồn tại dưới dạng `origin/release/y`, lệnh `git worktree add -b task/1 ../wt1 release/y` tạo nhánh cục bộ `release/y` (theo dõi origin) và checkout nhánh đó; nhánh `task/1` không bao giờ được tạo. Worktree của task thế là nằm trên (và sẽ commit vào) nhánh release của người dùng. Nguyên nhân: `agent/src/shared/worktree-base-ref.ts` coi đoạn đầu của tên có dấu `/` là tên remote (`refs/remotes/<name>`, `refs/heads/<name>`), nên `release/y` không bao giờ thành `refs/remotes/origin/release/y` và đi vào git ở dạng trần. Thêm nữa, `handleGitWorktreeAdd` (`git.worktree.add`) truyền `baseRef` thô cho git, không qua bộ phân giải nào.

### Sửa

- `agent/src/shared/worktree-base-ref.ts`: `resolveWorktreeStartPoint(baseRef, probe)` thay `resolveWorktreeAddBaseRef`. Thứ tự: bắt đầu bằng `refs/` thì giữ; nhánh cục bộ `refs/heads/<n>`; `refs/remotes/<n>` khi `<n>` bắt đầu bằng tên remote có thật + `/`; `refs/remotes/origin/<n>`; đúng một remote khác có `refs/remotes/<remote>/<n>` (nhiều hơn một là lỗi `AMBIGUOUS`, không đoán); `refs/tags/<n>`; cuối cùng commit-ish được xác thực bằng `rev-parse --verify --quiet <x>^{commit}` (trả về SHA đầy đủ). Tên rỗng hoặc bắt đầu bằng `-` bị từ chối (`INVALID`) trước mọi lệnh git. Không tìm thấy là `NOT_FOUND` với thông điệp chứa `invalid reference`.
- `agent/src/relay/worktree-start-point-git-probe.ts`: probe dùng `rev-parse --verify --quiet` và `git remote` (chạy được trên git 2.25). Exit 1 là "không có ref"; lỗi khác (không phải repo, thiếu git, timeout) được ném lại chứ không bị báo là ref không tồn tại.
- `agent/src/relay/agent-git-worktree-handler.ts`: `handleGitWorktreeAdd` phân giải điểm bắt đầu trước khi gọi `git worktree add -b`, từ chối `branch` bắt đầu bằng `-`, và sau khi thêm worktree kiểm `git symbolic-ref --short HEAD` bằng `worktree-add-branch-guard.ts`; sai nhánh thì `git worktree remove --force` rồi trả lỗi `ServerError`. `NOT_FOUND` được trả dưới dạng kết quả kiểu git (`exitCode` 128, `stderr` = `fatal: invalid reference: ...`) nên `isBaseRefNotFoundErr` của git-gateway-service (chứa `invalid reference`) vẫn phân loại ra `WORKTREE_BASE_REF_NOT_FOUND`; `AMBIGUOUS`/`INVALID` là lỗi `InvalidParams`.
- `agent/src/relay/git-handler-worktree-ops.ts` (`addWorktreeOp`, đường SSH relay `git.addWorktree`) dùng cùng bộ phân giải và cùng bước kiểm nhánh.

### Kiểm thử

- `agent/src/relay/__tests__/agent-git-worktree-start-point.test.ts` (git thật, kho dựng trong test bởi `worktree-start-point-repo-fixture.ts`): nhánh cục bộ `release/x`; `release/y` chỉ có trên origin (tên trần và `origin/release/y`); tag `v1`; nhánh cục bộ trùng tên nhánh remote (`release/dup`); `feature/z` trên hai remote (lỗi mơ hồ, không tạo thư mục; `upstream/feature/z` chạy được); checkout hiện tại ở nhánh thứ ba `other` (không ảnh hưởng điểm bắt đầu); `--detach` bị từ chối; SHA đầy đủ/ngắn; ref đã qualified; ref không tồn tại. Ở mỗi ca worktree nằm trên nhánh MỚI và `HEAD` == SHA của ref dự định.
- Hồi quy: test chạy đúng lệnh cũ bằng git trần (`git worktree add -b task/plain <path> release/y`) và khẳng định worktree KHÔNG nằm trên `task/plain` (tạo ra nhánh `release/y` cục bộ), rồi chứng minh handler cho `task/<n>` đúng SHA của `origin/release/y` và không tạo nhánh `release/y`. Đột biến thử: đổi handler về truyền `baseRef` thô thì test hồi quy và test SSH đỏ (bước kiểm nhánh cũng bắt được: "worktree was created on branch "release/y" instead of..."); đã hoàn nguyên.
- Test đơn vị thuần cho thứ tự ưu tiên: `agent/src/shared/worktree-base-ref.test.ts`. Test mock cũ của `addWorktreeOp` cập nhật theo lời gọi mới (`git-handler-worktree-ops.test.ts`, khối `addWorktree` trong `git-handler.test.ts`), có thêm ca "không nằm trên nhánh yêu cầu thì gỡ worktree".
- Lệnh đã chạy trong `agent/`: `npx vitest run src/shared src/relay/__tests__/agent-git src/relay/__tests__/agent-rpc src/relay/git-handler src/relay/agent-git` (xanh, trừ lỗi có sẵn `agent-rpc-dispatch.test.ts > notification.send ... delivered:false`); `src/relay/codeintel/security-repo-resolution.test.ts` xanh; `npx oxlint -c ../.oxlintrc.json <tệp đã đổi>` sạch; `node build.mjs` thành công.

### Blast radius

`resolveWorktreeAddBaseRef` chỉ có một bên gọi trong `agent/` (`addWorktreeOp`); `handleGitWorktreeAdd` chỉ được `agent-rpc-dispatch-git.ts` (`git.worktree.add`) gọi. Bản sao `worktree-base-ref.ts` trong `desktop/`, `backend/`, `frontend/` là gói khác, không đổi ở đây (nên áp cùng sửa nếu chúng chạm vào đường tạo worktree).

### Chưa kiểm chứng

Chưa chạy với agent và git-gateway thật (không chạy e2e theo quy tắc phiên). Chưa chạy trên git 2.25 thật; các lệnh dùng (`rev-parse --verify --quiet`, `remote`, `symbolic-ref --short`, `worktree remove --force`) đều có từ trước 2.25.

## Bổ sung 2026-10-20: BaseRefDefault

### Lỗi (e2e BRF-07)

Mọi worktree của Request không có `base_ref` xuất phát từ `BaseRefDefault` của git-gateway. Hàm này sai hai chỗ:

1. `lastRefSegment` (Go) và `.split('/').pop()` (agent) chỉ giữ đoạn sau dấu `/` cuối: `refs/remotes/origin/release/1.x` ra `1.x`, `refs/heads/feature/other` ra `other`.
2. Repo không có `origin` (fixture LedgerLite, mọi repo chỉ cục bộ): bước cuối cùng là "nhánh đang checkout" (`symbolic-ref HEAD`), nên Request không có `base_ref` bắt đầu từ một nhánh feature bất kỳ đang được checkout. Vi phạm quy tắc "mặc định = nhánh mặc định của repo".

### Thứ tự phân giải mới (Go `localgit` và agent giống hệt nhau)

(a) đích của `refs/remotes/origin/HEAD`, chỉ nhận khi nhánh đó tồn tại (cục bộ hoặc `refs/remotes/origin/<b>`), để origin/HEAD treo không thắng ứng viên thật; (b) `git ls-remote --symref origin HEAD`; (c) `preferredBranch` (nhánh mặc định cấu hình của project) khi nó là nhánh cục bộ; (d) `main` rồi `master` cục bộ; (e) nhánh đang checkout, chỉ cho repo `git init` chưa có commit. HEAD detached không panic: không có ứng viên nào thì trả lỗi. Chỉ cắt các tiền tố đã biết (`refs/heads/`, `refs/remotes/<remote>/`, tên remote không chứa `/`), nên tên nhánh có dấu `/` được giữ nguyên. Mọi kiểm tra tồn tại dùng `show-ref --verify --quiet refs/heads/<b>` (ref đầy đủ), nên gợi ý dạng `--upload-pack=x` không bao giờ bị hiểu là cờ.

### Thiết kế và hợp đồng

- `usecase.GitExecutor.BaseRefDefault(ctx, repoPath, preferredBranch)`: thêm tham số gợi ý tùy chọn (hai bản cài đặt: `localgit`, `grpcclient.RelayExecutor`, cộng agent).
- `usecase.ProjectClient.GetProjectDefaultBranch(ctx, projectID)` mới, cài bằng `GetProject` hiện có (`Project.default_branch`, field 6). Giữ `GetRepo` nguyên để không thêm một RPC cho mọi bên gọi khác. `Repo` proto không có `default_branch`, nên không cần đổi proto.
- `BaseRefDefault.Execute` tra nhánh cấu hình theo `repo.ProjectID`; lỗi tra cứu hoặc project không có ID chỉ làm mất gợi ý (không làm hỏng lời gọi).
- Dây dẫn `git.baseRefDefault` tương thích ngược: `RelayExecutor` chỉ gửi `preferredBranch` khi khác rỗng (agent cũ thấy đúng request cũ); agent mới coi trường thiếu là rỗng. Gateway `BaseRefDefaultRequest` (proto git-gateway) không đổi.

### Tệp đổi

- `backend-go/services/git-gateway-service/internal/adapter/localgit/base_ref_default.go` (mới; `BaseRefDefault`, `defaultBranchFromRemoteHead`, `branchFromRef` chuyển khỏi `executor.go` và viết lại; `executor.go` giảm 77 dòng) và `base_ref_default_test.go` (mới; 4 test cũ rời `executor_test.go`).
- `internal/usecase/ports.go`, `base_ref_default.go`, `base_ref_default_test.go`, `dispatch_test.go`, `worktree_fakes_test.go`; `internal/adapter/grpcclient/project_client.go`, `relay_executor.go`, `relay_executor_base_ref_default_test.go` (mới); `internal/adapter/grpc/server_test.go`.
- `agent/src/relay/agent-git-base-ref-resolver.ts` (mới), `agent-git-base-ref-handler.ts` (gọi resolver), `agent/src/relay/__tests__/agent-git-base-ref-handler.test.ts` (mới, git thật), `__tests__/agent-git-handler.test.ts` (bỏ khối `handleGitBaseRefDefault` cũ, đã chuyển sang tệp mới).
- `agent-rpc-dispatch-git.ts` không phải sửa: nó chuyển nguyên `rpc.params`.

### Kiểm thử (git thật, không mock git)

Bảng ca, giống nhau ở Go và TS: clone thật; đích origin/HEAD có dấu `/` (`release/1.x`) trong khi checkout đang ở `feature/work`; origin/HEAD thắng nhánh feature đang checkout; origin/HEAD treo; `init` + `remote add` qua ls-remote với nhánh có `/`; repo cục bộ với mặc định cấu hình `main` khi HEAD ở `feature/other` (trả `main`); mặc định cấu hình có `/` (có và không có `refs/heads/`); mặc định cấu hình không tồn tại cục bộ rơi về `main`; không gợi ý thì `main` thắng nhánh đang checkout; chỉ có `master`; có cả hai thì `main`; gợi ý dạng cờ bị bỏ qua; `git init` chưa commit trả nhánh hiện tại (và không trả gợi ý chưa tồn tại); remote rỗng; HEAD detached tìm được `main`; HEAD detached không ứng viên trả lỗi; thư mục không phải repo trả lỗi; bảng `branchFromRef`. Usecase: gợi ý được truyền xuống executor, lỗi tra project không làm hỏng, repo không có project ID không gọi tra cứu. Relay: `preferredBranch` đi tới agent; rỗng thì wire cũ.

Lệnh đã chạy:
- `cd backend-go/services/git-gateway-service && go build ./... && go vet ./... && go test ./... -race -count=1`: xanh.
- `cd agent && npx vitest run src/relay/__tests__/agent-git-base-ref-handler.test.ts src/relay/__tests__/agent-git-handler.test.ts`: xanh (89 test); thêm `agent-rpc-dispatch.test.ts`: chỉ lỗi có sẵn `notification.send ... delivered:false`. `node agent/build.mjs`: thành công. `oxlint` sạch, `oxfmt --check` sạch trên các tệp agent đã đổi; `tsc` không có lỗi ở tệp đã đổi.

### Blast radius (không sửa mã của họ)

Chung một RPC `GitGatewayService.BaseRefDefault` (proto và hình dạng response không đổi); đổi duy nhất là giá trị `ref` trả về cho repo cục bộ và tên nhánh có `/`:
- request-service `ProposalWorkspace.Ensure` (`proposal_workspace_create.go:87`): Request không có `base_ref` giờ xuất phát từ nhánh mặc định thật thay vì nhánh đang checkout; nhánh có `/` giờ đúng.
- task-service `worktree_provisioner.go:144`: như trên cho worktree của task.
- api-gateway kênh `repo.baseRefDefault` (`channels_repo_ssh_status_workspace.go:498`) và MCP `pack1_workspace.go`: cùng nguồn; frontend `getRuntimeRepoBaseRefDefault` (`runtime-repo-client.ts`), `use-repo-default-base-ref.ts`, `RepoRefPicker`, `SourceControl`, `useCreatePullRequestDialogFields`, `CreateRequestDialog` hiển thị và chọn mặc định theo giá trị này nên cũng sẽ thấy giá trị đúng.
- Bên cài `GitExecutor`/`ProjectClient`: chỉ trong git-gateway-service (đã cập nhật hết, kể cả fake trong test).

### Chưa kiểm chứng

Chưa chạy với agent và gateway thật hay e2e BRF-07 (không chạy e2e/docker theo quy tắc phiên); không có test hợp đồng gRPC cho `GetProjectDefaultBranch` với project-service thật (dùng `GetProject` có sẵn). Nếu project-service đặt `default_branch` mặc định `main` cho repo có nhánh khác, gợi ý chỉ được dùng khi nhánh đó tồn tại cục bộ, nên không gây hại. Chưa chạy với git 2.25: các lệnh dùng (`symbolic-ref`, `ls-remote --symref`, `show-ref --verify --quiet`) đều có từ trước 2.25; test chạy với git 2.54 cục bộ.
