# CR-REQ-042 — Gom prompt về một thư mục theo luồng, một relay dùng chung tới dev server, và đưa prompt `agent.execPrompt` qua stdin

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-REQ-042 |
| **Tên** | Thư mục `backend-go/prompts/<luồng>/` chứa mọi prompt gửi cho agent, tách khỏi mã vận chuyển; gói `common/agentrelay` thay bốn bản sao relay `agent.execPrompt` và các bản sao `ai.complete`; agent nhận prompt của `claude --print` qua stdin thay vì argv |
| **Loại** | Refactor kèm sửa hạ tầng (không đổi nội dung prompt, không đổi giao thức dây) |
| **Priority** | 🟡 P2 |
| **Effort** | Large (chia ba giai đoạn độc lập, mục 2.6) |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-19 |
| **Trạng thái** | ✅ Đã triển khai (2026-10-20), chưa merge |
| **Phụ thuộc** | CR-REQ-007, 008 (prompt phân tích), CR-REQ-012 (prompt Plan), CR-REQ-013, 029 (thực thi task, khối kết quả), CR-REQ-026 (OpenSpec), CR-REQ-033 (agent, `agent.execPrompt`) |
| **Mở khoá** | [CR-REQ-034](./CR-REQ-034-ai-governance-budgets-evals-prompt-versioning.md) mục 2.3 (`PromptRegistry`, phiên bản prompt, provenance) và `AIGateway` bọc relay |
| **Tác động** | `backend-go/prompts/` (module mới, thêm vào `go.work`), `backend-go/common/agentrelay/` (gói mới), `request-service`, `task-service`, `git-gateway-service`, `workflow-service`, `orchestration-service` (thay mã relay và gọi prompt), `agent/src/relay/agent-print-mode-exec.ts` (stdin), `ci/` (kiểm tra golden và kích thước prompt) |

> Nguồn: rà soát luồng gửi prompt từ backend sang dev server ngày 2026-10-19 (mục 1). Số dòng, tên file và nhận xét về hiện trạng dựa trên đọc mã; chưa đo kích thước prompt thật và chưa chạy lại luồng nào.

---

## 1. Bối cảnh và vấn đề

### 1.1 Đường đi hiện tại

Prompt dựng bằng chuỗi trong các service Go, đi qua `infra-fleet-service` (gRPC `Relay`/`RelayByDevServer`) rồi JSON-RPC trên WebSocket tới agent. Agent chạy Claude Code CLI theo hai chế độ:

| Chế độ | Agent làm gì | Prompt tới `claude` bằng |
|---|---|---|
| `ai.complete` | `claude -p --output-format json --tools ""` trong thư mục tạm (`ai-complete-cli-session.ts`) | stdin (`child.stdin.end(input.prompt)`) |
| `agent.execPrompt` | `claude --print <prompt> ...` trong repo, chỉ-đọc / hồ sơ ghi / trust preset (`agent-print-mode-exec.ts:200`) | **argv**, `stdio: ['ignore', 'pipe', 'pipe']` |

### 1.2 Vấn đề

