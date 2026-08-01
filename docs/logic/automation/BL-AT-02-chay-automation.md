# BL-AT-02 — Chạy Automation theo Schedule

| Thuộc tính | Giá trị |
|-----------|---------|
| **Mã nghiệp vụ** | BL-AT-02 |
| **Tên** | Chạy Automation theo Cron Schedule |
| **Nhóm** | Automation |
| **Actors** | Sam (Mobile-First), DevOps |
| **Ưu tiên** | P2 — Could Have |
| **Tính năng** | F14 Automations |
| **SRS** | FR-9.1 |

---

## Mô tả nghiệp vụ

Automation engine chạy workflow đã cấu hình đúng giờ, thực thi tuần tự các actions, và lưu run history.

---

## Luồng chính

```
1. Automation scheduler check mỗi 30 giây
2. Phát hiện automation đến giờ chạy
3. Precheck: validate conditions
4. Tạo run record: { automation_id, started_at, status: "running" }
5. Thực thi actions tuần tự:
   FOR each action:
     a. Execute action
     b. Nếu success: tiếp tục action sau
     c. Nếu fail: log error, tùy config (stop/continue)
6. Update run record: status = "completed" hoặc "failed"
7. Gửi notification kết quả (nếu cấu hình)
```

---

## Business Rules

| Rule | Mô tả |
|------|-------|
| BR-AT-05 | Missed run (app offline): catch-up run khi app startup |
| BR-AT-06 | Run timeout: 2 giờ per automation run |
| BR-AT-07 | Run history giữ 30 records gần nhất |
| BR-AT-08 | Concurrent run của cùng automation bị ngăn chặn |
