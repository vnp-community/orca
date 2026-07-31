# TDD-AG-01: Agent Architecture

**Version:** 4.0  
**Date:** 2026-07-28  
**Source:** `deploy/dev/agent/agent.js`

---

## 1. Process Model

Agent là một **Node.js single-process** chạy trên remote dev server:

```
agent.js
  ├─ main()
  │   ├─ 1. discoverTools()          → build tool registry
  │   ├─ 2. createConnection()       → WebSocket (direct-ws | relay-ws)
  │   └─ 3. handleSession(ws, tools) → handshake + dispatch loop
  │
  └─ Shutdown:
      ├─ ws.close() / connection drop → exit(2) (systemd restart)
      └─ SIGTERM → graceful stop
```

---

## 2. Lifecycle

```
Start
  │
  ├─ [direct-ws] Connect to ORCA_URL/agent?token=AGENT_TOKEN
  │   └─ HTTP Upgrade → WebSocket
  │
  ├─ [relay-ws] Connect to RELAY_URL
  │   └─ Auth with RELAY_TOKEN
  │
  ▼
Handshake (30s timeout)
  │
  ├─ Send: { type: 'handshake', agentToken, platform, arch, nodeVersion, agentVersion }
  ├─ Recv: { type: 'handshake-ack', orcaVersion, sessionId }
  └─ On failure → close + exit(2)
  │
  ▼
Session Loop
  │
  ├─ Keepalive: send every 10s (TYPE=0x01, LENGTH=0 — empty Regular frame)
  │
  ├─ Recv frames → decodeFrame() → JSON-RPC dispatch
  │   ├─ tools/list → list available tools
  │   └─ tools/call → call specific tool, stream output
  │
  └─ On disconnect → exit(2)
```

---

## 3. Error Handling

```javascript
// Unhandled errors:
process.on('uncaughtException', (err) => {
  console.error('[Agent] Uncaught exception:', err)
  process.exit(2)   // systemd will restart
})

process.on('unhandledRejection', (reason) => {
  console.error('[Agent] Unhandled rejection:', reason)
  process.exit(2)
})

// WebSocket error:
ws.on('error', (err) => {
  console.error('[Agent] WS error:', err.message)
  // ws 'close' event will fire → exit(2)
})

ws.on('close', (code, reason) => {
  console.log(`[Agent] Disconnected: ${code} ${reason}`)
  process.exit(2)   // Token one-time-use → systemd restart → fresh token
})
```

---

## 4. Concurrency Model

Agent xử lý tool calls **sequentially** theo default:
- Một tool call at a time per channel
- Multiple channels multiplexed qua SshChannelMultiplexer
- Each channel = one MCP tool call session
- Tool timeout: 5 minutes (configurable)

---

## 5. Security Model

| Constraint | Implementation |
|-----------|---------------|
| No shell injection | `spawn(cmd, args, { shell: false })` |
| Working dir scoped | All paths relative to `WORK_DIR` |
| No network access from tools | Tools chỉ exec local commands |
| Token expiry | exit(2) → fresh token per reconnect |
| No token logging | `AGENT_TOKEN` chỉ read từ env, không log |
| Allowed commands | Whitelist trong tool handlers (không exec arbitrary) |

---

## 6. Dependencies (minimal)

```json
{
  "dependencies": {
    "ws": "^8.0.0"
  }
}
// Không có thêm dependencies — tránh supply chain risks
// Node.js built-ins: child_process, fs, path, os, crypto
```

---

## 7. Platform Support

| Platform | Status |
|---------|--------|
| Linux x64 | ✅ Primary |
| Linux arm64 | ✅ |
| macOS x64/arm64 | ✅ |
| Windows x64 | ⚠️ Limited (WSL recommended) |

---

## 8. Logging

```javascript
// Console-based logging (systemd journal captures stdout/stderr)
console.log('[Agent] Connected: sessionId=' + info.sessionId)
console.log('[Agent] Tool call: ' + method)
console.error('[Agent] Error: ' + err.message)

// NO: AGENT_TOKEN in any log output
// YES: Token masked as '***' if ever referenced
```
