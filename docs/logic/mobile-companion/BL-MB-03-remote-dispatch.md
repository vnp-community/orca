# BL-MB-03 — Remote Dispatch từ Mobile

| Thuộc tính | Giá trị |
|-----------|---------|
| **Mã nghiệp vụ** | BL-MB-03 |
| **Tên** | Remote Dispatch — Gửi Prompt từ Mobile về Agent |
| **Nhóm** | Mobile Companion |
| **Actors** | Sam (Mobile-First User) |
| **Ưu tiên** | P1 — Should Have |
| **Tính năng** | F03 Mobile Companion |
| **SRS** | FR-5.3 |

---

## Mô tả nghiệp vụ

Người dùng gửi prompt từ Orca Mobile — desktop nhận và inject vào agent terminal, agent xử lý và trả kết quả.

---

## Luồng chính

```
1. Sam nhận notification "Agent đã xong task 1"
2. Mở Orca Mobile → xem agent status
3. Gõ follow-up prompt: "Tiếp tục với task 2: thêm unit tests"
4. Tap "Send"
5. Mobile encrypt prompt và gửi tới desktop
6. Desktop decrypt và validate
7. Desktop inject prompt vào agent PTY
8. Agent nhận và bắt đầu xử lý
9. Status update "running" gửi về mobile
10. Khi xong: notification "Task 2 completed" về mobile
```

---

## Business Rules

| Rule | Mô tả |
|------|-------|
| BR-MB-09 | Chỉ dispatch được tới agent đang ở trạng thái "idle" hoặc "waiting" |
| BR-MB-10 | Dispatch tới agent "running" sẽ được queue |
| BR-MB-11 | Prompt phải được validate (max 10,000 chars) |
| BR-MB-12 | Dispatch phải có confirmation nếu overwrite prompt đang queue |
