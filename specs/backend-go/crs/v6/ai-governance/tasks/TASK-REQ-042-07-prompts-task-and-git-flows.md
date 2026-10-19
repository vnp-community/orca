# TASK-REQ-042-07: Module `prompts/`: thực thi task, tách task, sinh prompt agent, git

**From Solution:** [BE-REQ-SOL-042](../solutions/BE-REQ-SOL-042-prompt-catalog-shared-agent-relay-and-stdin.md)
**Priority:** P2
**Service:** `backend-go/prompts, task-service, git-gateway-service`
**File:** `backend-go/prompts/{task_execution,task_decompose,task_agent_prompt,git_commit_message,git_agent_prompt}/` (mới), `services/task-service/internal/adapter/grpcclient/simple_executor.go`, `.../domain/execution_result_prompt.go`, `.../usecase/{ai_decompose,generate_agent_prompt}.go`, `services/git-gateway-service/internal/usecase/{agent_prompt,commit_message_prompt}.go` (thay bằng ánh xạ mỏng; xoá hàm cũ)
**Depends on:** TASK-REQ-042-05, 042-06 (gói `fence`, `redact`)
**Status:** [x] DONE (2026-10-20)

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

- [x] Prompt thực thi task, tách task, sinh prompt agent nằm trong `prompts/` (cả hai file git-gateway và prompt PR cũng vậy).
- [x] `simple_executor.go` chỉ còn logic vận chuyển/điều phối (599 dòng ở HEAD, 471 sau task này; xem ghi chú: Go không có giới hạn `max-lines`, 300 là của oxlint cho TS).
- [x] Golden trùng (49 golden của các luồng này, `check-prompt-golden.sh` 6/6 ok, không golden nào bị sửa).
- [x] e2e/acceptance xanh. Acceptance `--ai-stub platform devserver request matrix exits ai plan analysis task notfound aistub request_base_ref analysis_revision` (2026-10-20, sau khi triển khai lại request/task/git-gateway/orchestration): 121 đạt, 0 lỗi, 3 chặn (AIS-xx cần AI thật, MCP-01/02 có sẵn), 2 bỏ qua; chạy lại tập nhỏ hơn sau lần triển khai cuối: 102 đạt, 0 lỗi. UI `qa-bug-task-native-flow` 3/3; `--real native-execute` (Claude thật, repo tạm) 2/2 đạt trong 11,8 phút.

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

---

## Ghi chú triển khai

**Ngày:** 2026-10-20. **Kết quả:** mã, test và golden xong và đã chạy; acceptance `request`/`matrix` và UI native-execute (stub) chưa chạy nên trạng thái PARTIAL.

### Gói mới trong `backend-go/prompts` (6 gói, 810 dòng gồm test)

- `task_execution` (`execute.tmpl`, `result_block_suffix.tmpl`): `Render(Input{Task, Parent, CompletedDeps})` và `RenderResultBlock(ResultBlockInput{Nonce, RequirePassingCheck, KnowledgeHub})`. `Task.Title` bắt buộc (`render.Require`); `Nonce` rỗng vẫn render vì golden `empty_nonce` ghim byte đó.
- `task_decompose` (`decompose.tmpl`), `task_agent_prompt` (`generate.tmpl`): `Title` bắt buộc.
- `git_commit_message` (`commit_message.tmpl`, `stats_only.tmpl`): logic cắt sha 7 byte và lấy dòng đầu của message nằm ở Go của gói.
- `git_agent_prompt` (`issue.tmpl`): `Sanitize` (bỏ thẻ HTML, `html.UnescapeString`, cắt 4000 **byte**) chuyển nguyên vẹn vào gói, gọi từ mẫu bằng hàm `sanitize`; test `TestSanitize_*` chuyển từ git-gateway sang đây.
- `git_pull_request` (`fields.tmpl`): không có trong danh sách của task. Lý do: `prFieldsPromptPrefix` là chữ prompt gửi `ai.complete`, để lại trong service thì catalog không đầy đủ.
- Không có gói nào dùng `fence`: các prompt này không bọc chữ của người dùng trong khối có thẻ đóng (chúng chèn trực tiếp), đưa `fence` vào sẽ đổi byte khi nội dung chứa `</request`... Thay vào đó mỗi gói có test chuỗi đối nghịch (`</request>`, `{{.Title}}`, `{{template "x"}}`, HTML) phát ra đúng nguyên văn, không bị chạy như mẫu.
- README của `prompts` thêm 6 dòng vào bảng. `go.mod` của `prompts` không đổi (chỉ dùng stdlib thêm).

### Quyết định "git-gateway có phải prompt gửi agent không" (Correction 4)

