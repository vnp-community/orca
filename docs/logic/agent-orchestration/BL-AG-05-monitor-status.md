# BL-AG-05 — Monitor Trạng thái Agent Real-time

| Thuộc tính | Giá trị |
|-----------|---------|
| **Mã nghiệp vụ** | BL-AG-05 |
| **Tên** | Monitor Trạng thái Agent Real-time |
| **Nhóm** | Agent Orchestration |
| **Actors** | Alex, Maya, Carlos, Sam, DevOps |
| **Ưu tiên** | P0 — Must Have |
| **Tính năng** | F04, F11 |
| **SRS** | FR-2.2 |

---

## Mô tả nghiệp vụ

Theo dõi và hiển thị trạng thái của tất cả agent đang chạy trong thời gian thực — trên desktop và mobile.  
PTY output từ Dev Server liên tục được stream về Orca Server qua WebSocket (Dev Server đã mở kết nối WS vào Orca). Orca parse và push trạng thái xuống Renderer và Mobile App.

---

## Trạng thái Agent

| Status | Mô tả | Hiển thị |
|--------|-------|---------|
| `idle` | Agent đã start, chờ prompt | ⚪ |
| `running` | Đang xử lý, có output | 🟢 |
| `waiting` | Đang chờ user input | 🟡 |
| `completed` | Đã xong task | ✅ |
| `error` | Lỗi, cần intervention | 🔴 |
| `stopped` | Đã dừng | ⚫ |

---

## Luồng chính

```
1. Agent chạy trong PTY trên Dev Server → sinh OSC escape sequences + text output
2. Dev Server liên tục gửi PTY output về Orca qua WebSocket (persistent):
     JSON-RPC event: agent.output { ptyId, data: "<OSC/text>" }
3. Orca Server (AgentConnectionManager) nhận WS message:
   a. OSC 133 A  → command started    → status = "running"
   b. OSC 133 B  → command output     → stream to UI
   c. OSC 133 D  → command finished   → check exit code → status = "idle" / "error"
   d. Pattern match text:
      - "waiting for input" → status = "waiting"
      - RATE_LIMIT_PATTERNS → emit: agent:rateLimited
      - "task completed"    → status = "completed"
4. Orca emit: agent:statusChanged { sessionId, status, detail }
5. IPC push → Renderer: update agent card (indicator, spinner)
6. WebSocket TweetNaCl E2E push → Mobile App (Sam) nếu paired device
```

---

## Business Rules

| Rule | Mô tả |
|------|-------|
| BR-AG-14 | Status update phải xảy ra < 500ms sau khi detect |
| BR-AG-15 | Status được persist vào Server DB để resume sau restart |
| BR-AG-16 | Mobile nhận status update < 5 giây |
| BR-AG-17 | Status change → push notification nếu app không focused |
| BR-AG-19 | PTY output stream đến Orca qua WS Dev Server đã mở (không poll) |

---

## Luồng stream (chi tiết)

```
Dev Server PTY
    │ node-pty data event
    ▼
Dev Server Agent
    │ gửi qua WebSocket (Dev Server đã connect vào Orca):
    │ { jsonrpc:'2.0', method:'agent.output', params:{ ptyId, data } }
    ▼
Orca Server — AgentConnectionManager
    │ nhận WS message → route theo ptyId → sessionId
    ▼
AgentHookParser
    ├─ OSC parse
    ├─ pattern match
    └─ emit: agent:statusChanged
    │
    ├─► IPC → Renderer (React UI)
    └─► WebSocket TweetNaCl → Mobile App
```
