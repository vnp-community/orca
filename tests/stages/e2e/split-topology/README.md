# E2E tách máy: Orca một nơi, dev server một nơi

Môi trường e2e của [TDD-WF-AGENT-001 §11.1](../../../../docs/integrate/TDD-WF-AGENT-001-workflow-agent-pipeline.md): control plane
Orca và các dev server chạy ở hai project Docker Compose khác nhau, chỉ nối qua một mạng thay cho Internet.

```
project orca-e2e (máy Orca)                    project orca-e2e-devservers (máy dev server)
  orca-go-net: mọi service, postgres, nats       devserver-a  (agent.js, git, claude giả, tdd-agent)
  api-gateway  ── orca-e2e-wan (orca-gateway) ── devserver-b  (profile multi)
```

- Dev server chỉ tới được `ws://orca-gateway:8081/agent`; control plane chỉ tới dev server qua kết nối agent.
- Token agent do `split-up.sh` cấp phía host bằng `ORCA_AGENT_API_SECRET`; secret không vào container dev server.
- Stack dùng project `orca-e2e` (volume riêng) và Vault dev-mode; không sửa `deploy/dev/.env`, không đụng project `orca-go`.

## Chạy

```bash
TDD_AGENT_SRC=/opt/repos/tdd-agentic-ai bash tests/stages/e2e/split-topology/split-up.sh --build --multi
python3 tests/stages/e2e/split-topology/verify_topology.py --multi        # TOP-01..07
python3 tests/stages/e2e/split-topology/suite_workflow_tdd_agent.py       # TWA-*
bash tests/stages/e2e/split-topology/split-down.sh --volumes
```

`ORCA_E2E_GATEWAY_URL` mặc định là `http://localhost:${API_GATEWAY_PUBLIC_PORT}`; cổng bind vào `127.0.0.1` (`SPLIT_BIND_IP` để đổi).
Báo cáo: `reports/split-topology.{json,md}`, `reports/workflow-tdd-agent.{json,md}` (git bỏ qua).

| Biến | Ý nghĩa |
|---|---|
| `TDD_AGENT_SRC` | Thư mục repo `tdd-agentic-ai`: build wheel và cài tdd-agent + tsagent vào image dev server |
| `AI_MODE` | `stub` (mặc định, `claude` giả của `orca-flows`), `real` (Claude Code thật, cần `INSTALL_CLAUDE_CLI=1` và `CLAUDE_HOME_A=$HOME/.claude`), `none` |
| `ORCA_E2E_REAL_AI=1` | Bật các ca change request (TWA-07..09, TWA-16) cần mô hình ngôn ngữ ở gate G0/G2 |
| `--devservers` | Chỉ khởi động lại dev server với token mới (token dùng một lần, TTL 1 giờ) |

## Fixture trên dev server

| Đường dẫn | Từ đâu | Dùng cho |
|---|---|---|
| `/srv/projects/fixture-repo` | `orca-flows/ui-live/fixture-repo/template` | TOP-06/07 (project gắn dev server, bước `shell`) |
| `/srv/projects/tdd-project` | `fixtures/tdd-project` (URD/SRS/SAD nhỏ của test tdd-agent) | TWA-*: `tdd-agent init --no-prose --no-generate`, `[testspec] llm = "none"`, MCP ghi + duyệt cho client `orca-workflow` — không cần mô hình ngôn ngữ |

## Giới hạn

- `deploy/dev` cố định `container_name` (`orca-go-*`): không chạy song song với stack `orca-go` khác; `split-up.sh` từ chối khi thấy.
- Hai mạng bridge trên một Docker host mô phỏng cách ly mạng, không mô phỏng độ trễ hay mất gói.
- Container dev server không tự khởi động lại: agent chết là lỗi test. Khởi động lại bằng `split-up.sh --devservers`.
- Vault là dev-mode, nằm trong bộ nhớ: **đừng `docker stop`/`start` stack** — khoá mất, admin không đăng nhập được nữa
  (`invalid_credentials`). Hỏng thì `split-down.sh --volumes` rồi `split-up.sh` lại.
- Token agent chỉ được giữ 60 giây chờ kết nối (`agentwsserver.DefaultConnectTimeout`), nên `split-up.sh` build image dev server
  trước rồi mới cấp token.