Cả hai đều là prompt thật, nên chuyển (không để lại):
- `buildAgentPrompt` (BR-PI-05) là chữ được `SpawnAndInject` gõ vào phiên agent mở trong worktree tạo từ issue.
- `buildCommitMessagePrompt` + `statsOnlySummary` và `prFieldsPromptPrefix` + diff là chuỗi gửi qua relay `ai.complete` tới model.
Ở lại git-gateway vì là dữ liệu, không phải chữ prompt: `extractIssueRef`, `maxFullDiffFiles`, `truncate` (còn dùng bởi `branch_name.go`).

### Ánh xạ mỏng và hàm cũ đã xoá

- task-service: xoá `buildExecutePrompt` (simple_executor.go), `buildDecomposePrompt` (ai_decompose.go), `buildPromptGenerationPrompt` (generate_agent_prompt.go), thân của `domain.ContractPromptSuffix`. Mới: `adapter/grpcclient/execute_prompt_mapping.go` (59 dòng: `renderExecutePrompt`, `renderDefaultExecutePrompt`, `interpolateOutputs` chuyển từ simple_executor.go, chạy **sau** `Render` như cũ), `usecase/task_prompt_mapping.go` (36 dòng). `domain.ContractPromptSuffix` giữ tên và chữ ký thêm `error`, thân 1 dòng gọi `task_execution.RenderResultBlock`: để test golden `domain` (và dòng `task-service ./internal/domain` của `check-prompt-golden.sh`) giữ nguyên chỗ, và lời gọi vẫn nằm cạnh bộ phân tích kết quả.
- git-gateway: xoá `buildAgentPrompt`, `sanitize`, `maxPromptFieldLen`, `htmlTagPattern`, `buildCommitMessagePrompt`, `commitMessagePromptPrefix`, `firstLine`, `statsOnlySummary`, `prFieldsPromptPrefix`. Mới: `render*` trong `agent_prompt.go` (19 dòng, trước 39), `commit_message_prompt.go` (62 dòng, trước 90), `pull_request_prompt.go` (16 dòng).
- Số dòng: `simple_executor.go` 599 (HEAD) -> 529 (cây làm việc trước task này, do task 03/04) -> **471**; không tính dòng trống và comment: 301 -> 218. `ai_decompose.go` 183 -> 148, `generate_agent_prompt.go` 101 -> 86, `execution_result_prompt.go` 26 -> 9. Go không có `max-lines` (`.golangci.yml` không bật linter nào về độ dài; 300/400/600 trong `.oxlintrc.json` chỉ áp cho TS), nên không có ngưỡng để "xuống dưới"; file đã chỉ còn vận chuyển/điều phối, không tách giả tạo.
- Lỗi dựng prompt không còn là chuỗi rỗng ngầm: `TASK_EXECUTE_PROMPT_BUILD_FAILED`, `TASK_AI_DECOMPOSE_PROMPT_BUILD_FAILED`, `TASK_GENERATE_PROMPT_BUILD_FAILED`, `GITGATEWAY_PROMPT_BUILD_FAILED` (kind Internal, dừng trước khi gọi relay); với prompt agent của issue, lỗi chỉ ghi vào `AgentStartError` (cùng tư thế với lỗi spawn, worktree giữ nguyên). Thông điệp lỗi chỉ chứa tên mẫu/trường, không có nội dung task hay issue (C9); có test.

### Kiểm thử

- Mới: prompts 27 test hàm (task_execution 7, task_decompose 3, task_agent_prompt 3, git_commit_message 4, git_agent_prompt 7, git_pull_request 3; 60 ca con) gồm: văn bản chính xác theo từng nhánh, trường tuỳ chọn vắng khi rỗng, thiếu `Title` trả `ErrMissingField`, chuỗi đối nghịch phát nguyên văn. task-service 10 test hàm (`execute_prompt_mapping_test.go` 6: lỗi thiếu tiêu đề, nội suy `{{outputs.*}}` (wildcard = stdout, token không có dependency giữ nguyên, task lạ giữ nguyên, dependency chưa có output thành rỗng, token trong title/description/parent cũng được thay sau Render), nối result block cho contract run; `task_prompt_mapping_test.go` 4: lỗi dựng prompt không gọi relay).
- Test cũ chỉ đổi cách gọi (`must*Prompt(t, ...)`), kỳ vọng giữ nguyên. **Không golden nào bị sửa, sinh lại hay xoá** (không chạy `--update`).
- Sửa fixture: `Task{ID: "t1", ProjectID: "p1"}` không có `Title` ở 15 chỗ của `simple_executor_test.go` và 7 chỗ ở `ai_decompose_test.go`/`generate_agent_prompt_test.go` nay có `Title`, vì `Title` rỗng bị từ chối (`domain.NewTask` đã chặn `ErrEmptyTitle` lúc tạo). Hai test cũ (`RelayFailurePropagates`, `SaveFailurePropagates`) sẽ âm thầm pass vì sai lý do nếu không sửa.

### Lệnh đã chạy

