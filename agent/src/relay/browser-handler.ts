// src/relay/browser-handler.ts
// Part A (agent-rpc-dispatch.ts) implementation of browser.* — driving a real
// headless Chromium process ON THIS HOST, relayed from backend-go's
// wscompat/channels_browser.go via infra-fleet-service's Relay RPC.
//
// Why this exists (TASK-036 option b): the OLD `browser.*` feature
// (backend/src/main/browser/agent-browser-bridge.ts) drove Electron's own
// embedded WebContents via CDP — desktop-local, not reachable from a remote
// SSH-connected dev server. This is a genuinely new capability, not a port:
// the Dev Server Agent launches and drives its OWN headless browser process.
//
// Engine choice: `agent-browser` (vercel-labs) is already a vendored
// dependency (agent/package.json) and is the exact engine the OLD Electron
// bridge already shelled out to (see `execAgentBrowser` in
// agent-browser-bridge.ts) — it bundles/locates a real Chrome/Chromium,
// speaks CDP internally, and exposes the goto/click/snapshot/eval/mouse/tab
// vocabulary this file needs as a CLI with `--json` output. Reusing it here
// means this file does not hand-roll a CDP client — it shells out to a
// proven automation engine already in this codebase's dependency graph,
// exactly like the desktop bridge did, just against a real launched browser
// instead of an Electron WebContents.
//
// Session-scoping/cleanup model (decided here, since nothing upstream
// specifies one):
//   - One `agent-browser` session per worktree, keyed by `params.worktree`
//     via the CLI's `--session <worktreeId>` flag. `agent-browser` itself
//     keeps a persistent background daemon + Chrome process alive across
//     separate CLI invocations for the same `--session` name (confirmed by
//     spawning it: the daemon reparents to pid 1 and outlives the invoking
//     process) — so this file does NOT need to manage a long-lived child
//     process itself; each RPC call is a short-lived CLI invocation that
//     talks to (or lazily creates) that session's daemon.
//   - Idle timeout (primary cleanup mechanism): every invocation sets
//     AGENT_BROWSER_IDLE_TIMEOUT_MS so the daemon self-terminates after
//     BROWSER_SESSION_IDLE_TIMEOUT_MS of inactivity. Without this, a
//     worktree's headless Chrome process leaks on the host forever once
//     used even once — verified in this sandbox (leftover daemon + ~15
//     Chrome subprocesses per session after testing without it).
//   - Explicit teardown: `browser.tabClose` fully closes the session's
//     browser (not just the tab) once no tabs remain, so closing the last
//     browser-pane tab in the UI frees the host process immediately instead
//     of waiting out the idle timeout.
//   - NOT implemented (documented gap, not silently assumed): teardown tied
//     to the Orca<->agent WebSocket connection closing. Each browser.* call
//     is a stateless RPC dispatch with no connection-lifecycle hook wired in
//     this pass (agent-rpc-dispatch.ts's per-connection WireState is not
//     plumbed to this handler). The idle timeout is the safety net for a
//     dropped connection.
//
// Capability requirement this creates: the target host must have a
// Chrome/Chromium install `agent-browser` can find (or network access for
// its own first-run download). runBrowserCommand() maps the CLI's own
// "no usable browser" failure into a clear BROWSER_ENGINE_UNAVAILABLE error
// instead of a opaque spawn/exit-code failure.

import { execFile } from 'node:child_process'
import { createRequire } from 'node:module'
import path from 'node:path'
import type { AgentLogger } from './agent-logger'
import { AgentErrorCode } from '../shared/agent-wire-protocol'

const requireForBrowserHandler = createRequire(__filename)

/**
 * Hand-rolled promise wrapper instead of node:util's promisify(execFile) —
 * execFile has a special promisify.custom implementation that returns
 * {stdout, stderr}; a plain vi.fn() test double loses that annotation, which
 * would silently change what the promisified call resolves to under test.
 * This wrapper's behavior is identical either way and trivially mockable.
 */
function execFileAsync(
  command: string,
  args: string[],
  options: { encoding: 'utf-8'; timeout: number; env: NodeJS.ProcessEnv }
): Promise<{ stdout: string; stderr: string }> {
  return new Promise((resolve, reject) => {
    execFile(command, args, options, (error, stdout, stderr) => {
      if (error) {
        reject(error)
        return
      }
      resolve({ stdout: stdout.toString(), stderr: stderr.toString() })
    })
  })
}

