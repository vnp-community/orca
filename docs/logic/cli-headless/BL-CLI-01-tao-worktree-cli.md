# BL-CLI-01 — Tạo Worktree qua CLI

| Thuộc tính | Giá trị |
|-----------|---------|
| **Mã nghiệp vụ** | BL-CLI-01 |
| **Tên** | Tạo Worktree qua Orca CLI |
| **Nhóm** | CLI & Headless |
| **Actors** | DevOps Engineer, Alex |
| **Ưu tiên** | P1 — Should Have |
| **Tính năng** | F09 Orca CLI |
| **SRS** | FR-9.2 |

---

## Mô tả nghiệp vụ

Tạo worktree mới và khởi động agent hoàn toàn qua command line — không cần GUI, phù hợp cho CI/CD automation.

---

## Command Interface

```bash
orca worktree create \
  --base <branch>     \  # Base branch (required)
  --agent <type>      \  # Agent type (optional)
  --prompt <text>     \  # Initial prompt (optional)
  --name <name>       \  # Worktree name (optional)
  --json                 # Output as JSON

# Output
{
  "id": "wt-abc123",
  "path": "/repos/myapp-fix-auth",
  "branch": "fix/login-timeout-123",
  "status": "ready",
  "agentStatus": "running"
}
```

---

## Luồng chính

```
1. CLI gửi command tới Orca daemon qua Unix socket
2. Daemon thực thi: BL-WT-01 (tạo worktree)
3. Nếu --agent: BL-AG-01 (khởi động agent)
4. Nếu --prompt: inject prompt vào agent
5. Return worktree info qua stdout
6. Exit code 0 nếu success, 1 nếu error
```

---

## Business Rules

| Rule | Mô tả |
|------|-------|
| BR-CLI-01 | CLI phải idempotent với cùng input |
| BR-CLI-02 | JSON output phải valid JSON (dùng --json flag) |
| BR-CLI-03 | Exit code convention: 0=success, 1=error, 2=timeout |
| BR-CLI-04 | Error messages phải human-readable VÀ có machine-parseable JSON |
