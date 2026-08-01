# BL-PI-01 — Import và Duyệt GitHub/GitLab Issues

| Thuộc tính | Giá trị |
|-----------|---------|
| **Mã nghiệp vụ** | BL-PI-01 |
| **Tên** | Import và Duyệt GitHub/GitLab Issues |
| **Nhóm** | Project Integration |
| **Actors** | Maya (Tech Lead), Alex |
| **Ưu tiên** | P1 — Should Have |
| **Tính năng** | F06 GitHub & Linear Integration |
| **SRS** | FR-6.1 |

---

## Mô tả nghiệp vụ

Duyệt và tìm kiếm issues từ GitHub/GitLab trực tiếp trong Orca — không cần mở browser.

---

## Luồng chính

```
1. Người dùng mở GitHub panel trong Orca
2. Hệ thống authenticate (OAuth/PAT)
3. Load issues với default filter (open, assigned to me)
4. Hiển thị danh sách với: title, labels, assignees, updated_at
5. Người dùng filter theo: status, assignee, label, milestone
6. Click issue → xem detail: description, comments, linked PRs
7. Option: "Create Worktree từ issue này" → BL-PI-02
```

---

## Business Rules

| Rule | Mô tả |
|------|-------|
| BR-PI-01 | Cache issues 5 phút, refresh on demand |
| BR-PI-02 | Hỗ trợ GitHub REST + GraphQL API |
| BR-PI-03 | Rate limit handling: exponential backoff, show remaining quota |
