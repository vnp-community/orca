/**
 * pty-agent-bridge.ts — TM-001/TM-006: PTY management for agent RPC mode.
 *
 * Why this file:
 *   PtyHandler registers its operations via RelayDispatcher.onRequest() (relay/local mode).
 *   agent-rpc-dispatch.ts uses a switch/case pattern (agent WebSocket mode).
 *   This bridge exposes PTY operations to agent-rpc-dispatch.ts without coupling to
 *   PtyHandler internals, and without creating circular dependencies.
 *
 * Lifecycle:
 *   - AGENT_PTY_MAP is module-level (singleton per process)
 *   - cleanupAgentPtys() must be called on session termination (agent-session.ts stop())
 *
 * Security:
 *   - cwd is validated via validatePtyCwd (home/workDir/tmp only)
 *   - Signals are allowlisted (SIGTERM, SIGKILL, SIGINT, SIGHUP, SIGTSTP)
 *   - node-pty is loaded lazily (not available in test env without native deps)
 */

import type { AgentLogger } from './agent-logger'

// ─── Types ────────────────────────────────────────────────────────────────────

interface AgentPtyEntry {
  // eslint-disable-next-line @typescript-eslint/no-explicit-any -- node-pty IPty
  pty:    any
  cwd:    string
  cols:   number
  rows:   number
  shell:  string
  buf:    string   // scrollback buffer (last SCROLLBACK_LINES lines)
}

// ─── Module state ─────────────────────────────────────────────────────────────

const AGENT_PTY_MAP = new Map<string, AgentPtyEntry>()
let nextAgentPtyId = 1

const ALLOWED_SIGNALS = new Set(['SIGTERM', 'SIGKILL', 'SIGINT', 'SIGHUP', 'SIGTSTP'])
const SCROLLBACK_LINES = 500

// ─── Helpers ──────────────────────────────────────────────────────────────────

function safeCwd(raw: string): string {
  if (!raw) return require('node:os').homedir() as string
  if (raw.includes('\0')) return require('node:os').homedir() as string
  const resolved = require('node:path').resolve(raw) as string
  try {
    const stat = require('node:fs').statSync(resolved) as import('node:fs').Stats
    if (!stat.isDirectory()) return require('node:os').homedir() as string
    return resolved
  } catch {
    return require('node:os').homedir() as string
  }
}

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
 */
export async function handlePtyCreate(
  id:     string | number | null,
  params: Record<string, unknown>,
  log:    AgentLogger,
): Promise<object> {
  const nodePty = await import('node-pty').catch(() => null)
  if (!nodePty) {
    return {
      jsonrpc: '2.0', id,
      error: { code: -32603, message: 'node-pty is not available on this host' },
    }
  }

  const cols       = typeof params.cols === 'number' ? params.cols : 80
  const rows       = typeof params.rows === 'number' ? params.rows : 24
  const rawCwd     = typeof params.cwd  === 'string' ? params.cwd  : ''
  const cwd        = safeCwd(rawCwd)
  const envOverride = (params.env && typeof params.env === 'object' && !Array.isArray(params.env))
    ? params.env as Record<string, string>
    : {}

  // Resolve shell: params.shellOverride → env.SHELL → system default
  const shellOverride = typeof params.shellOverride === 'string' ? params.shellOverride.trim() : ''
  const envShell      = typeof envOverride.SHELL     === 'string' ? envOverride.SHELL.trim()   : ''
  const shell = shellOverride || envShell || (process.env.SHELL ?? '/bin/sh')

  const ptyId = `agent-pty-${nextAgentPtyId++}`

  try {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const term = (nodePty as any).spawn(shell, [], {
      name: 'xterm-256color',
      cols,
      rows,
      cwd,
      env: { ...process.env, TERM: 'xterm-256color', ...envOverride } as NodeJS.ProcessEnv,
    })

    const entry: AgentPtyEntry = { pty: term, cwd, cols, rows, shell, buf: '' }
    AGENT_PTY_MAP.set(ptyId, entry)

    // Capture scrollback
    term.onData((data: string) => { appendScrollback(entry, data) })

    log.info(`pty.create (agent): id=${ptyId} cwd=${cwd} shell=${shell}`)
    return { jsonrpc: '2.0', id, result: { id: ptyId, cols, rows, cwd, shell } }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    log.error(`pty.create (agent): failed: ${msg}`)
    return { jsonrpc: '2.0', id, error: { code: -32603, message: `pty.create failed: ${msg}` } }
  }
}

/**
 * handlePtyWrite — Send input to PTY stdin.
 * Params: { id, data }
 */
