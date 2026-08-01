# BUG-BE-AWS-001: `AgentWebSocketServer` không verify `agentToken` từ DB — dùng in-memory slot map thay vì `orca_agent_tokens`

**Status:** ✅ FIXED — 2026-08-01  
**Fixed by:** TASK-AWS-002  
**Implementation:** agent-ws-server.ts: SHA-256 hash stored, not plaintext  

## Mức độ: 🔴 HIGH

## Tóm tắt

HLD (BL-AWS-02) mô tả luồng xác thực direct-websocket mode:
```
[Orca Web Server — AgentWsRouter]
    ├─ Verify agentToken:
    │   SELECT from orca_agent_tokens WHERE token_hash=SHA256(agentToken) AND is_active=1
    │   ← Server DB
    ├─ IF valid: { type: 'handshake-ok', sessionId: uuid(), serverId }
    ├─ IF invalid: { type: 'handshake-error', code: 'INVALID_TOKEN' } → close
```

Nhưng trong `agent-ws-server.ts`, token được verify bằng in-memory `pendingSlots` map:
```typescript
// Lines 115-118
runOrcaReceiverHandshake(
  ws,
  (token) => this.pendingSlots.has(token),  // ← in-memory check, NO database!
  this.orcaVersion
)
```

## File liên quan

- [`src/main/dev-server/agent-ws-server.ts`](file:///Users/binhnt/Work/blockchain/vnp-blc/orca/src/main/dev-server/agent-ws-server.ts) — Lines 111-161

## Code sai

```typescript
// Lines 111-118
private handleConnection(ws: WebSocket): void {
  runOrcaReceiverHandshake(
    ws,
    (token) => this.pendingSlots.has(token),  // ← slot map, NOT orca_agent_tokens DB
    this.orcaVersion
  )
```

## Ảnh hưởng

1. **HLD Security Model vi phạm**: Custom AI Agents (BL-AWS-02) sử dụng long-lived tokens từ `orca_agent_tokens` — nhưng code chỉ accept tokens đã được pre-registered vào slot.
2. `orca_agent_tokens` table (nếu tồn tại) không bao giờ được query → BL-AWS-02 direct-websocket mode hoàn toàn không hoạt động.
3. BL-AWS-03 (Token Management) — Admin generate token → lưu DB → nhưng custom agent dùng token đó không connect được vì không có slot.
4. Slot mechanism (`registerSlot`) là cho DevServer agent ephemeral tokens — không phải cho custom agents long-lived tokens.

## Giải thích kiến trúc

`agent-ws-server.ts` đang implement một hybrid model không document trong HLD:
- **Dev Server Agent** (ephemeral): dùng agentToken → pre-register slot → connect một lần
- **Custom AI Agent** (persistent): dùng long-lived token từ DB → connect bất kỳ lúc nào

Code hiện tại chỉ implement path 1. Path 2 (BL-AWS-02) thiếu hoàn toàn.

## Liên quan đến luồng

- **BL-AWS-02**: Direct-websocket — custom agent token verify.
- **BL-AWS-03**: Token management — tokens generate nhưng không được dùng để auth.
