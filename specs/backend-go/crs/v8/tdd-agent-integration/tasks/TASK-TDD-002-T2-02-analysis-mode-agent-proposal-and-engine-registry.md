# TASK-TDD-002-T2-02: Chế độ `agent_proposal`, nối EngineRegistry vào luồng, PinSolutionEngine thật

**From Solution:** [SOL-TDD-002](../solutions/SOL-TDD-002-tdd-agent-as-knowledge-hub.md) mục 3.4 (bước 1)
**Checklist:** [T2.2](CHECKLIST-TDD-002-knowledge-hub.md)
**Priority:** P0 · **Area:** request-service (Go)
**Files:** `internal/domain/analysis_run.go`, `internal/usecase/{generate_solution,generate_plan,solution_engine,engine_native,pin_solution_engine}.go`, `internal/adapter/{postgres,mysql}/analysis_run_repository.go`, `cmd/server/wire_*.go`, test đi kèm
**Depends on:** T1.9
**Status:** `[ ]` TODO

---

## Context

CR-REQ-026 TASK-05 định nghĩa giao diện `SolutionEngine` (`Preflight`, `GenerateAnalysis`, `GeneratePlan`, `OnRequestCompleted`) và `EngineRegistry`; mã hiện có `DefaultEngineRegistry`, `nativeEngine` (bọc luồng cũ), nhưng **registry không được dùng ở đâu** và `PinSolutionEngine.Execute` là bản giả trả `EngineNative`. Cột `analysis_runs.mode` đã cho phép `agent_proposal` (migration 0006) nhưng Go chỉ có `complete` và `agent_readonly`.
Mục tiêu của task: đưa cơ chế chọn engine vào luồng mà **không đổi hành vi** khi engine là `native`.

## Việc cần làm

1. Đọc migration `0007_solution_engines` và `0006_analysis_runs` để xác nhận cột có sẵn (`project_engine_settings.solution_engine`, cột ghim engine của Request nếu có); ghi cột còn thiếu (cột `engine`, `proposal_branch` của `analysis_runs` theo CR-026 mục H).
2. `domain/analysis_run.go`: thêm `AnalysisModeAgentProposal = "agent_proposal"`, cập nhật mọi hàm kiểm hợp lệ và ánh xạ; hai repository đọc/ghi giá trị mới; truy vấn đếm run đồng thời (`CountRunning`) tính cả `agent_proposal` như run agent.
3. Thêm migration mới nếu thiếu cột (số kế tiếp sau `0093`, hai dialect, có `down`).
4. `PinSolutionEngine` thật: lần sinh đầu của Request đọc `ProjectEngineSettings` và ghim `EffectiveEngine(pinned, settings, profile)` vào Request (idempotent, không đổi giữa các lần sinh lại); trả engine đã ghim.
5. `GenerateSolution` và `GeneratePlan` nhận `EngineRegistry` + `PinSolutionEngine`; engine `native` đi đúng đường cũ (không đổi dòng nào của hành vi); engine `openspec` ở `GeneratePlan` tạo run `Kind=plan`, `Mode=agent_proposal`.
6. Nối trong `cmd/server` (composition): đăng ký `nativeEngine` và `openspecEngine` (T2.4 hiện thực nốt).
7. Cập nhật `analysis_runner.worker()` nếu cần (run `plan` luôn đi worker `plan`, `agent_proposal` do worker này xử lý trong T2.4).

## Tiêu chí hoàn thành

- [ ] Mọi test hiện có của request-service xanh khi engine là `native` (mặc định).
- [ ] `agent_proposal` đọc/ghi được ở cả Postgres và MySQL.
- [ ] Ghim engine idempotent: sinh lại không đổi engine dù cài đặt project đổi.
- [ ] Không còn hàm giả trong `PinSolutionEngine`.

## Kiểm thử

- Test đơn vị: chọn engine theo (ghim, cài đặt, profile); ghim lần đầu và lần sau.
- Test hợp đồng hai dialect cho `mode` mới và `CountRunning`.
- Test hồi quy: toàn bộ `go test ./...` của service.

## Rủi ro và ghi chú

- `EffectiveEngine` trả `native` khi profile là `none`; loại Request không có profile OpenSpec không bao giờ vào engine `openspec`.
- Chạy `gitnexus_impact` cho `GeneratePlan`, `GenerateSolution`, `analysis_runner` trước khi sửa và báo mức rủi ro.