1. **Prompt nằm rải rác, không có thư mục riêng.** Mỗi luồng có một hàm dựng chuỗi, nhưng vị trí khác nhau: `request-service/usecase/{solution,plan,openspec_plan,openspec_solution,clarification_answers}_prompt.go`, `request-service/adapter/grpcclient/classification_prompt.go`, `task-service/adapter/grpcclient/simple_executor.go` (`buildExecutePrompt`, trong file 599 dòng lẫn logic vận chuyển), `task-service/usecase/{ai_decompose,generate_agent_prompt}.go`, `task-service/domain/execution_result_prompt.go`, `git-gateway-service/usecase/{agent_prompt,commit_message_prompt}.go`. Chỉ prompt MCP có file mẫu thật (`api-gateway/.../prompts/builtin/*.tmpl`).
2. **Bốn bản sao relay `agent.execPrompt` và nhiều bản sao `ai.complete`.** `agent.execPrompt`: `request-service/adapter/grpcclient/agent_prompt_relay.go`, `task-service/adapter/grpcclient/simple_executor.go`, `workflow-service/adapter/infrafleetclient/agent_step_executor.go`, `orchestration-service/adapter/infrafleetclient/worker_dispatcher.go`. `ai.complete`: `request-service/adapter/grpcclient/{ai_completion_relay,request_classifier}.go`, `task-service/adapter/grpcclient/aidecompose_relay.go`, `git-gateway-service/adapter/grpcclient/relay_executor.go`. Mỗi nơi tự định nghĩa tham số (hai struct `agentExecPromptParams` khác nhau), tự phân tích kết quả và tự phân loại lỗi (`READONLY_MODE_UNSUPPORTED`, `KNOWLEDGE_*`). Đổi giao thức phải sửa nhiều nơi.
3. **`PromptRegistry` là mã chết.** `request-service/usecase/prompt_registry.go` có `GetPrompt` luôn trả chuỗi rỗng và không ai gọi. CR-REQ-034 đã đề xuất `PromptRegistry` thật nhưng chưa triển khai, và chỉ cho `request-service`.
4. **Prompt `execPrompt` đi qua argv.** Hệ quả cần kiểm chứng: (a) Linux giới hạn mỗi đối số khoảng 128 KB (`MAX_ARG_STRLEN`), Windows giới hạn cả dòng lệnh 32 KB (`CreateProcess`), macOS khoảng 1 MB tổng; (b) nội dung prompt, gồm mô tả Request, hiện trong danh sách tiến trình của máy dev server; (c) prompt bắt đầu bằng `-` có thể bị đọc như cờ. Prompt hiện bị cắt theo phần (`promptBodyRunes` 12000, `promptArtifactRunes` 8000) nên có thể chưa chạm trần, nhưng chưa có bảo vệ nào và chưa đo.
5. **Không có phiên bản hay ghi nhận prompt đã dùng.** Chưa thể trả lời "lượt này dùng bản prompt nào" (đã nêu ở CR-REQ-034).

## 2. Giải pháp đề xuất

### 2.1 Thư mục `backend-go/prompts/`

Module Go mới (`github.com/stablyai/orca-go/prompts`, thêm vào `go.work`), mỗi luồng một thư mục, văn bản prompt là **file mẫu**, Go chỉ lo nạp dữ liệu và kết xuất:

```
backend-go/prompts/
├── classify_request/        classify.tmpl
├── analysis/                solution.tmpl  diagnosis.tmpl  findings.tmpl  answer.tmpl
│                            agent_solution.tmpl  _request_block.tmpl  _feedback.tmpl
├── clarification_answers/   answers.tmpl
├── plan_native/             plan.tmpl  _shape_rules.tmpl  _type_rules.tmpl
├── plan_openspec/           plan.tmpl  result_block.tmpl
├── solution_openspec/       solution.tmpl
├── task_execution/          execute.tmpl  result_block_suffix.tmpl
├── task_decompose/          decompose.tmpl
├── task_agent_prompt/       generate.tmpl
├── git_commit_message/      commit_message.tmpl
├── git_agent_prompt/        agent_prompt.tmpl
├── fence/                   (gói Go: bọc dữ liệu không tin cậy, thay `fenceUntrusted`)
└── redact/                  (gói Go: che bí mật, thay `prompt_redaction.go`)
```

- Tên thư mục theo **luồng**, tên file theo **loại tài liệu**: `analysis/` có một file cho mỗi trong bốn loại (Solution, chẩn đoán, Findings, Answer), không còn một file 181 dòng phân nhánh bằng `switch`. Đoạn dùng chung là partial bắt đầu bằng `_`.
- Mỗi thư mục là một gói Go riêng (`package analysis`, ...) nhúng file bằng `//go:embed *.tmpl`, xuất một hàm `Render(Input) (string, error)` và một hằng `ID` (ví dụ `analysis/solution`). `Input` là struct dữ liệu thuần, **không import `internal/` của service nào**. Mỗi service ánh xạ miền của nó sang `Input` bằng một hàm mỏng.
- `text/template` là đủ cho các điều kiện hiện có (`AgentSolution`, `UseKnowledge`, loại Request). Mọi dữ liệu người dùng đi qua hàm `fence` trong mẫu, không nối chuỗi trong mã.
- Quy ước đặt tên file phiên bản của CR-REQ-034 (`v<MAJOR.MINOR.PATCH>.tmpl`) **không** áp dụng ở CR này; bản đầu của mỗi luồng giữ tên theo loại tài liệu. CR-REQ-034 mục 2.3 đổi sang tên có phiên bản khi nó làm `PromptRegistry` (xem mục 8).
- Các mẫu MCP (`api-gateway/.../prompts/builtin/*.tmpl`, dành cho client MCP bên ngoài, không đi qua dev server) chưa chuyển; có thể chuyển vào `prompts/mcp/` ở một CR sau.

