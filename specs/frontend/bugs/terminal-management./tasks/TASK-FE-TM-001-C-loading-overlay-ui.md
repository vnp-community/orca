# TASK-FE-TM-001-C: `TerminalLoadingOverlay` component + TerminalPane integration (TM-001)

**Domain:** terminal-management.  
**Solution Ref:** SOL-FE-TM-001B  
**Bug:** BUG-FE-TM-001  
**Priority:** 🟠 P1  
**Estimated:** 45 phút  
**Status:** ✅ DONE — Implemented

---

## Mục tiêu

Tạo `TerminalLoadingOverlay` component với progress bar và phase-based messages. Tích hợp vào `TerminalPane` để hiện khi cold start.

---

## Files cần tạo/sửa

- **TẠO MỚI:** `src/renderer/src/components/terminal-pane/terminal-loading-overlay.tsx`
- **MODIFY:** `src/renderer/src/components/terminal-pane/TerminalPane.tsx`

---

## Bước 1: `terminal-loading-overlay.tsx`

Tạo component với nội dung đầy đủ từ SOL-FE-TM-001B §Phần 1:

```
Props: isVisible, elapsedMs, retryAttempt, maxRetries, errorMessage, onRetry

Phases (dựa trên elapsedMs):
  < 5s    → 'connecting'   "Connecting..."
  5-20s   → 'spawning'     "Starting terminal..."
  > 20s   → 'cold-start'   "Cold start detected, please wait..."
  retry   → 'retrying'     "Retrying... attempt X of Y"
  error   → 'error'        ServerCrash icon + errorMessage

UI:
  - dark overlay (bg-black/80 backdrop-blur-sm)
  - Loader2 spinning / ServerCrash icon
  - Progress bar (elapsedMs / 60000 * 100)
  - "Retry connection" link khi error hoặc cold-start
```

## Bước 2: `TerminalPane.tsx` integration

Thêm state cho cold start:
```typescript
const [isConnecting, setIsConnecting] = useState(false)
const [connectingStartTime, setConnectingStartTime] = useState<number | null>(null)
const [elapsedMs, setElapsedMs] = useState(0)
const [retryAttempt, setRetryAttempt] = useState(0)
const [connectError, setConnectError] = useState<string | null>(null)

// Timer tick
useEffect(() => {
  if (!isConnecting || !connectingStartTime) return
  const interval = setInterval(() => setElapsedMs(Date.now() - connectingStartTime), 500)
  return () => clearInterval(interval)
}, [isConnecting, connectingStartTime])
```

Truyền callbacks xuống transport factory:
```typescript
onColdStartBegin: () => { setIsConnecting(true); setConnectingStartTime(Date.now()) }
onColdStartRetry: (attempt) => { setRetryAttempt(attempt); setConnectingStartTime(Date.now()) }
onColdStartComplete: () => { setIsConnecting(false); setConnectError(null) }
onColdStartFailed: (err) => { setIsConnecting(false); setConnectError(err.message) }
```

Render overlay trong JSX:
```tsx
<div className="terminal-pane relative">
  <TerminalLoadingOverlay
    isVisible={isConnecting || !!connectError}
    elapsedMs={elapsedMs}
    retryAttempt={retryAttempt}
    errorMessage={connectError ?? undefined}
    onRetry={() => { setConnectError(null); transport.reconnect?.() }}
  />
  <div ref={terminalContainerRef} className="h-full w-full" />
</div>
```

---

## Verify

```bash
grep -n "TerminalLoadingOverlay\|isConnecting" \
  src/renderer/src/components/terminal-pane/TerminalPane.tsx

ls src/renderer/src/components/terminal-pane/terminal-loading-overlay.tsx
```

## Depends on
TASK-FE-TM-001-B (callbacks)
