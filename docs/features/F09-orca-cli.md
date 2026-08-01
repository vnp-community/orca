# F09 — Orca CLI

| Thuộc tính | Giá trị |
|-----------|---------|
| **ID** | F09 |
| **Tên** | Orca CLI |
| **Ưu tiên** | P1 — Should Have |
| **Trạng thái** | ✅ Đã phát hành |
| **Tham chiếu PRD** | §3.8 |
| **Tham chiếu URD** | UR-071 |
| **Tham chiếu SRS** | FR-9.2 |
| **ADR References** | — |
| **HLD References** | C2 |

---

## Mô tả

CLI tool cho phép agent và script tự động hóa mọi workflow của Orca — tạo worktree, snapshot terminal, tương tác với UI, và chạy Orca ở chế độ headless. Orca CLI là cầu nối để AI agent tự điều khiển Orca.

---

## Vấn đề cần giải quyết

Developers và AI agents cần scripting Orca như một công cụ trong pipeline tự động hóa. Không có CLI, mọi thứ phải thực hiện qua GUI — không thể tích hợp vào CI/CD hoặc để agent tự khởi động worktree.

---

## Tính năng chi tiết

### Core Commands

```bash
# Worktree management
orca worktree create [--base <branch>] [--agent <type>] [--prompt <text>]
orca worktree list [--json]
orca worktree remove <id> [--force]

# Agent control
orca agent status [--worktree <id>] [--json]
orca agent send <prompt> [--worktree <id>]

# Terminal interaction
orca snapshot [--worktree <id>]           # Chụp trạng thái terminal
orca click <selector>                      # Click UI element
orca fill <selector> <value>              # Fill input field

# Server mode
orca serve [--port <port>]               # Chạy headless server
```

### Output Formats
- JSON output với `--json` flag
- Human-readable output mặc định
- Exit codes: 0 = success, 1 = error, 2 = timeout

### Headless Mode (`orca serve`)
- Orca chạy không có UI (headless)
- Tất cả commands hoạt động qua HTTP API
- Phù hợp cho Linux server không có display
- Dùng cho CI/CD pipeline

### Computer Use Integration
- `orca click` và `orca fill` cho phép agent tương tác với desktop UI
- Screenshot-based computer use
- Form automation

### IPC Architecture
- CLI giao tiếp với Orca daemon qua Unix socket
- Daemon xử lý commands và trả về kết quả
- Timeout configurable

---

## Use cases

### Agent-driven workflow
```bash
# Agent tự tạo worktree và bắt đầu làm việc
orca worktree create --base main --agent claude --prompt "Fix the login bug"

# Check status
orca agent status --json

# Capture output sau khi xong
orca snapshot --worktree <id>
```

### CI/CD Integration
```bash
# Trong GitHub Actions
orca serve --port 7777 &
orca worktree create --base $GITHUB_BASE_REF --agent codex
# ... wait for completion ...
orca agent status --json | jq '.status'
```

### Automation Script
```bash
#!/bin/bash
# Chạy mỗi sáng: tạo worktree cho backlog items
for issue in $(linear-cli list --status "To Do" --json | jq -r '.[].id'); do
  orca worktree create --base main --prompt "$(linear-cli show $issue)"
done
```

---

## Tiêu chí chấp nhận

- [ ] `orca worktree create` tạo worktree trong < 30 giây
- [ ] `orca snapshot` trả về terminal content chính xác
- [ ] `--json` output valid JSON
- [ ] Exit code 0 khi thành công, 1 khi lỗi
- [ ] CLI hoạt động trên macOS, Linux, Windows
- [ ] `--help` có documentation đầy đủ cho mọi command

---

## Yêu cầu kỹ thuật

| Thành phần | Chi tiết |
|-----------|---------|
| **CLI entry point** | `src/cli/` |
| **Binary** | `out/cli/index.js` → `orca` bin |
| **Build** | `pnpm build:cli` |
| **IPC** | Unix socket → Orca daemon |
| **Headless** | `orca serve` → Electron headless |
| **Computer use** | `src/main/computer/` |
| **CLI command name** | `src/shared/orca-cli-command-name.ts` |
| **Platform** | macOS, Linux, Windows (PowerShell) |

---

## Metrics

| KPI | Target |
|----|-------|
| Command response time | < 500ms |
| Worktree create via CLI | < 30 giây |
| JSON output validity | 100% |
