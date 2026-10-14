# TASK-TDD-002-T2-09: Solution bằng agent chỉ đọc có KG, kèm `design_references`

**From Solution:** [SOL-TDD-002](../solutions/SOL-TDD-002-tdd-agent-as-knowledge-hub.md) mục 3.3
**Checklist:** [T2.9](CHECKLIST-TDD-002-knowledge-hub.md)
**Priority:** P1 · **Area:** request-service (Go + migration)
**Files:** `internal/usecase/{generate_solution,run_agent_readonly_analysis,solution_prompt}.go`, `internal/domain/{solution_options,engine_settings}.go`, `internal/adapter/{postgres,mysql}/engine_settings_repository*.go`, migration mới (cột `solution_agent`), test đi kèm
**Depends on:** T1.2, T1.4, T2.1
**Status:** `[ ]` TODO

---

## Context

Hiện `modeFor` ép loại `solution` chạy `ai.complete` và **từ chối** ghi đè sang `agent_readonly` (`ErrSolutionModeNotAllowed`); các loại khác (diagnosis, findings, answer) đã dùng agent chỉ đọc (`RunAgentReadonlyAnalysis`: cần repo path và dev server, `EnsureProjectGate`, giới hạn `MaxAgentRunsPerProject`, kiểm repo "clean" trước/sau, phát hiện `READONLY_VIOLATION`). `BuildSolutionPrompt` cho loại `solution` là prompt `ai.complete` (không có preamble chỉ đọc).
Mục tiêu: cho Solution chạy bằng đường agent chỉ đọc đã có, khi project bật cờ, để agent tự tra KG bằng MCP và skill. Kết quả T1.4 quyết định có dùng được chế độ chỉ đọc cho MCP hay không.

## Việc cần làm

1. Migration: cột `solution_agent BOOLEAN NOT NULL DEFAULT FALSE` cho `request.project_engine_settings` (hai dialect); mở rộng `domain.ProjectEngineSettings` và repository; RPC get/set (T2.10).
2. `modeFor(kind, override, settings)`: với `kind=solution` và `settings.SolutionAgent` ⇒ `AnalysisModeAgentReadonly`; ngoài ra giữ nguyên. Giữ `ErrSolutionModeNotAllowed` cho ghi đè từ phía người gọi khi cờ tắt.
3. `BuildSolutionPrompt`: thêm `UseKnowledge bool` vào `SolutionPromptInput`; khi bật thêm preamble chỉ đọc (`agentPreamble`), chỉ dẫn dùng skill `tdd-knowledge`, nêu phần tử bị ảnh hưởng và xung đột Decision/Constraint trong cons của từng phương án, chỗ thiếu thành câu hỏi mở, và schema có `design_references{available, snapshot, read[], gaps[], conflicts[]}`; giữ nguyên rào chắn dữ liệu.
4. `domain/solution_options.go` + `ValidateByKind`: chấp nhận `design_references` tuỳ chọn (kiểm kiểu, giới hạn độ dài, khử bí mật như `RedactSecrets`); không đổi bất kỳ trường bắt buộc nào; lưu cùng `OptionsJSON`.
5. `RunAgentReadonlyAnalysis`: truyền `KnowledgeRoot` (gốc repo của kết nối) và dùng route có feature `knowledgeHub` (T2.1); thiếu feature ⇒ không bật chế độ, giữ `ai.complete` và ghi lý do; kiểm "clean" của repo giữ nguyên.
6. KG vắng hoặc agent không gọi được MCP: Solution vẫn sinh với `design_references.available=false`, không làm run thất bại.
7. Cấu hình MCP chỉ đọc; `proposeChangesets=false` ở bước này.

## Tiêu chí hoàn thành

- [ ] Khi cờ tắt: hành vi y như cũ (có test).
- [ ] Khi bật và đủ khả năng: Solution đi đường agent chỉ đọc; repo không bị đổi; `design_references` được lưu và hiển thị.
- [ ] Mọi test hiện có của `solution` xanh; thêm test cho nhánh mới.

## Kiểm thử

- Test đơn vị `modeFor` (bảng), prompt golden (có/không `UseKnowledge`), schema `design_references` (hợp lệ, quá dài, kiểu sai), `RunAgentReadonlyAnalysis` với agent giả trả JSON có và không có `design_references`.

## Rủi ro và ghi chú

- Nếu T1.4 kết luận chế độ chỉ đọc không gọi được MCP: đổi sang chế độ ghi hẹp trong worktree riêng cho Solution (cần thiết kế lại trong task này, báo người dùng).
- Chạy `gitnexus_impact` cho `modeFor` và `BuildSolutionPrompt` trước khi sửa.
