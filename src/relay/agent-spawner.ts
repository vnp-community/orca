/**
 * agent-spawner.ts — Dev Server tier SubAgent spawner (CR-AG-12)
 *
 * ⚠️ KHÔNG nhầm với src/main/project/ProfileAwareAgentSpawner.ts (Orca Server tier)
 *    Đây là relay-side spawner cho sub-agent process management.
 *
 * Exports:
 *   SubAgentSpawner   — class lifecycle manager (pure, testable)
 *   handleAgentSpawn  — RPC handler (fire-and-forget streaming)
 *   handleAgentKill   — RPC handler
 *   buildAgentEnv     — env builder (testable with mock credStore)
 *   resolveAgentSpec  — model → binary spec (pure, testable)
 *
 * @module relay/agent-spawner
 */
import * as nodePty from 'node-pty'
import type WebSocket from 'ws'
import type { AgentConfig } from './agent-config'
import type { AgentLogger } from './agent-logger'
import { encodeDataFrame, createWireState } from './agent-wire'
import { createTracer } from '../shared/trace'

const spawnerTracer = createTracer('agent:spawn')
import type { WireState } from './agent-wire'
import { AgentErrorCode } from '../shared/agent-wire-protocol'

// ── Types ──────────────────────────────────────────────────────────────────────

export type AgentLifecycleState = 'idle' | 'spawning' | 'running' | 'stopping' | 'stopped' | 'error'

export interface AgentSpawnRequest {
  taskId:    string
  userId:    string
  modelId:   string
  accountId: string
  cwd?:      string
}

export interface AgentStatusEvent {
  type:    'spawn.accepted' | 'spawn.started' | 'spawn.output' | 'spawn.exit' | 'spawn.error'
  ptyId?:  string
  taskId?: string
  data?:   string
  code?:   number
  error?:  string
}

// ── PTY Registry (in-process singleton) ──────────────────────────────────────

const PTY_REGISTRY = new Map<string, {
  pty:    nodePty.IPty
  taskId: string
  userId: string
}>()

// ── SubAgentSpawner (pure class — testable) ───────────────────────────────────

export class SubAgentSpawner {
  private state: AgentLifecycleState = 'idle'

  getState(): AgentLifecycleState { return this.state }

  transition(next: AgentLifecycleState): void {
    const VALID: Record<AgentLifecycleState, AgentLifecycleState[]> = {
      idle:     ['spawning'],
      spawning: ['running', 'error'],
      running:  ['stopping', 'error'],
      stopping: ['stopped', 'error'],
      stopped:  ['idle'],
      error:    ['idle'],
    }
    if (!VALID[this.state]?.includes(next)) {
      throw new Error(`SubAgentSpawner: invalid transition ${this.state} → ${next}`)
    }
    this.state = next
  }
}

// ── resolveAgentSpec (pure, testable) ─────────────────────────────────────────

export function resolveAgentSpec(modelId: string): { binary: string; args: string[] } {
  if (modelId.startsWith('claude')) {
    return { binary: 'claude', args: ['--output-format', 'stream-json', '--no-cache'] }
  }
  if (modelId.startsWith('gemini')) {
    return { binary: 'gemini', args: ['--stream'] }
  }
  throw new Error(`resolveAgentSpec: unknown modelId: ${modelId}`)
}

// ── buildAgentEnv (testable with mock credStore) ──────────────────────────────

export async function buildAgentEnv(
  accountId: string,
  apiKey:    string,
  cwd:       string,
): Promise<Record<string, string>> {
  return {
    ANTHROPIC_API_KEY: apiKey,
    OPENAI_API_KEY:    apiKey,
    GEMINI_API_KEY:    apiKey,
    ORCA_AGENT_CWD:    cwd,
    ORCA_ACCOUNT_ID:   accountId,
    HOME:              process.env.HOME ?? '/tmp',
    PATH:              process.env.PATH ?? '/usr/bin:/bin',
  }
}

// ── handleAgentSpawn (fire-and-forget) ────────────────────────────────────────

