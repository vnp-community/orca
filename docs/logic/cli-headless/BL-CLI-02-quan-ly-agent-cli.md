# BL-CLI-02 — Quản lý Agent qua CLI

| Thuộc tính | Giá trị |
|-----------|---------|
| **Mã nghiệp vụ** | BL-CLI-02 |
| **Tên** | Quản lý Agent qua Orca CLI |
| **Nhóm** | CLI & Headless |
| **Actors** | DevOps Engineer |
| **Ưu tiên** | P1 — Should Have |
| **Tính năng** | F09 Orca CLI |
| **SRS** | FR-9.2 |

---

## Mô tả nghiệp vụ

Kiểm tra trạng thái agent, gửi prompt, và chờ agent hoàn thành — hoàn toàn qua CLI cho automation scripts.

---

## Command Interface

```bash
# Check status
orca agent status --worktree <id> --json

# Wait for completion
orca agent wait --worktree <id> --timeout 300

# Send prompt
orca agent send "Run unit tests" --worktree <id>

# Get output snapshot
orca snapshot --worktree <id> --output result.txt
```

---

## `orca agent wait` Flow

```
1. Poll agent status mỗi 5 giây
2. Nếu status = completed/error: exit với code
3. Nếu timeout: exit code 2
4. Return final status qua stdout
```

---

## Business Rules

| Rule | Mô tả |
|------|-------|
| BR-CLI-05 | `wait` phải timeout nếu agent không xong trong thời gian chỉ định |
| BR-CLI-06 | `snapshot` phải capture toàn bộ scrollback hiện tại |
| BR-CLI-07 | Tất cả commands phải hoạt động khi Orca GUI đang mở song song |