// Why: an idle worktree browser session must not outlive the worktree's
// active use by more than a few minutes — this is a background headless
// Chrome process on a shared dev server host, not a per-user local resource.
const BROWSER_SESSION_IDLE_TIMEOUT_MS = 15 * 60 * 1000
const BROWSER_COMMAND_TIMEOUT_MS = 30_000

type JsonRpcId = string | number | null

type JsonRpcSuccess = {
  readonly jsonrpc: '2.0'
  readonly id: JsonRpcId
  readonly result: unknown
}

type JsonRpcError = {
  readonly jsonrpc: '2.0'
  readonly id: JsonRpcId
  readonly error: { code: number; message: string }
}

type JsonRpcResponse = JsonRpcSuccess | JsonRpcError

type BrowserCliEnvelope = {
  success: boolean
  data: unknown
  error: string | null
}

function makeSuccess(id: JsonRpcId, result: unknown): JsonRpcSuccess {
  return { jsonrpc: '2.0', id, result }
}

function makeFailure(id: JsonRpcId, message: string): JsonRpcError {
  return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message } }
}

let cachedBinPath: string | null | undefined

/**
 * Resolves the vendored `agent-browser` CLI entrypoint. Cached because
 * require.resolve() is not free and this is called on every browser.* RPC.
 */
function resolveAgentBrowserBin(): string {
  if (cachedBinPath !== undefined) {
    if (cachedBinPath === null) {
      throw new Error(
        'BROWSER_ENGINE_UNAVAILABLE: the agent-browser package is not installed on this agent — ' +
          'reinstall the Dev Server Agent so its bundled dependencies are present.'
      )
    }
    return cachedBinPath
  }
  try {
    const pkgJsonPath = requireForBrowserHandler.resolve('agent-browser/package.json')
    cachedBinPath = path.join(path.dirname(pkgJsonPath), 'bin', 'agent-browser.js')
    return cachedBinPath
  } catch (err) {
    cachedBinPath = null
    throw new Error(
      `BROWSER_ENGINE_UNAVAILABLE: the agent-browser package is not installed on this agent: ${
        err instanceof Error ? err.message : String(err)
      }`
    )
  }
}

/** Recognizes agent-browser's own "no usable Chrome/Chromium" failure text. */
function isMissingBrowserEngineError(message: string): boolean {
  const normalized = message.toLowerCase()
  return (
    normalized.includes('no usable browser') ||
    normalized.includes('could not find') ||
    normalized.includes('failed to launch') ||
    normalized.includes('executable doesn’t exist') ||
    normalized.includes("executable doesn't exist") ||
    normalized.includes('chrome not found') ||
    normalized.includes('browser not found')
  )
}

export type BrowserCommandParams = {
  worktree?: unknown
  page?: unknown
}

export function requireWorktreeId(params: Record<string, unknown>): string {
  const worktreeId = params.worktree
  if (typeof worktreeId !== 'string' || worktreeId.length === 0) {
    throw new Error('BROWSER_NO_WORKTREE: this operation requires a worktree selector')
  }
  return worktreeId
}

/**
 * Runs one `agent-browser` CLI command scoped to a worktree's persistent
 * session, parses its `--json` envelope, and unwraps success/error.
 */
export async function runBrowserCommand(worktreeId: string, args: string[]): Promise<unknown> {
  const bin = resolveAgentBrowserBin()
  const fullArgs = [bin, ...args, '--session', worktreeId, '--json']
  let stdout: string
  try {
    const result = await execFileAsync(process.execPath, fullArgs, {
      encoding: 'utf-8',
      timeout: BROWSER_COMMAND_TIMEOUT_MS,
      env: {
        ...process.env,
        // Why: this file's session-cleanup model — see header comment. Passed
        // on every call since the daemon reads it once, at creation.
        AGENT_BROWSER_IDLE_TIMEOUT_MS: String(BROWSER_SESSION_IDLE_TIMEOUT_MS)
      }
    })
    stdout = result.stdout
  } catch (err) {
    const message = err instanceof Error ? err.message : String(err)
    if (isMissingBrowserEngineError(message)) {
      throw new Error(
        `BROWSER_ENGINE_UNAVAILABLE: no Chrome/Chromium install found on this host for agent-browser to drive. ${message}`
      )
    }
    throw new Error(`BROWSER_COMMAND_FAILED: ${message}`)
  }

  let envelope: BrowserCliEnvelope
  try {
    envelope = JSON.parse(stdout) as BrowserCliEnvelope
  } catch {
    throw new Error(`BROWSER_COMMAND_FAILED: agent-browser returned non-JSON output: ${stdout.slice(0, 200)}`)
  }
  if (!envelope.success) {
    const message = envelope.error ?? 'unknown agent-browser error'
    if (isMissingBrowserEngineError(message)) {
      throw new Error(`BROWSER_ENGINE_UNAVAILABLE: ${message}`)
    }
    throw new Error(`BROWSER_COMMAND_FAILED: ${message}`)
  }
  return envelope.data
}

