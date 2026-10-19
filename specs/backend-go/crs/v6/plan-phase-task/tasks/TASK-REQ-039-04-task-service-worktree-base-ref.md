# TASK-REQ-039-04: task-service: nhận `base_ref`, tạo worktree thực thi từ ref đã chọn hoặc nhánh mặc định

**From Solution:** [BE-REQ-SOL-039](../solutions/BE-REQ-SOL-039-request-worktree-base-branch.md)
**Priority:** P1
**Service:** `task-service, request-service`
**File:** `proto/orca/task/v1/task.proto` (sửa) + mã sinh, `services/task-service/internal/{usecase/execute_task.go,usecase/ports.go,adapter/grpc/server.go,adapter/grpcclient/worktree_provisioner.go}` (sửa); `services/request-service/internal/{usecase/ports_execution.go,usecase/advance_execution.go,adapter/grpcclient/task_client.go}` (sửa) và mọi fake
**Depends on:** TASK-REQ-039-02
**Status:** [ ] TODO

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

- [ ] Cả hai đường dispatch mang `base_ref`.
- [ ] Không còn đường nào tạo worktree task từ HEAD hiện tại.
- [ ] Mọi fake và adapter biên dịch với chữ ký mới; test cũ xanh.

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
