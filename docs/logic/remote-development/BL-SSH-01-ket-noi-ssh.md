# BL-SSH-01 — Kết nối SSH Host

| Thuộc tính | Giá trị |
|-----------|---------|
| **Mã nghiệp vụ** | BL-SSH-01 |
| **Tên** | Kết nối SSH Host |
| **Nhóm** | Remote Development |
| **Actors** | Carlos (Remote Dev), DevOps |
| **Ưu tiên** | P1 — Should Have |
| **Tính năng** | F07 SSH Worktrees |
| **SRS** | FR-4.1 |

---

## Mô tả nghiệp vụ

Thiết lập kết nối SSH tới remote host — đọc SSH config, negotiate auth, và duy trì connection pool cho các operations tiếp theo.

---

## Tiền điều kiện

- Remote host reachable qua network
- SSH credentials có sẵn (key hoặc password)
- SSH server đang chạy trên remote

---

## Luồng chính

```
1. Người dùng thêm SSH host trong Orca
2. Hệ thống:
   a. Parse ~/.ssh/config (bao gồm Include directives)
   b. Resolve host config (HostName, Port, User, IdentityFile)
   c. Negotiate authentication:
      i.  SSH key authentication (IdentityFile)
      ii. SSH agent forwarding
      iii. Password authentication (fallback)
      iv. Keyboard-interactive (2FA)
   d. Thiết lập control channel
   e. Setup keepalive (ServerAliveInterval)
   f. Ghi nhận connection trong host registry
3. Host hiển thị "Connected" trong sidebar
4. Deploy relay binary (BL-SSH-02)
```

---

## Luồng thay thế

**[A1] Key authentication fail:**
- Thử SSH agent forwarding
- Nếu fail → prompt password
- Nếu fail → hiển thị error với hướng dẫn fix

**[A2] Host unreachable:**
- Timeout sau 10 giây
- Hiển thị: "Connection refused: <host>:<port>"

**[A3] ProxyJump required:**
- Parse ProxyJump config
- Kết nối qua jump host trước
- Forward tới target host

---

## Business Rules

| Rule | Mô tả |
|------|-------|
| BR-SSH-01 | Credentials không bao giờ được lưu plaintext — dùng OS keychain |
| BR-SSH-02 | SSH config phải được re-parse mỗi lần connect (file có thể thay đổi) |
| BR-SSH-03 | Keepalive interval = 30 giây để phát hiện drop sớm |
| BR-SSH-04 | Maximum 10 connections đồng thời tới cùng host |

---

## SSH Config Parsing Rules

```
# Hỗ trợ đầy đủ:
Host myserver
  HostName 192.168.1.100
  Port 22
  User ubuntu
  IdentityFile ~/.ssh/mykey
  ProxyJump jumphost

Include ~/.ssh/config.d/*
```
