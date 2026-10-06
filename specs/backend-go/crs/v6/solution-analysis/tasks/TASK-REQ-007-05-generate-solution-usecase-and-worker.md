# TASK-REQ-007-05: `GenerateSolution`, worker sinh và vòng phục hồi

**From Solution:** [BE-REQ-SOL-007](../solutions/BE-REQ-SOL-007-solution-generation-options-and-selection.md) mục E
**Priority:** P0
**Service/Area:** `request-service` / usecase
**File:** `internal/usecase/generate_solution.go` (mới), `run_solution_generation.go` (mới), `recover_interrupted_analysis_runs.go` (mới), `list_solutions.go` (mới), `internal/config/config.go` (sửa), `cmd/server/main.go` (sửa: khởi chạy vòng phục hồi), và `_test.go`
**Depends on:** TASK-REQ-007-03, TASK-REQ-007-04; CR-REQ-003 (`FlowFor`, `TransitionRequest`); TASK-REQ-009-04 (`OpenApproval`, `CancelPendingApprovalsForRequest`; dùng cổng no-op nếu chưa có)
**Status:** [ ] TODO

## Context

- Mẫu heartbeat và recovery: `task-service/internal/usecase/execution_lease.go` (`startHeartbeat`, `RunRecoveryLoop`).
- Điểm quan trọng: worker không dùng `ctx` của RPC (RPC trả ngay); dùng `context.WithoutCancel` cộng timeout riêng và khoá tenant/thông tin người dùng từ `ctx` gốc.
- Đường sinh lại có phản hồi gom trong một transaction: huỷ Approval, supersede Solution, `analysis_revision`, tạo run.

## Việc cần làm

1. `GenerateSolution.Execute(ctx, in)` theo solution mục E (bước 1 đến 6): `RequireTenantID`, quyền ghi, `FlowFor(...).Analysis.Kind == solution`, trạng thái hợp lệ, `ResolveConnection`, transaction chèn run `running` và Solution `draft`, trả `{solution_id, run_id}` (hoặc run đang chạy/idempotent).
2. `RunSolutionGeneration.Execute(runID)`: heartbeat mỗi 30s (TTL 90s) bằng goroutine dừng khi xong; dựng prompt; `Complete`; `ExtractJSONObject`; `ParseSolutionOptions` rồi `Validate(minOptions)`; sai thì một lần thử lại với lỗi cụ thể (`attempt=2`); vẫn sai thì `failed` `REQUEST_SOLUTION_INVALID_OUTPUT`; mất lease (`RenewLease` false) thì dừng và không ghi gì.
3. Thành công: một transaction (xem solution E) gồm cập nhật Solution, run `succeeded`, supersede cũ, outbox `orca.request.solution.proposed{tenant_id, request_id, solution_id, kind, option_count, recommended_option_id}`, `OpenApproval(subject_type=solution, stage=awaiting_analysis_approval)`, `TransitionRequest(analysis_ready, ExpectedFrom=analyzing)`. Lỗi thì `failed` và `DeleteDraft` cùng transaction; Request giữ `analyzing`.
4. `RecoverInterruptedAnalysisRuns.RunRecoveryLoop(ctx, 30s)`: `ClaimExpired`, đánh `failed` `REQUEST_SOLUTION_RUN_INTERRUPTED`, xoá `draft`.
5. `ListSolutions.Execute`: trả Solution và `runs` gần nhất (để UI thấy lỗi).
6. Cấu hình `REQUEST_AI_COMPLETE_TIMEOUT`, `REQUEST_ANALYSIS_LEASE_TTL`, `REQUEST_ANALYSIS_HEARTBEAT`, `REQUEST_ANALYSIS_RECOVERY_INTERVAL`; mặc định 120s, 90s, 30s, 30s.
7. Mã lỗi `REQUEST_SOLUTION_*` và `REQUEST_AI_NO_PROVIDER` đúng solution mục F.

## Kiểm thử

- Unit (fake Relay, clock, repo, `RequestGate`): thành công; lỗi AI; JSON sai rồi đúng; sai hai lần; không kết nối; hai `GenerateSolution` đồng thời trả cùng `run_id`; mất lease; sinh lại có `feedback` (Approval `cancelled`, Solution cũ `superseded`, trigger `analysis_revision`, run mới); registry không cho phép `kind` thì `KIND_NOT_ALLOWED`.
- Test thời gian: `GenerateSolution` trả trong dưới 1 giây khi Relay giả chặn 5 giây.
- Test rollback: ép `TransitionRequest` lỗi thì Solution vẫn `draft`, không có Approval, không outbox.
- Lệnh: `go test ./internal/usecase/... -run "GenerateSolution|SolutionGeneration|Recover"`.

## Tiêu chí hoàn thành

- [ ] Tiêu chí chấp nhận 2, 3, 4, 5, 6, 7, 8, 13 của CR-REQ-007 mục 4 có test tương ứng.
- [ ] Phục hồi: chạy trên hai DB ở test tích hợp của task 03/06.
- [ ] Worker không bị huỷ khi RPC kết thúc.
- [ ] Không còn Solution `draft` sau lỗi.

## Rủi ro và lưu ý

- Quyền ghi mức Request chưa chốt (câu hỏi mở 4); dùng `RequestWriteAuthorizer` giả lập reporter hoặc admin.
- Tiến trình chết khiến agent chạy tiếp ngoài tầm kiểm soát (chấp nhận ở v1).
