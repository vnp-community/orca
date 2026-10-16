/**
 * pty-agent-bridge.ts — TM-001/TM-006: PTY management, running inside the
 * detached pty-daemon process (pty-daemon-server.ts), not the agent itself.
 *
 * Why this file:
 *   PtyHandler registers its operations via RelayDispatcher.onRequest() (relay/local mode).
 *   agent-rpc-dispatch.ts uses a switch/case pattern (agent WebSocket mode).
 *   This bridge exposes PTY operations to agent-rpc-dispatch.ts without coupling to
 *   PtyHandler internals, and without creating circular dependencies.
 *
 * Lifecycle (2026-08 — reattach support, mirrors pty-handler.ts's relay design):
 *   - AGENT_PTY_MAP is module-level (singleton per process) — PTYs live here,
 *     independent of any single WebSocket connection to Orca, AND independent
 *     of the agent process itself: this module's exported handlers now run
 *     inside pty-daemon-server.ts, a separate detached process the agent
 *     spawns once and reconnects to as a client (pty-daemon-client.ts) after
 *     every agent restart — see that file for why.
 *   - A dropped Orca WebSocket does NOT kill running PTYs. The agent forwards
 *     "my WS to Orca closed" to the daemon (daemon.sessionClosed), which calls
 *     scheduleGracePeriodCleanup() instead of cleanupAgentPtys(): each live
 *     PTY gets a grace timer (PTY_GRACE_PERIOD_MS); if a new connection calls
 *     pty.attach on it before the timer fires, the timer is cancelled and the
 *     shell keeps running untouched. If the grace period elapses with no
 *     reattach, the PTY is killed for real. The agent process itself
 *     restarting does NOT trigger this at all — the daemon's client
 *     connection dropping carries no information about the user's intent.
 *   - cleanupAgentPtys() (immediate, unconditional) only runs on the DAEMON's
 *     own process shutdown (SIGTERM/SIGINT to the daemon itself) — the one
 *     event nothing can survive, since AGENT_PTY_MAP is in-memory.
 *
 * Security:
 *   - cwd is validated via validatePtyCwd (home/workDir/tmp only)
 *   - Signals are allowlisted (SIGTERM, SIGKILL, SIGINT, SIGHUP, SIGTSTP)
 *   - node-pty is loaded lazily (not available in test env without native deps)
 */

import type { AgentLogger } from './agent-logger'
import { Tracers } from '../shared/trace/tracers'
import { buildStartupCommandSubmission } from '../shared/startup-command-submission'
import { buildGhEnv, buildGlabEnv } from './external-api-connector'
import { AGENT_PTY_MAP, resumeFrom, type AgentPtyEntry } from './pty-agent-state'
import { safeCwd, usableCwd } from './pty-agent-cwd'

export { usableCwd }
export { handlePtySendSignal, handlePtyListProcesses } from './pty-agent-signals'

// Why: matches pty-handler.ts's STARTUP_COMMAND_WRITE_DELAY_MS (the SSH
// relay's non-shell-ready-gated default) — a short pause for node-pty to
// finish forking/exec before the write lands, not a real "is the shell
// prompt ready" check. Part A doesn't (yet) wire the shell-ready OSC-marker
// scanner pty-handler.ts uses for its more finicky multiline-prompt cases —
// see specs/agent/api/gaps-and-findings.md #5 for why a fixed delay was
// chosen as the right-sized fix here (closes the gh/glab auth-login bug
// without porting pty-shell-launch.ts's profile-injection machinery, which
// Part A doesn't use at all today).
const STARTUP_COMMAND_WRITE_DELAY_MS = 50

// ─── Module state ─────────────────────────────────────────────────────────────

let nextAgentPtyId = 1

const SCROLLBACK_LINES = 500
/** How long a PTY survives after its WebSocket disconnects, waiting for a
 *  reattach, before being killed for real. Fixed (not user-configurable) —
 *  a simpler v1 than the SSH relay's per-target grace period setting. Sized
 *  to comfortably cover a full agent PROCESS restart (systemd RestartSec=15
 *  + node startup + token fetch/retry + WS reconnect), not just a brief
 *  network blip — the daemon (this module's actual runtime home, see
 *  pty-daemon-server.ts) is what makes surviving a process restart possible
 *  at all, so the timer must outlast one. */
export const PTY_GRACE_PERIOD_MS = 120_000