### 2.2 Không đổi nội dung prompt (cổng an toàn)

Giai đoạn chuyển prompt (mục 2.6, giai đoạn 3) là di chuyển thuần. Trước khi chạm vào, ghi lại đầu ra hiện tại của mỗi hàm dựng cho một bộ fixture đại diện (mỗi loại Request, có/không feedback, có/không knowledge hub, có/không câu trả lời Clarification, có/không `RetryNote`) thành file golden. Mẫu mới phải kết xuất **trùng từng byte** với golden. Chỉ khi golden khớp mới được đổi nội dung, và việc đó thuộc CR khác (CR-REQ-034).

### 2.3 Gói `common/agentrelay`

`backend-go/common/agentrelay` (gói mới; `common` đã import `proto` ở `auditclient`, nên không tạo phụ thuộc mới giữa module):

- `Target{DevServerID, ConnectionID}`: chọn `RelayByDevServer` hay `Relay` ở một chỗ.
- `ExecPrompt(ctx, Target, ExecParams) (ExecResult, error)`: `ExecParams` là hợp của hai struct hiện có (stepId, prompt, worktreePath, trustPreset, model, env, initFile, accessMode, workspaceKind, reportChanges, maxOutputBytes, knowledgeRoot, proposeChangesets, permissionProfile, resultBlock, timeoutMs). Có hàm dựng sẵn `ReadOnly(...)`, `WriteProfile(...)`, `Contract(...)` thay cho `buildAgentParams`. Thêm khoảng chờ dư (`slackOverTimeout`) và phân tích kết quả (`parseAgentResult`, `statsFromAgent`) ở một nơi.
- `Complete(ctx, Target, CompleteParams) (CompleteResult, error)` cho `ai.complete`, kể cả `maxTokens`, `model` khi CR-REQ-034 cần.
- Phân loại lỗi một nơi: `ErrReadonlyUnsupported`, `ErrKnowledgeHubUnavailable`, hết hạn; các service ánh xạ sang lỗi miền của mình.
- Metadata tenant/người dùng: nhận một hàm `Metadata(ctx) (ctx, error)` từ service (mỗi service đang có `withTenantMetadata` riêng), không tự đoán.
- Giữ nguyên hành vi hiện có của đường truyền: thông báo đầu ra `agent.execPrompt.output` mà infra-fleet chuyển cho giao diện, huỷ qua context, và trần thời gian.

Mỗi service giữ một adapter mỏng thực hiện cổng của nó (`usecase.AgentPromptRunner`, `AICompleter`, ...) bằng cách gọi `agentrelay`. Không có cổng nào đổi chữ ký.

### 2.4 Agent: prompt `agent.execPrompt` qua stdin

Chỉ đổi phía agent; JSON-RPC đã mang `prompt` trong `params` nên **giao thức dây và backend không đổi**.

- `agent-print-mode-exec.ts`: `args = ['--print', ...]` không còn chứa prompt; `stdio: ['pipe', 'pipe', 'pipe']`; sau `spawn`, ghi `initFile + '\n' + prompt` vào `child.stdin` rồi `end()`; bắt lỗi `EPIPE` như `ai-complete-cli-session.ts` đã làm (`child.stdin.on('error', ...)`). Tôn trọng backpressure với prompt lớn.
- Công tắc quay lui: biến môi trường `ORCA_AGENT_PROMPT_VIA_ARGV=1` đưa về hành vi cũ. Mặc định mới là stdin. Ghi một dòng log khi dùng công tắc, không ghi nội dung.
- Không thêm tên vào `AGENT_FEATURES`: backend không cần biết agent dùng đường nào.
- Hành vi cần giữ: `--output-format stream-json --verbose`, `--permission-mode plan`, `--allowedTools`, `--mcp-config` vẫn là đối số riêng; hồ sơ ghi và chế độ chỉ-đọc không đổi.
- Lợi ích kiểm chứng được: không còn trần đối số của hệ điều hành (kể cả Windows), prompt không hiện trong danh sách tiến trình, prompt bắt đầu bằng `-` không bị hiểu thành cờ.

