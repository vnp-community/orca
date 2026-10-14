# TASK-TDD-002-T2-01: agent.execPrompt: tham số `knowledgeRoot` (MCP chỉ đọc) và khai báo khả năng

**From Solution:** [SOL-TDD-002](../solutions/SOL-TDD-002-tdd-agent-as-knowledge-hub.md) mục 3.2
**Checklist:** [T2.1](CHECKLIST-TDD-002-knowledge-hub.md)
**Priority:** P0 · **Area:** agent (TypeScript) + request-service/infra-fleet (Go)
**Files:** `agent/src/relay/agent-exec-prompt-options.ts`, `agent-print-mode-exec.ts`, mới `agent-exec-prompt-knowledge-hub.ts`, `agent-readonly-tool-policy.ts` (`ClaudeFlagSupport`), `agent-protocol-features.ts`, `agent-capability-report.ts`, test `*.test.ts` đi kèm; `backend-go/services/infra-fleet-service/internal/domain/agent_features.go`; `request-service/internal/domain/agent_route_selection.go`, `adapter/grpcclient/agent_prompt_relay.go`, `usecase/ports_solution.go` (`AgentPromptInput`); xoá `usecase/agent_capabilities.go` (stub)
**Depends on:** T1.3, T1.4
**Status:** `[ ]` TODO

---

## Context

Orca không được tự gọi MCP, nhưng agent do Orca chạy phải gọi được MCP của tdd-agent. Thiết kế an toàn: Orca **không** gửi đường dẫn tệp cấu hình hay lệnh; nó chỉ gửi `knowledgeRoot` (đường dẫn tuyệt đối của repo có `.tdd-agent/`). Agent tự dựng cấu hình MCP với lệnh cố định `tdd-agent -C <realpath> mcp serve` và danh sách tool cho phép cố định, rồi truyền cho `claude` bằng `--mcp-config` và `--allowedTools`. Như vậy không có chuỗi lệnh nào của Orca/Request đi vào tiến trình.
Hiện `agent.execPrompt` có `accessMode`, `workspaceKind`, `reportChanges`, `resultBlock` (parser `agent-exec-prompt-options.ts`) nhưng không có tham số MCP. Orca đọc khả năng của dev server qua `DevServerCapability.HasFeature(...)` (ví dụ `agent.execPrompt.readonly`), không qua `usecase/agent_capabilities.go` (file này là stub trả `nil, nil`, không ai gọi).
Giới hạn repo: file `.ts` không quá 300 dòng, không tắt `max-lines`; đặt mã mới trong file mới.

## Việc cần làm

1. `agent-exec-prompt-options.ts`: parse `knowledgeRoot` (chuỗi) và `proposeChangesets` (boolean, mặc định false); từ chối kiểu sai (`InvalidParams`).
2. File mới `agent-exec-prompt-knowledge-hub.ts`: `prepareKnowledgeHub({root, proposeChangesets, stepId, workDir})` → kiểm `root` là tuyệt đối, `realpath` tồn tại, là thư mục, có `.tdd-agent/config.toml`, không thoát khỏi thư mục cho phép (từ chối symlink dẫn ra ngoài); ghi `<workDir>/tmp/<stepId>.mcp.json` (mode 0600) có đúng một server `tdd-agent` với `command: "tdd-agent"`, `args: ["-C", <realpath>, "mcp", "serve"]`; trả `{args, cleanup}`.
3. Danh sách tool cố định: `mcp__tdd-agent__kg_get_element`, `kg_neighbors`, `kg_resolve_scope`, `kg_query`, `kg_search`, `kg_snapshots`, `source_get`; thêm `mcp__tdd-agent__changeset_propose` chỉ khi `proposeChangesets`. Không bao giờ đưa `run_start`, `run_resume`, `approval_submit`, `proposals_list` vào.
4. `agent-print-mode-exec.ts`: khi có tham số, thêm `--mcp-config <file>` và `--allowedTools <các tool>` (và `--strict-mcp-config` nếu probe cờ cho phép); kết hợp với cờ readonly theo kết luận T1.4; `cleanup()` trong `finally` kể cả khi timeout hoặc lỗi.
5. `agent-readonly-tool-policy.ts`: thêm `mcpConfig` và `allowedTools` vào `ClaudeFlagSupport` (probe `claude --help`, cache theo PATH như hiện nay); thiếu cờ ⇒ trả lỗi có mã `KNOWLEDGE_HUB_UNSUPPORTED`, không chạy lặng lẽ không MCP.
6. `agent-capability-report.ts`: thêm `tdd-agent` vào `CAPABILITY_TOOL_ALLOWLIST`; báo danh sách skill đã cài thuộc allowlist tên (`tdd-knowledge`, `tdd-openspec-author`, `tdd-knowledge-writeback`, `tdd-orca-task`) bằng cách kiểm `~/.claude/skills/<tên>/SKILL.md` (chỉ tên và hash nội dung, không đọc ra ngoài allowlist).
7. `agent-protocol-features.ts`: thêm `agent.execPrompt.knowledgeHub` **sau khi** mã chạy được (quy ước của file); cập nhật `agent-compat-matrix.test.ts`.
8. Go: `FeatureKnowledgeHub` trong `agent_features.go`; `agent_prompt_relay.go` thêm `KnowledgeRoot`, `ProposeChangesets` vào `agentExecPromptParams` (omitempty) và chỉ gửi khi chọn route có feature (giống `ReadOnlyEnforced` sau kiểm khả năng); `AgentPromptInput` thêm hai trường; chọn route trong `agent_route_selection.go`.
9. Xoá `usecase/agent_capabilities.go` (không có tham chiếu); không để stub.

## Tiêu chí hoàn thành

- [ ] Agent từ chối `knowledgeRoot` không tuyệt đối, không tồn tại, thiếu `.tdd-agent/config.toml`, hoặc symlink thoát ra ngoài.
- [ ] Cấu hình MCP sinh ra đúng một server, lệnh và danh sách tool cố định; file bị xoá sau mỗi run kể cả khi lỗi và timeout.
- [ ] Orca chỉ gửi tham số khi dev server khai báo `agent.execPrompt.knowledgeHub`; agent cũ không bị gửi khoá lạ.
- [ ] Không còn stub `GetAgentCapabilities`.

## Kiểm thử

- Vitest: parser tùy chọn (hợp lệ và các trường hợp từ chối), bộ dựng cấu hình (golden), dọn file, probe cờ thiếu, báo skill.
- Go: `buildAgentParams` có và không có tính năng; chọn route.
- Chạy thật một lần trên repo thử: `claude` gọi được `kg_search` qua cấu hình do agent dựng.

## Rủi ro và ghi chú

- Thiết kế thay thế `mcpConfig`/`allowedTools` do Orca gửi (bản đầu của SOL-002) bằng `knowledgeRoot`: tránh để Orca đưa đường dẫn hoặc lệnh tuỳ ý vào tiến trình.
- `tdd-agent` phải có trong PATH của agent; nếu không, báo trong capability và Orca không bật.
