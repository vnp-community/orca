# BL-PI-02 — Tạo Worktree từ Issue/Task

| Thuộc tính | Giá trị |
|-----------|---------|
| **Mã nghiệp vụ** | BL-PI-02 |
| **Tên** | Tạo Worktree từ GitHub Issue hoặc Linear Task |
| **Nhóm** | Project Integration |
| **Actors** | Maya (Tech Lead), Alex |
| **Ưu tiên** | P1 — Should Have |
| **Tính năng** | F01, F06 |
| **SRS** | FR-6.1 |

---

## Mô tả nghiệp vụ

Tạo worktree và khởi động agent trực tiếp từ một issue/task — branch được tạo với naming convention đúng, và agent nhận issue description làm context.

---

## Luồng chính

```
1. Người dùng click "Create Worktree" trong issue detail
2. Hệ thống:
   a. Extract issue title → generate branch name:
      "fix/login-timeout-123" (từ issue #123 "Fix login timeout")
   b. Check branch không tồn tại
   c. Tạo worktree với branch mới (BL-WT-01)
   d. Build agent prompt từ issue:
      - Issue title
      - Issue description
      - Acceptance criteria (nếu có)
      - Related comments
   e. Khởi động agent (BL-AG-01)
   f. Inject issue prompt vào agent
3. Update issue status → "In Progress" (BL-PI-03)
4. Worktree card link với issue
```

---

## Business Rules

| Rule | Mô tả |
|------|-------|
| BR-PI-04 | Branch name phải theo convention: type/description-issueId |
| BR-PI-05 | Issue content phải được sanitize trước khi đưa vào prompt |
| BR-PI-06 | Issue status update phải có thể disable (opt-out) |
