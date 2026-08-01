# SOL-FE-TM-001B: Cold Start Loading State UI cho `terminal.create`

## Bug Reference
- **Bug:** BUG-FE-TM-001 (Supplement — Loading State)
- **File:** `src/renderer/src/components/terminal-pane/TerminalPane.tsx`
- **Depends on:** SOL-FE-TM-001 (timeout tăng lên 60s)
- **TDD Reference:** TDD-FE-04 §2 xterm.js Integration, §3.2 PTY Transport Factory

---

## Vấn đề

SOL-FE-TM-001 giải quyết timeout nhưng chưa có **UI feedback** khi cold start. User sẽ thấy terminal trống trong 60s mà không biết chuyện gì đang xảy ra — UX tệ.

---

## Giải pháp: Terminal Loading Overlay

### Phần 1: `TerminalLoadingOverlay` Component

**File:** `src/renderer/src/components/terminal-pane/terminal-loading-overlay.tsx` (TẠO MỚI)

```typescript
// Terminal loading overlay — hiện trong cold start phase
// Show khi terminal.create đang pending > 2s

import { useEffect, useState } from 'react'
import { Loader2, ServerCrash } from 'lucide-react'

type LoadingPhase =
  | 'connecting'      // < 5s: "Connecting..."
  | 'spawning'        // 5-20s: "Starting process..."
  | 'cold-start'      // > 20s: "Cold start, please wait..."
  | 'retrying'        // retry attempt
  | 'error'           // failed

interface TerminalLoadingOverlayProps {
  isVisible: boolean
  elapsedMs: number
  retryAttempt?: number
  maxRetries?: number
  errorMessage?: string
  onRetry?: () => void
}

function getPhase(elapsedMs: number, retryAttempt: number): LoadingPhase {
  if (retryAttempt > 0) return 'retrying'
  if (elapsedMs < 5_000) return 'connecting'
  if (elapsedMs < 20_000) return 'spawning'
  return 'cold-start'
}

const PHASE_MESSAGES: Record<LoadingPhase, { title: string; subtitle: string }> = {
  connecting:  { title: 'Connecting...', subtitle: 'Establishing connection to dev server' },
  spawning:    { title: 'Starting terminal...', subtitle: 'Spawning PTY process on remote server' },
  'cold-start':{ title: 'Cold start detected', subtitle: 'First launch may take up to 60s. Please wait...' },
  retrying:    { title: 'Retrying...', subtitle: 'Connection attempt {attempt} of {max}' },
  error:       { title: 'Connection failed', subtitle: '' },
}

export function TerminalLoadingOverlay({
  isVisible,
  elapsedMs,
  retryAttempt = 0,
  maxRetries = 2,
  errorMessage,
  onRetry,
}: TerminalLoadingOverlayProps) {
  if (!isVisible) return null

  const phase = errorMessage ? 'error' : getPhase(elapsedMs, retryAttempt)
  const { title, subtitle } = PHASE_MESSAGES[phase]

  const resolvedSubtitle = subtitle
    .replace('{attempt}', String(retryAttempt))
    .replace('{max}', String(maxRetries))

  // Progress bar: 60s cold start
  const progressPct = Math.min((elapsedMs / 60_000) * 100, 99)

  return (
    <div className="absolute inset-0 z-10 flex flex-col items-center justify-center bg-black/80 backdrop-blur-sm">
      <div className="flex flex-col items-center gap-4 text-center max-w-xs">
        {phase === 'error' ? (
          <ServerCrash size={32} className="text-destructive" />
        ) : (
          <Loader2 size={32} className="text-primary animate-spin" />
        )}

        <div>
          <p className="text-sm font-medium text-white">{title}</p>
          <p className="text-xs text-white/60 mt-1">
            {phase === 'error' ? errorMessage : resolvedSubtitle}
          </p>
        </div>

        {phase !== 'error' && (
          <div className="w-48 h-1 bg-white/20 rounded-full overflow-hidden">
            <div
              className="h-full bg-primary rounded-full transition-all duration-500"
              style={{ width: `${progressPct}%` }}
            />
          </div>
        )}

        {(phase === 'error' || phase === 'cold-start') && onRetry && (
          <button
            className="text-xs text-primary hover:underline"
            onClick={onRetry}
          >
            Retry connection
          </button>
        )}
      </div>
    </div>
  )
}
```

---

### Phần 2: Integration trong TerminalPane

**File:** `src/renderer/src/components/terminal-pane/TerminalPane.tsx` (MODIFY)

