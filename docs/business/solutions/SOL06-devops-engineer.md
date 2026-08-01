# SOL06 — Giải pháp cho DevOps Engineer

| Thuộc tính | Giá trị |
|-----------|---------|
| **Actor ID** | SOL06 |
| **Actor** | DevOps Engineer |
| **Tham chiếu Painpoints** | [PP06](../painpoints/PP06-devops-engineer.md) |
| **Tính năng Orca liên quan** | F09, F14 |

---

## Tổng quan giải pháp

Orca CLI và headless mode mở ra khả năng **tích hợp AI agent vào CI/CD pipeline** như bất kỳ CLI tool nào khác — không cần GUI, không cần display server, hoàn toàn scriptable.

---

## Giải pháp cho từng Painpoint

### SOL06-01: Giải quyết PP06-01 — Không Có CLI / Headless Mode

**Giải pháp: Orca CLI + `orca serve` Headless Mode**

Orca CLI cung cấp đầy đủ command để script và automate mọi workflow — có thể chạy trong CI/CD pipeline mà không cần GUI.

**Cơ chế hoạt động:**
```bash
# CI/CD pipeline (GitHub Actions)
- name: AI Code Review
  run: |
    orca serve --port 7777 --daemon &
    sleep 2
    
    # Tạo worktree từ PR branch
    orca worktree create \
      --base $GITHUB_BASE_REF \
      --agent claude \
      --prompt "Review this PR for security issues and code quality"
    
    # Wait for completion
    WORKTREE_ID=$(orca worktree list --json | jq -r '.[0].id')
    orca agent wait --worktree $WORKTREE_ID --timeout 300
    
    # Get result
    orca snapshot --worktree $WORKTREE_ID --output review.txt
    
    # Comment trên PR
    gh pr comment --body "$(cat review.txt)"
```

**Available CLI commands:**
```bash
orca serve [--port] [--daemon]        # Headless server mode
orca worktree create [--base] [--agent] [--prompt]
orca worktree list [--json]
orca worktree remove <id>
orca agent status [--worktree] [--json]
orca agent wait [--worktree] [--timeout]
orca snapshot [--worktree] [--output]
```

**Kết quả đo lường được:**
- CI/CD integration: từ không thể → fully integrated
- Manual trigger: từ required → 0
- Automation coverage: từ 0% → 100% của repetitive tasks

**Tính năng Orca:** [F09 Orca CLI](../../features/F09-orca-cli.md)

---

### SOL06-02: Giải quyết PP06-02 — Không Có Observability

**Giải pháp: Telemetry Events + JSON Output**

Orca CLI với `--json` output và telemetry events cho phép DevOps build monitoring dashboard.

**Cơ chế hoạt động:**
```bash
# Export agent run metrics
orca agent status --json | jq '{
  worktree: .id,
  agent: .agent,
  status: .status,
  duration_seconds: .duration,
  tokens_used: .usage.tokens
}'

# Stream to monitoring system
orca worktree list --json | \
  jq '.[] | select(.status == "error")' | \
  curl -X POST https://alerting.internal/webhook -d @-
```

**Metrics available:**
- Agent run count, success/error rate
- Session duration distribution
- Token usage per agent/provider
- Worktree lifecycle events

**Integration với monitoring stack:**
- Export to Prometheus via JSON → node_exporter textfile
- Stream events to Datadog/CloudWatch via CLI piping
- Grafana dashboard từ collected metrics

**Kết quả đo lường được:**
- Visibility vào agent runs: từ 0% → 100%
- Time to detect abnormal runs: từ không biết → < 5 phút
- Capacity planning: có data thực để estimate

**Tính năng Orca:** [F09 Orca CLI](../../features/F09-orca-cli.md)

---

### SOL06-03: Giải quyết PP06-03 — Không Chạy Headless Linux

**Giải pháp: `orca serve` — True Headless Mode**

`orca serve` khởi động Orca daemon không có GUI, chạy native trên headless Linux (Docker, Kubernetes, bare metal).

**Cơ chế hoạt động:**
```dockerfile
# Dockerfile
FROM node:24-slim

RUN apt-get update && apt-get install -y \
    git \
    openssh-client \
    && rm -rf /var/lib/apt/lists/*

COPY orca-cli /usr/local/bin/orca
RUN chmod +x /usr/local/bin/orca

# Chạy orca daemon headless
CMD ["orca", "serve", "--port", "7777"]
```

