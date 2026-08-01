# BUG-BE-AWS-002: `agent-session.ts` handshake gửi `capabilities` array nhưng không gửi `agentToken` theo convention HLD

## Mức độ: 🟡 MEDIUM

## Tóm tắt

HLD (BL-AWS-02) mô tả custom agent handshake:
```
{ type: 'agent.handshake', agentToken: 'tok_xxx', capabilities: ['execute', 'stream'] }
```

Nhưng `agent-session.ts` gửi handshake dạng JSON-RPC `agent.hello` (AGENT_HANDSHAKE_METHOD) thay vì `type: 'agent.handshake'`:

```typescript
// agent-session.ts:58-73
const rpc = {
  jsonrpc: '2.0',
  id: 1,
  method: AGENT_HANDSHAKE_METHOD,  // = 'agent.hello' (không phải 'agent.handshake')
  params: {
    agentVersion: '5.0.0',
    capabilities: [...],
    agentToken: config.agentToken,
    ...
  },
}
```

## File liên quan

- [`src/relay/agent-session.ts`](file:///Users/binhnt/Work/blockchain/vnp-blc/orca/src/relay/agent-session.ts) — Lines 57-76
- [`src/shared/agent-wire-protocol.ts`](file:///Users/binhnt/Work/blockchain/vnp-blc/orca/src/shared/agent-wire-protocol.ts) — `AGENT_HANDSHAKE_METHOD`

## Phân tích

Có 2 protocol diverge:
- **HLD BL-AWS-01** (relay mode): Orca connect đến Agent với Bearer token header
- **HLD BL-AWS-02** (direct mode): Custom Agent gửi `agent.handshake` type message

Nhưng `agent-session.ts` implement một protocol thứ 3 (internal binary frame protocol dùng `agent.hello`). Điều này có thể intentional nếu server cũng implement cùng protocol — nhưng HLD không document protocol internal này.

**Low severity** nếu cả client (agent-session) và server (ws-handshake.ts) dùng cùng method name.

## Cần xác nhận

Cần kiểm tra `src/main/dev-server/ws-handshake.ts` — server side expect `AGENT_HANDSHAKE_METHOD` hay `'agent.handshake'`? Nếu match → không phải bug. Nếu không match → handshake fail.

## Liên quan đến luồng

- **BL-AWS-02**: Handshake protocol format.

---

## ✅ Fix Status: RESOLVED (2026-08-01)

**Fix:** Verified: method='agent.handshake' đúng theo HLD v5. WsHandshakeInfo.devServerId optional field added. ws-session-router.ts + relay-handshake.ts consistent.
