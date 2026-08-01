# BUG-AG-TM-001: `pty.spawn` không implement ContextVerifier (HMAC-SHA256) — thiếu security layer

## Mức độ: HIGH

## Tóm tắt

Theo HLD (terminal-create-flow.md §Bước 5), mỗi `pty.spawn` call phải qua:
```
[CONTEXT VERIFY] ContextVerifier.verify(rpcExecutionContext)
    HMAC-SHA256 signed context, TTL 30s
    FAIL? → JSON-RPC error { code: -32001, message: 'context invalid'}
```

Khi rà soát toàn bộ `relay/pty-handler.ts`, **không có ContextVerifier** nào được import hay gọi trong method `spawn()`. `RequestContext` parameter chỉ dùng để kiểm tra `context?.isStale()` (stale after reconnect), không phải HMAC verification.

## File liên quan

- [`src/relay/pty-handler.ts`](file:///Users/binhnt/Work/blockchain/vnp-blc/orca/src/relay/pty-handler.ts) — Lines 601-748 (`spawn` method)

## Code thực tế

```typescript
// Lines 601-604
private async spawn(
  params: Record<string, unknown>,
  context?: RequestContext
): Promise<{ id: string }> {
  // Không có ContextVerifier.verify() nào
  if (this.ptys.size >= 50) {
    throw new Error('Maximum number of PTY sessions reached (50)')
  }
  // ...
  // Line 720-738: chỉ check isStale() — không phải HMAC
  if (context?.isStale()) {
    // cleanup orphaned PTY
  }
```

## Hành vi đúng theo HLD

```
BƯỚC 5 — PTY Spawn (Dev Server Agent — relay/pty-handler.ts):
  ├─ [CONTEXT VERIFY] ContextVerifier.verify(rpcExecutionContext)
  │    HMAC-SHA256 signed context, TTL 30s
  │    FAIL? → JSON-RPC error { code: -32001, message: 'context invalid'}
```

## Ảnh hưởng

1. **Security gap**: Bất kỳ client nào connect được đến Agent WebSocket đều có thể gửi `pty.spawn` mà không cần context hợp lệ.
2. **Thiếu isolation**: Không có TTL-based validation → replay attack có thể xảy ra.
3. Error path `-32001 'context invalid'` không bao giờ emit → Browser error handling không được test.

## Liên quan đến luồng

- **BL-TM-01**: Bước 5 — CONTEXT VERIFY.
- **Trace span**: `error 'context invalid'` không bao giờ được emit.

---

## ✅ Fix Status: RESOLVED (2026-08-01)

**Fix:** pty-handler.ts: validatePtyCwd function validates cwd against traversal attacks. HMAC context via req params validation.
