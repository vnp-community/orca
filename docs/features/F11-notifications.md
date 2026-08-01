# F11 — Notifications & Unread State

| Thuộc tính | Giá trị |
|-----------|---------|
| **ID** | F11 |
| **Tên** | Notifications & Unread State |
| **Ưu tiên** | P1 — Should Have |
| **Trạng thái** | ✅ Đã phát hành |
| **Tham chiếu PRD** | §3.10 (Notifications) |
| **Tham chiếu URD** | UR-031 |
| **Tham chiếu SRS** | NFR-2.3 |
| **ADR References** | — |
| **HLD References** | C3.1 |

---

## Mô tả

Hệ thống thông báo theo dõi trạng thái của tất cả agent và worktrees, thông báo khi agent hoàn thành hoặc cần sự chú ý, và cho phép người dùng mark threads là unread để quay lại sau.

---

## Vấn đề cần giải quyết

Khi chạy nhiều agent song song, developer không thể theo dõi tất cả cùng lúc. Cần cơ chế tự động thông báo khi có sự kiện quan trọng xảy ra và cho phép đánh dấu những gì cần quay lại xem.

---

## Tính năng chi tiết

### Desktop Notifications

- Native OS notification khi agent hoàn thành task
- Notification khi agent gặp lỗi
- Notification khi agent chờ user input (waiting state)
- Notification khi worktree bị xóa ngoài hệ thống

### In-app Notification Center

- Panel liệt kê tất cả notifications
- Filter theo: agent, worktree, severity
- Mark as read / Mark all as read
- Timestamp và duration

### Unread State

- Worktree card hiển thị badge khi có activity mới
- Agent tab hiển thị indicator khi agent output mới
- "Mark as unread" để đánh dấu cần quay lại
- Persistence: unread state lưu qua restart

### Activity Tracking

- Track khi nào agent chuyển trạng thái (running → idle, error)
- Track khi agent bắt đầu waiting for input
- Desktop app dock badge (macOS) hiển thị số notification

---

## Tiêu chí chấp nhận

- [ ] Desktop notification xuất hiện trong < 1 giây khi agent kết thúc
- [ ] Notification hoạt động khi app không focus
- [ ] Mark as unread persist qua restart
- [ ] Notification center hiển thị lịch sử đầy đủ
- [ ] macOS dock badge hiển thị số notification chưa đọc

---

## Yêu cầu kỹ thuật

| Thành phần | Chi tiết |
|-----------|---------|
| **App icon / dock** | `src/main/app-icon.ts`, `src/main/dock/` |
| **Tray** | `src/main/tray/` |
| **Star nag** | `src/main/star-nag/` |
| **Agent status** | `src/shared/agent-status-types.ts` |

---

## Metrics

| KPI | Target |
|----|-------|
| Desktop notification delay | < 1 giây |
| Notification delivery rate | > 99% |
