# SOL-DS-001 — Fix relay-websocket Handshake Token Validation

**Fixes:** [BUG-DS-001](../BUG-DS-001-relay-ws-handshake-token.md)  
**TDD Ref:** TDD-13 §3 (DevServerRelay interface), TDD-05 §4 (relay protocol)  
**File:** `deploy/dev/agent/agent.js`  
**Effort:** ~30 phút  
**Status:** ✅ DONE — 2026-07-27 (TASK-DS-001)  
**Implemented in:** `deploy/dev/agent/agent.js` dng 709-731

---

## Phân Tích

Theo TDD-13 §3, `DevServerRelay` chỉ yêu cầu handshake thành công để wire `session`. Token validation là tầng bảo vệ — nhưng trong relay-ws mode, token đã được validate tại connection layer (`wss.on('connection')` check header/query). Validate lại trong handshake frame là sai và gây bug.

**Nguyên tắc**: Mỗi layer validate đúng thứ của nó:
- `wss.on('connection')`: validate HTTP-level token (Authorization header / query param)
- `handleSession`: validate JSON-RPC message format
- **Không** validate lại token trong handshake payload (Orca không gửi field này)

---

## Thay Đổi Cần Thực Hiện

### File: `deploy/dev/agent/agent.js`

**Tìm đoạn** (trong `handleSession()`, nhánh `rpc.method === 'agent.handshake'`):

```javascript
// TRƯỚC (sai):
if (rpc.method === 'agent.handshake') {
  const incoming = rpc.params?.agentToken;
  if (incoming !== relayToken) {          // ← luôn true vì Orca không gửi agentToken
    log.warn('Handshake rejected (bad token from Orca)');
    ws.close(1008, 'Unauthorized');
    return;
  }
  // ...reply OK
}
```

**Thay bằng**:

```javascript
// SAU (đúng):
if (rpc.method === 'agent.handshake') {
  // relay-ws mode: token đã được validate tại wss.on('connection')
  // qua Authorization header hoặc ?token= query param.
  // Orca (initiator) KHÔNG gửi agentToken trong handshake params — chỉ gửi orcaVersion.
  // Không cần re-validate token ở đây.
  const okResp = {
    jsonrpc: '2.0', id: rpc.id,
    result: {
      ok:           true,
      devServerId:  DEV_ID,
      platform:     process.platform,
      arch:         process.arch,
      nodeVersion:  process.version,
      agentVersion: '1.0.0',
      sessionId:    `sess-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`,
      tools:        discoveredTools.map(t => t.name),
    },
  };
  ws.send(encodeFrame(FRAME_TYPE.HANDSHAKE_OK, JSON.stringify(okResp)));
  handshakeOk = true;
  keepalive   = startKeepalive(ws);
  log.info(`Handshake OK [relay-ws]. devServerId=${DEV_ID}. Tools: ${discoveredTools.map(t=>t.name).join(', ')}`);
  return;
}
```

---

## Verification

Sau khi fix, test bằng relay-ws mode:

```bash
# 1. Start agent relay-ws trên dev server
ssh ubuntu@172.20.2.31 "cd ~/orca-agent && \
  AGENT_PORT=6799 AGENT_TOKEN=relay-secret MODE=relay-websocket node agent.js &"

# 2. Trong Orca UI: Add Dev Server
#    Type: relay-websocket
#    URL:  ws://172.20.2.31:6799/orca-relay?token=relay-secret
#    → Click "Test Connection"

# 3. Kiểm tra log agent:
ssh ubuntu@172.20.2.31 "tail -20 ~/orca-agent/logs/agent.log"
# Expected: "Handshake OK [relay-ws]" — KHÔNG phải "Handshake rejected"
```

---

## Không Thay Đổi

- `wss.on('connection')` token validation (giữ nguyên — đây là auth layer đúng)
- `runOrcaInitiatorHandshake` trong server (không cần thay đổi)
- `dev-server-relay-bridge.ts` token strip logic (giữ nguyên)
