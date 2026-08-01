# BL-WT-02 — Fan-out Prompt tới Nhiều Worktree

| Thuộc tính | Giá trị |
|-----------|---------|
| **Mã nghiệp vụ** | BL-WT-02 |
| **Tên** | Fan-out Prompt tới Nhiều Worktree |
| **Nhóm** | Worktree Management |
| **Actors** | Alex (Senior Dev) |
| **Ưu tiên** | P0 — Must Have |
| **Tính năng** | F01 Parallel Worktrees |
| **SRS** | FR-1.2 |

---

## Mô tả nghiệp vụ

Gửi cùng một prompt tới N agent đang chạy trên N worktree độc lập cùng lúc (fan-out). Người dùng nhận N kết quả song song và chọn ra kết quả tốt nhất.

---

## Tiền điều kiện

- Repository đã mở
- Có ít nhất 1 AI agent đã được cấu hình
- Disk còn đủ cho N worktrees
- Prompt text đã sẵn sàng

---

## Luồng chính

```
1. Người dùng nhập prompt trong Fan-out dialog
2. Chọn số lượng N (1–10, default: 3)
3. Chọn base branch
4. Chọn agent type (hoặc dùng mặc định)
5. Click "Fan-out"
6. Hệ thống thực thi song song:
   FOR i = 1..N:
     a. Tạo worktree[i] (theo BL-WT-01)
     b. Khởi động agent[i] trong worktree[i] (theo BL-AG-01)
     c. Inject prompt vào agent[i]
7. Hiển thị N worktree cards với status real-time
8. Người dùng theo dõi N agent chạy song song
```

---

## Luồng thay thế

**[A1] Một worktree tạo thất bại:**
- Hệ thống tạo tiếp các worktree còn lại
- Hiển thị warning cho worktree bị lỗi
- Người dùng có thể retry worktree đó

**[A2] N > resources cho phép:**
- Hệ thống cảnh báo performance impact
- Gợi ý giảm N
- Người dùng xác nhận để tiếp tục

---

## Hậu điều kiện

- N worktrees được tạo, mỗi cái chạy 1 agent với cùng prompt
- Sidebar hiển thị N worktree cards
- Mỗi agent độc lập, không chia sẻ state

---

## Business Rules

| Rule | Mô tả |
|------|-------|
| BR-WT-05 | N tối đa = 10 worktrees per fan-out |
| BR-WT-06 | Tất cả N worktrees phải cùng base branch |
| BR-WT-07 | Prompt được inject sau khi agent fully started |
| BR-WT-08 | Nếu 1 agent fail, các agent khác không bị ảnh hưởng |

---

## SLO

| Metric | Target |
|--------|--------|
| Thời gian bắt đầu tất cả N agents | < 60 giây (N=3) |
| Isolation giữa worktrees | 100% |