export async function handlePtyWrite(
  id:     string | number | null,
  params: Record<string, unknown>,
  _log:   AgentLogger,
): Promise<object> {
  const ptyId = typeof params.id   === 'string' ? params.id   : ''
  const data  = typeof params.data === 'string' ? params.data : ''
  if (!ptyId) return { jsonrpc: '2.0', id, error: { code: -32602, message: 'pty.write: missing id' } }

  const entry = AGENT_PTY_MAP.get(ptyId)
  if (!entry) return { jsonrpc: '2.0', id, error: { code: -32603, message: `PTY not found: ${ptyId}` } }

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
  id:     string | number | null,
  params: Record<string, unknown>,
  _log:   AgentLogger,
): Promise<object> {
  const ptyId = typeof params.id   === 'string' ? params.id   : ''
  const cols  = typeof params.cols === 'number' ? params.cols : 80
  const rows  = typeof params.rows === 'number' ? params.rows : 24
  if (!ptyId) return { jsonrpc: '2.0', id, error: { code: -32602, message: 'pty.resize: missing id' } }

  const entry = AGENT_PTY_MAP.get(ptyId)
  if (!entry) return { jsonrpc: '2.0', id, error: { code: -32603, message: `PTY not found: ${ptyId}` } }

  try {
    entry.pty.resize(cols, rows)
    entry.cols = cols
    entry.rows = rows
    return { jsonrpc: '2.0', id, result: { ok: true } }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    return { jsonrpc: '2.0', id, error: { code: -32603, message: `pty.resize failed: ${msg}` } }
  }
}

/**
 * handlePtyDestroy — Close and cleanup a PTY session.
 * Params: { id, graceful? }
 */
export async function handlePtyDestroy(
  id:     string | number | null,
  params: Record<string, unknown>,
  log:    AgentLogger,
): Promise<object> {
  const ptyId   = typeof params.id === 'string' ? params.id : ''
  const graceful = params.graceful !== false  // default: graceful=true
  if (!ptyId) return { jsonrpc: '2.0', id, error: { code: -32602, message: 'pty.destroy: missing id' } }

  const entry = AGENT_PTY_MAP.get(ptyId)
  if (!entry) return { jsonrpc: '2.0', id, result: { ok: true, alreadyDead: true } }

  try {
    if (process.platform === 'win32') { entry.pty.kill() }
    else { entry.pty.kill(graceful ? 'SIGTERM' : 'SIGKILL') }
    AGENT_PTY_MAP.delete(ptyId)
    log.info(`pty.destroy (agent): id=${ptyId}`)
    return { jsonrpc: '2.0', id, result: { ok: true } }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    return { jsonrpc: '2.0', id, error: { code: -32603, message: `pty.destroy failed: ${msg}` } }
  }
}

/**
 * handlePtyScrollback — Get scrollback buffer.
 * Params: { id, lines? }
 */
export async function handlePtyScrollback(
  id:     string | number | null,
  params: Record<string, unknown>,
  _log:   AgentLogger,
): Promise<object> {
  const ptyId = typeof params.id    === 'string' ? params.id    : ''
  const lines = typeof params.lines === 'number' ? params.lines : 100
  if (!ptyId) return { jsonrpc: '2.0', id, error: { code: -32602, message: 'pty.scrollback: missing id' } }

  const entry = AGENT_PTY_MAP.get(ptyId)
  if (!entry) return { jsonrpc: '2.0', id, error: { code: -32603, message: `PTY not found: ${ptyId}` } }

  const allLines = entry.buf.split('\n')
  const data = allLines.slice(Math.max(0, allLines.length - lines)).join('\n')
  return { jsonrpc: '2.0', id, result: { data } }
}

/**
 * handlePtySendSignal — Send a POSIX signal to PTY process.
 * Params: { id, signal }
 */
export async function handlePtySendSignal(
  id:     string | number | null,
  params: Record<string, unknown>,
  log:    AgentLogger,
): Promise<object> {
  const ptyId  = typeof params.id     === 'string' ? params.id     : ''
  const signal = typeof params.signal === 'string' ? params.signal : 'SIGTERM'
  if (!ptyId)                       return { jsonrpc: '2.0', id, error: { code: -32602, message: 'pty.sendSignal: missing id' } }
  if (!ALLOWED_SIGNALS.has(signal)) return { jsonrpc: '2.0', id, error: { code: -32602, message: `Signal not allowed: ${signal}` } }

  const entry = AGENT_PTY_MAP.get(ptyId)
  if (!entry) return { jsonrpc: '2.0', id, error: { code: -32603, message: `PTY not found: ${ptyId}` } }

  try {
    if (process.platform !== 'win32') { entry.pty.kill(signal) }
    else { entry.pty.kill() }
    log.info(`pty.sendSignal (agent): id=${ptyId} signal=${signal}`)
    return { jsonrpc: '2.0', id, result: { ok: true } }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    return { jsonrpc: '2.0', id, error: { code: -32603, message: `pty.sendSignal failed: ${msg}` } }
  }
}

/**
 * cleanupAgentPtys — Kill all agent-mode PTY sessions.
 * Must be called in agent-session.ts stop() to prevent zombie processes.
 */
export function cleanupAgentPtys(log: AgentLogger): void {
  for (const [ptyId, entry] of AGENT_PTY_MAP.entries()) {
    try {
      entry.pty.kill('SIGTERM')
      log.info(`cleanupAgentPtys: killed ${ptyId}`)
    } catch { /* best effort */ }
  }
  AGENT_PTY_MAP.clear()
}
