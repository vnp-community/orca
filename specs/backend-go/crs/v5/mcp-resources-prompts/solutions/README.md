# backend-go Solutions — MCP Resources & Prompts (v5)

**CRs:** [docs/crs/v5/mcp-resources-prompts](../../../../../../docs/crs/v5/mcp-resources-prompts/README.md)
**TDD tham chiếu:** `api-gateway.md` §5, §6; `arch/05`, `arch/07`, `arch/08`
**Hợp đồng FE↔BE:** [CONTRACT-mcp-ui-api.md](../../CONTRACT-mcp-ui-api.md) · **Phía FE:** [specs/frontend/crs/v5/mcp-resources-prompts](../../../../../frontend/crs/v5/mcp-resources-prompts/solutions/README.md)

## Solutions

| Solution | CR | Service / khu vực | Effort | Status |
|---|---|---|---|---|
| [BE-MCP-SOL-010](./BE-MCP-SOL-010-resources-templates-subscriptions.md) | CR-MCP-010 | `api-gateway` (`mcpserver/resources`), sửa nhỏ `git-gateway-service` | Medium | ✅ Implemented (unit/integration tests) — see service README for gaps |
| [BE-MCP-SOL-011](./BE-MCP-SOL-011-prompts.md) | CR-MCP-011 | `api-gateway` (`mcpserver/prompts`, `channels_mcp.go`), `mcp-service` | Small | ✅ Implemented (unit/integration tests) — see service README for gaps |

## Re-verify: khẳng định của CR vs mã thật (2026-10-01)

| Khẳng định của CR | Thực tế | Drift |
|---|---|---|
| Nguồn dữ liệu có sẵn cho task/worktree/diff/PR/file | Có qua channel (`task.get/getDependencies/listComments`, `git.status/diff/branchDiff`, `files.readPreview`, `github.project.workItemDetailsBySlug`, `gitlab.workItemDetails`) | Không |
| Task "hoạt động" có thể đọc | `task.activity.subscribe` chỉ là luồng đẩy 7 subject JetStream (ephemeral), không có API lịch sử | **Có** — resource task = trạng thái + comment |
| Chặn path traversal + symlink | `localfs.resolve` chỉ làm sạch lexical, không `EvalSymlinks`; không có cờ symlink trong `StatFileResponse` | **Có (quan trọng)** — sửa `git-gateway` + cờ tắt resource `file` |
| Dùng "cùng danh sách che dữ liệu nhạy cảm với `files_read`" | Chưa có danh sách nào | **Có** — tạo `sensitive_path_rules.go` dùng chung |
| Subscribe task/worktree/PR/terminal | Task: có nguồn; worktree: chỉ `WatchWorktree` (qua `files.watch`); PR: không có sự kiện (rate limit `gh`) ; terminal: ring BE-009 | PR không hỗ trợ v1 |
| NATS ephemeral | `SubscribeEphemeral` = JetStream ephemeral consumer, không phải core-NATS | Làm rõ T5 |
| Lưu prompt tenant ở mcp-service | Chưa có `mcp-service` / `mcpserver` trong repo (greenfield) | Hiển nhiên |
| Locale theo `Accept-Language`/hồ sơ | `Identity` không có locale; trường hồ sơ chưa xác minh | (chưa xác minh) |
| CONTRACT có đủ mã lỗi cho prompt | Không (`MCP_PROMPT_*` thiếu) | **Có** — đề nghị đổi CONTRACT |

## Thứ tự & phụ thuộc
```
BE-MCP-SOL-007 (ToolExecutor, redaction, Catalog) ─► BE-MCP-SOL-010 ─► BE-MCP-SOL-011
BE-MCP-SOL-004 (notifier, resume, session.closed) ──► 010 (subscribe), 011 (list_changed)
BE-MCP-SOL-009 (ring terminal) ─► 010 (resource terminal)
git-gateway symlink fix ─► bật MCP_RESOURCE_FILE_ENABLED
```
Làm 010 phần đọc (list/templates/read) trước, sau đó `subscribe`, cuối cùng bật `file` khi git-gateway xong; 011 độc lập kênh admin có thể làm song song với 010 (built-in chỉ cần `ResourceRouter` ở bước render).

## Sửa TDD kèm theo (tổng hợp)
T1, T2, T5 (làm rõ), T7 (bảng `custom_prompts` thuộc `mcp-service`); `git-gateway-service.md` §3 (symlink).

## Thay đổi CONTRACT đề nghị
Thêm vào §2.3: `MCP_PROMPT_INVALID`, `MCP_PROMPT_NAME_CONFLICT`, `MCP_PROMPT_VERSION_CONFLICT`, `MCP_PROMPT_BUILTIN_READONLY` (xem BE-MCP-SOL-011).
