# BL-CR-01 — Xem Diff của Agent Changes

| Thuộc tính | Giá trị |
|-----------|---------|
| **Mã nghiệp vụ** | BL-CR-01 |
| **Tên** | Xem Diff của Agent Changes |
| **Nhóm** | Code Review |
| **Actors** | Alex, Maya |
| **Ưu tiên** | P1 — Should Have |
| **Tính năng** | F08 Annotate AI Diffs |
| **SRS** | FR-6.2 |

---

## Mô tả nghiệp vụ

Hiển thị diff của tất cả thay đổi mà agent đã tạo ra — với syntax highlighting, file tree navigation, và large diff handling.

---

## Luồng chính

```
1. Agent hoàn thành task
2. Người dùng click "Review Changes"
3. Hệ thống:
   a. Chạy: git diff HEAD (hoặc git diff <base>..<worktree>)
   b. Parse diff output thành structured format
   c. Apply syntax highlighting theo file extension
   d. Render diff viewer với:
      - File tree bên trái (với badge count changes)
      - Unified/Split diff view bên phải
      - Line numbers
4. Người dùng browse diff theo file
5. Click file trong tree → scroll tới file đó trong diff
```

---

## Business Rules

| Rule | Mô tả |
|------|-------|
| BR-CR-01 | Large diff (> 1000 files): render incrementally, không block UI |
| BR-CR-02 | Binary files: hiển thị "Binary file changed", không render content |
| BR-CR-03 | Deleted files: hiển thị đầy đủ content với màu đỏ |
| BR-CR-04 | Diff view phải có thể scroll độc lập với file tree |
