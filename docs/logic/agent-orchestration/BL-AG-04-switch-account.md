# BL-AG-04 — Switch Account / Provider

| Thuộc tính | Giá trị |
|-----------|---------|
| **Mã nghiệp vụ** | BL-AG-04 |
| **Tên** | Switch Account / Provider khi Rate Limited |
| **Nhóm** | Agent Orchestration |
| **Actors** | Alex, Sam |
| **Ưu tiên** | P1 — Should Have |
| **Tính năng** | F04 AI Agent Support |
| **SRS** | FR-2.5 |

---

## Mô tả nghiệp vụ

Khi agent bị rate limited, người dùng hot-swap sang account khác hoặc provider khác mà không mất context.  
Orca Server nhận PTY output qua WS stream, phát hiện rate limit, sau đó stop + spawn agent mới qua cùng WebSocket connection.

---

## Luồng chính

```
1. Dev Server gửi PTY output qua WS (JSON-RPC event: agent.output)
   → Orca Server nhận, AgentHookParser parse pattern: rate-limit detected
   → emit: agent:rateLimited { resetAt }

2. Renderer hiển thị alert: "Claude Code bị rate limited. Reset lúc HH:MM"
3. Người dùng chọn:
   a. Switch to account 2 (cùng provider)
   b. Switch to Codex (provider khác)
   c. Wait until reset

4. Hệ thống:
   a. UPDATE orca_sessions SET status='stopped'   ← Server DB
   b. BL-AG-02: conn.call('agent.kill', { ptyId })
      → Dev Server: kill PTY qua WS
   c. AIProviderResolver.resolve() với account mới
      → priority cascade: user > project > company
      → { provider: 'openai', apiKeyEnvVar: 'OPENAI_API_KEY' }
   d. BL-AG-01: conn.call('agent.spawn', { agentBinary, newEnv })
      → Dev Server: spawn PTY mới với credentials mới (đọc .enc file mới)
   e. BL-AG-03: resume session nếu compatible

5. Agent tiếp tục với credentials mới
```

---

## Business Rules

| Rule | Mô tả |
|------|-------|
| BR-AG-11 | Credentials không lưu plaintext — đọc từ .enc file trên Dev Server khi spawn |
| BR-AG-12 | Rate limit detection qua pattern matching PTY output stream (qua WS) |
| BR-AG-13 | Usage counter reset sau khi switch account |
| BR-AG-19 | Kill + spawn đều qua JSON-RPC trên WS Dev Server đã mở |

---

## Rate Limit Detection Patterns

```typescript
const RATE_LIMIT_PATTERNS = {
  claude: /rate.?limit|quota.?exceed|too.?many.?request/i,
  codex: /rate.?limit|429|quota/i,
  opencode: /rate.?limit|quota/i,
};
// Áp dụng trên PTY output nhận được qua WS JSON-RPC event: agent.output
```
