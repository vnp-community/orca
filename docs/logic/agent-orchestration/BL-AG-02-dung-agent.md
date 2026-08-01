# BL-AG-02 — Dừng Agent

| Thuộc tính | Giá trị |
|-----------|---------|
| **Mã nghiệp vụ** | BL-AG-02 |
| **Tên** | Dừng Agent |
| **Nhóm** | Agent Orchestration |
| **Actors** | Alex, Maya, Carlos, Sam |
| **Ưu tiên** | P0 — Must Have |
| **Tính năng** | F04 AI Agent Support |
| **SRS** | FR-2.2 |

---

## Mô tả nghiệp vụ

Dừng AI agent đang chạy trên Dev Server — graceful (Ctrl+C) hoặc force kill (SIGKILL).  
Orca Server gửi lệnh kill qua kênh JSON-RPC trên WebSocket connection mà Dev Server đã mở sẵn.

---

## Luồng chính (Graceful Stop)

```
1. Người dùng click "Stop" trên agent card
2. Hệ thống:
   a. Lấy connection: conn = AgentConnectionManager.getConnection(devServerId)
   b. Gửi JSON-RPC: conn.call('agent.sendInput', { ptyId, data: '\x03' })
      → Orca Server gửi qua WS vào Dev Server
      → Dev Server: ptyHandle.write('\x03') → PTY stdin (Ctrl+C)
   c. Agent nhận Ctrl+C → thoát gracefully
   d. Dev Server gửi event: agent.exit { ptyId, code: 0 } → Orca qua WS
   e. UPDATE orca_sessions SET status='stopped', sessionId=<id>  ← Server DB
   f. emit: agent:stopped
3. PTY session vẫn còn record (để resume sau này)
4. Session ID được lưu để resume
```

## Luồng thay thế

**[A1] Agent không thoát sau 10 giây:**
- Hiển thị dialog "Force Kill?"
- Nếu đồng ý:
  ```
  conn.call('agent.kill', { ptyId, signal: 'SIGKILL' })
  → Dev Server: ptyHandle.kill('SIGKILL')
  → Dev Server: xóa ptySessionStore[userId + worktreeId]
  → Dev Server gửi: agent.exit { ptyId, code: -1 }
  ```

---

## Business Rules

| Rule | Mô tả |
|------|-------|
| BR-AG-05 | Ưu tiên graceful stop (Ctrl+C) trước force kill (SIGKILL) |
| BR-AG-06 | Không kill agent khi đang ghi file (check write lock) |
| BR-AG-07 | Session ID phải được lưu sau khi stop để support resume |
| BR-AG-19 | Kill command gửi qua JSON-RPC trên WS Dev Server đã mở sẵn |
