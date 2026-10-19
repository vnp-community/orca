# TASK-REQ-042-02: Chuyển relay của `request-service` sang `agentrelay`

**From Solution:** [BE-REQ-SOL-042](../solutions/BE-REQ-SOL-042-prompt-catalog-shared-agent-relay-and-stdin.md)
**Priority:** P1
**Service:** `request-service`
**File:** `services/request-service/internal/adapter/grpcclient/{agent_prompt_relay,ai_completion_relay,request_classifier}.go` (sửa), `cmd/server/wire_*.go`, test hiện có
**Depends on:** TASK-REQ-042-01
**Status:** [ ] TODO

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

- [ ] Không còn struct/tham số `execPrompt` riêng trong request-service.
- [ ] Hành vi lỗi và timeout không đổi.
- [ ] Acceptance và UI xanh.

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
