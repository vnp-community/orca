# BL-SSH-03 — SSH Auto-Reconnect

| Thuộc tính | Giá trị |
|-----------|---------|
| **Mã nghiệp vụ** | BL-SSH-03 |
| **Tên** | SSH Auto-Reconnect với Agent Continuity |
| **Nhóm** | Remote Development |
| **Actors** | Carlos (Remote Dev) |
| **Ưu tiên** | P1 — Must Have |
| **Tính năng** | F07 SSH Worktrees |
| **SRS** | FR-4.3 |

---

## Mô tả nghiệp vụ

Khi SSH connection bị gián đoạn, hệ thống tự động phát hiện và reconnect mà không cần người dùng can thiệp — agent trên remote vẫn tiếp tục chạy và output được buffer.

---

## Luồng chính

```
1. Phát hiện drop: socket close hoặc keepalive timeout
2. Hiển thị "Reconnecting..." overlay trên terminal
3. Dừng nhận input từ người dùng (buffer nếu có)
4. Bắt đầu reconnect loop với exponential backoff:
   - Attempt 1: sau 1 giây
   - Attempt 2: sau 2 giây
   - Attempt 3: sau 4 giây
   - ...
   - Maximum interval: 30 giây
5. Mỗi attempt: thử kết nối lại qua BL-SSH-01
6. Khi reconnected:
   a. Check relay status trên remote
   b. Relay vẫn chạy → reconnect WebSocket
   c. Relay đã die → restart relay (BL-SSH-02)
   d. Flush output buffer từ relay
   e. Resume terminal input
   f. Xóa "Reconnecting..." overlay
```

---

## Agent Continuity

```
[Local disconnected]            [Remote server]
      |                               |
      |     Agent process VẪN CHẠY   |
      |     Relay buffer output       |
      |                               |
[Reconnect thành công]          [Agent output buffered]
      |←─────── flush buffer ─────────|
      |
[Carlos thấy output đã xảy ra khi offline]
```

---

## Business Rules

| Rule | Mô tả |
|------|-------|
| BR-SSH-10 | Agent process trên remote PHẢI tiếp tục chạy khi local disconnect |
| BR-SSH-11 | Output buffer tối đa: 10MB per session |
| BR-SSH-12 | Reconnect attempts: unlimited (không give up) |
| BR-SSH-13 | Người dùng có thể manually cancel reconnect |
| BR-SSH-14 | Reconnect phải transparent — không cần user action |

---

## SLO

| Metric | Target |
|--------|--------|
| Reconnect success rate | > 95% |
| Reconnect time (khi network available) | < 10 giây |
| Agent continuity | 100% (agent không bị kill khi local disconnect) |
