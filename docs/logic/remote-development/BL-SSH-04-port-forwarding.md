# BL-SSH-04 — Auto Port Forwarding

| Thuộc tính | Giá trị |
|-----------|---------|
| **Mã nghiệp vụ** | BL-SSH-04 |
| **Tên** | Tự động Port Forwarding từ Remote |
| **Nhóm** | Remote Development |
| **Actors** | Carlos (Remote Dev), QA Engineer |
| **Ưu tiên** | P1 — Should Have |
| **Tính năng** | F07 SSH Worktrees |
| **SRS** | FR-4.4 |

---

## Mô tả nghiệp vụ

Relay trên remote host scan và phát hiện ports mới được mở, tự động forward về local — không cần người dùng setup SSH tunnel thủ công.

---

## Luồng chính

```
1. Relay chạy port scanner mỗi 2 giây
2. Phát hiện port mới open (ví dụ: 3000 từ dev server)
3. Relay gửi event về local: { port: 3000, process: "node" }
4. Orca nhận event và:
   a. Tìm local port không conflict (ví dụ: 3001)
   b. Thiết lập SSH tunnel: localhost:3001 → remote:3000
   c. Label tunnel theo worktree name
5. Hiển thị notification: "Port 3001 → remote:3000 [Open Browser]"
6. Người dùng click để mở browser
```

## Multiple Worktree Port Management

```
Worktree A (fix-login) → remote:3000 → local:3001
Worktree B (add-tests) → remote:3000 → local:3002
[Orca tự resolve port conflicts, không cần user manage]
```

---

## Business Rules

| Rule | Mô tả |
|------|-------|
| BR-SSH-15 | Port scan chỉ trên localhost (127.0.0.1), không scan external |
| BR-SSH-16 | Exclude well-known ports: 22, 25, 53, 80, 443 |
| BR-SSH-17 | Local port được chọn tự động từ range 3001–9999 |
| BR-SSH-18 | Tunnel được cleanup khi remote port đóng hoặc worktree xóa |
| BR-SSH-19 | Mỗi worktree có namespace port riêng để tránh conflict |
