# BL-TM-02 — Split Terminal

| Thuộc tính | Giá trị |
|-----------|---------|
| **Mã nghiệp vụ** | BL-TM-02 |
| **Tên** | Split Terminal |
| **Nhóm** | Terminal Management |
| **Actors** | Alex, Carlos |
| **Ưu tiên** | P0 — Must Have |
| **Tính năng** | F02 Terminal Splits |
| **SRS** | FR-3.1 |

---

## Mô tả nghiệp vụ

Chia terminal hiện tại thành 2 panels (horizontal hoặc vertical), mỗi panel có PTY session riêng độc lập.

---

## Luồng chính

```
1. Người dùng nhấn shortcut (Cmd+D horizontal, Cmd+Shift+D vertical)
   hoặc click menu "Split Terminal"
2. Hệ thống:
   a. Chia layout hiện tại theo hướng được chọn
   b. Tạo PTY mới trong panel mới (BL-TM-01)
   c. Copy working directory từ PTY hiện tại (nếu có thể)
   d. Focus về panel mới
3. Hai panels hiển thị độc lập
4. Người dùng có thể resize bằng cách kéo divider
```

---

## Business Rules

| Rule | Mô tả |
|------|-------|
| BR-TM-05 | Mỗi split panel có PTY riêng độc lập — không share process |
| BR-TM-06 | Minimum panel size = 80 cols × 10 rows |
| BR-TM-07 | Resize một panel không ảnh hưởng I/O của panel khác |
| BR-TM-08 | Đóng một panel không ảnh hưởng panel còn lại |