function attachIdentityMismatches(
  expected: { paneKey?: string; tabId?: string },
  entry: { paneKey?: string; tabId?: string }
): boolean {
  return Boolean(
    (expected.paneKey && entry.paneKey && expected.paneKey !== entry.paneKey) ||
    (expected.tabId && entry.tabId && expected.tabId !== entry.tabId)
  )
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

function appendScrollback(entry: AgentPtyEntry, data: string): void {
  entry.buf += data
  // Trim to last N lines
  const lines = entry.buf.split('\n')
  if (lines.length > SCROLLBACK_LINES) {
    entry.buf = lines.slice(lines.length - SCROLLBACK_LINES).join('\n')
  }
}

// ─── Handlers ─────────────────────────────────────────────────────────────────

/**
 * handlePtyCreate — Create a new PTY session for agent mode.
 * Returns: { id, cols, rows, cwd, shell }
 *
 * `notify` pushes live `pty.data`/`pty.exit` JSON-RPC notifications back over
 * the same connection as they occur — real-time streaming, not the polling
 * `pty.scrollback` fallback older Orca clients rely on.
 */
export async function handlePtyCreate(
  id: string | number | null,
  params: Record<string, unknown>,
  log: AgentLogger,
  notify: (method: string, params: Record<string, unknown>) => void
): Promise<object> {
  const cols = typeof params.cols === 'number' ? params.cols : 80
  const rows = typeof params.rows === 'number' ? params.rows : 24
  const rawCwd = typeof params.cwd === 'string' ? params.cwd : ''
  const span = Tracers.terminalCreate.start({ origin: 'agent-pty', cols, rows }, resumeFrom(params))

  const nodePty = await import('node-pty').catch(() => null)
  if (!nodePty) {
    span.fail('node-pty not available on this host')
    return {
      jsonrpc: '2.0',
      id,
      error: { code: -32603, message: 'node-pty is not available on this host' }
    }
  }

  const cwd = safeCwd(rawCwd)
  // Why: a caller that asked for a directory must be told when it was unusable and $HOME was opened instead (BUG-QA-01).
  const cwdFellBack = rawCwd !== '' && usableCwd(rawCwd) === null
  const envOverride =
    params.env && typeof params.env === 'object' && !Array.isArray(params.env)
      ? (params.env as Record<string, string>)
      : {}

  // Resolve shell: params.shellOverride → env.SHELL → system default
  const shellOverride = typeof params.shellOverride === 'string' ? params.shellOverride.trim() : ''
  const envShell = typeof envOverride.SHELL === 'string' ? envOverride.SHELL.trim() : ''
  const shell = shellOverride || envShell || (process.env.SHELL ?? '/bin/sh')

  // Why Date.now() prefix, not just the counter: this module runs inside
  // pty-daemon-server.ts, a separate long-lived process — but that daemon
  // CAN restart independently (crash, VM reboot, manual kill), which resets
  // nextAgentPtyId back to 1 while infra.terminal_sessions (postgres,
  // pty_id PRIMARY KEY) still has rows from the daemon's previous life.
  // Live-reproduced: INFRA_CREATE_TERMINAL_SESSION_FAILED (duplicate pty_id
  // "agent-pty-1") the first time a fresh daemon spawned a PTY after an
  // earlier "agent-pty-1" row from days before was already persisted. The
  // counter alone is only unique within one daemon process's lifetime.
  const ptyId = `agent-pty-${Date.now().toString(36)}-${nextAgentPtyId++}`
  const paneKey = typeof params.paneKey === 'string' ? params.paneKey : undefined
  const tabId = typeof params.tabId === 'string' ? params.tabId : undefined
  // Why: 'provider' means the caller has no renderer terminal pane to type
  // the command into (e.g. a headless gh/glab auth-login PTY) — the agent
  // must submit it itself. See specs/agent/api/gaps-and-findings.md #5.
  const command = typeof params.command === 'string' ? params.command : undefined
  const commandDelivery = params.commandDelivery === 'provider' ? 'provider' : 'renderer'
  const shouldProviderDeliverCommand = commandDelivery === 'provider' && command !== undefined
  const userId = typeof params.userId === 'string' ? params.userId : undefined
  // FIX BUG-BE-HLD-005 (Part A parity — this isolation previously only
  // existed on the SSH relay's pty-handler.ts): gh/glab auth-login PTYs need
  // per-user GH_CONFIG_DIR/GLAB_CONFIG_DIR, otherwise every user's
  // `gh auth login` on this Dev Server shares one default config. Prefix
  // match (not exact) since `command` carries the full shell command line.
  const providerEnv =
    userId && command?.startsWith('gh ')
      ? buildGhEnv(userId, {})
      : userId && command?.startsWith('glab ')
        ? buildGlabEnv(userId, {})
        : undefined

  try {
    span.step('node-pty-spawn', { shell, cwd })
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const term = (nodePty as any).spawn(shell, [], {
      name: 'xterm-256color',
      cols,
      rows,
      cwd,
      env: {
        ...process.env,
        TERM: 'xterm-256color',
        ...envOverride,
        // Why last: per-user isolation must win over a caller-supplied env
        // override, the same precedence pty-handler.ts uses.
        ...(providerEnv as Record<string, string> | undefined)
      } as NodeJS.ProcessEnv
    })

    const entry: AgentPtyEntry = {
      pty: term,
      cwd,
      cols,
      rows,
      shell,
      buf: '',
      paneKey,
      tabId,
      notify
    }
    AGENT_PTY_MAP.set(ptyId, entry)

    // Capture scrollback (backup for pty.scrollback polling) and stream live.
    // Why entry.notify (not the captured `notify` param): must reflect
    // whichever connection most recently reattached — see AgentPtyEntry.notify.
    term.onData((data: string) => {
      appendScrollback(entry, data)
      entry.notify('pty.data', { id: ptyId, data })
    })
    term.onExit(({ exitCode, signal }: { exitCode: number; signal?: number }) => {
      if (entry.graceTimer) {
        clearTimeout(entry.graceTimer)
      }
      AGENT_PTY_MAP.delete(ptyId)
      entry.notify('pty.exit', { id: ptyId, exitCode, signal: signal ?? null })
    })

    if (shouldProviderDeliverCommand) {
      setTimeout(() => {
        // Why: re-check liveness — the PTY may have already exited (or been
        // destroyed) in the short window since spawn; writing to a dead
        // node-pty instance throws.
        if (!AGENT_PTY_MAP.has(ptyId)) {
          return
        }
        const submit = process.platform === 'win32' ? '\r' : '\n'
        try {
          term.write(
            buildStartupCommandSubmission(command as string, { submit, bracketedPasteSafe: false })
          )
        } catch (err: unknown) {
          log.warn(
            `pty.create (agent): startup command delivery failed: id=${ptyId} err=${err instanceof Error ? err.message : String(err)}`
          )
        }
      }, STARTUP_COMMAND_WRITE_DELAY_MS)
    }

    log.info(`pty.create (agent): id=${ptyId} cwd=${cwd} shell=${shell}`)
    span.ok({ ptyId, shell, cwd })
    return {
      jsonrpc: '2.0',
      id,
      result: { id: ptyId, cols, rows, cwd, shell, ...(cwdFellBack ? { cwdFellBack: true } : {}) }
    }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    log.error(`pty.create (agent): failed: ${msg}`)
    span.fail(err, { cwd, shell })
    return { jsonrpc: '2.0', id, error: { code: -32603, message: `pty.create failed: ${msg}` } }
  }
}

/**
 * handlePtyAttach — Reattach to a still-running PTY after a WebSocket
 * reconnect. Cancels any pending grace-period cleanup on success.
 * Params: { id, cols?, rows?, suppressReplayNotification?, expectedPaneKey?, expectedTabId? }
 * Returns: { replay } when suppressReplayNotification, else {} (replay is
 * pushed via a `pty.replay` notification instead).
 */
export async function handlePtyAttach(
  id: string | number | null,
  params: Record<string, unknown>,
  log: AgentLogger,
  notify: (method: string, params: Record<string, unknown>) => void
): Promise<object> {
  const ptyId = typeof params.id === 'string' ? params.id : ''
  const span = Tracers.terminalReattach.start({ origin: 'agent-pty', ptyId }, resumeFrom(params))

  if (!ptyId) {
    span.fail('missing id')
    return { jsonrpc: '2.0', id, error: { code: -32602, message: 'pty.attach: missing id' } }
  }

  const entry = AGENT_PTY_MAP.get(ptyId)
  if (!entry) {
    span.fail('pty not found', { ptyId })
    return { jsonrpc: '2.0', id, error: { code: -32603, message: `PTY "${ptyId}" not found` } }
  }

  const expectedPaneKey =
    typeof params.expectedPaneKey === 'string' ? params.expectedPaneKey : undefined
  const expectedTabId = typeof params.expectedTabId === 'string' ? params.expectedTabId : undefined
  if (attachIdentityMismatches({ paneKey: expectedPaneKey, tabId: expectedTabId }, entry)) {
    span.fail('identity mismatch', { ptyId })
    return {
      jsonrpc: '2.0',
      id,
      error: { code: -32603, message: `PTY "${ptyId}" not found (identity mismatch)` }
    }
  }

  // Why here, unconditionally: this is the only signal the agent gets that a
  // new connection now owns this PTY. Rebind before anything else so a data
  // burst arriving mid-attach (e.g. the resize below) already pushes over the
  // new connection instead of the dead one — see AgentPtyEntry.notify.
  entry.notify = notify

  // Why: cancel the grace timer FIRST — a reconnecting client's cols/rows may
  // differ from what the shell was last sized to (the window resized while
  // disconnected); resizing must not race a cleanup that fires mid-attach.
  const wasWithinGracePeriod = Boolean(entry.graceTimer)
  if (entry.graceTimer) {
    clearTimeout(entry.graceTimer)
    entry.graceTimer = null
  }
  const cols = typeof params.cols === 'number' ? params.cols : undefined
  const rows = typeof params.rows === 'number' ? params.rows : undefined
  if (cols && rows) {
    try {
      entry.pty.resize(cols, rows)
      entry.cols = cols
      entry.rows = rows
    } catch {
      /* best effort — attach still succeeds without a resize */
    }
  }

  log.info(`pty.attach (agent): id=${ptyId}`)
  span.ok({ ptyId, wasWithinGracePeriod, replayBytes: entry.buf.length })
  if (params.suppressReplayNotification) {
    return { jsonrpc: '2.0', id, result: { replay: entry.buf } }
  }
  if (entry.buf) {
    notify('pty.replay', { id: ptyId, data: entry.buf })
  }
  return { jsonrpc: '2.0', id, result: {} }
}

/**
 * scheduleGracePeriodCleanup — Called when the agent's WebSocket to Orca
 * disconnects. Instead of killing PTYs immediately, arms a grace timer per
 * PTY; a reattach within PTY_GRACE_PERIOD_MS cancels it (see handlePtyAttach).
 * If no reattach arrives in time, the PTY is killed for real.
 */
export function scheduleGracePeriodCleanup(
  log: AgentLogger,
  graceTimeMs = PTY_GRACE_PERIOD_MS
): void {
  for (const [ptyId, entry] of AGENT_PTY_MAP.entries()) {
    if (entry.graceTimer) {
      continue
    } // already counting down from an earlier disconnect
    entry.graceTimer = setTimeout(() => {
      const current = AGENT_PTY_MAP.get(ptyId)
      if (!current || current.graceTimer !== entry.graceTimer) {
        return
      } // reattached or already gone
      try {
        current.pty.kill('SIGTERM')
        log.info(`scheduleGracePeriodCleanup: grace period expired, killed ${ptyId}`)
      } catch {
        /* best effort */
      }
      AGENT_PTY_MAP.delete(ptyId)
    }, graceTimeMs)
  }
  if (AGENT_PTY_MAP.size > 0) {
    log.info(
      `scheduleGracePeriodCleanup: armed grace timers for ${AGENT_PTY_MAP.size} PTY(s) (${graceTimeMs}ms)`
    )
  }
}

/**
 * handlePtyWrite — Send input to PTY stdin.
 * Params: { id, data }
 */
export async function handlePtyWrite(
  id: string | number | null,
  params: Record<string, unknown>,
  _log: AgentLogger
): Promise<object> {
  const ptyId = typeof params.id === 'string' ? params.id : ''
  const data = typeof params.data === 'string' ? params.data : ''
  if (!ptyId) {
    return { jsonrpc: '2.0', id, error: { code: -32602, message: 'pty.write: missing id' } }
  }

  const entry = AGENT_PTY_MAP.get(ptyId)
  if (!entry) {
    return { jsonrpc: '2.0', id, error: { code: -32603, message: `PTY not found: ${ptyId}` } }
  }

  try {
    entry.pty.write(data)
    return { jsonrpc: '2.0', id, result: { ok: true } }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    return { jsonrpc: '2.0', id, error: { code: -32603, message: `pty.write failed: ${msg}` } }
  }
}

/**
 * handlePtyResize — Resize PTY window.
 * Params: { id, cols, rows }
 */
export async function handlePtyResize(
  id: string | number | null,
  params: Record<string, unknown>,
  _log: AgentLogger
): Promise<object> {
  const ptyId = typeof params.id === 'string' ? params.id : ''
  const cols = typeof params.cols === 'number' ? params.cols : 80
  const rows = typeof params.rows === 'number' ? params.rows : 24
  const span = Tracers.terminalResize.start(
    { origin: 'agent-pty', ptyId, cols, rows },
    resumeFrom(params)
  )

  if (!ptyId) {
    span.fail('missing id')
    return { jsonrpc: '2.0', id, error: { code: -32602, message: 'pty.resize: missing id' } }
  }

  const entry = AGENT_PTY_MAP.get(ptyId)
  if (!entry) {
    span.fail('pty not found', { ptyId })
    return { jsonrpc: '2.0', id, error: { code: -32603, message: `PTY not found: ${ptyId}` } }
  }

  try {
    entry.pty.resize(cols, rows)
    entry.cols = cols
    entry.rows = rows
    span.ok({ ptyId, cols, rows })
    return { jsonrpc: '2.0', id, result: { ok: true } }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    span.fail(err, { ptyId })
    return { jsonrpc: '2.0', id, error: { code: -32603, message: `pty.resize failed: ${msg}` } }
  }
}

/**
 * handlePtyDestroy — Close and cleanup a PTY session.
 * Params: { id, graceful? }
 */
export async function handlePtyDestroy(
  id: string | number | null,
  params: Record<string, unknown>,
  log: AgentLogger
): Promise<object> {
  const ptyId = typeof params.id === 'string' ? params.id : ''
  const graceful = params.graceful !== false // default: graceful=true
  const span = Tracers.terminalDestroy.start(
    { origin: 'agent-pty', ptyId, graceful },
    resumeFrom(params)
  )

  if (!ptyId) {
    span.fail('missing id')
    return { jsonrpc: '2.0', id, error: { code: -32602, message: 'pty.destroy: missing id' } }
  }

  const entry = AGENT_PTY_MAP.get(ptyId)
  if (!entry) {
    span.ok({ ptyId, alreadyDead: true })
    return { jsonrpc: '2.0', id, result: { ok: true, alreadyDead: true } }
  }

  try {
    if (entry.graceTimer) {
      clearTimeout(entry.graceTimer)
    }
    if (process.platform === 'win32') {
      entry.pty.kill()
    } else {
      entry.pty.kill(graceful ? 'SIGTERM' : 'SIGKILL')
    }
    AGENT_PTY_MAP.delete(ptyId)
    log.info(`pty.destroy (agent): id=${ptyId}`)
    span.ok({ ptyId, graceful })
    return { jsonrpc: '2.0', id, result: { ok: true } }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    span.fail(err, { ptyId })
    return { jsonrpc: '2.0', id, error: { code: -32603, message: `pty.destroy failed: ${msg}` } }
  }
}

/**
 * handlePtyScrollback — Get scrollback buffer.
 * Params: { id, lines? }
 */
export async function handlePtyScrollback(
  id: string | number | null,
  params: Record<string, unknown>,
  _log: AgentLogger
): Promise<object> {
  const ptyId = typeof params.id === 'string' ? params.id : ''
  const lines = typeof params.lines === 'number' ? params.lines : 100
  if (!ptyId) {
    return { jsonrpc: '2.0', id, error: { code: -32602, message: 'pty.scrollback: missing id' } }
  }

  const entry = AGENT_PTY_MAP.get(ptyId)
  if (!entry) {
    return { jsonrpc: '2.0', id, error: { code: -32603, message: `PTY not found: ${ptyId}` } }
  }

  const allLines = entry.buf.split('\n')
  const data = allLines.slice(Math.max(0, allLines.length - lines)).join('\n')
  return { jsonrpc: '2.0', id, result: { data } }
}

/** Number of PTYs currently tracked — used by pty-daemon-server.ts to decide
 *  whether it's safe to idle-shutdown. */
export function activePtyCount(): number {
  return AGENT_PTY_MAP.size
}

/**
 * cleanupAgentPtys — Kill all agent-mode PTY sessions.
 * Must be called in agent-session.ts stop() to prevent zombie processes.
 */
export function cleanupAgentPtys(log: AgentLogger): void {
  for (const [ptyId, entry] of AGENT_PTY_MAP.entries()) {
    try {
      if (entry.graceTimer) {
        clearTimeout(entry.graceTimer)
      }
      entry.pty.kill('SIGTERM')
      log.info(`cleanupAgentPtys: killed ${ptyId}`)
    } catch {
      /* best effort */
    }
  }
  AGENT_PTY_MAP.clear()
}
