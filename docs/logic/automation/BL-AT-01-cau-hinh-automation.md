# BL-AT-01 — Cấu hình Automation

| Thuộc tính | Giá trị |
|-----------|---------|
| **Mã nghiệp vụ** | BL-AT-01 |
| **Tên** | Cấu hình Automation Workflow |
| **Nhóm** | Automation |
| **Actors** | Sam (Mobile-First), DevOps |
| **Ưu tiên** | P2 — Could Have |
| **Tính năng** | F14 Automations |
| **SRS** | FR-9.1 |

---

## Mô tả nghiệp vụ

Người dùng tạo automation workflow định nghĩa trigger và chuỗi actions — được lưu và thực thi tự động theo lịch hoặc event.

---

## Luồng chính

```
1. Người dùng mở Automations panel
2. Click "New Automation"
3. Điền thông tin:
   - Name: "Morning Review"
   - Trigger: Cron (0 9 * * 1-5) hoặc Event
   - Actions: danh sách actions theo thứ tự
4. Validate automation definition
5. Enable automation
6. Lưu vào database
```

---

## Automation Schema

```yaml
name: string
description?: string
enabled: boolean
trigger:
  type: cron | manual | event
  cron?: string          # "0 9 * * 1-5"
  event?: string         # "agent:completed"
actions:
  - type: create_worktree | run_agent | commit | create_pr | notify | cleanup
    params: object
```

---

## Business Rules

| Rule | Mô tả |
|------|-------|
| BR-AT-01 | Cron expression phải valid trước khi save |
| BR-AT-02 | Tối đa 20 automations per project |
| BR-AT-03 | Automation không được chạy nếu disabled |
| BR-AT-04 | Circular automation trigger bị detect và reject |
