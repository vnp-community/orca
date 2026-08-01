# SOLUTION: Terminal Management Domain — Fix Bugs (relay-dispatch level)

**Domain:** terminal-management  
**TDD Reference:** TDD-AG-07 (JSON-RPC Dispatch), TDD-AG-06 (Tool Handlers)  
**Files cần thay đổi:** `src/relay/agent-rpc-dispatch.ts`, `src/relay/pty-handler.ts`  
**Tổng số bugs:** 1 (TM-001)

---

## BUG-TM-001 — Fix relay dispatch thiếu `pty.*` handlers

**Mức độ:** 🔴 CRITICAL  
**Root cause:** `agent-rpc-dispatch.ts` không có handler cho `pty.create`, `pty.destroy`, `pty.resize`, `pty.scrollback`, `pty.write`. BL-TM-01 hoàn toàn không thể hoạt động.

### Phân tích hiện trạng

```
src/relay/agent-rpc-dispatch.ts:
  case 'agent.spawn':   ✅ — spawn agent trong PTY (nhưng không phải generic PTY)
  case 'pty.create':    ❌ MISSING
  case 'pty.destroy':   ❌ MISSING
  case 'pty.resize':    ❌ MISSING
  case 'pty.scrollback':❌ MISSING
  case 'pty.write':     ❌ MISSING

src/relay/pty-handler.ts:
  spawn()   ✅ (Lines 601-748 — PTY spawn implementation exists!)
  resize()  ✅ (exists in pty-handler.ts)
  destroy() ✅ (exists in pty-handler.ts)
  write()   ✅ (exists in pty-handler.ts)
```

**Kết luận:** `pty-handler.ts` đã có implementation đầy đủ. Vấn đề là chúng **chưa được expose** qua `agent-rpc-dispatch.ts` switch cases.

### Fix: Đăng ký tất cả pty.* handlers trong dispatch

```typescript
// src/relay/agent-rpc-dispatch.ts — thêm các cases:

// ─── PTY Management Handlers (BL-TM-01) ───────────────────────────────────

case 'pty.create': {
  /**
   * Tạo PTY session mới trên Dev Server.
   * Params: { cwd, cols, rows, env?, shellOverride? }
   * Returns: { id: ptyId, cols, rows, cwd }
   */
  try {
    const result = await ptyHandlerInstance.handleRequest('spawn', rpc.params ?? {}, context)
    return makeOk(rpc.id, result)
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    return makeError(rpc.id, AgentErrorCode.ServerError, `pty.create failed: ${msg}`)
  }
}

case 'pty.write': {
  /**
   * Gửi input vào PTY stdin.
   * Params: { id: ptyId, data: string }
   * Returns: { ok: true }
   */
  try {
    const ptyId = typeof rpc.params?.id === 'string' ? rpc.params.id : ''
    const data  = typeof rpc.params?.data === 'string' ? rpc.params.data : ''
    if (!ptyId) return makeError(rpc.id, AgentErrorCode.InvalidParams, 'Missing pty id')
    await ptyHandlerInstance.handleRequest('write', { id: ptyId, data }, context)
    return makeOk(rpc.id, { ok: true })
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    return makeError(rpc.id, AgentErrorCode.ServerError, `pty.write failed: ${msg}`)
  }
}

case 'pty.resize': {
  /**
   * Resize PTY window.
   * Params: { id: ptyId, cols, rows }
   * Returns: { ok: true }
   */
  try {
    const ptyId = typeof rpc.params?.id   === 'string' ? rpc.params.id   : ''
    const cols  = typeof rpc.params?.cols === 'number' ? rpc.params.cols : 80
    const rows  = typeof rpc.params?.rows === 'number' ? rpc.params.rows : 24
    if (!ptyId) return makeError(rpc.id, AgentErrorCode.InvalidParams, 'Missing pty id')
    await ptyHandlerInstance.handleRequest('resize', { id: ptyId, cols, rows }, context)
    return makeOk(rpc.id, { ok: true })
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    return makeError(rpc.id, AgentErrorCode.ServerError, `pty.resize failed: ${msg}`)
  }
}

case 'pty.destroy': {
  /**
   * Đóng và cleanup PTY session.
   * Params: { id: ptyId }
   * Returns: { ok: true }
   */
  try {
    const ptyId = typeof rpc.params?.id === 'string' ? rpc.params.id : ''
    if (!ptyId) return makeError(rpc.id, AgentErrorCode.InvalidParams, 'Missing pty id')
    await ptyHandlerInstance.handleRequest('destroy', { id: ptyId }, context)
    return makeOk(rpc.id, { ok: true })
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    return makeError(rpc.id, AgentErrorCode.ServerError, `pty.destroy failed: ${msg}`)
  }
}

case 'pty.scrollback': {
  /**
   * Lấy scrollback buffer của PTY.
   * Params: { id: ptyId, lines?: number }
   * Returns: { data: string }
   */
  try {
    const ptyId = typeof rpc.params?.id    === 'string' ? rpc.params.id    : ''
    const lines = typeof rpc.params?.lines === 'number' ? rpc.params.lines : 100
    if (!ptyId) return makeError(rpc.id, AgentErrorCode.InvalidParams, 'Missing pty id')
    const result = await ptyHandlerInstance.handleRequest('scrollback', { id: ptyId, lines }, context)
    return makeOk(rpc.id, result)
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    return makeError(rpc.id, AgentErrorCode.ServerError, `pty.scrollback failed: ${msg}`)
  }
}
```

