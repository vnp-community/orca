# BL-WT-04 — So sánh Kết quả Giữa Các Worktrees

| Thuộc tính | Giá trị |
|-----------|---------|
| **Mã nghiệp vụ** | BL-WT-04 |
| **Tên** | So sánh Kết quả Giữa Các Worktrees |
| **Nhóm** | Worktree Management |
| **Actors** | Alex (Senior Dev) |
| **Ưu tiên** | P1 — Should Have |
| **Tính năng** | F01, F08 |
| **SRS** | FR-1.2, FR-6.2 |

---

## Mô tả nghiệp vụ

Sau khi nhiều agents hoàn thành cùng task trong các worktrees riêng, người dùng so sánh kết quả để chọn worktree tốt nhất để merge.

---

## Luồng chính

```
1. Người dùng mở "Compare" view sau khi N agents xong
2. Hệ thống thu thập diff từ mỗi worktree vs base branch
3. Hiển thị side-by-side comparison:
   - File changes count per worktree
   - Lines added/removed
   - Test results (nếu có)
   - Agent summary output
4. Người dùng review từng worktree
5. Người dùng chọn worktree "winner"
6. Tiến hành BL-WT-05 (Merge)
```

---

## Business Rules

| Rule | Mô tả |
|------|-------|
| BR-WT-13 | Chỉ so sánh được worktrees có cùng base branch |
| BR-WT-14 | Diff được tính từ cùng base SHA |
| BR-WT-15 | Không tự động chọn winner — người dùng phải chủ động chọn |
