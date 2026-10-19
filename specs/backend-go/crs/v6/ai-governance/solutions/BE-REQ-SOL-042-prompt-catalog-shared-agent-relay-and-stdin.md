# BE-REQ-SOL-042: Thư mục prompt theo luồng, `common/agentrelay` dùng chung và `agent.execPrompt` qua stdin

> **📝 Chưa triển khai (0/8 task).** Soạn 2026-10-19 từ đọc mã. Giai đoạn 2 (stdin) độc lập, làm trước được.

**CR:** [CR-REQ-042](../../../../../../docs/crs/v6/ai-governance/CR-REQ-042-prompt-catalog-shared-agent-relay-and-stdin.md)
**Service:** `backend-go/prompts` (module mới) · `backend-go/common/agentrelay` (gói mới) · `request-service` · `task-service` · `workflow-service` · `orchestration-service` · `git-gateway-service` · **`agent/`** (TypeScript)
**TDD tham chiếu:** [`arch/03`](../../../../tdd/architecture/03-clean-architecture-guidelines.md), [`arch/08`](../../../../tdd/architecture/08-inter-service-communication.md) (relay), [`arch/07`](../../../../tdd/architecture/07-security-architecture.md) (bí mật không vào prompt, tham số tiến trình)
**Phụ thuộc solution:** [BE-REQ-SOL-007](../../solution-analysis/solutions/BE-REQ-SOL-007-solution-generation-options-and-selection.md), SOL-008, SOL-012, SOL-013, SOL-026, SOL-029. Mở khoá CR-REQ-034 (phiên bản prompt, `AIGateway`).

---

## 1. Trạng thái hiện tại (re-verify)

Đã đọc ngày 2026-10-19:

- **Prompt** (hàm dựng, tất cả là chuỗi trong mã): `request-service/usecase/{solution_prompt (181 dòng), plan_prompt (101), openspec_plan_prompt (106), openspec_solution_prompt (38), clarification_answers_prompt (67), prompt_redaction (46)}.go`, `request-service/adapter/grpcclient/classification_prompt.go` (49), `task-service/adapter/grpcclient/simple_executor.go` (`buildExecutePrompt`, file 599 dòng lẫn vận chuyển), `task-service/domain/execution_result_prompt.go` (`ContractPromptSuffix`), `task-service/usecase/{ai_decompose (`buildDecomposePrompt`), generate_agent_prompt (`buildPromptGenerationPrompt`)}.go`, `git-gateway-service/usecase/{agent_prompt,commit_message_prompt}.go`. MCP: 5 mẫu `.tmpl` (en, vi) ở `api-gateway/.../mcpserver/prompts/builtin/` (không đi qua dev server).
- **`PromptRegistry`**: `request-service/usecase/prompt_registry.go`, `GetPrompt` trả `""`, không có tham chiếu nào ngoài file.
- **Relay `agent.execPrompt`** ở bốn nơi, mỗi nơi một struct tham số: `request-service/adapter/grpcclient/agent_prompt_relay.go` (`buildAgentParams`, `parseAgentResult`, `classifyAgentRelayError`, `slackOverTimeout`), `task-service/adapter/grpcclient/simple_executor.go` (`agentExecPromptParams` có `Model`, `InitFile`, `Env`, hợp đồng), `workflow-service/adapter/infrafleetclient/agent_step_executor.go` (thêm `AccountID`, `Model`, `InitFile`), `orchestration-service/adapter/infrafleetclient/worker_dispatcher.go` (prompt = `task.TaskTitle`, chỉ `stepId`, `worktreePath`, `prompt`). Lựa chọn `RelayByDevServer` (theo dev server) hay `Relay` (theo connection) lặp ở request-service và task-service.
- **Relay `ai.complete`**: `request-service/adapter/grpcclient/{ai_completion_relay,request_classifier}.go`, `task-service/adapter/grpcclient/aidecompose_relay.go`, `git-gateway-service/adapter/grpcclient/relay_executor.go` (`Complete`, chỉ gửi `prompt`). Tham số thật của agent: `prompt` (bắt buộc), `format?`, `taskId?`, `model?`, `accountId?`, `resolvedApiKey?` (`ai-complete-handler.ts`). **`resolvedApiKey` không bao giờ được gửi từ gói chung.**
- **Agent**: `agent/src/relay/agent-print-mode-exec.ts:200` `args = ['--print', initFile ? `${initFile}\n${prompt}` : prompt]`; `spawn(spec.binary, args, { stdio: ['ignore', 'pipe', 'pipe'] })`; chỉ `spec.binary === 'claude'` được hỗ trợ. `ai-complete-cli-session.ts`: `args = ['-p','--output-format','json','--tools','']`, `spawn(..., stdio: ['pipe','pipe','pipe'])`, `child.stdin.end(input.prompt)`, `child.stdin.on('error', ...)`.
- `backend-go/common` là module riêng (`go.mod`), đã import `proto` ở `auditclient`; `go.work` liệt kê các module. `backend-go/ci/` tồn tại.

