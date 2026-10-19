# BE-REQ-SOL-039: `base_ref` trên Request: chọn nhánh gốc cho worktree đề xuất OpenSpec và worktree thực thi task

> **📝 Chưa triển khai (0/6 task).** Soạn 2026-10-19 từ đọc mã. Mọi mô tả hiện trạng chưa được kiểm bằng chạy thật.

**CR:** [CR-REQ-039](../../../../../../docs/crs/v6/plan-phase-task/CR-REQ-039-request-worktree-base-branch.md)
**Service:** `request-service` · `task-service` · `git-gateway-service` (chỉ dùng RPC có sẵn) · `api-gateway` (`wscompat`, MCP) · `proto/orca/{request,task}/v1`
**TDD tham chiếu:** [`arch/03`](../../../../tdd/architecture/03-clean-architecture-guidelines.md) (domain thuần), [`arch/05`](../../../../tdd/architecture/05-data-architecture.md) (hai dialect), [`arch/07`](../../../../tdd/architecture/07-security-architecture.md) (kiểm tham số git)
**Phụ thuộc solution:** [BE-REQ-SOL-013](./BE-REQ-SOL-013-phase-execution-and-feedback-loop.md) (worktree dùng chung của Plan), [BE-REQ-SOL-038](./BE-REQ-SOL-038-continuous-execution-activity-feed-and-execution-summary.md) (cùng chỗ ghim trên `Request`). Frontend: [FE-REQ-SOL-040](../../../../../frontend/crs/v6/request-frontend/solutions/FE-REQ-SOL-040-request-base-branch-picker.md).

---

## 1. Trạng thái hiện tại (re-verify)

Đã đọc ngày 2026-10-19:

- `request.proto`: `Request` dừng ở trường 33 (`engine` 32, `execution_mode` 33). `CreateRequestRequest` có trường 1 đến 8, không có nhánh.
- Worktree đề xuất OpenSpec: `request-service/internal/adapter/grpcclient/proposal_workspace.go` `Ensure` gọi `git.BaseRefDefault` rồi `create(... base.GetRef() ...)`; `EnsureWorkspaceInput` (`usecase/ports_workspace.go`) không có `BaseRef`. Nhánh `domain.ProposalBranch(số)`; khoá idempotency `request-proposal:<id>`.
- Worktree thực thi task: `task-service/internal/adapter/grpcclient/worktree_provisioner.go` `EnsureWorktree` gửi `CreateWorktreeRequest{ProjectId, RepoId, Branch:"task/<id>", TaskId}`, **không** `BaseRef`. `agent/src/relay/agent-git-worktree-handler.ts`: không có `baseRef` thì `git worktree add -b <branch> <path>` rẽ từ HEAD của `cwd`, tức nhánh đang checkout ở checkout chính.
- `request-service` gọi task-service qua hai đường: `TaskExecutor.Execute(ctx, taskID, executionRequestID)` (`usecase/ports_execution.go:42`) và `ContractTaskClient.ExecuteContract(ctx, taskID, executionRequestID, run ContractRun)` (`:167`), chọn trong `AdvanceExecution.dispatch` (`advance_execution.go:177`). Không đường nào mang nhánh. `TaskServiceExecuteRequest` (`task.proto:342`) có `task_id`, `request_id`, `prompt`, `result_nonce`, `use_knowledge_hub`, ...
- Cả hai nơi chọn repo đầu tiên của dự án (`repoFor`, `resolveRepoID`).
- Một Plan dùng chung một worktree (`advance_execution.go` `sharedWorktree`); task nhận lại worktree đã tạo cho cùng issue ngoài (`worktree_issue_reuse.go`).
- Nơi ghi nhận worktree đã có: bảng `request.openspec_changes` (`0007_solution_engines`, cột `branch`, `UNIQUE (tenant_id, request_id)`) cho worktree đề xuất; `TaskView.WorktreeID` trong cây thực thi cho worktree task.
- git-gateway: `CreateWorktreeRequest.base_ref`, `PrefetchCreateBase{repo_id, base_ref} → resolved_sha`, `BaseRefDefault`; lỗi `WORKTREE_BASE_REF_NOT_FOUND` kèm danh sách `available`.
- Migration mới nhất của `request-service`: `0098_execution_mode` (cả hai dialect). Mẫu ghim ở repository: `PinExecutionModeIfUnset` (`adapter/postgres/request_repository.go:167`); cột trong `request_scan.go`; khai báo cột cho test hợp đồng ở `adapter/contracttest/expected_columns_execution_mode.go`.

