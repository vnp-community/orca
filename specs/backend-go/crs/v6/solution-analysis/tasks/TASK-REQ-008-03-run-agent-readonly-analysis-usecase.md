# TASK-REQ-008-03: `RunAgentReadonlyAnalysis`, giới hạn đồng thời và prompt theo `kind`

**From Solution:** [BE-REQ-SOL-008](../solutions/BE-REQ-SOL-008-diagnosis-findings-answer-analysis.md) mục B, D
**Priority:** P1
**Service/Area:** `request-service` / usecase
**File:** `internal/usecase/run_agent_readonly_analysis.go` (mới), `solution_prompt.go` (sửa, nhánh theo `kind`), `generate_solution.go` (sửa, rẽ nhánh theo registry và `analysis_mode`), `internal/config/config.go` (sửa), và `_test.go`
**Depends on:** TASK-REQ-008-01, TASK-REQ-008-02, TASK-REQ-007-05
**Status:** [x] DONE (đã kiểm chứng 2026-10-08: `go test -race ./internal/usecase/... -run "AgentReadonly|GenerateSolution"` và luồng hai DB `go test -tags integration ./internal/adapter/... -run SolutionContract`)

## Context

- Chạy qua `analysis_runs` của SOL-007 (lease, vòng phục hồi, chỉ mục một run `running`). Không dùng `ExecuteTask` (cần Task, worktree, đổi status; `execute_task.go` gọi `EnsureWorktree`).
- `FlowFor(type).Analysis.Kind` và `GateSubject`, `CompletesAfterAnalysis` do CR-REQ-003 cung cấp.
- Cổng đồng thời mặc định 2 run `agent_readonly` mỗi `(tenant, project)` (đề xuất, chưa đo).

## Việc cần làm

1. `GenerateSolution` rẽ nhánh: `kind=solution` thì SOL-007; `kind` khác thì `mode=agent_readonly` (trừ `analysis_mode=COMPLETE`); hotfix `timeoutMs=300000`, còn lại `600000` (`REQUEST_AGENT_READONLY_TIMEOUT_MS`).
2. Cổng đồng thời trong transaction chèn run: `CountRunning(project, agent_readonly) >= max` thì `REQUEST_ANALYSIS_BUSY` và không chèn dòng nào; Postgres dùng khoá tư vấn theo `(tenant, project)` hoặc `SELECT ... FOR UPDATE` trên dòng `projects` cục bộ nếu có; MySQL dùng `FOR UPDATE` trên các dòng `running` của project. Chọn một cơ chế và ghi lý do trong PR.
3. `RunAgentReadonlyAnalysis.Execute(runID)` theo solution D bước 1 đến 9: `ResolveConnection` (`NO_CONNECTION`, `NO_REPO_PATH`), snapshot trước (hoặc `repo_check=skipped`), prompt, `ExecPrompt`, snapshot sau, so sánh (`REPO_MODIFIED`: bỏ kết quả), trích JSON, `ValidateByKind` (một lần thử lại), `RedactDocument`, transaction persist.
4. Prompt theo `kind` trong `solution_prompt.go`: chung (chỉ dẫn chỉ đọc: cấm sửa, xoá, tạo file, cấm `git commit/checkout/reset/clean/push`, cấm gọi mạng ngoài; "đúng một JSON, không code fence"; khối `<request>` là dữ liệu), `diagnosis` (nguyên nhân gốc kèm `file:line`, cách tái hiện, phạm vi ảnh hưởng, size; `security` thêm mức khai thác và dữ liệu lộ, không in bí mật; `hotfix` trả lời trong 5 phút), `findings`, `answer` (hạ `confidence` và liệt kê `limitations` khi thiếu căn cứ).
5. Persist: nếu `GateSubject != ""` thì `OpenApproval`; `TransitionRequest(analysis_ready)`; `hotfix` (`GateSubject == ""`): Solution `approved`, outbox `solution.approved{auto:true, mode}`, không Approval.
6. Sự kiện `repo_modified`: log + metric; việc thông báo admin chờ quyết định (câu hỏi mở 4), không tạo subject mới.

## Kiểm thử

- Unit (fake Relay, probe, gate): thành công theo từng `kind`; `exitCode != 0`; `timedOut`; repo bị sửa (kết quả không lưu); `worktree_id` rỗng (skipped, vẫn thành công, `repo_check=skipped` ghi vào run); thứ ba đồng thời bị `BUSY`; JSON sai rồi đúng; hotfix tự duyệt không tạo Approval.
- Khẳng định fake git-gateway có 0 lời gọi tạo worktree và `trustPreset=="default"` ở mọi lời gọi Relay.
- Lệnh: `go test ./internal/usecase/... -run "AgentReadonly|GenerateSolutionRouting"`.

## Tiêu chí hoàn thành

- [x] Tiêu chí chấp nhận 1, 2, 3, 4, 5, 6, 7 của CR-REQ-008 mục 4 có test.
- [x] `BUSY` không để lại dòng run.
- [x] Mọi use case gọi `tenant.RequireTenantID`.
- [x] Không có đường tự hạ xuống `ai.complete` khi agent lỗi.

## Rủi ro và lưu ý

- Chỉ chạy sau khi loại đã được người xác nhận (kể cả hotfix và security): kiểm trạng thái Request, không chỉ loại.
- `trustPreset=default` có thể chặn công cụ hoặc treo chờ quyền: chưa kiểm chứng, cần chạy thật.

## Ghi chú triển khai (2026-10-08)

- Dùng chung `GenerateSolution`: rẽ nhánh theo `FlowFor(type).AnalysisKind` và `analysis_mode` ngay ở đó (`modeFor`); `solution_prompt.go` nhận `Kind`. Worker là `run_agent_readonly_analysis.go`; giao dịch kết quả dùng `AnalysisResultWriter` chung với nhánh `ai.complete`.
- Cổng đồng thời: hàng khoá `analysis_project_gates` + `FOR UPDATE` trong cùng giao dịch chèn run (cả hai dialect, cùng một cơ chế), mặc định 2 run `agent_readonly` mỗi `(tenant, project)` (`REQUEST_AGENT_READONLY_MAX_PER_PROJECT`); `BUSY` chặn ở `GenerateSolution` và không để lại dòng nào (kiểm trên Postgres và MySQL thật với 8 goroutine, đúng 2 qua).
- Ba lớp bảo vệ ghi vào run: `enforcement` (`agent_enforced` khi `SelectReadonlyRoute` chọn `RouteAgentEnforced`, ngược lại `prompt_only`) và `repo_check` (`clean`, `modified`, `skipped` khi không có `worktree_id` hoặc git-gateway lỗi). `REQUEST_REQUIRE_ENFORCED_READONLY=true` thì agent cũ bị từ chối `REQUEST_ANALYSIS_AGENT_TOO_OLD` (CR-REQ-033 bảng 2.7). Kết quả có `READONLY_VIOLATION`, `applied.accessMode` khác `readonly`, `changes` khác rỗng/`headMoved`, hoặc snapshot khác nhau đều bị loại (`REQUEST_ANALYSIS_REPO_MODIFIED`).
- Thông báo admin khi `REPO_MODIFIED` chưa làm (câu hỏi mở 4): chỉ log cảnh báo; chưa có metric riêng. Chưa kiểm chứng: hành vi `trustPreset=default` / `accessMode=readonly` trên dev server thật.
