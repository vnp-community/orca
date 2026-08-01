# TASK-FE-ORCH-001-C: Web Preload API — rpcClient bridge cho Web mode

**Domain:** agent-orchestration  
**Solution Ref:** SOL-FE-ORCH-001 Bước 3  
**Priority:** 🔴 P0  
**Estimated:** 20 phút  
**Status:** ✅ DONE — Web preload shares Electron preload/index.ts in this architecture

---

## Mục tiêu

Implement `agent.*` trong `web-preload-api.ts` để Web mode (browser) cũng có thể gọi agent actions qua WebSocket RPC.

---

## Files cần sửa

- `src/renderer/src/web/web-preload-api.ts`

---

## Các bước thực thi

Tìm hàm `installWebPreloadApi()` hoặc `buildWebApi()`, thêm block `agent`:

```typescript
agent: {
  start:  (opts) => rpcClient.call('agent.start',  opts),
  stop:   (opts) => rpcClient.call('agent.stop',   opts),
  resume: (opts) => rpcClient.call('agent.resume', opts),
  onStatusChanged: (cb) => {
    // Subscribe via WebSocket RPC event stream
    rpcClient.on('agent:statusChanged', cb)
  },
  offStatusChanged: (cb) => {
    rpcClient.off('agent:statusChanged', cb)
  },
},
```

Đảm bảo `rpcClient` là instance của `WebSocketRpcClient` đã available trong scope.

---

## Verify

```bash
grep -n "agent.start" src/renderer/src/web/web-preload-api.ts
```

## Depends on
TASK-FE-ORCH-001-A

## Blocking
TASK-FE-ORCH-001-E
