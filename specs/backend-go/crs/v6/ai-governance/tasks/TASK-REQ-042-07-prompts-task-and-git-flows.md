# TASK-REQ-042-07: Module `prompts/`: thực thi task, tách task, sinh prompt agent, git

**From Solution:** [BE-REQ-SOL-042](../solutions/BE-REQ-SOL-042-prompt-catalog-shared-agent-relay-and-stdin.md)
**Priority:** P2
**Service:** `backend-go/prompts, task-service, git-gateway-service`
**File:** `backend-go/prompts/{task_execution,task_decompose,task_agent_prompt,git_commit_message,git_agent_prompt}/` (mới), `services/task-service/internal/adapter/grpcclient/simple_executor.go`, `.../domain/execution_result_prompt.go`, `.../usecase/{ai_decompose,generate_agent_prompt}.go`, `services/git-gateway-service/internal/usecase/{agent_prompt,commit_message_prompt}.go` (thay bằng ánh xạ mỏng; xoá hàm cũ)
**Depends on:** TASK-REQ-042-05, 042-06 (gói `fence`, `redact`)
**Status:** [ ] TODO

---

## Context

- `buildExecutePrompt` nằm trong `simple_executor.go` 599 dòng; `ContractPromptSuffix` ở domain; nội suy `{{outputs.<taskId>.*}}` giữ ở task-service (logic dữ liệu, không phải văn bản).
- Cần đọc `git-gateway-service/usecase/{agent_prompt,commit_message_prompt}.go` để biết có phải prompt gửi agent không (SOL-042 Correction 4).

## Việc cần làm

1. Gói `task_execution` (`execute.tmpl`, `result_block_suffix.tmpl`): `Input` thuần (`Task`, `Parent`, `CompletedDeps`, `Nonce`, `RequirePassingCheck`, `KnowledgeHub`); `interpolateOutputs` ở lại task-service và chạy **sau** `Render` như hiện nay.
2. Gói `task_decompose`, `task_agent_prompt`: chuyển `buildDecomposePrompt`, `buildPromptGenerationPrompt`.
3. git-gateway: nếu hai file là prompt gửi agent thì tạo `git_commit_message`, `git_agent_prompt`; nếu không phải thì để nguyên và ghi lý do vào PR (không ép vào catalog).
4. Đổi nơi gọi; xoá hàm cũ; `simple_executor.go` xuống dưới `max-lines` nhờ bỏ hàm dựng prompt.
5. Golden của task 05 trùng từng byte.

## Kiểm thử

- Golden cho mọi ca của task 05 thuộc các luồng này.
- Test nội suy `{{outputs.*}}` vẫn đúng (token không có dependency giữ nguyên).
- Acceptance `request`, `matrix`; UI native-execute (stub) và `--real native-execute` (Claude thật, repo tạm).

## Tiêu chí hoàn thành

- [ ] Prompt thực thi task, tách task, sinh prompt agent nằm trong `prompts/`.
- [ ] `simple_executor.go` dưới giới hạn dòng và chỉ còn logic vận chuyển/điều phối.
- [ ] Golden trùng; e2e xanh.

## Rủi ro và lưu ý

- Prompt thực thi task ảnh hưởng chất lượng chạy thật: chạy `--real native-execute` trước khi merge để thấy số lần gọi công cụ và thời lượng không lệch.

## Chuẩn hoàn thành chung (áp dụng cho mọi task, không thương lượng)

- Mã chạy thật, **không stub, không TODO, không `panic` cho lỗi thường, không bỏ qua lỗi**; mọi nhánh lỗi có mã lỗi và test.
- Chạy `gitnexus_impact` trước khi sửa symbol có sẵn và báo blast radius; chạy `detect_changes` trước khi commit (CLAUDE.md).
- Hai dialect: mọi thay đổi lưu trữ có migration và test hợp đồng trên **Postgres 16 và MySQL 8** (testcontainers).
- Không file nào vượt `max-lines` (tách file khi cần, không thêm disable); không đặt tên `helpers`, `utils`, `common`, `misc`.
- Chạy được trên macOS, Linux, Windows và qua SSH; lệnh git theo baseline 2.25 (`guides/reference/git-compatibility.md`).
- Không ghi nội dung Request, Solution, phản hồi hoặc bí mật vào log, span hay khung sự kiện (C9).
- `go build ./...` và `go test` của gói đã đổi xanh; `go vet` xanh; lint xanh; không làm hỏng test hiện có.
