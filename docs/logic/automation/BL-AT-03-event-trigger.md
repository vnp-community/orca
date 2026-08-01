# BL-AT-03 — Event-based Automation Trigger

| Thuộc tính | Giá trị |
|-----------|---------|
| **Mã nghiệp vụ** | BL-AT-03 |
| **Tên** | Kích hoạt Automation theo Sự kiện |
| **Nhóm** | Automation |
| **Actors** | DevOps, Sam |
| **Ưu tiên** | P2 — Could Have |
| **Tính năng** | F14 Automations |
| **SRS** | FR-9.1 |

---

## Mô tả nghiệp vụ

Automation được kích hoạt tự động khi một sự kiện trong Orca xảy ra — agent hoàn thành, PR merged, issue assigned.

---

## Supported Events

| Event | Trigger khi |
|-------|-----------|
| `agent:completed` | Agent hoàn thành task (status → completed) |
| `agent:error` | Agent gặp lỗi |
| `worktree:created` | Worktree mới được tạo |
| `pr:merged` | PR được merge trên GitHub |
| `issue:assigned` | Issue được assign cho user |

---

## Business Rules

| Rule | Mô tả |
|------|-------|
| BR-AT-09 | Event matching phải hỗ trợ filter (ví dụ: chỉ trigger khi agent = "claude") |
| BR-AT-10 | Event-based automation không được trigger chính nó (circular prevention) |
