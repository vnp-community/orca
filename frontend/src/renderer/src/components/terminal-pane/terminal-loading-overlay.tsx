// src/renderer/src/components/terminal-pane/terminal-loading-overlay.tsx
// TM-001-C: Cold-start loading overlay for remote terminals
// Shown during terminal.create when server is cold-starting (may take up to 60s)

import { useEffect, useState } from 'react'
import { Loader2 } from 'lucide-react'

export type ColdStartPhase = 'connecting' | 'retrying' | 'failed' | null

type TerminalLoadingOverlayProps = {
  phase: ColdStartPhase
  retryAttempt?: number
  /** Elapsed seconds since connect began (optional, animated by timer if not provided) */
  elapsedSec?: number
}

const PHASE_MESSAGES: Record<Exclude<ColdStartPhase, null>, (retry?: number) => string> = {
  connecting: () => 'Connecting to remote terminal…',
  retrying:   (r) => `Reconnecting to remote terminal${r && r > 0 ? ` (attempt ${r + 1})` : ''}…`,
  failed:     () => 'Failed to connect to remote terminal',
}

export function TerminalLoadingOverlay({
  phase,
  retryAttempt,
}: TerminalLoadingOverlayProps) {
  const [elapsed, setElapsed] = useState(0)

  // Animate elapsed time counter
  useEffect(() => {
    if (!phase || phase === 'failed') {
      setElapsed(0)
      return
    }
    setElapsed(0)
    const start = Date.now()
    const timer = setInterval(() => {
      setElapsed(Math.floor((Date.now() - start) / 1000))
    }, 1000)
    return () => clearInterval(timer)
  }, [phase])

  if (!phase) {return null}

  const message = PHASE_MESSAGES[phase](retryAttempt)
  const isFailed = phase === 'failed'

  // Progress bar fills from 0→85% over 55s (leaves room for success)
  const progressPct = isFailed ? 100 : Math.min(85, (elapsed / 55) * 85)

  return (
    <div
      className="terminal-loading-overlay absolute inset-0 z-20 flex flex-col items-center justify-center bg-background/90 backdrop-blur-sm"
      role="status"
      aria-label={message}
    >
      <div className="flex flex-col items-center gap-3 max-w-xs text-center px-4">
        {/* Spinner or error icon */}
        {!isFailed ? (
          <Loader2 size={28} className="animate-spin text-muted-foreground" />
        ) : (
          <div className="h-7 w-7 rounded-full bg-destructive/20 flex items-center justify-center text-destructive text-sm font-bold">
            ✕
          </div>
        )}

        {/* Status message */}
        <p className={`text-sm font-medium ${isFailed ? 'text-destructive' : 'text-foreground'}`}>
          {message}
        </p>

        {/* Sub-message */}
        {!isFailed && (
          <p className="text-xs text-muted-foreground">
            {elapsed > 5
              ? 'Remote server may be cold-starting. This can take up to 60 seconds.'
              : 'Establishing secure connection…'
            }
          </p>
        )}

        {/* Elapsed timer */}
        {!isFailed && elapsed > 0 && (
          <p className="text-[10px] text-muted-foreground font-mono tabular-nums">
            {elapsed}s elapsed
          </p>
        )}

        {/* Progress bar */}
        {!isFailed && (
          <div className="w-full h-0.5 bg-muted rounded-full overflow-hidden">
            <div
              className="h-full bg-primary transition-all duration-1000 ease-linear"
              style={{ width: `${progressPct}%` }}
            />
          </div>
        )}

        {isFailed && (
          <p className="text-xs text-muted-foreground">
            Check your network connection and try reopening the terminal.
          </p>
        )}
      </div>
    </div>
  )
}

/**
 * Hook to manage cold-start phase state.
 * Connects to IpcPtyTransportOptions callbacks.
 */
export function useColdStartPhase() {
  const [phase, setPhase]       = useState<ColdStartPhase>(null)
  const [retryAttempt, setRetryAttempt] = useState(0)

  const callbacks = {
    onColdStartBegin:    () => { setPhase('connecting'); setRetryAttempt(0) },
    onColdStartRetry:    (attempt: number) => { setPhase('retrying'); setRetryAttempt(attempt) },
    onColdStartComplete: () => setPhase(null),
    onColdStartFailed:   () => setPhase('failed'),
  }

  return { phase, retryAttempt, callbacks }
}