- `bash backend-go/ci/check-prompt-golden.sh`: 6/6 ok (task-service domain, grpcclient, usecase; git-gateway usecase; request-service 2). Khớp từng byte ngay lần chạy đầu cho 32 golden task-service và 17 golden git-gateway.
- `cd backend-go/prompts && go vet ./... && go test ./... -race -count=1`: 17 gói ok. `gofmt -l`: rỗng.
- `cd backend-go/services/{task-service,git-gateway-service} && go build ./... && go vet ./... && go test ./... -race -count=1`: tất cả ok.
- `GOWORK=off go build ./...` ở `prompts`: ok. Ở hai service: `GOWORK=off go build ./internal/domain ./internal/usecase ./internal/adapter/grpcclient` (task-service) và `./internal/usecase` (git-gateway): ok. `GOWORK=off go build ./...` đầy đủ **thất bại ở cả hai** vì thiếu dòng `go.sum` cho `nats.go`/`klauspost/compress` (qua `common/eventbus`), có sẵn từ trước: `usage-service` không đụng tới cũng thất bại y hệt; không liên quan `prompts`. Không đổi `go.sum`.

### Blast radius (grep, GitNexus không có)

- task-service: `simple_executor.go` (`runAgentPrompt`, 1 nơi gọi `renderDefaultExecutePrompt`), `ai_decompose.go` (`AIDecompose.Execute`), `generate_agent_prompt.go` (`GenerateAgentPrompt.Execute`). `domain.ContractPromptSuffix` chỉ gọi từ mapping trên. Comment nhắc tên cũ sửa ở `usecase/ports.go`, `usecase/execute_task.go`, `domain/task.go`.
- git-gateway: `generate_commit_message.go` (2 chỗ), `generate_pull_request_fields.go`, `create_worktree_from_issue.go` (`buildAgentPrompt`), comment ở `adapter/grpcclient/scm_client.go`.
- Ngoài Go: `frontend/.../TaskPromptEditor.tsx` và `hooks/useTask.ts` còn comment nhắc `SimpleExecutor.buildExecutePrompt`/`TaskAIPlanner.buildDecomposePrompt` (chỉ là chữ, chưa đổi). Không gói nào khác import hàm đã xoá (grep toàn repo, trừ `specs/`, `docs/`, `testdata/`).

### Build, CI

- `go.mod` của task-service và git-gateway-service thêm `require` + `replace ../../prompts`; Dockerfile của hai service đã có `COPY prompts ./prompts` từ task 06 (đã kiểm). Chưa chạy `docker build`.
- `.github/workflows/backend-go-task-service.yml`: thêm path `backend-go/prompts/**`. git-gateway chưa có workflow riêng.

### Lệch so với task/brief và lý do

1. Thêm gói `git_pull_request` (ngoài danh sách), xem trên.
2. `task_execution` có hai struct đầu vào (`Input` và `ResultBlockInput`) thay vì một `Input` gộp: văn bản thực thi và result block không dùng chung trường.
3. Không dùng `fence` (xem trên); "chuỗi đối nghịch nằm trong khối được fence" áp dụng cho các luồng của task 06, ở đây thay bằng "phát nguyên văn, không bị chạy như mẫu".
4. `Title` rỗng nay là lỗi thay vì prompt `Task: \n` (hành vi mới, chỉ chạm tới dữ liệu cũ hỏng).
5. `domain.ContractPromptSuffix` còn lại như một lớp mỏng, domain import `prompts` (xem trên).

### Lỗi đã thấy, KHÔNG sửa (golden ghim byte hiện tại)

- `git_agent_prompt.Sanitize` cắt theo byte nên có thể chặt đôi một ký tự nhiều byte và để lại UTF-8 sai (golden dùng ASCII để tránh; test `TestSanitize_CutIsByBytesNotRunes` ghim hành vi). Nên đổi sang `fence.Truncate` (theo rune) trong một thay đổi prompt có chủ đích kèm `--update` golden.
- Điều kiện hiện diện của mục (Description, Acceptance Criteria) xét trên giá trị thô trước khi làm sạch: trường chỉ có `<br>` vẫn in tiêu đề mục với thân rỗng (có test).
- Prompt PR là `instruction + diff` ghép trực tiếp, thứ tự quan trọng vì dòng đầu của phản hồi thành tiêu đề PR; diff rỗng vẫn gửi mỗi câu lệnh. `parsePRFields` phụ thuộc điều này.
- Dòng `Tech stack:` in `; ` cuối khi chỉ có ngôn ngữ mà không có framework (có test).

### Chưa kiểm chứng

- Acceptance `request`, `matrix`; UI native-execute (stub); `--real native-execute` (Claude thật, repo tạm): chưa chạy (không được chạy e2e/stack). Prompt thực thi task ảnh hưởng chất lượng chạy thật: số lần gọi công cụ và thời lượng chưa so; byte prompt đã chứng minh trùng bằng golden nên rủi ro chỉ còn ở lỗi dựng prompt mới (Title rỗng).
- `docker build` thật của hai service; `GOWORK=off go build ./...` đầy đủ của hai service (lỗi `go.sum` có sẵn).
