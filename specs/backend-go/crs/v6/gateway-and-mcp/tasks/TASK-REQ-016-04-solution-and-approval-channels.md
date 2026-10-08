# TASK-REQ-016-04: Kênh `solution.*` và `approval.*`

**From Solution:** BE-REQ-SOL-016
**Priority:** P0
**Service:** `api-gateway`
**File:** `backend-go/services/api-gateway/internal/adapter/wscompat/channels_solution.go` (mới), `.../channels_approval.go` (mới), `.../channels_solution_test.go`, `.../channels_approval_test.go` (mới), `.../excluded_channels.yaml`
**Depends on:** TASK-REQ-016-02; RPC `ListSolutions`, `GenerateSolution`, `ChooseSolutionOption` (CR-REQ-007), `ApprovalService` (CR-REQ-009)
**Status:** [x] DONE (đã kiểm chứng 2026-10-08: cd backend-go/services/api-gateway && go build ./... && go vet ./... && go test ./... -count=1)

---

## Context

- Proto CR-REQ-007 mục 2.7: `ListSolutionsResponse{solutions, runs, next_page_token}`, `GenerateSolutionRequest{request_id, idempotency_key, feedback, analysis_mode}`, `GenerateSolutionResponse{solution_id, run_id}` (bất đồng bộ), `ChooseSolutionOptionRequest{request_id, solution_id, option_id, comment}`, `ChooseSolutionOptionResponse{solution, approval_digest}`.
- Proto CR-REQ-009 mục 2.6: `Approval{id, request_id, subject_type, subject_id, stage, status, requested_by, decided_by, decided_at, comment, due_at, version, subject_digest, created_at}`; `DecideApprovalRequest{approval_id, comment, expected_version, expected_digest}`; `DecideApprovalResponse{approval, request_status}`; `CancelApprovalRequest{approval_id, reason}`.
- CONTRACT mục 2.2 và 2.3; mã lỗi mục 5.

## Việc cần làm

1. `channels_solution.go`: `solution.list`, `solution.generate`, `solution.choose` (8s mỗi kênh). `solution.generate` kiểm `feedback` ≤ 2000 ký tự ở gateway (lỗi `INVALID_ARGUMENT`), `analysisMode` ∈ `complete|agent_readonly` (rỗng bằng 0). `solution.choose` trả `{solution, approvalDigest}`.
2. `channels_approval.go`: `approval.get`, `approval.list`, `approval.listPending`, `approval.approve`, `approval.reject`, `approval.cancel`. `approve` và `reject` bắt buộc `expectedVersion` (số nguyên >= 1) và `expectedDigest` (chuỗi không rỗng) ở gateway, không thì `INVALID_ARGUMENT: expectedVersion and expectedDigest are required`; `reject` bắt buộc `comment` khác rỗng sau `strings.TrimSpace` (service cũng kiểm, trả `REQUEST_APPROVAL_COMMENT_REQUIRED`). Trả `{approval, requestStatus}`.
3. `approval.listPending`: không nhận người dùng trong tham số; danh tính chỉ từ `Identity`.
4. Không đăng ký `approval.request` (CONTRACT D4).
5. `excluded_channels.yaml`: thêm 9 kênh, `reason: "Chờ BE-REQ-SOL-017"` (task 017-01 sẽ chuyển `solution.choose`, `approval.approve|reject|cancel` thành loại trừ vĩnh viễn và gỡ dòng tạm của kênh có tool).

## Kiểm thử

- `fakeRequestClient` và `fakeApprovalClient`: ánh xạ tham số (`optionId` đến `OptionId`, `id` đến `ApprovalId`), `expectedVersion` và `expectedDigest` bắt buộc, thiếu một trong hai thì không gọi RPC (fake đếm lời gọi bằng 0).
- `solution.generate` trả `{solutionId, runId}` và deadline 8s.
- `solution.list` kết quả rỗng là `{solutions: [], runs: [], nextPageToken: ""}`.
- `approval.listPending` không có đường nhận `userId` (test gửi `userId` giả).
- Lệnh: `go test ./internal/adapter/wscompat/... -run 'Solution|Approval'`.

## Tiêu chí hoàn thành

- [x] 9 kênh đăng ký và có test; parity xanh.
- [x] Duyệt không có `expectedDigest` bị chặn ở gateway.
- [x] Mọi view camelCase; lỗi qua `requestChannelError`.

## Rủi ro và lưu ý

- Nếu `ListPendingForUserResponse` sau này mang thêm tiêu đề Request (CONTRACT Q3), thêm trường vào view dạng additive, không đổi kênh.
- `solutionView.Options` có thể tới 64 KB; không cắt.

## Ghi chú triển khai (2026-10-08)

9 kênh solution/approval thật; `approval.approve|reject` bắt buộc `expectedVersion`+`expectedDigest` ở gateway. Lưu ý: danh mục RPC của request-service (`rpc_catalog.go`) đánh `AgentAllowed:false` cho `GenerateSolution`, nên tool MCP `solution_generate` sẽ nhận từ chối quyền cho tới khi CR-035 đổi.
