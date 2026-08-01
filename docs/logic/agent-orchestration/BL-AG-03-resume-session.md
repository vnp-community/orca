# BL-AG-03 — Resume Agent Session

| Thuộc tính | Giá trị |
|-----------|---------|
| **Mã nghiệp vụ** | BL-AG-03 |
| **Tên** | Resume Agent Session |
| **Nhóm** | Agent Orchestration |
| **Actors** | Alex, Maya, Carlos |
| **Ưu tiên** | P1 — Should Have |
| **Tính năng** | F04 AI Agent Support |
| **SRS** | FR-2.3 |

---

## Mô tả nghiệp vụ

Tiếp tục session agent đã dừng trước đó — agent khôi phục context, conversation history, và tiếp tục từ điểm đã dừng.  
Orca Server load sessionId từ DB, sau đó gửi JSON-RPC spawn với resume flag qua WebSocket connection Dev Server đã mở sẵn.

---

## Tiền điều kiện

- Session ID đã được lưu từ session trước (trong Server DB)
- Worktree vẫn tồn tại trên Dev Server
- Dev Server Agent đang kết nối WebSocket vào Orca Server
- Agent binary version compatible với session

---

## Luồng chính

```
1. Người dùng click "Resume" trên worktree (session cũ)
2. Hệ thống load session ID từ persistence store
3. Khởi động agent với resume flag:
   a. SELECT sessionId, devServerId FROM orca_sessions
      WHERE worktreeId=? ORDER BY startedAt DESC   ← Server DB
   b. Build resume args:
      - Claude Code: ["claude", "--resume", sessionId]
      - Codex:  ["codex", "--session-file", sessionFilePath]
      - OpenCode: ["opencode", "resume", sessionId]
   c. conn = AgentConnectionManager.getConnection(devServerId)
   d. conn.call('agent.spawn', { agentBinary, args: resumeArgs, cwd, env, userId })
      → Orca Server gửi qua WS vào Dev Server
      → Dev Server: node-pty.spawn(agentBinary, resumeArgs, { cwd, env })
4. Dev Server stream output về Orca qua WS
5. Orca parse OSC → agent khôi phục context
6. Agent hiển thị context summary
7. Agent sẵn sàng nhận prompt tiếp theo
```

---

## Luồng thay thế

**[A1] Session ID không còn valid (expired > 7 ngày):**
- Thông báo "Session expired hoặc không tìm thấy"
- Gợi ý bắt đầu session mới

**[A2] Dev Server chưa kết nối:**
- `getConnection(devServerId)` trả null → lỗi "Dev Server chưa kết nối"

**[A3] Agent version không tương thích:**
- JSON-RPC result trả lỗi version mismatch
- Hỏi: "Bắt đầu session mới?"

---

## Business Rules

| Rule | Mô tả |
|------|-------|
| BR-AG-08 | Session ID expire sau 7 ngày không hoạt động |
| BR-AG-09 | Resume phải sử dụng cùng agent version với session gốc |
| BR-AG-10 | Session ID được lưu per-worktree trong Server DB (bao gồm devServerId) |
| BR-AG-19 | Resume spawn gửi qua JSON-RPC trên WS Dev Server đã mở |

---

## Session Resume Support by Agent

| Agent | Resume Method | Support |
|-------|--------------|---------|
| Claude Code | `--resume <id>` | ✅ Full |
| Codex | session file | ✅ Full |
| OpenCode | `resume <id>` | ✅ Full |
| Gemini | Partial | ⚠️ Partial |
| Cursor | None | ❌ Không hỗ trợ |
