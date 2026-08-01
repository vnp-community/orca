# BL-AT-04 — Cleanup Worktrees Theo Policy

| Thuộc tính | Giá trị |
|-----------|---------|
| **Mã nghiệp vụ** | BL-AT-04 |
| **Tên** | Cleanup Worktrees Theo Retention Policy |
| **Nhóm** | Automation |
| **Actors** | DevOps, Alex |
| **Ưu tiên** | P2 — Could Have |
| **Tính năng** | F14 Automations, F09 CLI |
| **SRS** | FR-9.1 |

---

## Mô tả nghiệp vụ

Tự động xóa worktrees cũ theo policy (age, status, disk usage) — giữ disk space trong kiểm soát khi chạy nhiều agent tự động.

---

## Cleanup Policy Definition

```yaml
actions:
  - type: cleanup_worktrees
    params:
      filters:
        - status: [completed, error]
          older_than: "7d"
        - status: [stopped]
          older_than: "3d"
      exclude:
        - has_uncommitted_changes
        - is_linked_to_open_pr
      dry_run: false
```

---

## Luồng chính

```
1. Automation trigger (cron hoặc manual)
2. Hệ thống query worktrees matching filters
3. Nếu dry_run: chỉ log, không xóa
4. Nếu không dry_run:
   FOR each matching worktree:
     a. Run safety checks (BL-WT-03)
     b. Nếu safe: xóa (BL-WT-03 main flow)
     c. Nếu không safe: skip và log
5. Report: X deleted, Y skipped
```

---

## Business Rules

| Rule | Mô tả |
|------|-------|
| BR-AT-11 | Cleanup không bao giờ xóa worktree có uncommitted changes |
| BR-AT-12 | Cleanup không xóa worktree linked với open PR |
| BR-AT-13 | dry_run mode phải có để preview trước khi thực thi |
| BR-AT-14 | Audit log của tất cả worktrees đã xóa |
