# F14 — Automations

| Thuộc tính | Giá trị |
|-----------|---------|
| **ID** | F14 |
| **Tên** | Automations |
| **Ưu tiên** | P2 — Could Have |
| **Trạng thái** | 🚧 Đang phát triển |
| **Tham chiếu PRD** | §3.10 (Automations) |
| **Tham chiếu URD** | UR-070 |
| **Tham chiếu SRS** | FR-9.1 |
| **ADR References** | — |
| **HLD References** | C3.1 |

---

## Mô tả

Hệ thống automation cho phép lên lịch và trigger các workflow tự động — tạo worktree, chạy agent, commit kết quả — theo cron schedule hoặc event-based trigger.

---

## Vấn đề cần giải quyết

Nhiều tác vụ lặp đi lặp lại có thể tự động hóa: chạy test review mỗi sáng, generate weekly summary, tự động tạo worktree khi issue mới được assign. Hiện tại tất cả phải thực hiện thủ công.

---

## Tính năng chi tiết

### Trigger Types

| Trigger | Mô tả |
|---------|-------|
| **Cron** | Chạy theo lịch (*/5 * * * *, 0 9 * * 1) |
| **Manual** | Run ngay khi click |
| **Event** | Khi agent kết thúc, khi PR merged |

### Automation Actions

| Action | Mô tả |
|--------|-------|
| **Create Worktree** | Tạo worktree từ branch |
| **Run Agent** | Khởi động agent với prompt |
| **Commit & Push** | Commit và push kết quả |
| **Create PR** | Tạo PR tự động |
| **Send Notification** | Mobile push hoặc webhook |
| **Run Script** | Chạy shell script |

### Automation Definition (YAML-like)

```yaml
name: "Morning Code Review"
trigger:
  cron: "0 9 * * 1-5"  # Weekdays 9am
actions:
  - type: create_worktree
    base: main
  - type: run_agent
    agent: claude
    prompt: "Review all TODOs and suggest fixes"
  - type: commit
    message: "chore: automated TODO review"
  - type: create_pr
    title: "Weekly TODO cleanup"
```

### Automation Management
- List tất cả automations
- Enable/disable automation
- View run history với status và logs
- Retention policy: giữ N last runs

---

## Tiêu chí chấp nhận

- [ ] Cron automation chạy đúng giờ (±30 giây)
- [ ] Automation run history hiển thị đủ thông tin
- [ ] Enable/disable automation ngay lập tức
- [ ] Failed automation hiển thị error rõ ràng

---

## Yêu cầu kỹ thuật

| Thành phần | Chi tiết |
|-----------|---------|
| **Automation types** | `src/shared/automations-types.ts` |
| **Schedules** | `src/shared/automation-schedules.ts` |
| **Run identity** | `src/shared/automation-run-identity.ts` |
| **Run retention** | `src/shared/automation-run-retention.ts` |
| **Precheck** | `src/shared/automation-precheck.ts` |
| **Workspace provenance** | `src/shared/automation-workspace-provenance.ts` |
| **Main module** | `src/main/automations/` |