### 2.5 Dọn mã chết

Xoá `request-service/usecase/prompt_registry.go` (và chỗ nào khai báo nó) trong giai đoạn 3; CR-REQ-034 sẽ đưa `PromptRegistry` thật vào đúng chỗ.

### 2.6 Ba giai đoạn, mỗi giai đoạn tự đứng được

| Giai đoạn | Việc | Rủi ro | Kiểm chứng chính |
|---|---|---|---|
| 1. Relay dùng chung | Tạo `common/agentrelay`; chuyển lần lượt request-service, task-service, workflow-service, orchestration-service, git-gateway-service sang dùng; xoá bản sao | Mất hành vi vận chuyển (đầu ra theo dòng, huỷ, thời gian chờ) | Test hiện có của từng relay chạy lại; acceptance `request`, `matrix`, `qa`; e2e native-execute |
| 2. Stdin ở agent | Đổi `agent-print-mode-exec.ts`; thêm test prompt lớn và prompt bắt đầu bằng `-` | Phiên bản `claude` không đọc stdin ở `--print` kèm `stream-json` | Test với `claude` thật và với `claude` giả; chạy được trên Linux, macOS, Windows |
| 3. Thư mục `prompts/` | Ghi golden, tạo gói và mẫu, đổi nơi gọi, xoá hàm cũ và `PromptRegistry` chết | Lệch khoảng trắng hoặc thứ tự khối | Golden trùng từng byte; toàn bộ test prompt hiện có |

Giai đoạn 2 độc lập với hai giai đoạn còn lại và có thể làm trước; nó là phần giá trị cao nhất so với công sức.

## 3. Quyết định thiết kế

| # | Quyết định | Lý do |
|---|---|---|
| P1 | Một module `prompts/` dùng chung, không phải mỗi service một thư mục | Một nơi để xem, rà soát và sau này đánh phiên bản; mọi service build cùng monorepo nên không thêm gánh nặng phát hành |
| P2 | Đầu vào là struct thuần, không import `internal/` | Module dùng chung không thể phụ thuộc miền của service |
| P3 | Văn bản là file `.tmpl` nhúng bằng `go:embed`, không phải chuỗi trong mã | Đổi câu chữ không đụng logic; dễ diff và rà soát |
| P4 | Di chuyển thuần có golden byte-for-byte | Không lẫn thay đổi hành vi vào refactor |
| P5 | `common/agentrelay` thay cả `execPrompt` lẫn `ai.complete` | Cùng lỗi, cùng timeout, cùng phân loại ở một nơi |
| P6 | Stdin chỉ là thay đổi của agent | Không cần phối hợp phát hành backend và agent; có công tắc quay lui |
| P7 | Giữ nguyên prompt MCP ở chỗ cũ | Không đi qua dev server; ngoài mục tiêu |

## 4. Tiêu chí chấp nhận

