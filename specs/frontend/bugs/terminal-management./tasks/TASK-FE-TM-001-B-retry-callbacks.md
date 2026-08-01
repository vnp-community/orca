# TASK-FE-TM-001-B: Cold start retry + onColdStart* callbacks (TM-001)

**Domain:** terminal-management.  
**Solution Ref:** SOL-FE-TM-001B §Phần 3  
**Bug:** BUG-FE-TM-001  
**Priority:** 🟠 P1  
**Estimated:** 30 phút  
**Status:** ✅ DONE — Implemented

---

## Mục tiêu

Thêm `callRuntimeWithRetry` function với retry logic (max 2 lần) và các callbacks `onColdStartBegin/Retry/Complete/Failed` để UI có thể reflect trạng thái loading.

---

## Files cần sửa

- `src/renderer/src/components/terminal-pane/remote-runtime-pty-transport.ts`

---

## Các bước thực thi

### Bước 1: Thêm callbacks vào options interface

Tìm `interface RemoteRuntimePtyTransportOptions` hoặc `RemoteRuntimePtyTransportCallbacks`, thêm:

```typescript
onColdStartBegin?:    () => void
onColdStartRetry?:    (attempt: number, maxRetries: number) => void
onColdStartComplete?: () => void
onColdStartFailed?:   (err: Error) => void
```

### Bước 2: Implement `callRuntimeWithRetry`

```typescript
async function callRuntimeWithRetry<TResult>(
  method: string,
  params?: unknown,
  opts: { maxRetries?: number } = {}
): Promise<TResult> {
  const { maxRetries = 2 } = opts

  if (LONG_TIMEOUT_METHODS.has(method)) {
    storedCallbacks.onColdStartBegin?.()
  }

  for (let attempt = 0; attempt <= maxRetries; attempt++) {
    try {
      const response = await window.api.runtimeEnvironments.call({
        selector: currentRuntimeEnvironmentId,
        method,
        params,
        timeoutMs: TERMINAL_CREATE_TIMEOUT_MS * (attempt + 1),
      })
      storedCallbacks.onColdStartComplete?.()
      return unwrapRuntimeRpcResult(response as RuntimeRpcResponse<TResult>)
    } catch (err: any) {
      if (attempt === maxRetries || err.code !== 'TIMEOUT') {
        storedCallbacks.onColdStartFailed?.(err)
        throw err
      }
      storedCallbacks.onColdStartRetry?.(attempt + 1, maxRetries)
      await sleep(1_000)
    }
  }
  throw new Error('Max retries exceeded')
}

function sleep(ms: number) { return new Promise(r => setTimeout(r, ms)) }
```

### Bước 3: Dùng `callRuntimeWithRetry` thay `callRuntime` cho long-timeout methods

Thay call `callRuntime('terminal.create', ...)` thành `callRuntimeWithRetry(...)`.

---

## Verify

```bash
grep -n "callRuntimeWithRetry\|onColdStartBegin" \
  src/renderer/src/components/terminal-pane/remote-runtime-pty-transport.ts
```

## Depends on
TASK-FE-TM-001-A

## Blocking
TASK-FE-TM-001-C (Loading UI)
