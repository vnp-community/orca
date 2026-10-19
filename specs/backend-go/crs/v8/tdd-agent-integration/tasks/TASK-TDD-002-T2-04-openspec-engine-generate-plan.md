# TASK-TDD-002-T2-04: Engine `openspec`: sinh gói OpenSpec bằng agent và dựng đề xuất Plan

**From Solution:** [SOL-TDD-002](../solutions/SOL-TDD-002-tdd-agent-as-knowledge-hub.md) mục 3.4 (bước 3 đến 5)
**Checklist:** [T2.4](CHECKLIST-TDD-002-knowledge-hub.md)
**Priority:** P0 · **Area:** request-service (Go)
**Files:** `internal/domain/{tasks_md_parser,tasks_md_render}.go` + test mới, `internal/usecase/{engine_openspec,openspec_plan_prompt,proposal_scope_check,run_plan_generation}.go` + test, `testdata/openspec/tasks_*.md` (mới)
**Depends on:** T1.5, T2.1, T2.2, T2.3
**Status:** `[ ]` TODO

---

## Context

Đây là lõi của `plan generate` trả bộ OpenSpec. Khung có sẵn: parser/renderer `tasks.md` (mã thật, **chưa có test**), `CheckProposalScope` (mã thật, chưa có test), `openspecEngine.GeneratePlan` (stub trả chuỗi rỗng), `BuildOpenSpecPlanPrompt` (17 dòng, chưa đủ). `RunPlanGeneration.Execute` hiện chỉ gọi `ai.complete` rồi `ParsePlanOutput`, `ValidatePlanProposal`, `policy.PlanPreconditions`, `finish` (lưu `EncodePlanProposal` vào `run.RawOutput`).
Văn phạm vùng `tasks.md` (parser): `<!-- orca:begin plan request=<ref> schema=1 -->` … `<!-- orca:end plan -->`; `## PH-<n> <tên>` hoặc `## TASKS`; `> mô tả`; `- [ ] T<p>.<q> <tiêu đề> [type=…] [h=…] [ac=…] [labels=…] [depends=…]`; dòng thụt hai dấu cách là mô tả; dòng lạ là `TASKSMD_UNKNOWN_LINE`.

## Việc cần làm

1. **Trước hết viết test** cho `tasks_md_parser.go`, `tasks_md_render.go`, `proposal_scope_check.go` theo CR-REQ-026 TASK-03 mục 8: fixture `testdata/openspec/` (`tasks_ok.md` hai Phase ba task có phụ thuộc và AC và tiếng Việt có dấu, `tasks_no_phase.md`, `tasks_prose_checkbox.md` với checkbox và khối JSON giả ngoài vùng, `tasks_cycle.md`, `tasks_unknown_line.md`, `tasks_crlf.md`); thêm fuzz; sửa lỗi test tìm ra; làm rõ `TickTask` đổi `[ ]` thành `[x]` (mô tả trong TASK-03 viết nhầm).
2. Viết lại `BuildOpenSpecPlanPrompt(in)`: change id; `request=<ref>` dạng `REQ-<số>`; Request (tiêu đề, body trong khối rào chắn `fenceUntrusted`, tiêu chí chấp nhận kèm id bằng `writeAcceptanceCriteria`); Solution đã duyệt (`approvedAnalysisText`); giới hạn (`domain.PlanLimits`, khoảng số task theo cỡ); chỉ dẫn dùng skill `tdd-openspec-author` và `tdd-knowledge`; vùng được ghi `openspec/changes/<id>/`; khối kết thúc chuẩn (qua result block `ORCA_RESULT_BEGIN/END <nonce>`); `RetryNote` (danh sách `Violation`).
3. `openspecEngine.GeneratePlan(ctx, p, in)`: `Preflight` (cache 10 phút), `Ensure` workspace, chạy `AgentPromptRunner.ExecPrompt` với `AccessMode=write`, `WorkspaceKind=worktree`, `ReportChanges=true`, `KnowledgeRoot=<gốc repo>`, `ResultBlockNonce`; kiểm `res.TimedOut`, `ExitCode`, `Warnings`; `ChangedPaths` + `CheckProposalScope` (ngoài phạm vi ⇒ `REQUEST_SPEC_RUN_OUT_OF_SCOPE`, giữ worktree); `ReadFile` `openspec/changes/<id>/tasks.md` (≤ 256 KiB) và trả nội dung thô.
4. Mở rộng `AgentPromptInput`/`agentExecPromptParams` cho ghi: `AccessMode`, `WorkspaceKind`, `ReportChanges`, `ResultBlockNonce` (các trường đã có ở agent, chưa có ở Go).
5. `RunPlanGeneration.Execute`: khi `run.Mode == agent_proposal` đi nhánh engine: `GeneratePlan` → `domain.ParsePlanRegion(raw, ref)`; có `Violation` ⇒ vòng thử lại (một lần, qua `run.RecordAttempt`, đưa danh sách lỗi vào prompt và chạy lại agent trong cùng worktree); sau đó `ValidatePlanProposal`, `policy.PlanPreconditions`, kiểm cấu trúc T2.5, `finish`.
6. Mã lỗi thất bại ánh xạ sang `AnalysisFailure` (không để lộ nội dung nhạy cảm; áp `RedactRaw`).

## Tiêu chí hoàn thành

- [ ] Parser, renderer, `CheckProposalScope` có bộ test đầy đủ và xanh (fixture, CRLF, tiếng Việt có dấu, fuzz).
- [ ] Với agent giả ghi gói mẫu, `Execute` trả đề xuất Plan hợp lệ; với agent ghi ra ngoài vùng, run `failed` có mã và worktree được giữ.
- [ ] Một lần chạy thật trên repo thử tạo được `PlanProposal` từ `tasks.md` do AI viết.

## Kiểm thử

- Golden test cho prompt (`testdata/`).
- Test với `AgentPromptRunner` và `ProposalWorkspace` giả: thành công, `Violation` rồi thành công ở lần hai, `Violation` hai lần ⇒ thất bại, ngoài phạm vi, timeout.

## Rủi ro và ghi chú

- Không đưa nội dung KG vào prompt (Orca không đọc KG): chỉ dẫn agent tự tra qua skill.
- Lần chạy thật phụ thuộc kết luận T1.5 về chất lượng.
