# BL-PI-04 — Submit PR Review lên GitHub

| Thuộc tính | Giá trị |
|-----------|---------|
| **Mã nghiệp vụ** | BL-PI-04 |
| **Tên** | Submit PR Review Comments lên GitHub |
| **Nhóm** | Project Integration |
| **Actors** | Maya (Tech Lead) |
| **Ưu tiên** | P1 — Should Have |
| **Tính năng** | F06, F08 |
| **SRS** | FR-6.1, FR-6.2 |

---

## Mô tả nghiệp vụ

Sau khi annotate diff trong Orca, người dùng submit comments như một GitHub PR Review — comments xuất hiện trên GitHub với đúng file và line number.

---

## Luồng chính

```
1. Người dùng annotate diff (BL-CR-02)
2. Click "Submit as GitHub Review"
3. Chọn review type: Comment / Approve / Request Changes
4. Hệ thống:
   a. Map DiffComments → GitHub review comments format
   b. Submit via GitHub API: POST /repos/:repo/pulls/:pr/reviews
   c. Mỗi DiffComment → line-level review comment
5. GitHub PR hiển thị review với comments
6. Optional: cũng gửi về agent (BL-CR-03)
```

---

## Business Rules

| Rule | Mô tả |
|------|-------|
| BR-PI-10 | Phải có ít nhất 1 comment để submit review |
| BR-PI-11 | Review type mặc định = "Request Changes" khi có comment |
| BR-PI-12 | GitHub review và agent feedback có thể submit cùng lúc |
