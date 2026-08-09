import { useCallback, useEffect, useRef, useState } from 'react'
import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import { Loader2, Terminal as TerminalIcon, X, CheckCircle2 } from 'lucide-react'
import { Button } from '../../components/ui/button'
import { getRemoteRuntimeTerminalMultiplexer } from '../../runtime/remote-runtime-terminal-multiplexer'
import {
  getRemoteRuntimeTerminalHandle,
  getRemoteRuntimePtyEnvironmentId,
} from '../../runtime/runtime-terminal-stream'

// Why: Catppuccin Mocha-inspired dark palette — consistent with Orca's default
// terminal theme. Does not depend on the user's terminal appearance preference
// because this is a transient auth panel, not a full terminal pane.
const PTY_THEME = {
  background: '#1e1e2e',
  foreground: '#cdd6f4',
  cursor: '#f5e0dc',
  selectionBackground: '#363659',
} as const

// Why: In Web Server mode, users cannot run `gh auth login` / `glab auth login`
// locally. This component spawns the CLI as a PTY on the Dev Server relay
// and shows an inline xterm.js terminal for the interactive auth flow.
// After the PTY exits, onComplete() is called to trigger a preflight re-check.
// (FE-SOL-02 Phase 2 — CR-GH-002, CR-INT-001)

type Provider = 'github' | 'gitlab'

type WebModeCliAuthSectionProps = {
  provider: Provider
  devServerId: string
  onComplete: () => void
}

export function WebModeCliAuthSection({
  provider,
  devServerId,
  onComplete,
}: WebModeCliAuthSectionProps): React.JSX.Element {
  const [isLoading, setIsLoading] = useState(false)
  const [ptyInfo, setPtyInfo] = useState<{ ptyId: string; devServerId: string } | null>(null)
  const [error, setError] = useState<string | null>(null)

  const handleStartLogin = async () => {
    setIsLoading(true)
    setError(null)
    try {
      const result =
        provider === 'github'
          ? await window.api.github.startAuthLogin(devServerId)
          : await window.api.gitlab.startAuthLogin(devServerId)
      setPtyInfo(result)
    } catch (err) {
      setError(err instanceof Error ? err.message : `Failed to start ${provider} auth login`)
    } finally {
      setIsLoading(false)
    }
  }

  const handlePtyClose = useCallback(() => {
    setPtyInfo(null)
    onComplete()
  }, [onComplete])

  if (ptyInfo) {
    return (
      <WebModeInlinePty
        ptyId={ptyInfo.ptyId}
        devServerId={ptyInfo.devServerId}
        onClose={handlePtyClose}
      />
    )
  }

  return (
    <div className="flex flex-col gap-2">
      <p className="text-xs text-muted-foreground">
        {provider === 'github'
          ? 'Authenticate the GitHub CLI (gh) on your Dev Server.'
          : 'Authenticate the GitLab CLI (glab) on your Dev Server.'}
      </p>
      {error && <p className="text-xs text-destructive">{error}</p>}
      <Button
        variant="outline"
        size="sm"
        disabled={isLoading}
        onClick={handleStartLogin}
        id={`${provider}-auth-login-btn`}
      >
        {isLoading ? (
          <Loader2 className="size-3.5 mr-1.5 animate-spin" />
        ) : (
          <TerminalIcon className="size-3.5 mr-1.5" />
        )}
        {provider === 'github' ? 'Login with GitHub CLI' : 'Login with GitLab CLI'}
      </Button>
    </div>
  )
}

// ── Inline PTY terminal (xterm.js) ───────────────────────────────────────────
// Why: Full xterm.js integration (FE-SOL-02 Phase 2).
// Subscribes to the remote PTY stream from the Dev Server relay via
// getRemoteRuntimeTerminalMultiplexer, renders output in an xterm.js instance,
// and sends user keystrokes back to the PTY via stream.sendInput.

type WebModeInlinePtyProps = {
  ptyId: string
  devServerId: string
  onClose: () => void
}

// Why: import xterm CSS lazily via side-effect import so it only loads when
// the PTY terminal is actually rendered (settings panel path only).
// Using dynamic import string ensures Vite chunks it separately.
const XTERM_CSS_LOADED = { done: false }

function ensureXtermCss() {
  if (!XTERM_CSS_LOADED.done) {
    XTERM_CSS_LOADED.done = true
    import('@xterm/xterm/css/xterm.css').catch(() => {
      // Why: graceful fallback — xterm still works without CSS, just looks unstyled.
    })
  }
}

type PtyState = 'connecting' | 'connected' | 'exited' | 'error'

