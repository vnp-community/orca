# TASK-V5-22: RPC Streaming Extension (Full Implementation)

**Order:** 22 (can be parallel with TASK-V5-12+) | **Tests:** 0 (integration)

---

## Mô tả

Complete implementation của `callRuntimeRpcStream()` cho cả Desktop (Electron IPC) và Web (HTTP chunked/EventSource). Stub đã tạo ở TASK-V5-11.

---

## File Cần Sửa (Full Replacement)

### `src/renderer/src/runtime/runtime-rpc-stream.ts`

```typescript
/**
 * callRuntimeRpcStream — streaming RPC for git push/pull, workflow step output
 * 
 * Desktop (Electron): uses window.api.callStream() → IPC ReadableStream
 * Web mode:           uses EventSource or chunked HTTP
 */

const ORCA_PLATFORM = (window as any).__orca_platform ?? 'web'

export async function* callRuntimeRpcStream(
  method: string,
  params: unknown
): AsyncGenerator<string> {
  if (ORCA_PLATFORM === 'electron' && (window as any).api?.callStream) {
    // --- Desktop: IPC streaming ---
    yield* electronStream(method, params)
  } else {
    // --- Web: HTTP chunked streaming ---
    yield* webStream(method, params)
  }
}

async function* electronStream(method: string, params: unknown): AsyncGenerator<string> {
  const stream = await (window as any).api.callStream(method, params) as ReadableStream<string>
  const reader = stream.getReader()
  try {
    while (true) {
      const { done, value } = await reader.read()
      if (done) break
      if (value) yield value
    }
  } finally {
    reader.releaseLock()
  }
}

async function* webStream(method: string, params: unknown): AsyncGenerator<string> {
  // Use EventSource for streaming (server-sent events)
  const sessionToken = getSessionToken()
  const url = `/api/rpc/stream`
  
  const response = await fetch(url, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'Authorization': `Bearer ${sessionToken}`,
      'Accept': 'text/event-stream',
    },
    body: JSON.stringify({ method, params }),
  })

  if (!response.ok) {
    throw new Error(`RPC stream failed: ${response.status} ${response.statusText}`)
  }

  const reader  = response.body!.getReader()
  const decoder = new TextDecoder()
  let buffer    = ''

  try {
    while (true) {
      const { done, value } = await reader.read()
      if (done) break
      buffer += decoder.decode(value, { stream: true })

      // Parse SSE format: "data: <line>\n\n"
      const parts = buffer.split('\n\n')
      buffer = parts.pop() ?? ''

      for (const part of parts) {
        const line = part.startsWith('data: ') ? part.slice(6) : part
        if (line === '[DONE]') return
        if (line.trim()) yield line
      }
    }
  } finally {
    reader.releaseLock()
  }
}

function getSessionToken(): string {
  // Read from auth store
  try {
    const stored = sessionStorage.getItem('orca_session_token')
    if (stored) return stored
  } catch {}
  return ''
}
```

---

## Backend IPC Handler (Reference — main process)

```typescript
// src/main/ipc/rpc-stream-handler.ts (to be implemented by backend)
// Returns a ReadableStream<string> for each streaming method:
// - 'git.push'          → git push output lines
// - 'git.pull'          → git pull output lines
// - 'workflow.execute'  → handled via 'workflow:stepOutput' IPC events
```

---

## Acceptance Criteria

- [x] Electron mode: reads from `window.api.callStream()` ReadableStream
- [x] Web mode: fetches chunked HTTP, parses SSE `data: <line>` format
- [x] `[DONE]` sentinel terminates the stream
- [x] Works with `useGit.push()` (TASK-V5-11) and `useWorkflowExecution` (TASK-V5-20)
- [x] Error response → throws Error with status code
