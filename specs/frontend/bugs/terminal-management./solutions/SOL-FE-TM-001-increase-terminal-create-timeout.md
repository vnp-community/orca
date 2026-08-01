# SOL-FE-TM-001: Tăng RPC timeout từ 15s → 60s cho `terminal.create`

## Bug Reference
- **Bug:** BUG-FE-TM-001
- **Mức độ:** MEDIUM
- **File:** `src/renderer/src/components/terminal-pane/remote-runtime-pty-transport.ts` Lines 247-255
- **TDD Reference:** TDD-FE-04 §3 PTY Transport Layer; TDD-FE-03 Runtime Client Layer

---

## Root Cause

```typescript
// Lines 247-254 — timeout quá ngắn
async function callRuntime<TResult>(method: string, params?: unknown): Promise<TResult> {
  const response = await window.api.runtimeEnvironments.call({
    selector: currentRuntimeEnvironmentId,
    method,
    params,
    timeoutMs: 15_000  // ← 15s — quá ngắn cho cold start (lên tới 30-60s)
  })
  return unwrapRuntimeRpcResult(response as RuntimeRpcResponse<TResult>)
}
```

Cold start flow có thể mất 30-60s:
- `SessionManager.getOrSpawnUserProcess()`: tối đa 30s
- `relay:agentCall` (`pty.spawn`): tối đa 30s
- Reconnect queue: 20s

---

## Giải pháp

### Option A (Recommended): Method-specific timeout

```typescript
// src/renderer/src/components/terminal-pane/remote-runtime-pty-transport.ts
// Lines 247-255 — REPLACE callRuntime function

// Timeout constants — align với HLD timeouts
const TERMINAL_CREATE_TIMEOUT_MS = 60_000  // cold start: up to 30s spawn + buffer
const TERMINAL_SUBSCRIBE_TIMEOUT_MS = 60_000
const DEFAULT_RUNTIME_TIMEOUT_MS = 15_000   // fast operations unchanged

// Methods cần long timeout (cold start):
const LONG_TIMEOUT_METHODS = new Set([
  'terminal.create',
  'terminal.subscribe',
  'terminal.attach',
])

async function callRuntime<TResult>(method: string, params?: unknown): Promise<TResult> {
  const timeoutMs = LONG_TIMEOUT_METHODS.has(method)
    ? TERMINAL_CREATE_TIMEOUT_MS
    : DEFAULT_RUNTIME_TIMEOUT_MS

  const response = await window.api.runtimeEnvironments.call({
    selector: currentRuntimeEnvironmentId,
    method,
    params,
    timeoutMs,
  })
  return unwrapRuntimeRpcResult(response as RuntimeRpcResponse<TResult>)
}
```

### Option B (Better UX): Retry với exponential backoff + loading state

```typescript
// Thêm loading indicator trong TerminalPane khi đang cold start:

// src/renderer/src/components/terminal-pane/remote-runtime-pty-transport.ts
async function callRuntimeWithRetry<TResult>(
  method: string,
  params?: unknown,
  opts: { maxRetries?: number; baseTimeoutMs?: number } = {}
): Promise<TResult> {
  const { maxRetries = 2, baseTimeoutMs = 30_000 } = opts

  for (let attempt = 0; attempt <= maxRetries; attempt++) {
    try {
      const response = await window.api.runtimeEnvironments.call({
        selector: currentRuntimeEnvironmentId,
        method,
        params,
        timeoutMs: baseTimeoutMs * (attempt + 1),  // 30s, 60s, 90s
      })
      return unwrapRuntimeRpcResult(response as RuntimeRpcResponse<TResult>)
    } catch (err: any) {
      if (attempt === maxRetries || err.code !== 'TIMEOUT') throw err
      // Notify UI about retry (cold start)
      storedCallbacks.onColdStartRetry?.(attempt + 1, maxRetries)
      await sleep(1000)  // 1s delay giữa các retry
    }
  }
  throw new Error('Max retries exceeded')
}

function sleep(ms: number) { return new Promise(r => setTimeout(r, ms)) }
```

#### Loading state trong `connect()`:

```typescript
// Trong connect(), thêm cold start indicator:
async function connect(worktreeId: string, options: ConnectOptions) {
  // ... existing setup ...

  // Show cold start loading nếu timeout > 15s
  const timeoutMs = TERMINAL_CREATE_TIMEOUT_MS
  if (timeoutMs > 15_000) {
    storedCallbacks.onColdStartBegin?.()  // UI có thể show spinner với message
  }

  try {
    const created = await callRuntime<{ terminal: RuntimeTerminalCreate }>(
      'terminal.create',
      { /* params */ },
    )
    storedCallbacks.onColdStartComplete?.()
    // ... rest of connect ...
  } catch (err) {
    storedCallbacks.onColdStartFailed?.(err)
    throw err
  }
}
```

---

## Minimal Fix (nếu không implement Option B)

**Chỉ thay đổi 1 dòng:**

```typescript
// Line 252 — CHANGE:
//   timeoutMs: 15_000
// TO:
    timeoutMs: method === 'terminal.create' || method === 'terminal.subscribe'
      ? 60_000
      : 15_000,
```

**Diff:**
```diff
  async function callRuntime<TResult>(method: string, params?: unknown): Promise<TResult> {
    const response = await window.api.runtimeEnvironments.call({
      selector: currentRuntimeEnvironmentId,
      method,
      params,
-     timeoutMs: 15_000
+     timeoutMs: method === 'terminal.create' || method === 'terminal.subscribe'
+       ? 60_000   // cold start can take 30-60s
+       : 15_000,  // other methods unchanged
    })
    return unwrapRuntimeRpcResult(response as RuntimeRpcResponse<TResult>)
  }
```

---

## Files cần sửa

| File | Lines | Change |
|------|-------|--------|
| `src/renderer/src/components/terminal-pane/remote-runtime-pty-transport.ts` | 247-254 | Tăng timeout cho `terminal.create` |

---

## Verification

```bash
# 1. Grep confirm change:
grep -n "timeoutMs" src/renderer/src/components/terminal-pane/remote-runtime-pty-transport.ts

# 2. Test cold start scenario:
# - Disconnect dev server
# - Open new terminal
# - Verify: không timeout trong 60s
# - Verify: terminal appears after dev server reconnects
```

---

## Liên quan

- **BL-TM-01**: Browser bước `callRuntime('terminal.create')` ✅ fixed
- **TDD-FE-04**: §3.2 PTY Transport Factory — `callRuntimeRpc()` timeout
- **BUG-TM-003**: Orphan PTY issue (liên quan nhưng fix riêng)