### Correction relative to CR-REQ-042

1. CR liệt kê bốn relay `execPrompt`; **orchestration-service chỉ gửi ba trường và prompt là tiêu đề task**, không phải prompt dựng bởi catalog; vẫn dùng `agentrelay` nhưng không có mẫu prompt (ghi rõ để không bịa mẫu).
2. `ExecParams` phải có `accountId` và `model` (workflow-service, task-service) ngoài các trường CR đã nêu.
3. `git-gateway` `relay()` là bộ gửi chung cho mọi lệnh git/fs; **chỉ `Complete` chuyển sang `agentrelay`**, các lệnh còn lại ở nguyên chỗ.
4. `agent_prompt.go` và `commit_message_prompt.go` của git-gateway cần đọc kỹ trước khi xếp vào catalog (task 07): nếu là prompt gửi cho agent thì chuyển, nếu là chuỗi giao diện thì không.

## 2. Giải pháp

### 2.1 `common/agentrelay`

```
backend-go/common/agentrelay/
  target.go          Target{DevServerID, ConnectionID}, hàm chọn RelayByDevServer/Relay
  exec_prompt.go     ExecParams, ExecResult, ExecPrompt()
  complete.go        CompleteParams, CompleteResult, Complete()
  errors.go          ErrReadonlyUnsupported, ErrKnowledgeHubUnavailable, ErrTimeout, phân loại từ lỗi relay
  result_parse.go    phân tích resultJson (stats, changes, parsed, truncated) chịu được agent cũ
  options.go         WithMetadata(func(ctx) (ctx, error)), WithTimeoutSlack(d)
  *_test.go
```

- Phụ thuộc: `proto/gen/go/orca/infrafleet/v1` (như `auditclient`). Không phụ thuộc `internal/` của service nào.
- `ExecParams` = hợp của bốn struct hiện có: `StepID, Prompt, WorktreePath, TrustPreset, Model, AccountID, Env, InitFile, TimeoutMS, AccessMode, WorkspaceKind, ReportChanges, MaxOutputBytes, KnowledgeRoot, ProposeChangesets, PermissionProfile, ResultBlock{Nonce}`; `omitempty` đúng như các struct cũ để JSON gửi đi của mỗi luồng **giống hệt trước**. Hàm dựng `ReadOnly()`, `WriteProfile()`, `Contract()` thay `buildAgentParams` của request-service.
- `Complete`: `CompleteParams{Prompt, Model, AccountID, MaxTokens, TaskID, Format}`; không có trường khoá API.
- Metadata tenant/người dùng nhận qua `WithMetadata`; service truyền `withTenantMetadata` của nó.

### 2.2 Agent: stdin

`agent-print-mode-exec.ts`: bỏ prompt khỏi `args`; `stdio: ['pipe','pipe','pipe']`; sau `spawn`: `child.stdin.on('error', () => undefined)`; `child.stdin.end(initFile ? initFile + '\n' + prompt : prompt)`. Công tắc `ORCA_AGENT_PROMPT_VIA_ARGV=1` giữ đường cũ. Không đổi `AGENT_FEATURES`. Thử `claude` thật với `--print --output-format stream-json --verbose` cộng `--permission-mode plan` và `--mcp-config` trước khi coi stdin là mặc định (R1 của CR).

### 2.3 Catalog `backend-go/prompts`

Cấu trúc theo CR mục 2.1. Mỗi gói luồng:

```go
//go:embed *.tmpl
var files embed.FS
const ID = "analysis/solution"
func Render(in Input) (string, error)   // text/template, missingkey=error, hàm fence/redact/truncate
```

