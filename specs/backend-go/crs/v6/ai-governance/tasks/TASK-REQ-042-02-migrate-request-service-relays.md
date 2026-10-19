# TASK-REQ-042-02: Chuyển relay của `request-service` sang `agentrelay`

**From Solution:** [BE-REQ-SOL-042](../solutions/BE-REQ-SOL-042-prompt-catalog-shared-agent-relay-and-stdin.md)
**Priority:** P1
**Service:** `request-service`
**File:** `services/request-service/internal/adapter/grpcclient/{agent_prompt_relay,ai_completion_relay,request_classifier}.go` (sửa), `cmd/server/wire_*.go`, test hiện có
**Depends on:** TASK-REQ-042-01
**Status:** [x] DONE (2026-10-20)

---

## Context

- `AgentPromptRelay` cài `usecase.AgentPromptRunner`; `AICompletionRelay` và `RelayClassifier` cài `AICompleter`/bộ phân loại.
- Hành vi cần giữ: chọn `RelayByDevServer` hay `Relay`, `slackOverTimeout`, phân loại `READONLY_MODE_UNSUPPORTED` và `KNOWLEDGE_*`, `parseAgentResult` kèm `statsFromAgent`.

## Việc cần làm

1. `AgentPromptRelay.ExecPrompt` dựng `agentrelay.ExecParams` bằng `ReadOnly/WriteProfile/Contract` rồi gọi `agentrelay.ExecPrompt`; ánh xạ `agentrelay.Err*` sang `usecase.ErrAgentReadonlyUnsupported`, `ErrAgentKnowledgeHubUnavailable`.
2. `AICompletionRelay.Complete` và `relayAIComplete` (classifier) gọi `agentrelay.Complete`.
3. Xoá `agentExecPromptParams`, `buildAgentParams`, `parseAgentResult`, `classifyAgentRelayError`, `statsFromAgent` và hằng liên quan khỏi gói này (chuyển sang `agentrelay`); không để mã chết.
4. Giữ nguyên cổng và chữ ký của usecase.

## Kiểm thử

- Toàn bộ test hiện có của ba file vẫn xanh (đổi import nếu cần, không đổi kỳ vọng).
- So JSON gửi đi với bản chụp của task 01.
- Acceptance `request`, `matrix`, `qa`; UI `qa-bug-task-native-flow --ai-stub`.
- `go test ./services/request-service/internal/adapter/grpcclient/... -v`.

## Tiêu chí hoàn thành

- [x] Không còn struct/tham số `execPrompt` riêng trong request-service.
- [x] Hành vi lỗi và timeout không đổi.
- [x] Acceptance và UI xanh. (Xem "Kiểm chứng của người điều phối" bên dưới.)

## Rủi ro và lưu ý

- Chạy `gitnexus_impact` cho `AgentPromptRelay.ExecPrompt` và `AICompletionRelay.Complete`.

## Chuẩn hoàn thành chung (áp dụng cho mọi task, không thương lượng)

- Mã chạy thật, **không stub, không TODO, không `panic` cho lỗi thường, không bỏ qua lỗi**; mọi nhánh lỗi có mã lỗi và test.
- Chạy `gitnexus_impact` trước khi sửa symbol có sẵn và báo blast radius; chạy `detect_changes` trước khi commit (CLAUDE.md).
- Hai dialect: mọi thay đổi lưu trữ có migration và test hợp đồng trên **Postgres 16 và MySQL 8** (testcontainers).
- Không file nào vượt `max-lines` (tách file khi cần, không thêm disable); không đặt tên `helpers`, `utils`, `common`, `misc`.
- Chạy được trên macOS, Linux, Windows và qua SSH; lệnh git theo baseline 2.25 (`guides/reference/git-compatibility.md`).
- Không ghi nội dung Request, Solution, phản hồi hoặc bí mật vào log, span hay khung sự kiện (C9).
- `go build ./...` và `go test` của gói đã đổi xanh; `go vet` xanh; lint xanh; không làm hỏng test hiện có.

---

## Ghi chú triển khai (2026-10-19)

**Blast radius** (grep, GitNexus không dùng được): chữ ký công khai không đổi (`NewAgentPromptRelay`, `NewAICompletionRelay`, `NewRelayClassifier`, `usecase.AgentPromptRunner`, `ProjectAICompleter`, `RequestClassifier`), nên `cmd/server/wire_solution.go` và `wire_classification.go` **không phải sửa**. Người gọi: `usecase/run_agent_readonly_analysis.go`, `engine_openspec.go` (qua `AgentPromptRunner`), `run_solution_generation.go`, `run_plan_generation.go` (qua `ProjectAICompleter`), classifier trong `wire_classification.go`. Symbol bị xoá chỉ được dùng trong gói `grpcclient`; `truncateUTF8`, `slackOverTimeout`, `maxStdoutBytes` còn được `dev_server_executor.go` dùng (xem dưới).

