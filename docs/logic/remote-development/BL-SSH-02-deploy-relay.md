# BL-SSH-02 — Deploy Orca Relay Binary

| Thuộc tính | Giá trị |
|-----------|---------|
| **Mã nghiệp vụ** | BL-SSH-02 |
| **Tên** | Deploy Orca Relay Binary lên Remote Host |
| **Nhóm** | Remote Development |
| **Actors** | Carlos (Remote Dev), DevOps |
| **Ưu tiên** | P1 — Should Have |
| **Tính năng** | F07 SSH Worktrees |
| **SRS** | FR-4.2 |

---

## Mô tả nghiệp vụ

Tự động deploy và duy trì Orca relay binary trên remote host — relay cung cấp bridge cho terminal I/O, file operations, và port scanning mà không cần người dùng setup thủ công.

---

## Luồng chính

```
1. Sau khi SSH connected (BL-SSH-01)
2. Hệ thống kiểm tra relay trên remote:
   a. Chạy: orca-relay --version (nếu tồn tại)
   b. So sánh với version local
3. Nếu MISSING hoặc OUTDATED:
   a. Detect remote OS/arch (uname -m, uname -s)
   b. Select đúng relay binary (linux-x64, linux-arm64)
   c. Upload qua SFTP tới ~/.local/bin/orca-relay
   d. chmod +x
   e. Verify SHA256 hash
4. Khởi động relay process:
   orca-relay --listen <port> --token <session-token>
5. Kết nối tới relay qua WebSocket
6. Relay sẵn sàng serve requests
```

---

## Luồng thay thế

**[A1] Upload fail (network):**
- Retry 3 lần
- Nếu vẫn fail: hiển thị error, gợi ý manual install

**[A2] Hash mismatch:**
- Log security warning
- Re-upload binary
- Nếu tiếp tục fail: từ chối connect

**[A3] Relay crash ngay sau khi start:**
- Collect stderr log
- Hiển thị diagnostic information
- Gợi ý check remote OS requirements

---

## Business Rules

| Rule | Mô tả |
|------|-------|
| BR-SSH-05 | Relay binary PHẢI được verify hash trước khi execute |
| BR-SSH-06 | Relay session token phải là random, expire sau session |
| BR-SSH-07 | Relay binary chỉ được upload khi version mismatch |
| BR-SSH-08 | Relay không được require root permission |
| BR-SSH-09 | Version mismatch phải được report rõ ràng tới người dùng |