function WebModeInlinePty({ ptyId, devServerId, onClose }: WebModeInlinePtyProps) {
  const containerRef = useRef<HTMLDivElement>(null)
  const termRef = useRef<Terminal | null>(null)
  const fitAddonRef = useRef<FitAddon | null>(null)
  const streamRef = useRef<{
    sendInput: (text: string) => boolean
    resize: (cols: number, rows: number) => boolean
    close: () => void
  } | null>(null)
  const [ptyState, setPtyState] = useState<PtyState>('connecting')
  const [errorMsg, setErrorMsg] = useState<string | null>(null)

  // Why: Read the active terminal theme from the store so the inline PTY
  // matches the main terminal appearance (dark/light background).
  useEffect(() => {
    ensureXtermCss()

    const container = containerRef.current
    if (!container) {return}

    // ── Parse ptyId → environmentId + handle ──────────────────────────────
    // Why: ptyId returned from startAuthLogin is a remote PTY id of the form
    //   "remote:<environmentId>@@<handle>"
    // Use getRemoteRuntimeTerminalHandle / getRemoteRuntimePtyEnvironmentId
    // to decode it, matching the pattern in subscribeToRuntimeTerminalData.
    const handle = getRemoteRuntimeTerminalHandle(ptyId)
    const environmentId = getRemoteRuntimePtyEnvironmentId(ptyId) ?? devServerId

    if (!handle) {
      setPtyState('error')
      setErrorMsg(`Invalid PTY id: ${ptyId}`)
      return
    }

    // ── Create xterm.js Terminal instance ─────────────────────────────────
    const cols = Math.floor(container.clientWidth / 9) || 80
    const rows = 24

    const term = new Terminal({
      cols,
      rows,
      fontFamily: 'Menlo, Monaco, "Cascadia Code", "Courier New", monospace',
      fontSize: 13,
      lineHeight: 1.2,
      cursorBlink: true,
      allowProposedApi: true,
      theme: PTY_THEME,
    })

    const fitAddon = new FitAddon()
    term.loadAddon(fitAddon)
    term.open(container)
    fitAddon.fit()

    termRef.current = term
    fitAddonRef.current = fitAddon

    // ── Subscribe to Dev Server relay PTY stream ──────────────────────────
    let closed = false

    getRemoteRuntimeTerminalMultiplexer(environmentId)
      .subscribeTerminal({
        terminal: handle,
        client: { id: `auth-panel-${Date.now()}`, type: 'desktop' },
        callbacks: {
          onData: (data) => {
            if (!closed) {
              term.write(data)
            }
          },
          onSnapshot: (data) => {
            if (!closed) {
              term.clear()
              term.write(data)
            }
          },
          onSubscribed: () => {
            if (!closed) {
              setPtyState('connected')
            }
          },
          onEnd: () => {
            if (!closed) {
              setPtyState('exited')
            }
          },
          onError: (message) => {
            if (!closed) {
              setPtyState('error')
              setErrorMsg(message ?? 'PTY stream error')
            }
          },
        },
      })
      .then((stream) => {
        if (closed) {
          stream.close()
          return
        }
        streamRef.current = stream

        // Why: send initial cols/rows so the remote PTY resizes to fit the container
        stream.resize(cols, rows)

        // Send user keyboard input to the PTY
        term.onData((data) => {
          if (!closed) {
            stream.sendInput(data)
          }
        })
      })
      .catch((err: unknown) => {
        if (!closed) {
          setPtyState('error')
          setErrorMsg(err instanceof Error ? err.message : 'Failed to connect to PTY stream')
        }
      })

    // ── ResizeObserver — keep xterm fitted to container ──────────────────
    const resizeObserver = new ResizeObserver(() => {
      if (closed || !containerRef.current) {return}
      try {
        fitAddon.fit()
        const dims = fitAddon.proposeDimensions()
        if (dims && streamRef.current) {
          streamRef.current.resize(dims.cols, dims.rows)
        }
      } catch {
        // Why: FitAddon.fit() can throw if the container has zero dimensions
      }
    })
    resizeObserver.observe(container)

    return () => {
      closed = true
      resizeObserver.disconnect()
      streamRef.current?.close()
      streamRef.current = null
      term.dispose()
      termRef.current = null
      fitAddonRef.current = null
    }
    // Why: ptyId and devServerId are stable for the lifetime of this mount;
    // re-running the effect on terminalAppearance changes would re-create the
    // stream unnecessarily — theme is set at init time only.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [ptyId, devServerId])

  return (
    <div className="flex flex-col gap-2 rounded-md border overflow-hidden">
      {/* ── Header bar ────────────────────────────────────────────────── */}
      <div className="flex items-center justify-between px-3 py-1.5 bg-muted/60 border-b">
        <div className="flex items-center gap-1.5 text-xs text-muted-foreground font-mono">
          <TerminalIcon className="size-3" />
          <span>
            PTY — <span className="text-foreground">{devServerId}</span>
          </span>
          {ptyState === 'connecting' && (
            <Loader2 className="size-3 animate-spin ml-1 text-muted-foreground" />
          )}
          {ptyState === 'connected' && (
            <span className="ml-1 text-green-500 text-xs">● connected</span>
          )}
          {ptyState === 'exited' && (
            <span className="ml-1 text-yellow-500 text-xs">● exited</span>
          )}
          {ptyState === 'error' && (
            <span className="ml-1 text-destructive text-xs">● error</span>
          )}
        </div>
        <Button
          variant="ghost"
          size="icon"
          className="size-5"
          onClick={onClose}
          aria-label="Close terminal"
        >
          <X className="size-3" />
        </Button>
      </div>

      {/* ── xterm.js container ────────────────────────────────────────── */}
      <div
        ref={containerRef}
        className="w-full"
        style={{
          height: 280,
          backgroundColor: PTY_THEME.background,
          padding: '4px 6px',
        }}
      />

      {/* ── Error message ─────────────────────────────────────────────── */}
      {ptyState === 'error' && errorMsg && (
        <p className="px-3 pb-2 text-xs text-destructive">{errorMsg}</p>
      )}

      {/* ── Action footer ─────────────────────────────────────────────── */}
      <div className="flex items-center justify-between px-3 pb-2 pt-1">
        <p className="text-xs text-muted-foreground">
          {ptyState === 'exited'
            ? 'Authentication complete. Click Done to re-check status.'
            : 'Complete authentication in the terminal above.'}
        </p>
        <Button
          variant={ptyState === 'exited' ? 'default' : 'outline'}
          size="sm"
          onClick={onClose}
          id="pty-done-btn"
        >
          {ptyState === 'exited' ? (
            <>
              <CheckCircle2 className="size-3.5 mr-1.5" />
              Done — Re-check status
            </>
          ) : (
            'Done'
          )}
        </Button>
      </div>
    </div>
  )
}