### Correction relative to CR-REQ-039

1. CR viết `Tasks.Execute` truyền `base_ref`; thực tế có **hai** đường (`Execute` và `ExecuteContract`) và cổng `Execute` không có chỗ cho tham số, nên đổi chữ ký cổng (task 04) và cập nhật mọi fake.
2. CR nêu khoá "đã có worktree" suy ra; cần một cổng đọc rõ ràng (`WorktreePresence`) gộp `openspec_changes` và cây thực thi, không để usecase tự biết cả hai nguồn (task 02).
3. `request.proto` còn trống từ trường 34: dùng 34 (`base_ref`), 35 (`base_ref_locked`), 36 (`base_ref_lock_reason`); `CreateRequestRequest` dùng trường 9.
4. Gateway cũ bỏ qua tham số lạ nên frontend không thể tự biết `baseRef` có được áp dụng; backend phải công bố cờ tính năng: `request.flowStatus` trả thêm `features: ["request.baseRef"]` (task 05).

## 2. Giải pháp

### 2.1 Cây file (đường dẫn dưới `backend-go/`)

```
services/request-service/
  migrations/{postgres,mysql}/0099_request_base_ref.{up,down}.sql              (mới)
  internal/domain/worktree_base_ref.go                                          (mới) kiểu, kiểm cú pháp, Resolve
  internal/domain/worktree_base_ref_test.go                                     (mới)
  internal/domain/request.go                                                    (sửa) BaseRef
  internal/domain/base_ref_errors.go                                            (mới) 3 mã lỗi
  internal/usecase/set_request_base_ref.go                                      (mới) + test
  internal/usecase/create_request.go                                            (sửa) nhận BaseRef, kiểm tra
  internal/usecase/base_ref_checker.go                                          (mới) cổng kiểm ref tồn tại
  internal/usecase/worktree_presence.go                                         (mới) cổng "đã có worktree?"
  internal/usecase/ports_workspace.go                                           (sửa) EnsureWorkspaceInput.BaseRef
  internal/usecase/ports_execution.go                                           (sửa) TaskExecutor.Execute(+opts), ContractRun.BaseRef
  internal/usecase/advance_execution.go                                         (sửa) truyền base ref
  internal/adapter/postgres|mysql/request_repository.go, request_scan.go       (sửa) cột base_ref; SetBaseRef có khoá lạc quan
  internal/adapter/postgres|mysql/worktree_presence.go                          (mới)
  internal/adapter/grpcclient/proposal_workspace.go                             (sửa) dùng ref đã phân giải
  internal/adapter/grpcclient/git_base_ref_checker.go                           (mới) PrefetchCreateBase
  internal/adapter/grpcclient/task_client.go                                    (sửa) base_ref vào Execute/ExecuteContract
  internal/adapter/grpc/server_request.go (hoặc tương đương)                    (sửa) CreateRequest/SetRequestBaseRef/mapper
  internal/adapter/contracttest/expected_columns_base_ref.go, ...               (mới/sửa)
services/task-service/
  internal/usecase/execute_task.go, ports.go                                    (sửa) ExecuteTaskInput.BaseRef, WorktreeProvisioner
  internal/adapter/grpcclient/worktree_provisioner.go                           (sửa) base_ref / BaseRefDefault
  internal/adapter/grpc/server.go                                               (sửa) đọc base_ref từ RPC
services/api-gateway/internal/adapter/wscompat/channels_request.go              (sửa) request.create baseRef; request.setBaseRef (mới)
services/api-gateway/internal/adapter/mcpserver/tools/                          (sửa) request_create baseRef; request_setBaseRef
proto/orca/request/v1/request.proto, proto/orca/task/v1/task.proto              (sửa) + mã sinh
```

### 2.2 Domain: kiểu `WorktreeBaseRef` và quy tắc phân giải

