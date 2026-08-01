# BL-PI-03 — Cập nhật Trạng thái Issue/Task

| Thuộc tính | Giá trị |
|-----------|---------|
| **Mã nghiệp vụ** | BL-PI-03 |
| **Tên** | Cập nhật Trạng thái Issue/Task tự động |
| **Nhóm** | Project Integration |
| **Actors** | Maya (Tech Lead) |
| **Ưu tiên** | P2 — Could Have |
| **Tính năng** | F06 GitHub & Linear Integration |
| **SRS** | FR-6.4 |

---

## Mô tả nghiệp vụ

Tự động cập nhật trạng thái của Linear issue hoặc GitHub issue khi workflow Orca thay đổi — agent start → In Progress, PR created → In Review, PR merged → Done.

---

## Mapping Workflow → Status

| Orca Event | Linear Status | GitHub Issue |
|-----------|--------------|-------------|
| Worktree created from issue | In Progress | Label: "in-progress" |
| PR created | In Review | Linked PR |
| PR merged | Done | Closed |
| Worktree deleted (no PR) | Cancelled | — |

---

## Business Rules

| Rule | Mô tả |
|------|-------|
| BR-PI-07 | Status update phải có thể disable per-project |
| BR-PI-08 | Nếu API call fail, retry 3 lần trước khi give up |
| BR-PI-09 | Status update không được block main workflow |
