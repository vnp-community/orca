# BL-WT-03 — Xóa Worktree An Toàn

| Thuộc tính | Giá trị |
|-----------|---------|
| **Mã nghiệp vụ** | BL-WT-03 |
| **Tên** | Xóa Worktree An Toàn |
| **Nhóm** | Worktree Management |
| **Actors** | Alex, Maya, Carlos |
| **Ưu tiên** | P0 — Must Have |
| **Tính năng** | F01 Parallel Worktrees |
| **SRS** | FR-1.3 |

---

## Mô tả nghiệp vụ

Xóa worktree đảm bảo an toàn dữ liệu — hệ thống kiểm tra các điều kiện nguy hiểm trước khi xóa và yêu cầu xác nhận tường minh từ người dùng.

---

## Luồng chính

```
1. Người dùng click "Delete" trên worktree card
2. Hệ thống chạy safety checks:
   a. Kiểm tra uncommitted changes (git status)
   b. Kiểm tra untracked files
   c. Kiểm tra agent có đang chạy không
   d. Kiểm tra process nào đang dùng thư mục
3. Nếu tất cả an toàn: hiện dialog xác nhận
4. Người dùng xác nhận "Delete"
5. Hệ thống thực thi:
   a. Kill agent process (nếu có)
   b. Đóng terminal sessions
   c. git worktree remove --force <path>
   d. Xóa database record
   e. Xóa worktree card khỏi sidebar
6. Worktree đã bị xóa hoàn toàn
```

---

## Luồng thay thế

**[A1] Có uncommitted changes:**
- Dialog cảnh báo: "Worktree này có X files chưa commit"
- 3 lựa chọn:
  1. "Discard & Delete" — xóa luôn
  2. "Commit First" — mở terminal để commit
  3. "Cancel" — hủy xóa

**[A2] Agent đang chạy:**
- Dialog: "Agent đang chạy. Dừng agent trước khi xóa?"
- 2 lựa chọn: Stop & Delete / Cancel

**[A3] Thư mục đang được dùng bởi process khác:**
- Cảnh báo danh sách processes đang giữ lock
- Force delete option

---

## Business Rules

| Rule | Mô tả |
|------|-------|
| BR-WT-09 | Không bao giờ xóa worktree có uncommitted changes mà không có xác nhận tường minh |
| BR-WT-10 | Không xóa worktree khi agent đang trong trạng thái "running" |
| BR-WT-11 | Xóa phải atomic — không để worktree ở trạng thái half-deleted |
| BR-WT-12 | Database record phải được xóa cùng với filesystem |

---

## SLO

| Metric | Target |
|--------|--------|
| Safety check time | < 2 giây |
| Delete execution time | < 5 giây |
| Data loss incidents | 0 |
