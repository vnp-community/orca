# BUG-AG-ORCH-001: `agent.sendInput` không được implement trong RPC dispatch — không thể dừng agent bằng Ctrl+C

## Mức độ: 🔴 HIGH

## Tóm tắt

HLD (BL-AG-02) mô tả luồng dừng Agent:
```
[Main Process — AgentManager.stop()]
    ├─ conn.call('agent.sendInput', { ptyId, data: '\x03' })   ← Ctrl+C
    │     → Dev Server: ptyHandle.write('\x03') → PTY stdin
    ├─ Wait 10s cho graceful exit
    └─ [A1 Timeout] conn.call('agent.kill', { ptyId, signal: 'SIGKILL' })
```

Nhưng trong `agent-rpc-dispatch.ts`, **không có case `'agent.sendInput'`** nào được register. Agent chỉ có `agent.spawn` và `agent.kill`.

## File liên quan

- [`src/relay/agent-rpc-dispatch.ts`](file:///Users/binhnt/Work/blockchain/vnp-blc/orca/src/relay/agent-rpc-dispatch.ts) — Lines 461-483 (chỉ có `agent.spawn` và `agent.kill`)
- [`src/relay/agent-spawner.ts`](file:///Users/binhnt/Work/blockchain/vnp-blc/orca/src/relay/agent-spawner.ts) — (không có `handleAgentSendInput`)

## Code thực tế

```typescript
// agent-rpc-dispatch.ts:461-483
case 'agent.spawn': { ... }   // ✅ OK
case 'agent.kill':  { ... }   // ✅ OK — nhưng dùng SIGTERM, không phải SIGKILL!
// agent.sendInput  ← MISSING
// agent.resume     ← MISSING
```

## Ảnh hưởng

1. **BL-AG-02 hoàn toàn không hoạt động qua graceful path**: Main Process gọi `conn.call('agent.sendInput', { ptyId, data: '\x03' })` → Agent trả `MethodNotFound` error → Ctrl+C không đến được PTY stdin.
2. Graceful stop timeout 10s sẽ không bao giờ được trigger → luôn phải dùng force kill.
3. Khi force kill gọi `agent.kill` → dùng **SIGTERM** thay vì **SIGKILL** như HLD chỉ định cho force path.

## Cách fix đề xuất

Thêm case `'agent.sendInput'` trong `agent-rpc-dispatch.ts`:
```typescript
case 'agent.sendInput': {
  const { handleAgentSendInput } = await import('./agent-spawner')
  return (await handleAgentSendInput(rpc.id, rpc.params ?? {}, config, log)) as JsonRpcResponse
}
```

Và implement `handleAgentSendInput` trong `agent-spawner.ts`:
```typescript
export async function handleAgentSendInput(
  id: string | number | null,
  params: Record<string, unknown>,
  _config: AgentConfig,
  log: AgentLogger,
): Promise<object> {
  const ptyId = typeof params.ptyId === 'string' ? params.ptyId : ''
  const data  = typeof params.data  === 'string' ? params.data  : ''
  const entry = PTY_REGISTRY.get(ptyId)
  if (!entry) {
    return { jsonrpc: '2.0', id, error: { code: -32004, message: 'PTY not found' } }
  }
  entry.pty.write(data)
  log.info(`agent.sendInput: ptyId=${ptyId} bytes=${data.length}`)
  return { jsonrpc: '2.0', id, result: { ok: true } }
}
```

## Liên quan đến luồng

- **BL-AG-02**: Stop Agent — graceful path bị thiếu hoàn toàn.
- **BL-AG-04**: Switch Account — BL-AG-02 được gọi nội bộ, nên cũng bị ảnh hưởng.

---

## ✅ Fix Status: RESOLVED (2026-08-01)

**Fix:** agent-rpc-dispatch.ts case 'agent.sendInput' added. Reads ptyId, looks up PTY_REGISTRY, calls pty.write(input).
