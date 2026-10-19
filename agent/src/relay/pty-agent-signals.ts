/**
 * pty-agent-signals.ts — pty.sendSignal and pty.listProcesses for the agent-mode PTY table.
 * Split out of pty-agent-bridge.ts so each file stays under the max-lines budget.
 */

import type { AgentLogger } from './agent-logger'
import { Tracers } from '../shared/trace/tracers'
import { getForegroundProcessName } from './pty-shell-utils'
import { AGENT_PTY_MAP, resumeFrom } from './pty-agent-state'

const ALLOWED_SIGNALS = new Set(['SIGTERM', 'SIGKILL', 'SIGINT', 'SIGHUP', 'SIGTSTP'])

/**
 * handlePtySendSignal — Send a POSIX signal to PTY process.
 * Params: { id, signal }
 */
export async function handlePtySendSignal(
  id: string | number | null,
  params: Record<string, unknown>,
  log: AgentLogger
): Promise<object> {
  const ptyId = typeof params.id === 'string' ? params.id : ''
  const signal = typeof params.signal === 'string' ? params.signal : 'SIGTERM'
  // Only SIGKILL/SIGTERM end the process — those are the ones worth tracing
  // as a terminal:destroy event; SIGINT/SIGHUP/SIGTSTP are routine in-session
  // signals (Ctrl+C, etc.) and would be pure per-keystroke noise (CR-TRACE-000 §5).
  const isTerminating = signal === 'SIGKILL' || signal === 'SIGTERM'
  const span = isTerminating
    ? Tracers.terminalDestroy.start(
        { origin: 'agent-pty', ptyId, signal, via: 'pty.sendSignal' },
        resumeFrom(params)
      )
    : undefined

  if (!ptyId) {
    span?.fail('missing id')
    return { jsonrpc: '2.0', id, error: { code: -32602, message: 'pty.sendSignal: missing id' } }
  }
  if (!ALLOWED_SIGNALS.has(signal)) {
    span?.fail(`signal not allowed: ${signal}`)
    return { jsonrpc: '2.0', id, error: { code: -32602, message: `Signal not allowed: ${signal}` } }
  }

  const entry = AGENT_PTY_MAP.get(ptyId)
  if (!entry) {
    span?.fail('pty not found', { ptyId })
    return { jsonrpc: '2.0', id, error: { code: -32603, message: `PTY not found: ${ptyId}` } }
  }

  try {
    if (process.platform !== 'win32') {
      entry.pty.kill(signal)
    } else {
      entry.pty.kill()
    }
    log.info(`pty.sendSignal (agent): id=${ptyId} signal=${signal}`)
    span?.ok({ ptyId, signal })
    return { jsonrpc: '2.0', id, result: { ok: true } }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    span?.fail(err, { ptyId })
    return { jsonrpc: '2.0', id, error: { code: -32603, message: `pty.sendSignal failed: ${msg}` } }
  }
}

/**
 * handlePtyListProcesses — Enumerate every PTY this daemon currently tracks.
 *
 * Why this exists: DevServerPtyProvider.listProcesses() (backend) used to
 * always return [] with a "no agent-wide PTY enumeration RPC exists yet"
 * comment — mirrors SshPtyProvider.listProcesses(), which has always called
 * this same-named RPC against the SSH relay daemon (pty-handler.ts). Without
 * it, the backend's liveness sweep (refreshPtyWorktreeRecordsFromController)
 * could never learn a Dev-Server-hosted PTY died on its own: a dead ptyId
 * stayed marked "ready" in session-tabs bookkeeping until the tab/leaf was
 * closed outright, so every reattach attempt — fresh spawn included, since
 * mirrored web tabs always ask "what handle is ready for this tab" — kept
 * being handed back the same dead id (BUG-FE-PTY-001).
 */
export async function handlePtyListProcesses(
  id: string | number | null,
  _params: Record<string, unknown>,
  _log: AgentLogger
): Promise<object> {
  const results: { id: string; cwd: string; title: string; pid: number }[] = []
  for (const [ptyId, entry] of AGENT_PTY_MAP) {
    const title =
      (await getForegroundProcessName(entry.pty.pid, entry.pty.process || null)) || 'shell'
    // pid is the PTY's own (shell) process id, not the foreground child's —
    // added so backend-go's InspectProcess can report a real pid instead of
    // always 0 (see infra-fleet-service's devserveragent/methods.go).
    results.push({ id: ptyId, cwd: entry.cwd, title, pid: entry.pty.pid })
  }
  return { jsonrpc: '2.0', id, result: results }
}