export async function handleAgentSpawn(
  id:     string | number | null,
  params: Record<string, unknown>,
  config: AgentConfig,
  log:    AgentLogger,
  ws:     WebSocket,
  _state: WireState,
): Promise<void> {
  const wireState = createWireState()

  const req: AgentSpawnRequest = {
    taskId:    typeof params.taskId    === 'string' ? params.taskId    : '',
    userId:    typeof params.userId    === 'string' ? params.userId    : '',
    modelId:   typeof params.modelId   === 'string' ? params.modelId   : '',
    accountId: typeof params.accountId === 'string' ? params.accountId : '',
    cwd:       typeof params.cwd       === 'string' ? params.cwd       : config.workDir,
  }

  const span = spawnerTracer.start({ method: 'agent.spawn', taskId: req.taskId, modelId: req.modelId })

  if (!req.taskId || !req.modelId || !req.accountId) {
    span.fail('missing taskId/modelId/accountId', { taskId: req.taskId, modelId: req.modelId })
    const errFrame = JSON.stringify({
      jsonrpc: '2.0', id,
      error: { code: AgentErrorCode.InvalidParams, message: 'Missing taskId/modelId/accountId' },
    })
    ws.send(encodeDataFrame(wireState, errFrame))
    return
  }

  const spawner = new SubAgentSpawner()

  try {
    spawner.transition('spawning')

    const spec = resolveAgentSpec(req.modelId)
    const env  = await buildAgentEnv(req.accountId, 'placeholder-key', req.cwd ?? config.workDir)

    const ptyId = `${req.taskId}-${Date.now()}`
    const pty = nodePty.spawn(spec.binary, spec.args, {
      name: 'xterm-256color',
      cols: 220, rows: 50,
      cwd:  req.cwd ?? config.workDir,
      env,
    })

    PTY_REGISTRY.set(ptyId, { pty, taskId: req.taskId, userId: req.userId })
    spawner.transition('running')
    span.step('pty-running', { ptyId, modelId: req.modelId })

    log.info(`agent.spawn: ptyId=${ptyId} model=${req.modelId}`)

    pty.onData((data) => {
      const frame = JSON.stringify({ jsonrpc: '2.0', id, result: { type: 'spawn.output', ptyId, data } })
      ws.send(encodeDataFrame(wireState, frame))
    })

    pty.onExit(({ exitCode }) => {
      PTY_REGISTRY.delete(ptyId)
      spawner.transition('stopping')
      spawner.transition('stopped')
      if (exitCode === 0) {
        span.ok({ ptyId, exitCode })
      } else {
        span.fail(`exit code ${exitCode}`, { ptyId, exitCode })
      }
      const frame = JSON.stringify({ jsonrpc: '2.0', id, result: { type: 'spawn.exit', ptyId, code: exitCode } })
      ws.send(encodeDataFrame(wireState, frame))
      log.info(`agent.spawn: ptyId=${ptyId} exited code=${exitCode}`)
    })

  } catch (err: unknown) {
    spawner.transition('error')
    const msg = err instanceof Error ? err.message : String(err)
    span.fail(err, { taskId: req.taskId, modelId: req.modelId })
    log.error(`agent.spawn: error ${msg}`)
    const errWireState = createWireState()
    const frame = JSON.stringify({ jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: msg } })
    ws.send(encodeDataFrame(errWireState, frame))
  }
}

// ── handleAgentKill ───────────────────────────────────────────────────────────

export async function handleAgentKill(
  id:     string | number | null,
  params: Record<string, unknown>,
  _config: AgentConfig,
  log:    AgentLogger,
): Promise<object> {
  const ptyId = typeof params.ptyId === 'string' ? params.ptyId : ''
  const span  = spawnerTracer.start({ method: 'agent.kill', ptyId: ptyId || '(empty)' })

  if (!ptyId) {
    span.fail('missing ptyId', { method: 'agent.kill' })
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.InvalidParams, message: 'Missing ptyId' } }
  }

  const entry = PTY_REGISTRY.get(ptyId)
  if (!entry) {
    span.ok({ ptyId, note: 'already dead' })
    return { jsonrpc: '2.0', id, result: { ok: true, note: 'pty not found (already dead)' } }
  }

  entry.pty.kill('SIGTERM')
  PTY_REGISTRY.delete(ptyId)
  span.ok({ ptyId, signal: 'SIGTERM' })
  log.info(`agent.kill: ptyId=${ptyId} SIGTERM sent`)
  return { jsonrpc: '2.0', id, result: { ok: true } }
}
