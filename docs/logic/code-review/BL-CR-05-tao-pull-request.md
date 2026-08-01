# BL-CR-05 — Tạo Pull Request với AI

| Thuộc tính | Giá trị |
|-----------|---------|
| **Mã nghiệp vụ** | BL-CR-05 |
| **Tên** | Tạo Pull Request với AI-generated Description |
| **Nhóm** | Code Review |
| **Actors** | Alex, Maya |
| **Ưu tiên** | P1 — Should Have |
| **Tính năng** | F06 GitHub & Linear Integration |
| **SRS** | FR-6.1 |

---

## Mô tả nghiệp vụ

Tạo Pull Request trên GitHub/GitLab từ worktree branch với AI-generated title và description, suggest reviewers, và link với issue/task tương ứng.

---

## Luồng chính

```
1. Người dùng click "Create PR" sau khi commit xong
2. Hệ thống thu thập:
   a. Branch name → base branch
   b. All commits trong branch
   c. Changed files với diff stats
   d. Linked issue (nếu có từ Linear/GitHub)
3. AI generate:
   - PR title (ngắn gọn, mô tả)
   - PR description (summary, changes, testing notes)
4. Người dùng review và chỉnh sửa
5. Suggest reviewers dựa trên code ownership
6. Submit PR → GitHub/GitLab API
7. Update linked issue status → "In Review"
8. PR link hiển thị trong Orca
```

---

## Business Rules

| Rule | Mô tả |
|------|-------|
| BR-CR-17 | PR không được tạo nếu branch chưa push lên remote |
| BR-CR-18 | Reviewer suggestion dựa trên CODEOWNERS file nếu có |
| BR-CR-19 | Linked issue status phải được update sau khi PR tạo thành công |
| BR-CR-20 | Draft PR option phải có (không auto-ready for review) |
