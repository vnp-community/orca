# TASK-REQ-042-06: Module `prompts/`: classify, analysis, clarification, plan, OpenSpec; xoá `PromptRegistry` chết

**From Solution:** [BE-REQ-SOL-042](../solutions/BE-REQ-SOL-042-prompt-catalog-shared-agent-relay-and-stdin.md)
**Priority:** P1
**Service:** `backend-go/prompts, request-service`
**File:** `backend-go/prompts/{go.mod,fence,redact,classify_request,analysis,clarification_answers,plan_native,plan_openspec,solution_openspec}/` (mới), `backend-go/go.work` (sửa), `services/request-service/internal/usecase/{solution_prompt,plan_prompt,openspec_plan_prompt,openspec_solution_prompt,clarification_answers_prompt,prompt_redaction,prompt_registry}.go` và `adapter/grpcclient/classification_prompt.go` (thay bằng ánh xạ mỏng; xoá hàm cũ)
**Depends on:** TASK-REQ-042-05
**Status:** [ ] TODO

---

## Context

- Hàm cũ: xem SOL-042 mục 1. `fenceUntrusted`, `truncateRunes`, `promptBodyRunes=12000`, `promptArtifactRunes=8000`.
- Golden của task 05 là điều kiện.
- `analysis/` tách theo loại tài liệu (Solution, chẩn đoán, Findings, Answer).

## Việc cần làm

1. Tạo module `prompts` (`github.com/stablyai/orca-go/prompts`), thêm vào `go.work` và CI.
2. `fence` (thay `fenceUntrusted`) và `redact` (thay `prompt_redaction.go`): chuyển nguyên hành vi, có test.
3. Mỗi luồng một gói: `//go:embed *.tmpl`, `const ID`, `Input` thuần, `Render`; `Option("missingkey=error")`. `analysis/` có `solution.tmpl`, `diagnosis.tmpl`, `findings.tmpl`, `answer.tmpl`, `agent_solution.tmpl` và partial `_request_block.tmpl`, `_feedback.tmpl`, `_prior_artifacts.tmpl`. Phần logic quá phức tạp cho mẫu (ví dụ chọn quy tắc theo loại Request) giữ trong Go của gói `prompts`, **không** ở service.
4. Ánh xạ miền → `Input` ở request-service (hàm mỏng + test).
5. Đổi nơi gọi (`BuildSolutionPrompt`, `BuildPlanPrompt`, `BuildOpenSpecPlanPrompt`, `BuildOpenSpecSolutionPrompt`, `BuildClassificationPrompt`, khối câu trả lời Clarification) sang `prompts/...Render`.
6. Chạy golden của task 05: phải trùng từng byte; sửa mẫu cho đến khi trùng, không sửa golden.
7. Xoá hàm cũ và `PromptRegistry` chết; không để mã chết hay tham chiếu treo.
8. Phối hợp CR-REQ-041: khối phản hồi và `prior_artifacts` (TASK-REQ-041-02) phải nằm trong partial tương ứng; nếu 041-02 chưa xong thì làm sau hoặc cập nhật golden có chủ đích kèm giải thích.

## Kiểm thử

- Golden của task 05 trùng từng byte cho mọi ca của các luồng này.
- Test `Render` với dữ liệu thiếu trường (lỗi `missingkey`), với nội dung đối nghịch (đóng khối `</request>`) vẫn bị bọc.
- Acceptance `request`, `matrix`; UI native (stub).
- `go test ./prompts/... ./services/request-service/...`.

## Tiêu chí hoàn thành

- [ ] Mọi prompt của request-service nằm trong `prompts/` và render trùng golden.
- [ ] `PromptRegistry` chết đã xoá, không còn tham chiếu.
- [ ] Không còn hàm dựng prompt trong `usecase/` hay `adapter/` của request-service.
- [ ] Acceptance xanh.

## Rủi ro và lưu ý

- Lệch khoảng trắng: golden bắt được; ưu tiên sửa mẫu.
- Xung đột với CR-REQ-041: thứ tự trong kế hoạch tổng.

## Chuẩn hoàn thành chung (áp dụng cho mọi task, không thương lượng)

- Mã chạy thật, **không stub, không TODO, không `panic` cho lỗi thường, không bỏ qua lỗi**; mọi nhánh lỗi có mã lỗi và test.
- Chạy `gitnexus_impact` trước khi sửa symbol có sẵn và báo blast radius; chạy `detect_changes` trước khi commit (CLAUDE.md).
- Hai dialect: mọi thay đổi lưu trữ có migration và test hợp đồng trên **Postgres 16 và MySQL 8** (testcontainers).
- Không file nào vượt `max-lines` (tách file khi cần, không thêm disable); không đặt tên `helpers`, `utils`, `common`, `misc`.
- Chạy được trên macOS, Linux, Windows và qua SSH; lệnh git theo baseline 2.25 (`guides/reference/git-compatibility.md`).
- Không ghi nội dung Request, Solution, phản hồi hoặc bí mật vào log, span hay khung sự kiện (C9).
- `go build ./...` và `go test` của gói đã đổi xanh; `go vet` xanh; lint xanh; không làm hỏng test hiện có.