### File
- Sửa: `agent_prompt_relay.go` (còn 78 dòng: `AgentPromptRelay{relay *agentrelay.Relayer}`, `execParamsFor`, `mapAgentRelayError`, `newTenantRelayer` dùng `agentrelay.WithMetadata(withTenantMetadata)`), `ai_completion_relay.go`, `request_classifier.go` (`relayAIComplete` nhận `*agentrelay.Relayer`, thêm `mapAICompleteError`).
- Mới: `agent_prompt_result.go` (`agentPromptResultFrom`, `runStatsFrom`, `maxStderrBytes`).
- Sửa nhỏ ngoài danh sách: `dev_server_executor.go` (2 dòng: `slackOverTimeout` -> `agentrelay.DefaultTimeoutSlack`, `truncateUTF8` -> `agentrelay.TruncateUTF8`, `16*1024` -> `maxStderrBytes`) vì hai symbol cũ bị xoá; giá trị không đổi (30 s).
- Đã xoá: `agentExecPromptParams`, `resultBlock`, `buildAgentParams`, `agentExecPromptResult`, `parseAgentResult`, `classifyAgentRelayError`, `statsFromAgent`, `truncateUTF8`, `trustPresetDefault`, `maxOutputBytesAsk`, `slackOverTimeout`, map `params` và giải mã `ai.complete` thủ công.
- Test: `agent_wire_golden_test.go` (mới), `ai_complete_error_mapping_test.go` (mới); `agent_prompt_knowledge_test.go`, `agent_prompt_stats_test.go` (không đổi nội dung), `solution_clients_test.go`: chỉ đổi cách gọi (`execParamsFor` thay `buildAgentParams`; `classifyAgentRelayError`/`parseAgentResult` được thay bằng hai hàm trợ giúp test chạy thật `ExecPrompt` trên giả client). Kỳ vọng giữ nguyên.

### Lệch so với văn bản task (và lý do)
- Luồng "mặc định" của request-service dùng `agentrelay.Plain` với trust `default` (có `resultBlock` nếu có nonce), **không** `agentrelay.Contract`: `Contract` thêm `reportChanges`, 1 MiB output và timeout 25 phút mà request-service chưa từng gửi, và golden `request_contract.json`/`request_plain.json` không có các khoá đó. Đột biến thử (đổi sang `Contract`) làm hai ca golden fail, đã hoàn lại.
- `ErrAgentKnowledgeHubUnavailable` bọc nguyên nhân bằng `%w: %w` (bản cũ `%v`); `errors.Is` cho lỗi gRPC gốc nay cũng đúng. Lỗi lấy tenant/giải mã kết quả có tiền tố `agentrelay:` thay vì `grpcclient:`; test chỉ kiểm `err != nil`.
- Lỗi `ai.complete` có mã: `mapAICompleteError` gọi lại `mapAIRelayError` trên `CompleteError.Cause` (gRPC gốc) nên `aiCompleteCodes`/`ai_relay_errors.go` giữ nguyên, không sửa.
- `stepId` rỗng nay bị bỏ khỏi JSON (bản cũ gửi `""`); cả hai nơi gọi thật luôn truyền `run.ID`.

### Kiểm chứng JSON không đổi
`TestAgentPromptRelay_SendsTheJSONRequestServiceAlwaysSent` (6 ca) và `TestAICompletionRelay_SendsTheJSONRequestServiceAlwaysSent` (2 ca) dựng input của từng luồng request-service, chạy qua relay thật với client giả, rồi so JSON đã gửi (sắp xếp khoá) với `common/agentrelay/testdata/wire/{request_*,complete_request_*}.json`; bằng nhau.

### Lệnh
```
cd backend-go/services/request-service
go build ./... && go vet ./...                 # OK
gofmt -l internal/adapter/grpcclient           # sạch
go test ./... -race -count=1                   # tất cả ok (cmd/server, e2e/stubs, adapter/contracttest, grpcclient, usecase, ...)
```
Không có test hợp đồng nào khác dùng ba relay này (`adapter/contracttest` là test kho lưu trữ, vẫn xanh).

### Chưa kiểm chứng
- Acceptance `request`, `matrix`, `qa` và UI `qa-bug-task-native-flow --ai-stub` (rule của lượt này cấm chạy e2e).
- Test `-tags=integration` Postgres/MySQL: không liên quan thay đổi này, không chạy.
- `golangci-lint` không chạy được trên máy này.

### Kiểm chứng của người điều phối (2026-10-19/20)

Stack local đã được redeploy với `request-service`, `task-service`, `git-gateway-service`, `api-gateway` (build bằng `GOFLAGS=-overlay` để bỏ qua hai file workflow không biên dịch của người khác) và `infra-fleet-service`. Agent local chạy bản `agent/out/agent.js` có stdin.

- `run_acceptance.py --ai-stub platform devserver request matrix exits ai plan analysis task notfound qa aistub`: các ca của luồng gửi prompt (RAN-01..03, AIS-02, AIS-04, PLN-08, AST-02/03/06) **xanh**. Còn đỏ chỉ có `QA-GH-01`, `QA-GH-03` (GitHub, hoãn theo yêu cầu) và `QA-TERM-01..03` (agent giả không spawn được PTY; chạy lại không stub thì xanh: `run_acceptance.py devserver qa` → 17 pass, 2 fail GitHub, 1 skip).
- `RQL-13` đỏ một lần vì `REQUEST_VERSION_CONFLICT` (phân loại nền chạy đua với bước xác nhận); chạy lại suite `request` hai lần liên tiếp: 30 pass, 0 fail.
- `run_ui_flow.py --ai-stub -- qa-bug-task-native-flow`: 3 passed.
- Để suite stub chạy được cần ba chỉnh sửa hạ tầng test (đã commit): fake `claude` đọc prompt từ stdin; AST-02/06 kiểm hợp đồng mới; dự án thử dưới `--ai-stub` đặt engine `native` và Solution thường vì fake claude không nạp được knowledge hub MCP.
- Lỗi có sẵn được phát hiện và sửa trong lúc chạy: `TruncatedForMobile` (infra-fleet) cắt xem trước đầu ra terminal theo byte, làm `ListTerminalSessions` lỗi `string field contains invalid UTF-8` và làm đỏ spec UI vì kiểm console.