async function dispatchBrowserCommand(
  id: JsonRpcId,
  params: Record<string, unknown>,
  log: AgentLogger,
  methodLabel: string,
  buildArgs: (worktreeId: string, params: Record<string, unknown>) => string[],
  shapeResult?: (data: unknown, worktreeId: string) => unknown
): Promise<JsonRpcResponse> {
  try {
    const worktreeId = requireWorktreeId(params)
    const data = await runBrowserCommand(worktreeId, buildArgs(worktreeId, params))
    return makeSuccess(id, shapeResult ? shapeResult(data, worktreeId) : data)
  } catch (err) {
    const message = err instanceof Error ? err.message : String(err)
    log.error(`browser.${methodLabel} failed: ${message}`)
    return makeFailure(id, message)
  }
}

function stringParam(params: Record<string, unknown>, key: string): string {
  const value = params[key]
  if (typeof value !== 'string' || value.length === 0) {
    throw new Error(`BROWSER_MISSING_ARGS: '${key}' is required`)
  }
  return value
}

function numberParam(params: Record<string, unknown>, key: string): number {
  const value = params[key]
  if (typeof value !== 'number' || !Number.isFinite(value)) {
    throw new Error(`BROWSER_MISSING_ARGS: '${key}' is required`)
  }
  return value
}

function optionalStringParam(params: Record<string, unknown>, key: string): string | undefined {
  const value = params[key]
  return typeof value === 'string' && value.length > 0 ? value : undefined
}

function optionalNumberParam(params: Record<string, unknown>, key: string): number | undefined {
  const value = params[key]
  return typeof value === 'number' && Number.isFinite(value) ? value : undefined
}

// ─── browser.goto ────────────────────────────────────────────────────────────

export async function handleBrowserGoto(
  id: JsonRpcId,
  params: Record<string, unknown>,
  log: AgentLogger
): Promise<JsonRpcResponse> {
  return dispatchBrowserCommand(id, params, log, 'goto', () => ['open', stringParam(params, 'url')])
}

// ─── browser.snapshot ────────────────────────────────────────────────────────

export async function handleBrowserSnapshot(
  id: JsonRpcId,
  params: Record<string, unknown>,
  log: AgentLogger
): Promise<JsonRpcResponse> {
  return dispatchBrowserCommand(id, params, log, 'snapshot', () => ['snapshot'])
}

// ─── browser.click ───────────────────────────────────────────────────────────

export async function handleBrowserClick(
  id: JsonRpcId,
  params: Record<string, unknown>,
  log: AgentLogger
): Promise<JsonRpcResponse> {
  return dispatchBrowserCommand(
    id,
    params,
    log,
    'click',
    () => ['click', stringParam(params, 'element')],
    (data) => ({ clicked: (data as { clicked?: unknown } | null)?.clicked ?? params.element })
  )
}

// ─── browser.eval ────────────────────────────────────────────────────────────

export async function handleBrowserEval(
  id: JsonRpcId,
  params: Record<string, unknown>,
  log: AgentLogger
): Promise<JsonRpcResponse> {
  return dispatchBrowserCommand(
    id,
    params,
    log,
    'eval',
    () => ['eval', stringParam(params, 'expression')],
    (data) => ({ result: data })
  )
}

// ─── browser.keypress ────────────────────────────────────────────────────────

export async function handleBrowserKeypress(
  id: JsonRpcId,
  params: Record<string, unknown>,
  log: AgentLogger
): Promise<JsonRpcResponse> {
  return dispatchBrowserCommand(id, params, log, 'keypress', () => ['press', stringParam(params, 'key')])
}

// ─── browser.mouseMove ───────────────────────────────────────────────────────

export async function handleBrowserMouseMove(
  id: JsonRpcId,
  params: Record<string, unknown>,
  log: AgentLogger
): Promise<JsonRpcResponse> {
  return dispatchBrowserCommand(id, params, log, 'mouseMove', () => [
    'mouse',
    'move',
    String(numberParam(params, 'x')),
    String(numberParam(params, 'y'))
  ])
}