```yaml
# Kubernetes deployment
apiVersion: apps/v1
kind: Deployment
metadata:
  name: orca-daemon
spec:
  containers:
  - name: orca
    image: orca-headless:latest
    command: ["orca", "serve", "--port", "7777"]
    # Không cần display, không cần GPU
```

**No display required:**
- Orca daemon chạy pure Node.js, không cần Electron renderer
- Không có X11/Wayland dependency trong headless mode
- Nhẹ hơn: chỉ cần Node.js runtime

**Kết quả đo lường được:**
- Deploy lên Linux server: từ không thể → fully supported
- Docker/K8s compatible: ✅
- Resource usage: ~100MB RAM (headless) vs ~500MB (full Electron)

**Tính năng Orca:** [F09 Orca CLI](../../features/F09-orca-cli.md)

---

### SOL06-04: Giải quyết PP06-04 — Thiếu Cleanup Policy Cho Worktrees

**Giải pháp: Automation với Retention Policy**

Orca Automation engine cho phép cấu hình cleanup policy cho worktrees — tự động xóa worktrees cũ theo age, status, hoặc disk usage.

**Cơ chế hoạt động:**
```yaml
# Orca automation: cleanup old worktrees
name: "Cleanup stale worktrees"
trigger:
  cron: "0 2 * * *"  # 2am hằng ngày
actions:
  - type: cleanup_worktrees
    filters:
      - status: completed
        older_than: "7d"
      - status: error
        older_than: "3d"
    dry_run: false
```

```bash
# Hoặc CLI
orca worktree cleanup \
  --status completed \
  --older-than 7d \
  --dry-run  # Preview trước khi xóa
```

**Safety mechanisms:**
- Không xóa worktree có uncommitted changes
- `--dry-run` để preview
- Confirmation prompt trước batch delete
- Audit log của tất cả cleanup actions

**Kết quả đo lường được:**
- Manual cleanup: từ required → 0
- Disk usage: controlled, không tăng unbounded
- Orphaned worktrees: tự động cleanup sau configured retention

**Tính năng Orca:** [F14 Automations](../../features/F14-automations.md), [F09 Orca CLI](../../features/F09-orca-cli.md)

---

### SOL06-05: Giải quyết PP06-05 — Không Có Access Control

**Giải pháp: Per-project Configuration + Audit Logging**

Orca hỗ trợ project-level configuration với trust presets và audit logging — cơ sở để build access control.

**Cơ chế hoạt động:**
```yaml
# orca.yaml per project
agents:
  default: claude
  trust_preset: standard  # minimal | standard | trusted

audit:
  log_file: /var/log/orca/audit.jsonl
  log_events:
    - worktree_create
    - agent_start
    - file_write
    - command_execute
```

**Audit log format (JSON):**
```json
{
  "timestamp": "2026-07-21T09:00:00Z",
  "event": "file_write",
  "worktree": "fix-auth-abc123",
  "agent": "claude",
  "path": "/src/auth.ts",
  "size_bytes": 2048
}
```

**Token quota via environment:**
```bash
# Per-user token limits via API key management
CLAUDE_API_KEY=<user-specific-key>  # User quotas managed at provider level
CODEX_API_KEY=<team-key-with-org-limits>
```

**Kết quả đo lường được:**
- Audit trail: 100% coverage của file writes và command executes
- Governance: có data để enforce policy
- Per-user tracking: via provider-level API keys

**Tính năng Orca:** [F04 AI Agent Support](../../features/F04-ai-agent-support.md) (Trust Presets)

---

## Tổng hợp ROI cho DevOps

| Painpoint | Trước Orca | Sau Orca | Impact |
|-----------|-----------|---------|--------|
| CLI/headless mode | Không thể | ✅ Fully supported | Unblocked |
| Observability | 0% | Metrics + events | Full visibility |
| Headless Linux | Không thể | ✅ Docker/K8s ready | Unblocked |
| Cleanup policy | Manual | Automated | Disk controlled |
| Access control | Không có | Audit log + trust presets | Governance |

**DevOps có thể build fully automated AI-assisted SDLC pipeline** với Orca như một building block.

---

*Tham chiếu: PP06 Painpoints, PRD §3.8 (F09 Orca CLI), §3.10 (F14 Automations)*
