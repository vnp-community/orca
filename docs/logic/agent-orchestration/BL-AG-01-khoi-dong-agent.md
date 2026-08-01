# BL-AG-01 — Khởi động AI Agent

| Thuộc tính | Giá trị |
|-----------|---------|
| **Mã nghiệp vụ** | BL-AG-01 |
| **Tên** | Khởi động AI Agent |
| **Nhóm** | Agent Orchestration |
| **Actors** | Alex, Maya, Carlos, Sam |
| **Ưu tiên** | P0 — Must Have |
| **Tính năng** | F04 AI Agent Support |
| **SRS** | FR-2.2 |

---

## Mô tả nghiệp vụ

Khởi động một AI agent (Claude Code, Codex, v.v.) trong worktree cụ thể. Agent được chạy trong PTY session **trên Dev Server** thông qua kênh JSON-RPC.

**Quan trọng về topology kết nối:**  
Dev Server chủ động **mở WebSocket kết nối vào Orca Server** (`ws://orca:6768/agent`).  
Khi cần spawn agent, Orca Server gửi JSON-RPC request **ngược lại qua kết nối đó**.  
Dev Server không cần expose port ra ngoài.

---

## Tiền điều kiện

- Worktree đã tồn tại trên Dev Server
- Dev Server Agent (systemd service) đã **kết nối WebSocket vào Orca Server** và handshake thành công
- Orca Server đã đăng ký connection trong `AgentConnectionManager`
- Agent binary (claude, codex...) đã có trong PATH **trên Dev Server**
- AI credential file đã có: `~/.orca/ai-providers/<accountId>.enc` **trên Dev Server**
- Trust preset đã được cấu hình

---

## Luồng chính

```
1. Người dùng click "Start Agent" trong worktree
2. Chọn agent type (nếu chưa có mặc định)
3. Hệ thống thực thi:
   a. Resolve agent config (binary, args, trustPresetEnvVars)
   b. ProfileResolver.resolve(userId) → build env vars từ profile 3-layer
   c. AIProviderResolver.resolve() → xác định provider + apiKeyEnvVar
   d. Lấy WS connection đã mở sẵn từ Dev Server:
        conn = AgentConnectionManager.getConnection(devServerId)
   e. Gửi JSON-RPC request qua connection đó:
        conn.call('agent.spawn', { agentBinary, args, cwd, env, userId })
        → Orca Server gửi ngược qua WS vào Dev Server
   f. Dev Server nhận request:
        - Verify RpcExecutionContext (HMAC-SHA256, 30s TTL)
        - node-pty.spawn(agentBinary, args, { cwd: worktreePath, env })
        - Lưu: ptySessionStore[userId + worktreeId] = ptyHandle
        - Trả result: { ptyId, pid }
   g. Dev Server stream PTY output về Orca:
        → JSON-RPC event: agent.output { ptyId, data }
   h. Orca Server parse OSC sequences → detect status "idle"
   i. INSERT orca_sessions { sessionId, worktreeId, devServerId, startedAt }
4. Agent hiển thị prompt → trạng thái "idle"
5. Agent card update trong sidebar
```

---

## Luồng thay thế

**[A1] Agent binary không tìm thấy trên Dev Server:**
- JSON-RPC result trả lỗi: `{ error: { code: -32001, message: "binary not found: claude" } }`
- Hiển thị: "Claude Code không tìm thấy trên Dev Server"
- Link hướng dẫn cài đặt trên Dev Server

**[A2] Dev Server chưa kết nối (WS chưa có trong AgentConnectionManager):**
- `getConnection(devServerId)` trả null → lỗi: "Dev Server chưa kết nối"
- Gợi ý: kiểm tra Dev Server Agent service có đang chạy không

**[A3] Agent startup timeout (> 30s không thấy OSC "idle"):**
- Gửi `conn.call('agent.kill', { ptyId })` để cleanup PTY trên Dev Server
- Hiển thị error log (đã nhận qua stream)

**[A4] Trust preset conflict:**
- Cảnh báo nếu agent cần permission cao hơn preset
- Yêu cầu user upgrade trust level

---

## Hậu điều kiện

- Agent process đang chạy trong PTY **trên Dev Server**
- PTY handle được lưu trong `ptySessionStore[userId+worktreeId]` trên Dev Server
- Session record INSERT vào Server DB (bao gồm `devServerId`, `ptyId`)
- Agent status = "idle" (sẵn sàng nhận prompt)
- Sidebar card hiển thị đúng trạng thái
- OSC hook parser active để detect trạng thái qua WS stream

---

## Business Rules

| Rule | Mô tả |
|------|-------|
| BR-AG-01 | Chỉ có thể chạy 1 agent chính per worktree per userId |
| BR-AG-02 | Agent chạy với cwd = worktree path **trên Dev Server** |
| BR-AG-03 | Trust preset env vars phải được apply TRƯỚC khi spawn |
| BR-AG-04 | Startup timeout 30 giây → cleanup PTY + trả lỗi |
| BR-AG-18 | Agent process chạy trên Dev Server, không phải Orca Server |
| BR-AG-19 | Dev Server là WS **client** — chủ động connect đến Orca Server |
| BR-AG-20 | Orca Server gửi JSON-RPC request ngược lại qua WS connection đó |
| BR-AG-21 | RpcExecutionContext phải có HMAC-SHA256 hợp lệ, TTL 30 giây |

---

## Agent Config

```typescript
interface AgentConfig {
  id: AgentId;
  name: string;
  binary: string;             // e.g., "claude" — binary trên Dev Server
  startupCommand: string[];   // e.g., ["claude", "--no-auto-commit"]
  sessionResumeFlag?: string; // e.g., "--resume"
  usageModule?: string;       // e.g., "claude-usage"
  trustPresetEnvVars: Record<string, string>;
}
```

## JSON-RPC Contract (Orca → Dev Server)

```typescript
// Orca Server gửi qua WS connection mà Dev Server đã mở sẵn
conn.call('agent.spawn', {
  agentBinary: string,         // e.g., "claude"
  args: string[],              // e.g., ["--no-auto-commit"]
  cwd: string,                 // worktree path trên Dev Server
  env: Record<string, string>, // merged: profile + trustPreset + AI key ref
  userId: string,              // để isolate PTY per user
  cols: number,
  rows: number,
})
// Dev Server trả: { ptyId: string, pid: number }
// Dev Server stream: agent.output { ptyId, data } | agent.exit { ptyId, code }
```

---

## SLO

| Metric | Target |
|--------|--------|
| Thời gian từ conn.call → agent:idle | < 5 giây |
| Startup success rate | > 98% |
| JSON-RPC round-trip qua WS | < 200ms |