```typescript
// Thêm state management cho cold start
import { TerminalLoadingOverlay } from './terminal-loading-overlay'

// Trong TerminalPane component:
const [isConnecting, setIsConnecting] = useState(false)
const [connectingStartTime, setConnectingStartTime] = useState<number | null>(null)
const [elapsedMs, setElapsedMs] = useState(0)
const [retryAttempt, setRetryAttempt] = useState(0)
const [connectError, setConnectError] = useState<string | null>(null)

// Tick timer khi đang connecting
useEffect(() => {
  if (!isConnecting || !connectingStartTime) return
  const interval = setInterval(() => {
    setElapsedMs(Date.now() - connectingStartTime)
  }, 500)
  return () => clearInterval(interval)
}, [isConnecting, connectingStartTime])

// Trong createTransport() hoặc useEffect connect:
const transport = createRemoteRuntimePtyTransport({
  worktreeId,

  // === Cold start callbacks (BUG-FE-TM-001) ===
  onColdStartBegin: () => {
    setIsConnecting(true)
    setConnectingStartTime(Date.now())
    setElapsedMs(0)
    setConnectError(null)
    setRetryAttempt(0)
  },
  onColdStartRetry: (attempt: number) => {
    setRetryAttempt(attempt)
    setElapsedMs(0)
    setConnectingStartTime(Date.now())
  },
  onColdStartComplete: () => {
    setIsConnecting(false)
    setConnectingStartTime(null)
    setElapsedMs(0)
    setConnectError(null)
    setRetryAttempt(0)
  },
  onColdStartFailed: (err: Error) => {
    setIsConnecting(false)
    setConnectError(err.message)
  },
  // ... other options
})

// Trong JSX:
return (
  <div className="terminal-pane relative" ref={paneRef}>
    {/* Loading overlay (GAP-3 fix) */}
    <TerminalLoadingOverlay
      isVisible={isConnecting || !!connectError}
      elapsedMs={elapsedMs}
      retryAttempt={retryAttempt}
      errorMessage={connectError ?? undefined}
      onRetry={() => {
        setConnectError(null)
        transport.reconnect?.()
      }}
    />
    {/* xterm.js container — luôn mount để không lose xterm state */}
    <div ref={terminalContainerRef} className="terminal-container h-full w-full" />
  </div>
)
```

---

### Phần 3: Transport callbacks trong `remote-runtime-pty-transport.ts`

Đây là bridge giữa transport và UI callbacks (phần này bổ sung cho SOL-FE-TM-001):

```typescript
// Thêm vào RemoteRuntimePtyTransportCallbacks interface:
interface RemoteRuntimePtyTransportCallbacks {
  // ... existing callbacks ...
  onColdStartBegin?: () => void
  onColdStartRetry?: (attempt: number, maxRetries: number) => void
  onColdStartComplete?: () => void
  onColdStartFailed?: (err: Error) => void
}

// Trong callRuntimeWithRetry():
async function callRuntimeWithRetry<TResult>(
  method: string,
  params?: unknown,
  opts: { maxRetries?: number; baseTimeoutMs?: number } = {}
): Promise<TResult> {
  const { maxRetries = 2, baseTimeoutMs = 30_000 } = opts

  // Trigger onColdStartBegin khi method cần long timeout
  if (LONG_TIMEOUT_METHODS.has(method)) {
    storedCallbacks.onColdStartBegin?.()
  }

  for (let attempt = 0; attempt <= maxRetries; attempt++) {
    try {
      const response = await window.api.runtimeEnvironments.call({
        selector: currentRuntimeEnvironmentId,
        method,
        params,
        timeoutMs: baseTimeoutMs * (attempt + 1),
      })
      storedCallbacks.onColdStartComplete?.()
      return unwrapRuntimeRpcResult(response as RuntimeRpcResponse<TResult>)
    } catch (err: any) {
      if (attempt === maxRetries || err.code !== 'TIMEOUT') {
        storedCallbacks.onColdStartFailed?.(err)
        throw err
      }
      storedCallbacks.onColdStartRetry?.(attempt + 1, maxRetries)
      await sleep(1000)
    }
  }
  throw new Error('Max retries exceeded')
}
```

---

## Files cần tạo/sửa

| File | Action | Change |
|------|--------|--------|
| `src/renderer/src/components/terminal-pane/terminal-loading-overlay.tsx` | CREATE | Loading overlay UI |
| `src/renderer/src/components/terminal-pane/TerminalPane.tsx` | MODIFY | Thêm state + `<TerminalLoadingOverlay />` |
| `src/renderer/src/components/terminal-pane/remote-runtime-pty-transport.ts` | MODIFY | Thêm `onColdStart*` callbacks |

---

## Liên quan

- **SOL-FE-TM-001**: Timeout fix (prerequisite)
- **BL-TM-01**: Connect phase UX ✅ complete với loading overlay
- **TDD-FE-04**: §2 xterm.js Integration, §3.2 PTY Transport callbacks
