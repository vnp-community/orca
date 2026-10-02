# backend-go Solutions — MCP Tool Catalog (v5)

**CRs:** [docs/crs/v5/mcp-tool-catalog](../../../../../../docs/crs/v5/mcp-tool-catalog/README.md)
**TDD tham chiếu:** [`api-gateway.md`](../../../../tdd/services/api-gateway.md) §2, §6; [`arch/08`](../../../../tdd/architecture/08-inter-service-communication.md); mã `wscompat` thật
**Hợp đồng FE↔BE:** [CONTRACT-mcp-ui-api.md](../../CONTRACT-mcp-ui-api.md) · **Phía FE:** [specs/frontend/crs/v5/mcp-tool-catalog](../../../../../frontend/crs/v5/mcp-tool-catalog/solutions/README.md)

## Solutions

| Solution | CR | Service / khu vực | Effort | Status |
|---|---|---|---|---|
| [BE-MCP-SOL-007](./BE-MCP-SOL-007-registry-introspection-and-descriptors.md) | CR-MCP-007 | `api-gateway` (`wscompat` + `mcpserver/tools`) | Large | ✅ Implemented (unit/integration tests) — see service README for gaps |
| [BE-MCP-SOL-008](./BE-MCP-SOL-008-domain-tool-packs.md) | CR-MCP-008 | `api-gateway` (`mcpserver/tools/pack*.go`) | XL (4 đợt) | 🟡 Packs 1-3 implemented (unit tests); pack 4 (destructive/admin tools) not implemented — see service README |
| [BE-MCP-SOL-009](./BE-MCP-SOL-009-long-running-and-streaming-tools.md) | CR-MCP-009 | `api-gateway`, `infra-fleet-service` (cột `origin`) | Large | 🔲 Designed — chưa implement |

## Re-verify: khẳng định của CR vs mã thật (2026-10-01)

| Khẳng định của CR | Thực tế | Drift |
|---|---|---|
| ~417 `Register*("ns.method")`, 58 namespace (grep literal) | Đo runtime (`NewRegistry` + 6 hàm đăng ký của `main.go`, client `nil`, `go test -overlay`): **449 channel duy nhất** = 427 unary + 12 stream + 8 streamChannel + 2 binary; **57 namespace**; không tên trùng giữa các loại. Grep literal: 418 literal / 424 lời gọi (6 dùng biến `channel`); nhiều đăng ký đi qua helper (`simpleFileOp`, `registerAccountsRelay`, `registerBrowserRelay("browser."+op)`…); `git.diff` đăng ký hai lần | **Có** — CR thiếu ~32 và sai số namespace |
| Registry không có API liệt kê | Đúng | Không |
| `Dispatch` gọi được mọi channel | Chỉ `handlers` (unary). 8 channel `streamChannel` (terminal.create/subscribe, agent.start/resume/switchAccount, ephemeralVm.provision, files.watch, onboarding.openGhAuthTerminal) cần `DispatchStreamChannel` + ctx per-connection (`terminalStreamsContext`…) | **Có** |
| Tham số vị trí, `Args()` ánh xạ | Đúng; khoá không nhất quán (`worktree` vs `worktreeId`; `terminal`/`text`) | Không (củng cố) |
| Đợt 1 đọc "checks" CI | Không có channel checks | **Có** |
| Đợt 4 force-push/reset, xoá workflow, merge PR | Không có `git.reset`/force-push (BUG-021), không có `workflow.delete`; merge chỉ GitHub (`github.mergePR`) | **Có** |
| Loại trừ `devServerAgentTokens`, `admin` force-revoke | Tên thật `devServer.agentTokens.*`, `admin.forceRevokeSession/AllSessions/createUser` | Tên |
| "scrollback đã có (`channels_terminal_scrollback.go`)" | Chỉ save/restore snapshot xterm; không đọc sống/không con trỏ. Có RPC `GetTerminalScrollback`/`SendTerminalInput` ở infra-fleet nhưng chỉ lộ qua REST `httpgateway` | **Có (lớn)** |
| `agent_status`/`agent_send` | Không có channel `agent.status`/`agent.send`; dùng `terminal.agentStatus` + `terminal.send`; `agentSession.listActive` là dispatch context của orchestration, không phải phiên `agent.start` | **Có** |
| Workflow/automation tạo ở trạng thái `draft` | Không xác minh được `draft` | (chưa xác minh) |
| `Identity.Role` | Chỉ nhánh cookie; token MCP phải được gán Role bởi BE-003/005/006 | Phụ thuộc |
| Cơ chế ephemeral (T5 "core-NATS") | `SubscribeEphemeral` là JetStream ephemeral consumer | Làm rõ TDD |

## Thứ tự & phụ thuộc

```
BE-MCP-SOL-003 (khung /mcp, Identity từ token) ─┐
BE-MCP-SOL-002/012/013 (mcp-service: PolicyGate) ─┤
                                                  ▼
BE-MCP-SOL-007 (Channels(), ToolSpec, Executor, parity)
     ├──► BE-MCP-SOL-008 (pack 1 → 2; pack 3/4 chỉ bật sau 012/013)
     └──► BE-MCP-SOL-009 (pack 3: terminal/agent/workflow; cần BE-004 hook đóng phiên)
BE-MCP-SOL-007 ──► BE-MCP-SOL-010/011 (resources/prompts dùng ToolExecutor)
```
Trong 007: làm `Channels()` + `RegisterProductionChannels` + parity (CI đỏ khi thiếu) **trước** khi viết spec; 008 làm pack 1 trước, e2e đợt 1, rồi đợt 2; 009 sau khi có `ToolSession` và hook đóng phiên.

## Sửa TDD kèm theo (tổng hợp)
T1, T2 (007/008/009); T5 làm rõ (007/009/010); `infra-fleet-service.md` §3/§5 (009: `SessionOrigin`, cột `origin_*`).

## Thay đổi CONTRACT đề nghị (từ feature này)
Không có thay đổi bắt buộc. Lưu ý: `origin` (CONTRACT §5) cần FE biết hình `terminal.list` thực tế là mảng trần `[]terminalSessionView`, khác `RuntimeTerminalListResult` mà một số code FE kỳ vọng — xử lý ở FE-MCP-SOL-006.
