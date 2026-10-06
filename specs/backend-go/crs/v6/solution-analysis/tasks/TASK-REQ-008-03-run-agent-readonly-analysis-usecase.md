# TASK-REQ-008-03: `RunAgentReadonlyAnalysis`, giới hạn đồng thời và prompt theo `kind`

**From Solution:** [BE-REQ-SOL-008](../solutions/BE-REQ-SOL-008-diagnosis-findings-answer-analysis.md) mục B, D
**Priority:** P1
**Service/Area:** `request-service` / usecase
**File:** `internal/usecase/run_agent_readonly_analysis.go` (mới), `solution_prompt.go` (sửa, nhánh theo `kind`), `generate_solution.go` (sửa, rẽ nhánh theo registry và `analysis_mode`), `internal/config/config.go` (sửa), và `_test.go`
**Depends on:** TASK-REQ-008-01, TASK-REQ-008-02, TASK-REQ-007-05
**Status:** [ ] TODO

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

- [ ] Tiêu chí chấp nhận 1, 2, 3, 4, 5, 6, 7 của CR-REQ-008 mục 4 có test.
- [ ] `BUSY` không để lại dòng run.
- [ ] Mọi use case gọi `tenant.RequireTenantID`.
- [ ] Không có đường tự hạ xuống `ai.complete` khi agent lỗi.

## Rủi ro và lưu ý

- Chỉ chạy sau khi loại đã được người xác nhận (kể cả hotfix và security): kiểm trạng thái Request, không chỉ loại.
- `trustPreset=default` có thể chặn công cụ hoặc treo chờ quyền: chưa kiểm chứng, cần chạy thật.
