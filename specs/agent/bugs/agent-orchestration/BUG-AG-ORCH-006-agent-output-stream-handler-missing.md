# BUG-AG-ORCH-006: `agent.output` stream handler không tồn tại trên Orca server — PTY output bị mất

## Mức độ: 🔴 CRITICAL

## Tóm tắt

HLD (BL-AG-01, BL-AG-05) mô tả:
```
[Dev Server] → JSON-RPC event: agent.output { ptyId, data: "<OSC output>" } [qua WS]
    ↓
[Orca Server — AgentHookParser]
    Parse OSC 133 sequences → status detection
    INSERT orca_sessions { id, worktreeId, agentType, devServerId, startedAt }
    emit: agent:started
```

Thực tế implementation trong Dev Server (`agent-spawner.ts:163-165`):
```typescript
// agent-spawner.ts:163
pty.onData((data) => {
  const frame = JSON.stringify({
    jsonrpc: '2.0',
    id,          // ← gửi với id của request KHÔNG PHẢI notification!
    result: { type: 'spawn.output', ptyId, data }
  })
  ws.send(encodeDataFrame(wireState, frame))
})
```

Vấn đề 1: `spawn.output` được gửi như **response** (có `id`) thay vì **notification** (không có `id`). Điều này vi phạm JSON-RPC 2.0 — một response chỉ có thể gửi một lần cho mỗi request id.

Vấn đề 2: Trên Orca server, `DevServerRelayBridge.callWithTimeout()` sử dụng `SshChannelMultiplexer.request()` — mechanism này **chỉ nhận một response** cho mỗi request id, không hỗ trợ streaming responses.

Vấn đề 3: Grep toàn bộ `src/main`:
```
spawn.output  → No results
agent.output  → No results (trừ amp/hook-service.ts không liên quan)
AgentHookParser → No results
```

**Orca server không có handler nào nhận và xử lý output stream từ Dev Server.**

## Luồng bị broken

```
Dev Server PTY output
    → ws.send(encodeDataFrame({ result: { type: 'spawn.output', ptyId, data } }))
    ← Orca SshChannelMultiplexer.request() đã timeout hoặc đã resolve lần đầu
    ← Các frame output tiếp theo: BỊ IGNORE hoặc THROW "Unexpected response"
```

## Ảnh hưởng

1. **BL-AG-05**: Agent output stream không bao giờ đến Orca server → terminal không hiển thị output
2. **BL-AG-01**: `agent:started` event không bao giờ được emit
3. Rate limit detection (BL-AG-04) không hoạt động vì không có output stream
4. OSC 133 parsing (BL-AG-05) không thể xảy ra nếu data không đến

## Root Cause

`agent.spawn` được thiết kế là **fire-and-forget** với streaming responses, nhưng:
- Dev Server gửi multiple responses với cùng `id` (vi phạm JSON-RPC)
- Orca server không có streaming result handler
- Cần implement **Server-Sent Events hoặc WebSocket notification** pattern

## Cách fix đề xuất

**Option A**: Dùng JSON-RPC Notifications (không có `id`):
```typescript
// Dev Server: gửi notification thay response
pty.onData((data) => {
  const notification = JSON.stringify({
    jsonrpc: '2.0',
    method: 'agent.output',   // ← notification có method, không có id
    params: { ptyId, data }
  })
  ws.send(encodeDataFrame(wireState, notification))
})
```

**Option B**: Dùng separate WebSocket channel với multiplexed streams.

## Liên quan đến luồng

- **BL-AG-01**: Agent Start — output stream bị mất
- **BL-AG-05**: Status Monitor — không có data để parse

---

## ✅ Fix Status: RESOLVED (2026-08-01)

**Fix:** pty.onData sends JSON-RPC notification {method: 'agent.output'} (no id). pty.onExit sends {method: 'agent.exited'}. Compliant with JSON-RPC 2.0.
