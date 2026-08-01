# BL-TM-03 — Lưu và Khôi phục Scrollback Buffer

| Thuộc tính | Giá trị |
|-----------|---------|
| **Mã nghiệp vụ** | BL-TM-03 |
| **Tên** | Lưu và Khôi phục Scrollback Buffer |
| **Nhóm** | Terminal Management |
| **Actors** | Alex, Carlos |
| **Ưu tiên** | P1 — Should Have |
| **Tính năng** | F02 Terminal Splits |
| **SRS** | FR-3.3 |

---

## Mô tả nghiệp vụ

Lưu trữ terminal scrollback buffer vào database khi đóng app hoặc worktree, và khôi phục đầy đủ khi mở lại — người dùng xem được output từ session trước.

---

## Luồng Serialize (Lưu)

```
1. Trigger: app close, worktree close, hoặc idle timeout
2. Hệ thống:
   a. Serialize terminal state qua @xterm/addon-serialize
   b. Compress output (gzip)
   c. Lưu vào SQLite: { worktree_id, serialized, cursor_pos, timestamp }
3. Database record updated
```

## Luồng Restore (Khôi phục)

```
1. Người dùng mở lại worktree
2. Hệ thống load snapshot từ SQLite
3. Decompress và deserialize
4. Restore terminal state (output + cursor position + attributes)
5. Terminal hiển thị đúng output từ session trước
```

---

## Business Rules

| Rule | Mô tả |
|------|-------|
| BR-TM-09 | Serialize chỉ khi terminal đang idle (không có active output) |
| BR-TM-10 | Snapshot size tối đa: 50MB per worktree |
| BR-TM-11 | Cursor position và text attributes phải được restore chính xác |
| BR-TM-12 | Snapshot expire sau 30 ngày nếu worktree không mở |
