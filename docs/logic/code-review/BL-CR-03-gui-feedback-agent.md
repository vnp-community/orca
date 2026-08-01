# BL-CR-03 — Gửi Feedback về Agent

| Thuộc tính | Giá trị |
|-----------|---------|
| **Mã nghiệp vụ** | BL-CR-03 |
| **Tên** | Gửi Feedback Review về Agent |
| **Nhóm** | Code Review |
| **Actors** | Maya (Tech Lead), Alex |
| **Ưu tiên** | P1 — Should Have |
| **Tính năng** | F08 Annotate AI Diffs |
| **SRS** | FR-6.2 |

---

## Mô tả nghiệp vụ

Thu thập tất cả annotations từ review session, format thành structured prompt với context đầy đủ, và inject vào agent terminal để agent sửa code theo feedback.

---

## Luồng chính

```
1. Người dùng click "Send to Agent" sau khi xong review
2. Hệ thống thu thập tất cả DiffComments
3. Format thành structured prompt:
   ---
   Review feedback for {worktree-name}:
   
   File: src/auth.ts, Line 42 (new)
   Code: `if (user.role === 'admin') {`
   Feedback: Cần check null trước khi access user.role
   
   File: src/api/routes.ts, Line 128 (new)
   Code: `app.get('/admin', adminHandler)`
   Feedback: Thiếu authentication middleware
   ---
4. Inject prompt vào agent terminal (paste vào PTY)
5. Agent nhận và xử lý feedback
6. Annotation count badge reset
7. Review buffer cleared
```

---

## Business Rules

| Rule | Mô tả |
|------|-------|
| BR-CR-09 | Prompt format phải nhất quán để agent có thể parse |
| BR-CR-10 | File path phải là relative path từ repo root |
| BR-CR-11 | Code context phải include 2 dòng trước/sau để disambiguate |
| BR-CR-12 | Gửi thành công phải được confirm bằng visual indicator |
