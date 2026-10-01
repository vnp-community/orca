# Kiểm tra API backend-go

Bộ script Python kiểm tra các API của `backend-go` qua **api-gateway** (REST `/auth`, `/v1`, `/admin/api`,
SSE `/api/trace-stream`, WebSocket `/ws`). Căn cứ: `specs/backend-go/tdd/` và mã trong
`backend-go/services/api-gateway/internal/adapter/`.

## Chạy

```bash
cd tests/backend
pip install -r requirements.txt        # requests (+ websocket-client cho suite websocket)
cp .env.example .env                   # rồi sửa ORCA_API_BASE_URL, ORCA_ADMIN_EMAIL/PASSWORD

python run_all.py                      # tất cả suite
python run_all.py auth projects        # một số suite
python api_tasks.py                    # mỗi file chạy độc lập được
python run_all.py --list               # danh sách suite
python run_all.py --verify-catalog     # đối chiếu danh mục route với mã Go
```

Mã thoát khác 0 nếu có `FAIL`. Cấu hình đọc từ `tests/backend/.env`; biến môi trường shell ghi đè.
Nếu thiếu `ORCA_ADMIN_*` sẽ dùng `BOOTSTRAP_ADMIN_*`. Xem `.env.example` cho toàn bộ biến.

## Suite

| Suite | File | Phạm vi |
|---|---|---|
| auth | `api_auth.py` | `/auth/*`, cli-tokens, paired-devices, pairing công khai |
| auth-enforcement | `api_auth_enforcement.py` | mọi route có xác thực phải trả 401 khi ẩn danh / cookie giả |
| admin | `api_admin.py` | `/v1/auth/users\|sessions\|audit-log`, `/admin/api/*` |
| tenants | `api_tenants.py` | companies, email-domains, departments, teams, profile |
| projects | `api_projects.py` | projects, members, repos, worktrees, project-groups |
| tasks | `api_tasks.py` | tasks, edges, grants, comments, execute, subtree |
| annotations-git | `api_annotations_git.py` | annotations, `/v1/git/*`, `POST /v1/worktrees` |
| automation-workflow | `api_automation_workflow.py` | automations, workflow templates/executions |
| scm-issues | `api_scm_issues.py` | SCM, OAuth, webhook, issue tracking |
| infra-orchestration | `api_infra_orchestration.py` | dev-servers, connections, agent proxy, orchestration |
| notifications-usage-ai | `api_notifications_usage_ai.py` | notifications/push/SSE, usage, ai-providers |
| websocket | `api_websocket_channels.py` | `/ws`: xác thực + probe kênh quét từ mã Go |

Cuối `run_all.py` in **độ phủ route HTTP** (155 route trong `orca_route_catalog.py`).

## Quy ước

- **Ghi dữ liệu**: tạo bản ghi có tiền tố `ORCA_TEST_PREFIX` và dọn ở cuối suite. Đặt `ORCA_ALLOW_WRITES=false`
  để chỉ chạy GET + kiểm tra xác thực.
- **Phụ thuộc hạ tầng ngoài** (git host, GitHub/Jira, dev server thật, push service): script dùng id giả/provider
  chưa kết nối và chỉ khẳng định gateway ánh xạ lỗi đúng (4xx), không 5xx/501. Muốn kiểm tra luồng thật thì
  bổ sung dữ liệu thật vào môi trường.
- **Mã trạng thái**: `5xx`, `501` (stub), lỗi mạng luôn là FAIL. Spec trạng thái: `int`, `"2xx"`, `"4xx"`,
  `"handled"` (<500 và không phải 401/403/405), `"no5xx"`.
- **Rate-limit đăng nhập** 10 lần/phút/IP: suite `auth` dùng ~6 lần đăng nhập; tránh chạy lặp liên tục.
- **WebSocket**: mặc định chỉ probe kênh chỉ-đọc (`list*/get*/status*/is*/has*...`) với args rỗng;
  `ORCA_WS_PROBE_ALL=true` probe mọi kênh (có thể đổi dữ liệu).
- Thêm route mới vào gateway → cập nhật `orca_route_catalog.py`; `--verify-catalog` báo lệch.
