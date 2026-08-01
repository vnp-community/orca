# BL-WT-05 — Merge Worktree Thắng vào Nhánh Chính

| Thuộc tính | Giá trị |
|-----------|---------|
| **Mã nghiệp vụ** | BL-WT-05 |
| **Tên** | Merge Worktree Thắng vào Nhánh Chính |
| **Nhóm** | Worktree Management |
| **Actors** | Alex (Senior Dev), Maya (Tech Lead) |
| **Ưu tiên** | P1 — Should Have |
| **Tính năng** | F01, F06 |
| **SRS** | FR-1.2 |

---

## Mô tả nghiệp vụ

Sau khi chọn worktree tốt nhất, người dùng merge changes của worktree đó vào nhánh chính và cleanup các worktrees còn lại.

---

## Luồng chính

```
1. Người dùng chọn "Merge" cho worktree winner
2. Hệ thống kiểm tra:
   a. Không có conflict với main branch
   b. Tất cả changes đã commit
3. Người dùng chọn merge strategy:
   - Merge commit
   - Squash và merge
   - Rebase
4. Hệ thống thực thi merge
5. Hỏi cleanup: "Xóa các worktrees khác?"
6. Người dùng chọn cleanup hoặc giữ lại để review
```

---

## Business Rules

| Rule | Mô tả |
|------|-------|
| BR-WT-16 | Phải commit tất cả changes trong worktree trước khi merge |
| BR-WT-17 | Conflict resolution phải do người dùng thực hiện, không auto-resolve |
| BR-WT-18 | Cleanup worktrees khác là optional, không bắt buộc |
