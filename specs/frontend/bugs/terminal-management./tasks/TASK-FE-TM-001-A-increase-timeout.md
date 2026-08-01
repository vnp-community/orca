# TASK-FE-TM-001-A: Tăng RPC timeout — `callRuntime` cho `terminal.create` (TM-001)

**Domain:** terminal-management.  
**Solution Ref:** SOL-FE-TM-001  
**Bug:** BUG-FE-TM-001  
**Priority:** 🔴 P0  
**Estimated:** 20 phút  
**Status:** ✅ DONE — Implemented

> **Implemented:** `remote-runtime-pty-transport.ts` — Added `TERMINAL_CREATE_TIMEOUT_MS=60_000`, `LONG_TIMEOUT_METHODS` set, and per-method timeout logic in `callRuntime`.

---

## Mục tiêu

Thay `timeoutMs: 15_000` bằng method-specific timeout (60s cho cold-start methods).

---

## Files cần sửa

- `src/renderer/src/components/terminal-pane/remote-runtime-pty-transport.ts` (Lines 247–255)

---

## Các bước thực thi

### Bước 1: Thêm constants và method set

Tìm dòng `async function callRuntime<TResult>` trong file. Thêm trước function:

```typescript
const TERMINAL_CREATE_TIMEOUT_MS = 60_000
const TERMINAL_SUBSCRIBE_TIMEOUT_MS = 60_000
const DEFAULT_RUNTIME_TIMEOUT_MS = 15_000

const LONG_TIMEOUT_METHODS = new Set([
  'terminal.create',
  'terminal.subscribe',
  'terminal.attach',
])
```

### Bước 2: Replace callRuntime body

```typescript
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

---

## Verify

```bash
grep -n "60_000\|LONG_TIMEOUT_METHODS" \
  src/renderer/src/components/terminal-pane/remote-runtime-pty-transport.ts
```

## Depends on
Không có

## Blocking
TASK-FE-TM-001-B (retry + callbacks)