- [ ] Mọi prompt gửi tới dev server (mục 2.1) kết xuất từ `backend-go/prompts/<luồng>/`; không còn hàm dựng prompt nào nằm trong `adapter/` hay trong file vận chuyển.
- [ ] Mỗi luồng có thư mục riêng; `analysis/` có bốn file riêng cho Solution, chẩn đoán, Findings, Answer.
- [ ] Golden: với bộ fixture của mục 2.2, đầu ra mới trùng từng byte với đầu ra cũ.
- [ ] Không còn nơi nào ngoài `common/agentrelay` gọi `Relay`/`RelayByDevServer` với method `agent.execPrompt` hoặc `ai.complete`; hai struct `agentExecPromptParams` bị xoá.
- [ ] Phân loại lỗi (`READONLY_MODE_UNSUPPORTED`, `KNOWLEDGE_*`, hết hạn) cho cùng kết quả ở request-service như trước.
- [ ] Giao diện vẫn nhận đầu ra theo dòng của `agent.execPrompt` và vẫn huỷ được lượt chạy.
- [ ] Agent: `claude` nhận prompt qua stdin; một prompt 500 KB chạy thành công trên Linux, macOS và Windows; một prompt bắt đầu bằng `-` được hiểu là nội dung; nội dung prompt không có trong `ps`/danh sách tiến trình.
- [ ] `ORCA_AGENT_PROMPT_VIA_ARGV=1` đưa agent về hành vi cũ và có test.
- [ ] `PromptRegistry` chết bị xoá; không có tham chiếu treo.
- [ ] Acceptance `request`, `matrix`, `qa`, UI `qa-bug-task-native-flow` (stub) và `run_ui_flow.py --real native-execute` xanh sau mỗi giai đoạn.
- [ ] Không file nào vượt `max-lines`; không thêm `max-lines` disable.

## 5. Kiểm thử

- Golden theo luồng và theo loại Request (mục 2.2) trong `prompts/<luồng>/*_test.go`, sinh lại bằng một cờ rõ ràng và có kiểm tra trong CI rằng không sinh lại âm thầm.
- `common/agentrelay`: bảng kiểm cho `Target` (dev server hay connection), `ReadOnly/WriteProfile/Contract`, phân loại lỗi, timeout dư, phân tích kết quả (kể cả agent cũ không gửi `usage`, `toolCalls`).
- Mỗi service: test adapter mỏng với `agentrelay` giả; test hợp đồng hiện có không đổi.
- Agent: test đơn vị chạy `agent-print-mode-exec` với một `claude` giả đọc stdin; prompt lớn (500 KB) và prompt bắt đầu bằng `-`; `EPIPE` khi tiến trình thoát sớm; công tắc quay lui. Thêm một ca chạy với `claude` thật khi có tài khoản đã đăng nhập (chạy tay, không bắt buộc trong CI).
- E2E: `run_ui_flow.py --ai-stub` (claude giả) và `--real native-execute` sau giai đoạn 2; so sánh số lần gọi công cụ và thời lượng để phát hiện lệch.
- Lưu ý khi chạy e2e thật: không sửa checkout trong lúc `--real` chạy (guard quiet-checkout).

## 6. Rủi ro và điểm chưa kiểm chứng

- **R1: `claude --print` đọc stdin cùng `--output-format stream-json --verbose`.** `ai.complete` đã dùng stdin với `-p --output-format json`, nhưng tổ hợp của `execPrompt` (stream-json, `--permission-mode plan`, `--mcp-config`) chưa được thử. Giai đoạn 2 phải xác nhận bằng `claude` thật trước khi làm mặc định; công tắc quay lui là lưới an toàn.
- **R2: lệch từng byte khi chuyển sang `text/template`.** Khoảng trắng và dòng trống dễ lệch; golden bắt được, nhưng tốn công. Có thể giữ phần nối chuỗi trong Go cho những chỗ quá phức tạp (ví dụ `planTypeRules`) và chỉ đưa văn bản dài ra mẫu.
- **R3: gom relay làm mất hành vi riêng của một service.** `task-service` thêm `interpolateOutputs`, `workflow-service` có hồ sơ môi trường (`BuildAgentEnv`) và `orchestration-service` có bộ điều phối worker; các phần đó nằm trên `agentrelay`, không bên trong. Cần đọc kỹ từng bản sao trước khi xoá. Chưa đọc `worker_dispatcher.go` và `agent_step_executor.go` đến chi tiết.
- **R4: module `prompts/` dùng chung nghĩa là đổi prompt kéo theo build lại mọi service dùng nó.** Chấp nhận được vì mọi service build cùng monorepo; cần `go.work` và CI nhận module mới.
- **R5:** các file `.tmpl` không có kiểm tra kiểu như mã Go; thiếu trường chỉ lộ lúc kết xuất. Dùng `Option("missingkey=error")` và test kết xuất cho mọi mẫu.
- **R6:** CR-REQ-034 cũng muốn sở hữu cấu trúc thư mục prompt; nếu hai CR làm song song sẽ xung đột. Thứ tự đề xuất ở mục 8.
- Chưa chạy hệ thống; mọi mô tả hiện trạng dựa trên đọc mã ngày 2026-10-19.

