# Lưu trữ tsagent v1

| Mục | Giá trị |
| --- | --- |
| Ngày | 2026-10-12 |
| Hình thức | Tệp nén (phương án b của T0.2). Thư mục `backend/agents/testspec-agent/` chưa từng được commit nên không có lịch sử git để tham chiếu |
| Vị trí | `/opt/repos/archive/tsagent-v1/tsagent-v1-2026-10-12.tar.gz` (ngoài repo) |
| SHA-256 | trong `tsagent-v1-2026-10-12.tar.gz.sha256` cùng thư mục |
| Nội dung | `tsagent/`, `tests/`, `examples/`, `specs/`, `deploy/`, `Makefile`, `pyproject.toml`, `README.md`, `.env.example`, `.gitignore`, `.dockerignore` |
| Không lưu | `.venv`, `.tsagent/`, `bin/`, `__pycache__`, `.pytest_cache`, `*.egg-info`, `tmp/` (demo minipay giữ nguyên tại chỗ), `output-docs/` (giữ nguyên tại chỗ) |
| Kiểm khôi phục | Giải nén vào thư mục tạm, `make init`, `pytest`: toàn bộ test xanh (2 test bỏ qua như khi còn chạy) |

Nếu chủ dự án muốn lưu trong git (phương án a): giải nén vào một worktree mới, commit vào nhánh `archive/tsagent-v1`, tag `tsagent-v1-final`.

## Khôi phục và chạy lại v1

```bash
mkdir -p /tmp/tsagent-v1 && tar -xzf /opt/repos/archive/tsagent-v1/tsagent-v1-2026-10-12.tar.gz -C /tmp/tsagent-v1
```

```bash
cd /tmp/tsagent-v1/testspec-agent && make init && TSAGENT_STORE_DIR=/tmp/tsagent-v1/store TSAGENT_TOKEN=dev-token TSAGENT_MCP_REVIEW=1 .venv/bin/tsagent serve --port 8767
```

Dùng cổng và store riêng để không đụng bản v2.

## Lưu trữ khi chuyển repo (2026-10-13)

Khi tsagent v2 chuyển sang repo `tdd-agentic-ai`, toàn bộ `backend/agents/testspec-agent/` (gồm demo minipay ở `tmp/minipay-demo`, `output-docs/`, `bin/`) và `docs/crs/v17/testspec-agent/` (gồm tài liệu v1 `TDD-TSAGENT.md`, `KG-TSAGENT.md`, `TEST-ONTOLOGY-TSAGENT.md`) đã được xóa khỏi vnp-workplace. Hai thư mục này không nằm trong git; bản lưu trữ:

| Tệp | Ghi chú |
| --- | --- |
| `/opt/repos/archive/testspec-agent/vnp-workplace-testspec-agent-2026-10-13.tar.gz` (+ `.sha256`) | Không gồm `.venv` và cache; giải nén ở gốc vnp-workplace để khôi phục đúng đường dẫn |

`output-docs/` đã chép sang [output-docs/](output-docs/) cạnh tài liệu này.