### Khởi tạo ptyHandlerInstance trong dispatch

```typescript
// src/relay/agent-rpc-dispatch.ts — thêm vào createRpcDispatcher():

import { PtyHandler } from './pty-handler'

export function createRpcDispatcher(
  tools:  ToolDefinition[],
  config: AgentConfig,
  log:    AgentLogger,
): RpcDispatcher {
  // Khởi tạo PtyHandler instance (shared cho tất cả sessions)
  const ptyHandlerInstance = new PtyHandler(log)

  return {
    async dispatch(
      ws:        WebSocket,
      wireState: WireState,
      rpc:       JsonRpcRequest,
      context?:  RequestContext,
    ): Promise<void> {
      const response = await route(ws, wireState, rpc, context, tools, config, log, ptyHandlerInstance)
      ws.send(encodeDataFrame(wireState, JSON.stringify(response)))
    }
  }
}
```

### PTY output streaming (notifications)

Khi `pty.create` (spawn) thành công, PTY output cần được stream về Orca:

```typescript
// pty-handler.ts — trong spawn(), sau khi pty được tạo:
// Thêm onData handler gửi notification:
term.onData((data: string) => {
  // Gửi notification thay vì response (tránh vi phạm JSON-RPC)
  const notification = JSON.stringify({
    jsonrpc: '2.0',
    method:  'pty.output',
    params:  {
      id:   ptyId,
      data: Buffer.from(data).toString('base64'),
    },
  })
  ws.send(encodeDataFrame(wireState, notification))
})
```

---

## Verification Plan

```bash
# 1. Type check:
pnpm tsc --noEmit -p config/tsconfig.node.json

# 2. Integration test (manual):
# - Gọi relay.call('pty.create', { cwd: '/home/ubuntu', cols: 120, rows: 40 })
# - Expect: { id: 'pty-xxx', cols: 120, rows: 40, cwd: '/home/ubuntu' }
# - Gọi relay.call('pty.write', { id: 'pty-xxx', data: 'ls -la\n' })
# - Verify pty.output notification received với output của 'ls -la'
# - Gọi relay.call('pty.resize', { id: 'pty-xxx', cols: 200, rows: 50 })
# - Gọi relay.call('pty.destroy', { id: 'pty-xxx' })

# 3. Unit tests:
pnpm vitest run src/relay/__tests__/pty-handler.test.ts
```

---

## Tóm tắt file changes

| File | Action | Bug |
|------|--------|-----|
| `src/relay/agent-rpc-dispatch.ts` | ADD `pty.create`, `pty.write`, `pty.resize`, `pty.destroy`, `pty.scrollback` cases | TM-001 |
| `src/relay/agent-rpc-dispatch.ts` | ADD `PtyHandler` initialization trong `createRpcDispatcher` | TM-001 |
| `src/relay/pty-handler.ts` | ADD `pty.output` notification trong `onData` callback | TM-001 |

> **Ghi chú:** `pty-handler.ts` đã có implementation đầy đủ cho spawn/write/resize/destroy.  
> Chỉ cần thêm cases trong `agent-rpc-dispatch.ts` để expose chúng qua JSON-RPC.  
> Bugs TM-001 đến TM-004 trong `terminal-management.` (với dấu chấm) cover các security/correctness issues trong pty-handler.ts implementation — xem file solutions riêng.

---

## ✅ Implementation Status (2026-08-01)

TM-001: pty-agent-bridge.ts created. pty.create/write/resize/destroy/scrollback handlers DONE.