// ─── browser.mouseDown ───────────────────────────────────────────────────────

export async function handleBrowserMouseDown(
  id: JsonRpcId,
  params: Record<string, unknown>,
  log: AgentLogger
): Promise<JsonRpcResponse> {
  return dispatchBrowserCommand(id, params, log, 'mouseDown', () => {
    const button = optionalStringParam(params, 'button')
    return button ? ['mouse', 'down', button] : ['mouse', 'down']
  })
}

// ─── browser.mouseUp ─────────────────────────────────────────────────────────

export async function handleBrowserMouseUp(
  id: JsonRpcId,
  params: Record<string, unknown>,
  log: AgentLogger
): Promise<JsonRpcResponse> {
  return dispatchBrowserCommand(id, params, log, 'mouseUp', () => {
    const button = optionalStringParam(params, 'button')
    return button ? ['mouse', 'up', button] : ['mouse', 'up']
  })
}

// ─── browser.mouseWheel ──────────────────────────────────────────────────────

export async function handleBrowserMouseWheel(
  id: JsonRpcId,
  params: Record<string, unknown>,
  log: AgentLogger
): Promise<JsonRpcResponse> {
  return dispatchBrowserCommand(id, params, log, 'mouseWheel', () => {
    const dy = numberParam(params, 'dy')
    const dx = optionalNumberParam(params, 'dx')
    return dx === undefined ? ['mouse', 'wheel', String(dy)] : ['mouse', 'wheel', String(dy), String(dx)]
  })
}

// ─── browser.viewport ────────────────────────────────────────────────────────

export async function handleBrowserViewport(
  id: JsonRpcId,
  params: Record<string, unknown>,
  log: AgentLogger
): Promise<JsonRpcResponse> {
  return dispatchBrowserCommand(id, params, log, 'viewport', () => [
    'set',
    'viewport',
    String(numberParam(params, 'width')),
    String(numberParam(params, 'height'))
  ])
}

// ─── browser.tabCreate ───────────────────────────────────────────────────────

export async function handleBrowserTabCreate(
  id: JsonRpcId,
  params: Record<string, unknown>,
  log: AgentLogger
): Promise<JsonRpcResponse> {
  return dispatchBrowserCommand(
    id,
    params,
    log,
    'tabCreate',
    () => ['tab', 'new'],
    (data) => {
      const tabData = data as { tabId?: unknown; url?: unknown } | null
      const tabId = typeof tabData?.tabId === 'string' ? tabData.tabId : undefined
      return { browserPageId: tabId, tabId, url: tabData?.url }
    }
  )
}

// ─── browser.tabClose ────────────────────────────────────────────────────────

/**
 * Closes one tab, then — per this file's documented session-cleanup model —
 * tears down the worktree's entire browser session if that was the last tab,
 * so closing a browser pane frees the host's headless Chrome process
 * immediately instead of waiting out the idle timeout.
 */
export async function handleBrowserTabClose(
  id: JsonRpcId,
  params: Record<string, unknown>,
  log: AgentLogger
): Promise<JsonRpcResponse> {
  try {
    const worktreeId = requireWorktreeId(params)
    const tabId = optionalStringParam(params, 'page')
    const closeArgs = tabId ? ['tab', 'close', tabId] : ['tab', 'close']
    const closeData = await runBrowserCommand(worktreeId, closeArgs)

    let remainingTabs = 1
    try {
      const listData = (await runBrowserCommand(worktreeId, ['tab', 'list'])) as {
        tabs?: unknown[]
      } | null
      remainingTabs = Array.isArray(listData?.tabs) ? listData.tabs.length : 0
    } catch {
      // Why: if `tab list` fails right after the session's last tab closed,
      // treat it as "nothing left" — the safer default is tearing the
      // session down rather than leaking its Chrome process.
      remainingTabs = 0
    }

    if (remainingTabs === 0) {
      try {
        await runBrowserCommand(worktreeId, ['close'])
      } catch (err) {
        log.warn(`browser.tabClose: failed to tear down empty session ${worktreeId}: ${String(err)}`)
      }
    }

    return makeSuccess(id, closeData)
  } catch (err) {
    const message = err instanceof Error ? err.message : String(err)
    log.error(`browser.tabClose failed: ${message}`)
    return makeFailure(id, message)
  }
}