## 7. Câu hỏi còn mở

1. Giữ một module `prompts/` dùng chung (đề xuất) hay mỗi service một `internal/prompts/` như CR-REQ-034 đã viết? Phương án thứ hai tránh phụ thuộc chéo nhưng prompt của git-gateway, task-service và request-service lại nằm ba nơi.
2. Có chuyển luôn các mẫu MCP vào `prompts/mcp/` trong CR này không? Đề xuất: không.
3. Prompt trong định nghĩa workflow do người dùng soạn (`workflow-service`) không thuộc catalog này; chỉ relay của nó được gom. Xác nhận phạm vi.
4. Có cần tắt công tắc `ORCA_AGENT_PROMPT_VIA_ARGV` sau một thời gian ổn định không (xoá hẳn đường argv)?

## 8. Tác động tới CR hiện có

| CR | Cần sửa gì |
|---|---|
| CR-REQ-034 | Mục 2.3: `PromptRegistry` đọc từ `backend-go/prompts/<luồng>/` thay vì `request-service/internal/prompts/`, và quy ước `v<semver>.tmpl` áp dụng lên cấu trúc này khi CR đó làm; `AIGateway` bọc `common/agentrelay` thay vì hai relay riêng. **Thứ tự: CR-REQ-042 trước, CR-REQ-034 sau** |
| CR-REQ-007, 008 | Prompt Solution, chẩn đoán, Findings, Answer chuyển sang `prompts/analysis/` |
| CR-REQ-012, 026 | Prompt Plan và OpenSpec chuyển sang `prompts/plan_native/`, `prompts/plan_openspec/`, `prompts/solution_openspec/` |
| CR-REQ-013, 029 | Prompt thực thi task và khối kết quả chuyển sang `prompts/task_execution/` |
| CR-REQ-033 | Ghi chú: `agent.execPrompt` nhận prompt qua stdin; giao thức không đổi |
| CR-REQ-041 | Phản hồi chỉnh vẫn nằm trong khối `<feedback>` của mẫu `analysis/_feedback.tmpl` |

## 9. Tham chiếu

- Prompt hiện tại: `backend-go/services/request-service/internal/usecase/{solution,plan,openspec_plan,openspec_solution,clarification_answers}_prompt.go`, `prompt_redaction.go`, `prompt_registry.go`; `request-service/internal/adapter/grpcclient/classification_prompt.go`; `task-service/internal/adapter/grpcclient/simple_executor.go` (`buildExecutePrompt`), `task-service/internal/usecase/{ai_decompose,generate_agent_prompt}.go`, `task-service/internal/domain/execution_result_prompt.go`; `git-gateway-service/internal/usecase/{agent_prompt,commit_message_prompt}.go`
- Relay hiện tại: `request-service/internal/adapter/grpcclient/{agent_prompt_relay,ai_completion_relay,request_classifier}.go`; `task-service/internal/adapter/grpcclient/{simple_executor,aidecompose_relay}.go`; `workflow-service/internal/adapter/infrafleetclient/agent_step_executor.go`; `orchestration-service/internal/adapter/infrafleetclient/worker_dispatcher.go`; `git-gateway-service/internal/adapter/grpcclient/relay_executor.go`
- Agent: `agent/src/relay/agent-print-mode-exec.ts`, `ai-complete-cli-session.ts`, `agent-protocol-features.ts`
- `backend-go/proto/orca/infrafleet/v1/infrafleet.proto` (`Relay`, `RelayByDevServer`, `RelayStream`)
- [CR-REQ-034](./CR-REQ-034-ai-governance-budgets-evals-prompt-versioning.md), [CR-REQ-033](../agent-capabilities/CR-REQ-033-agent-readonly-worktree-and-capability-report.md), [CR-REQ-041](../solution-analysis/CR-REQ-041-analysis-revision-by-prompt.md)
- Mới: `backend-go/prompts/`, `backend-go/common/agentrelay/`, `ci/check-prompt-golden.sh`