`Input` là struct thuần (không import `internal/`). Service có hàm ánh xạ miền → `Input` (mỏng, có test). Golden: `testdata/golden/<case>.txt` sinh bằng cờ `-update` và có kiểm tra CI rằng không sinh lại âm thầm.

### 2.4 Thứ tự và tách giai đoạn

Giai đoạn 2 (stdin, task 04) → giai đoạn 1 (relay, task 01 đến 03) → giai đoạn 3 (golden task 05 **trước**, rồi task 06, 07) → task 08. Golden (task 05) phải xong trước khi chạm vào bất kỳ hàm dựng prompt nào.

## 3. Quyết định thiết kế

Như CR-REQ-042 mục 3 (P1 đến P7). Bổ sung: **Q1** `agentrelay.ExecParams` giữ tên JSON chính xác của agent (`stepId`, `worktreePath`, `trustPreset`, `accountId`, `initFile`, ...); **Q2** kết quả `ExecResult` tách `Stats`, `Changes`, `Parsed` thành kiểu có tên, không `json.RawMessage` rò ra ngoài.

## 4. Phụ thuộc và thứ tự

04 độc lập. 01 → 02 → 03. **TASK-REQ-041-02** → 05 → 06 → 07 (golden chụp sau khi prompt phân tích của CR-041 đã đổi, để không phải chụp hai lần). 03 sau TASK-REQ-039-04 (cùng sửa `task-service/simple_executor.go`). 08 sau cùng. 06 cần 01 chỉ ở chỗ ánh xạ (không bắt buộc), nên 06 có thể chạy song song với 02 nếu hai người khác nhau làm.

## 5. Kiểm thử

- Golden theo luồng/loại Request; hợp đồng JSON của `ExecParams` (so sánh JSON gửi đi của từng luồng với bản chụp trước khi gom, để bảo đảm không đổi dây).
- `agentrelay`: bảng kiểm `Target`, lỗi, timeout dư, agent cũ.
- Agent: `claude` giả đọc stdin; prompt 500 KB; prompt bắt đầu bằng `-`; `EPIPE`; công tắc quay lui; Windows (chạy test spawn trên runner Windows nếu CI có, nếu không thì kiểm bằng giả lập đường dẫn và ghi giới hạn).
- E2E: acceptance `request`, `matrix`, `qa`; UI `qa-bug-task-native-flow` (stub) và `--real native-execute` sau mỗi giai đoạn.

## 6. Rủi ro và điểm chưa kiểm chứng

- R1 (stdin với `stream-json`, `--mcp-config`) chưa thử; task 04 phải thử bằng `claude` thật hoặc bằng bản `claude` thật ghi lại hành vi.
- R3: chưa đọc đến chi tiết `worker_dispatcher.go` ngoài đoạn gửi và `agent_step_executor.go` ngoài struct; task 03 đọc đầy đủ trước khi xoá.
- Golden không bắt được thay đổi ngữ nghĩa nằm ngoài chuỗi (thứ tự gọi, điều kiện chọn mẫu); test ánh xạ miền → `Input` bù phần đó.
- Module `prompts` thêm vào `go.work` và CI (`ci/`) cần nhận biết.

## 7. Câu hỏi mở

Như CR-REQ-042 mục 7. Giả định tạm cho đến khi trả lời: một module `prompts/` dùng chung (Q1); chưa chuyển MCP `.tmpl` (Q2); prompt workflow người dùng soạn ngoài phạm vi (Q3); giữ công tắc argv một bản phát hành rồi xoá (Q4).

## 8. Tham chiếu

- Các file ở mục 1; `agent/src/relay/{agent-print-mode-exec,ai-complete-cli-session,agent-protocol-features}.ts`
- [CR-REQ-042](../../../../../../docs/crs/v6/ai-governance/CR-REQ-042-prompt-catalog-shared-agent-relay-and-stdin.md), [CR-REQ-034](../../../../../../docs/crs/v6/ai-governance/CR-REQ-034-ai-governance-budgets-evals-prompt-versioning.md), kế hoạch tổng: [`IMPLEMENTATION-PLAN-CR-REQ-039-042`](../../../../../IMPLEMENTATION-PLAN-CR-REQ-039-042.md)
