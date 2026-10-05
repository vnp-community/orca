# Kiểm tra giao tiếp MCP của Orca

Bộ script Python kiểm tra MCP server của Orca (api-gateway, endpoint `/mcp`, streamable HTTP, MCP 2025-06-18)
cùng các kênh WebSocket `mcp.*` mà giao diện Settings → MCP dùng. Căn cứ:
`backend-go/services/api-gateway/internal/adapter/mcpserver/` và [docs/guides/mcp](../../docs/guides/mcp/README.md).

## Chạy

```bash
cd tests/mcp
pip install -r requirements.txt
cp .env.example .env            # rồi sửa ORCA_API_BASE_URL, ORCA_ADMIN_EMAIL/PASSWORD

python run_all.py               # tất cả suite
python run_all.py protocol tools
python check_mcp_protocol.py    # mỗi file chạy độc lập được
python run_all.py --list

python get_mcp_token.py         # tạo PAT bằng tài khoản admin, ghi ORCA_MCP_TOKEN vào .env
python get_mcp_token.py --list  # liệt kê PAT;  --revoke <id> để thu hồi

python3 mcp_list_projects.py                      # project mà token thấy (qua project_list)
python3 mcp_list_tasks.py --project <tên|uuid>    # task của một project (task_list)
python3 mcp_create_task.py --project <tên|uuid> --title "..." [--parent-id ..] [--labels a,b] [--dry-run]
```

`mcp_create_task.py` gọi `task_create` (cần scope `orca:write` và gateway bật pack 2 — `MCP_TOOL_PACKS_ENABLED=1,2`).
Qua MCP task chỉ nhận `title` (bắt buộc, ≤500 ký tự), `project_id` và `parent_id`; `labels` đặt được sau bằng `task_update`.

Mọi cấu hình đọc từ `tests/mcp/.env` (biến môi trường shell ghi đè). Thiếu `ORCA_ADMIN_*` thì dùng
`BOOTSTRAP_ADMIN_*` trong `deploy/dev/.env` (chỉ đọc các khóa `BOOTSTRAP_ADMIN_*`, `MCP_PUBLIC_BASE_URL`,
`PUBLIC_BASE_URL`). Mã thoát khác 0 nếu có `FAIL`. Xem `.env.example` cho toàn bộ biến.

## Suite

| Suite | File | Kiểm tra |
|---|---|---|
| discovery | `check_mcp_discovery.py` | `/.well-known/oauth-protected-resource`, authorization-server, scope, PKCE |
| auth-enforcement | `check_mcp_auth_enforcement.py` | 401 + `WWW-Authenticate`, token rác, cookie-only, Origin lạ, body 2 MiB, thiếu `initialize`, PAT không tạo PAT |
| protocol | `check_mcp_protocol.py` | `initialize`, thương lượng phiên bản, `ping`, `tools/resources/prompts`, lỗi JSON-RPC, vòng đời session |
| tools | `check_mcp_tools.py` | lọc công cụ theo scope, quét mọi tool đọc không cần tham số, hình dạng kết quả, tham số sai |
| task-worktree-flow | `check_mcp_task_worktree_flow.py` | `task_create/get/list/update`, `task_createFromSource`/`getSource`, (tuỳ chọn) `task_execute` + poll trạng thái |
| ws-channels | `check_mcp_ws_channels.py` | kênh `mcp.*` chỉ-đọc, `MCP_NOT_ADMIN`, độ phủ so với mã Go |
| token-lifecycle | `check_mcp_token_lifecycle.py` | tạo/liệt kê/dùng/thu hồi PAT (REST + WS), tham số sai, bị chặn sau thu hồi |

## Quy ước

- **PAT tạm:** các suite tự tạo PAT bằng phiên admin (`POST /v1/auth/mcp-tokens`) và thu hồi ở cuối. Không có tài khoản
  đăng nhập thì dùng `ORCA_MCP_TOKEN`.
- **Ghi dữ liệu** chỉ gồm task có tiền tố `ORCA_TEST_PREFIX`, xoá ở cuối qua kênh `/ws` `task.delete` (công cụ
  `task_delete` cần người phê duyệt). Thiếu `websocket-client` thì không dọn được và script in id để xoá tay.
  `ORCA_ALLOW_WRITES=false` bỏ các suite ghi.
- **Công cụ ghi/exec phụ thuộc cấu hình gateway:** mặc định `MCP_TOOL_PACKS_ENABLED=1` (chỉ công cụ đọc). Khi pack 2/3
  chưa bật, các kiểm tra liên quan tự SKIP và suite `task-worktree-flow` bị bỏ qua.
- **`task_execute` mặc định KHÔNG chạy.** Đặt `ORCA_MCP_PROBE_EXEC=true` để thử: cần dev server đang kết nối, và
  yêu cầu sẽ chờ người duyệt trong Settings → MCP → Approvals. Nếu bị từ chối trước khi chạy, script kiểm tra task không
  kẹt `in_progress`.
- **Thu hồi PAT** có hiệu lực sau tối đa khoảng một phút nên suite `token-lifecycle` chờ đến `ORCA_MCP_REVOKE_WAIT_S`
  (mặc định 75 giây).
- **Kết quả công cụ** `isError` là hợp lệ khi thiếu hạ tầng ngoài (git host, Jira); chỉ lỗi giao thức, HTTP 5xx và
  `MCP_INTERNAL` bị tính FAIL.
- MCP tắt (`MCP_ENABLED=false`) thì preflight FAIL; đặt `ORCA_MCP_EXPECT_ENABLED=false` nếu cố ý tắt (các suite SKIP).

## Chưa được kiểm tra

- Luồng OAuth đầy đủ (DCR, trang consent, đổi code lấy token) vì cần trình duyệt; chỉ kiểm tra metadata.
- Phê duyệt (`mcp.approval.decide`), kill switch, policy ghi, external server và các kênh ghi khác.
- Luồng SSE `GET /mcp` (server chủ động đẩy) và `tools/list_changed`.
- Script chưa được chạy với hệ thống thật khi viết; lần chạy đầu có thể lộ khác biệt về hình dạng phản hồi.
