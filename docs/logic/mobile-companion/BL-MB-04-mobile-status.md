# BL-MB-04 — Xem Agent Status từ Mobile

| Thuộc tính | Giá trị |
|-----------|---------|
| **Mã nghiệp vụ** | BL-MB-04 |
| **Tên** | Xem Trạng thái Agent từ Mobile |
| **Nhóm** | Mobile Companion |
| **Actors** | Sam (Mobile-First User), Carlos |
| **Ưu tiên** | P1 — Should Have |
| **Tính năng** | F03 Mobile Companion |
| **SRS** | FR-5.3 |

---

## Mô tả nghiệp vụ

Người dùng xem real-time status của tất cả agent đang chạy trên desktop từ Orca Mobile — mà không cần mở laptop.

---

## Luồng chính

```
1. Mở Orca Mobile → màn hình chính
2. Mobile gửi status request tới desktop
3. Desktop response với current state:
   {
     worktrees: [
       { id, name, agent, status, duration, lastOutput }
     ]
   }
4. Mobile hiển thị danh sách:
   [✅] Claude Code — fix-auth — Completed 5' ago
   [🔄] Codex — add-tests — Running (23')
   [⏸️] OpenCode — refactor — Waiting for input
5. Pull-to-refresh để update
6. Live update qua WebSocket khi app foreground
```

---

## Business Rules

| Rule | Mô tả |
|------|-------|
| BR-MB-13 | Status data phải được encrypt trong transit |
| BR-MB-14 | Live update chỉ khi app foreground (save battery) |
| BR-MB-15 | Last output được truncate tới 500 chars cho mobile display |
| BR-MB-16 | Offline: hiển thị cached status với timestamp "Last updated X ago" |