`domain/worktree_base_ref.go`: `type WorktreeBaseRef string`; `ParseWorktreeBaseRef(raw string) (WorktreeBaseRef, error)`: cắt khoảng trắng hai đầu; rỗng là hợp lệ (mặc định); từ chối `REQUEST_BASE_REF_INVALID` khi: bắt đầu bằng `-`; chứa `..`, `@{`, `\`, byte NUL, ký tự điều khiển, khoảng trắng giữa chuỗi, `~`, `^`, `:`, `?`, `*`, `[`; kết thúc bằng `/`, `.lock` hoặc `.`; chứa `//`; dài hơn 255 byte. (Đây là tập con an toàn của `git check-ref-format`; git-gateway vẫn là bên kiểm cuối.) `func (r Request) WorktreeBaseRefOrDefault(defaultRef string) string`: trả `BaseRef` nếu khác rỗng, ngược lại `defaultRef`; **không bao giờ trả "HEAD hiện tại"**.

### 2.3 Đặt, sửa và khoá

- `CreateRequest`: `Input.BaseRef`; `ParseWorktreeBaseRef`; kiểm tồn tại bằng `BaseRefChecker` (mục 2.4) nhưng **fail-open** khi hạ tầng không với tới (dev server offline, agent không phản hồi): vẫn lưu, ghi cảnh báo trong kết quả (`base_ref_unverified=true`) để giao diện báo; chỉ **fail-closed** khi git-gateway trả rõ `WORKTREE_BASE_REF_NOT_FOUND`.
- `SetRequestBaseRef{request_id, base_ref, expected_version}` (RPC mới): quyền như `ChangeRequestType`; từ chối `REQUEST_BASE_REF_LOCKED` khi `WorktreePresence.Has(ctx, requestID)`; cập nhật bằng `UPDATE ... SET base_ref=$, version=version+1 WHERE id=$ AND version=$expected`; không khớp thì `REQUEST_VERSION_CONFLICT` (C12). Kiểm khoá và cập nhật trong **cùng transaction** (đọc khoá hàng) để không có khe giữa kiểm và ghi.
- `WorktreePresence.Has`: `true` nếu có dòng `openspec_changes` (không `abandoned`) hoặc có task của Request với `worktree_id` khác rỗng. Cài một lần cho mỗi dialect, trả cả lý do (`proposal` hoặc `task`) để giao diện giải thích.
- Request ở trạng thái cuối (`completed`, `cancelled`) không sửa: `REQUEST_BASE_REF_LOCKED`.

### 2.4 Kiểm tồn tại

`BaseRefChecker.Check(ctx, projectID, baseRef) (CheckResult, error)` cài bằng `git-gateway.PrefetchCreateBase{repo_id, base_ref}`; repo là repo đầu của dự án (`ListRepos`, giống `repoFor`). Timeout 10 giây. Phân loại: `Found`, `NotFound` (kèm tối đa 5 gợi ý từ `available`), `Unreachable` (lỗi mạng, dev server offline, `DeadlineExceeded`).

**Điều chỉnh khi triển khai TASK-039-03 (2026-10-19).**
- `PrefetchCreateBase` của git-gateway nay phân loại: ref không có (cả trên remote lẫn trong clone cục bộ) là `NotFound` mã `WORKTREE_BASE_REF_NOT_FOUND` kèm `available: ...` như `CreateWorktree`; remote/dev server không với tới là `Unavailable` hoặc `DeadlineExceeded`. Executor relay không còn gọi `git.fetchRef` (agent không có) mà dùng `git.exec fetch` + `rev-parse`.
- Lý do khoá có thêm `execution`: Request ở trạng thái `executing` luôn coi là đã có worktree (worktree task được tạo ở task-service ngay khi giao việc, chỉ khi Request đang thực thi). Các giá trị: `proposal`, `task`, `execution`, `terminal`.
- Khe giữa các dịch vụ được đóng bằng `ReservingProposalWorkspace`: mọi `Ensure` giữ `LockRequest`, đọc `base_ref` mới từ DB, ghi dấu `preparing` rồi mới tạo worktree ngoài khoá. Dấu bị `abandoned` khi git nói ref không tồn tại (chắc chắn chưa tạo gì), giữ nguyên với mọi lỗi khác.
- Ref do người dùng chọn không bao giờ được gửi trần cho `git worktree add`: nếu ref chỉ có dạng `origin/<tên>`, git (2.54) tạo nhánh cục bộ trùng tên ref và checkout nó thay cho nhánh đề xuất. Gửi dạng đầy đủ `refs/heads/`, `refs/remotes/origin/`, `refs/tags/` theo thứ tự đó. Task 04 phải làm tương tự cho worktree của task.

### 2.5 Truyền xuống

1. `ProposalWorkspace.Ensure`: nhận `EnsureWorkspaceInput.BaseRef`; `base := in.BaseRef`; nếu rỗng thì `BaseRefDefault`; truyền vào `create` và `existingFor` như hiện nay.
2. `AdvanceExecution.dispatch`: tính `baseRef := req.BaseRef` và truyền cho cả hai đường. `TaskExecutor.Execute(ctx, taskID, executionRequestID string, opts ExecuteOptions) error` với `ExecuteOptions{BaseRef string}`; `ContractRun` thêm `BaseRef`. Cập nhật `TaskClient` (gRPC) và mọi fake (`contracttest/execution_fake_task_service.go`, test usecase).
3. `task.proto` `TaskServiceExecuteRequest.base_ref` (số trường trống kế tiếp, kiểm bằng `buf`); `task-service/adapter/grpc/server.go` đọc vào `ExecuteTaskInput.BaseRef`; `WorktreeProvisioner.EnsureWorktree(ctx, tenantID, task, baseRef)`.
4. `worktree_provisioner.go`: nếu `baseRef` rỗng thì gọi `git.BaseRefDefault{repo_id}` và dùng kết quả; truyền vào `CreateWorktreeRequest.BaseRef`. Nếu `BaseRefDefault` lỗi thì lỗi rõ (`TASK_EXECUTE_WORKTREE_FAILED` kèm nguyên nhân), **không** rơi về HEAD.
5. Worktree nhận lại (`task.WorktreeID` đã có hoặc `findIssueWorktree`): giữ nguyên, bỏ qua `baseRef`; ghi log mức info (không nội dung Request).

### 2.6 Gateway và MCP

`request.create` nhận `baseRef`; `request.setBaseRef {id, baseRef, expectedVersion}` (`GroupWrite`, thêm vào `request_rpc_channel_map` và chính sách OPA `agent_rpcs` nếu agent được gọi); `request.get`/`list` trả `baseRef`, `baseRefLocked`, `baseRefLockReason`; `request.flowStatus` trả thêm `features` (mảng chuỗi, gồm `request.baseRef`) để frontend biết runtime hỗ trợ. MCP `request_create` nhận `baseRef`; thêm `request_setBaseRef` hoặc dòng loại trừ có lý do; `parity_test` xanh. Cập nhật `CONTRACT-request-ui-api.md` (mục mới 2.11).

### 2.7 Mã lỗi

`REQUEST_BASE_REF_INVALID` (InvalidArgument), `REQUEST_BASE_REF_NOT_FOUND` (NotFound, kèm gợi ý trong `details`), `REQUEST_BASE_REF_LOCKED` (FailedPrecondition, kèm `reason`).

## 3. Quyết định thiết kế

| # | Quyết định | Lý do |
|---|---|---|
| S1 | `base_ref` là tên nhánh/tag, không lưu SHA | Phân giải lúc tạo worktree, sau khi fetch |
| S2 | Quy tắc phân giải ở một hàm miền | Hai luồng (đề xuất, task) không thể lệch |
| S3 | Đổi chữ ký cổng `TaskExecutor.Execute` thay vì type-assertion thêm | Không để đường thứ ba lặng lẽ thiếu nhánh |
| S4 | Fail-open khi hạ tầng không với tới, fail-closed khi git nói không có ref | Không chặn tạo Request vì agent offline, nhưng không nhận giá trị sai |
| S5 | Kiểm khoá và ghi trong cùng transaction | Không có khe tạo worktree xen giữa |
| S6 | Task bỏ HEAD, dùng nhánh mặc định khi rỗng | Đúng yêu cầu "mặc định là nhánh mặc định" |

**Giả định chờ xác nhận (câu hỏi mở của CR chưa được trả lời):** (a) không dùng `worktreeBaseRef` mức repo (Q1); (b) chấp nhận đổi mặc định của task sang nhánh mặc định (Q2, suy ra từ yêu cầu gốc); (c) worktree nhận lại từ issue khoá `base_ref` (Q3); (d) chỉ nhánh và tag, chưa SHA (Q4); (e) không gợi ý nhánh từ issue/PR (Q5).

## 4. Phụ thuộc và thứ tự

01 migration và repository → 02 miền, tạo/sửa, khoá → 03 worktree đề xuất và kiểm ref → 04 task-service và cổng thực thi → 05 gateway, MCP, CONTRACT → 06 tích hợp và e2e. 03 và 04 song song sau 02; 05 cần 02.

## 5. Kiểm thử

- Unit miền: bảng `ParseWorktreeBaseRef` (hợp lệ, từng loại ký tự bị cấm, độ dài), `WorktreeBaseRefOrDefault`.
- Usecase có fake: `CreateRequest` (rỗng, hợp lệ, không hợp lệ, `NotFound`, `Unreachable`); `SetRequestBaseRef` (chưa khoá, khoá do đề xuất, khoá do task, trạng thái cuối, xung đột phiên bản, hai lời gọi đồng thời).
- Hợp đồng hai dialect: cột `base_ref`, mặc định rỗng, CHECK độ dài, `SetBaseRef` có khoá lạc quan, `WorktreePresence` cho cả hai nguồn.
- Git thật (testcontainers hoặc repo tạm, **không mock git**): repo có `main`, `release/x`, checkout chính ở nhánh thứ ba; tạo worktree đề xuất và worktree task; kiểm `git rev-parse` của worktree bằng SHA của nhánh đã chọn; ca không chỉ định thì bằng SHA của nhánh mặc định, **không** của nhánh thứ ba.
- Gateway: `wscompat` cho `request.create`, `request.setBaseRef`; `parity_test` MCP.
- E2E: suite `suite_request_base_ref.py` và spec UI trên repo fixture tài chính (`tests/stages/e2e/fixtures/ledgerlite-finance/`), xem task 06.

## 6. Rủi ro và điểm chưa kiểm chứng

- **Blast radius của cổng `TaskExecutor`**: mọi fake và adapter phải đổi cùng lúc; chạy `gitnexus_impact` cho `TaskExecutor.Execute` và `AdvanceExecution.dispatch`.
- **Đổi mặc định task** có thể làm khác kết quả của dự án đang chạy; ghi chú phát hành.
- **`PrefetchCreateBase` có fetch mạng**: đo thời gian trên SSH; timeout 10 giây có thể chưa đủ cho repo lớn (chưa đo).
- **Nhiều repo trong dự án**: `base_ref` gắn với repo đầu tiên; chưa hỗ trợ chọn repo.
- Chưa kiểm hành vi `PrefetchCreateBase` với tag và nhánh remote có `/` trên mọi phiên bản git.

## 7. Câu hỏi mở

Như CR-REQ-039 mục 7 (năm câu); các giả định ở mục 3 là mặc định tạm cho đến khi được trả lời.

## 8. Tham chiếu

- `backend-go/services/request-service/internal/adapter/grpcclient/proposal_workspace.go`, `usecase/{create_request,advance_execution,ports_execution,ports_workspace}.go`, `adapter/postgres/request_repository.go`
- `backend-go/services/task-service/internal/adapter/grpcclient/{worktree_provisioner,worktree_issue_reuse}.go`, `usecase/execute_task.go`
- `backend-go/proto/orca/gitgateway/v1/gitgateway.proto` (`CreateWorktreeRequest`, `PrefetchCreateBase`, `BaseRefDefault`)
- `agent/src/relay/agent-git-worktree-handler.ts`
- [CR-REQ-039](../../../../../../docs/crs/v6/plan-phase-task/CR-REQ-039-request-worktree-base-branch.md), kế hoạch tổng: [`IMPLEMENTATION-PLAN-CR-REQ-039-042`](../../../../../IMPLEMENTATION-PLAN-CR-REQ-039-042.md)
