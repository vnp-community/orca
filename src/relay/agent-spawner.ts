/**
 * agent-spawner.ts — Dev Server tier SubAgent spawner (CR-AG-12)
 *
 * ⚠️ KHÔNG nhầm với src/main/project/ProfileAwareAgentSpawner.ts (Orca Server tier)
 *    Đây là relay-side spawner cho sub-agent process management.
 *
 * Exports:
 *   SubAgentSpawner        — class lifecycle manager (pure, testable)
 *   handleAgentSpawn       — RPC handler (fire-and-forget streaming)
 *   handleAgentKill        — RPC handler
 *   handleAgentSendInput   — RPC handler (write to PTY stdin)
 *   cleanupAllPtys         — cleanup PTYs on session close
 *   buildAgentEnv          — env builder (testable with mock credStore)
 *   resolveAgentSpec       — model → binary spec (pure, testable)
 *
 * @module relay/agent-spawner
 */
// node-pty is loaded lazily (dynamic import) so agent.js can start on servers
// that do NOT have node-pty installed (it is marked external in the build).
// The dynamic import happens only when agent.spawn is actually called.
import type * as nodePtyTypes from 'node-pty'
import type WebSocket from 'ws'
import type { AgentConfig } from './agent-config'
import type { AgentLogger } from './agent-logger'
import { encodeDataFrame, createWireState } from './agent-wire'
import { createTracer } from '../shared/trace'
import { readDecryptedKey } from './agent-credential-store'

const spawnerTracer = createTracer('agent:spawn')
import type { WireState } from './agent-wire'
import { AgentErrorCode } from '../shared/agent-wire-protocol'

// ── Types ──────────────────────────────────────────────────────────────────────

export type AgentLifecycleState = 'idle' | 'spawning' | 'running' | 'stopping' | 'stopped' | 'error'

/**
 * AgentBinarySpec: mô tả binary và cách build args cho mỗi model type.
 * apiKeyEnvVar = null nghĩa là model không cần API key (local inference / opencode).
 */
export interface AgentBinarySpec {
  readonly binary:         string
  readonly buildArgs:      (req?: { resumeId?: string }) => string[]
  readonly apiKeyEnvVar:   string | null
  readonly localInference?: boolean
}

export interface AgentSpawnRequest {
  taskId:        string
  userId:        string
  modelId:       string
  accountId:     string
  cwd?:          string
  resumeId?:     string  // ORCH-009: --resume <sessionId> for claude/codex
  worktreePath?: string  // WT-Issue-3: absolute path of worktree (usually same as cwd)
  branchName?:   string  // WT-Issue-3: git branch this worktree corresponds to
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

// PTY registry — keyed by ptyId, value holds the IPty instance (type-erased to any
// to avoid pulling in the static node-pty types at module load time).
// eslint-disable-next-line @typescript-eslint/no-explicit-any
const PTY_REGISTRY = new Map<string, {
  pty:    nodePtyTypes.IPty
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
//
// ORCH-012: Removed invalid --no-cache flag from claude args.
// ORCH-004: Added codex (gpt-* prefix), opencode, and ollama (local inference).
// Uses prefix-matching so claude-opus-4, gpt-4o, gemini-2.0 all resolve correctly.

const AGENT_SPECS: AgentBinarySpec[] = [
  // index 0: claude — output-format stream-json for automation; --verbose for tracing
  {
    binary: 'claude',
    buildArgs: (req) => req?.resumeId
      ? ['--resume', req.resumeId]
      : ['--output-format', 'stream-json', '--verbose'],
    apiKeyEnvVar: 'ANTHROPIC_API_KEY',
  },
  // index 1: codex / openai compatible
  {
    binary: 'codex',
    buildArgs: (req) => req?.resumeId
      ? ['--session-file', `~/.codex/${req.resumeId}.json`]
      : [],
    apiKeyEnvVar: 'OPENAI_API_KEY',
  },
  // index 2: gemini
  { binary: 'gemini',   buildArgs: () => ['--stream'], apiKeyEnvVar: 'GEMINI_API_KEY' },
  // index 3: opencode — no API key needed (uses its own auth)
  { binary: 'opencode', buildArgs: () => [],            apiKeyEnvVar: null },
  // index 4: ollama — local inference, no external API key
  { binary: 'ollama',   buildArgs: () => [],            apiKeyEnvVar: null, localInference: true },
]

const MODEL_PREFIX_MAP: Array<[prefix: string, specIndex: number]> = [
  ['claude',   0],
  ['gpt-',     1],
  ['codex',    1],
  ['gemini',   2],
  ['opencode', 3],
  ['ollama',   4],  // matches 'ollama' and 'ollama-*'
]

export function resolveAgentSpec(modelId: string): AgentBinarySpec | undefined {
  if (!modelId) return undefined
  for (const [prefix, idx] of MODEL_PREFIX_MAP) {
    if (modelId === prefix || modelId.startsWith(prefix + '-') || modelId.startsWith(prefix)) {
      return AGENT_SPECS[idx]
    }
  }
  return undefined
}

// ── buildAgentArgs helper ──────────────────────────────────────────────────────

function buildAgentArgs(spec: AgentBinarySpec, req: AgentSpawnRequest): string[] {
  return spec.buildArgs({ resumeId: req.resumeId })
}

// ── buildAgentEnv (testable with mock credStore) ──────────────────────────────
//
// ORCH-003: Removed 'placeholder-key'. Now reads the real encryptedBlob from
// the credential store (Layer 1 ciphertext from browser). Injects only the
// env var that corresponds to the specific model's provider.
//
// NOTE on credential architecture (TDD-AG-09):
//   Layer 1: Browser encrypts apiKey with SubtleCrypto → encryptedBlob
//   Layer 2: Dev Server double-encrypts encryptedBlob → .enc file
//   readDecryptedKey() decrypts Layer 2 → returns encryptedBlob (Layer 1)
//   The Orca Server is responsible for injecting resolvedApiKey (plaintext)
//   via the spawn request params when it has the Layer 1 session key.
//   If resolvedApiKey is provided, it takes priority over credStore lookup.

export interface AgentEnvRequest {
  accountId:   string
  userId:      string
  taskId:      string
  projectId?:  string
  cwd:         string
  model?:      string
  trustPreset?: string
  extraEnv?:   Record<string, string>
}

export async function buildAgentEnv(
  req:           AgentEnvRequest | AgentSpawnRequest,
  spec:          AgentBinarySpec,
  config:        AgentConfig,
  resolvedApiKey: string | null,
  log?:          AgentLogger,
): Promise<Record<string, string>> {
  // Normalise: AgentSpawnRequest uses taskId/userId/accountId directly
  const accountId  = ('accountId'  in req) ? req.accountId  : ''
  const userId     = ('userId'     in req) ? req.userId     : ''
  const taskId     = ('taskId'     in req) ? req.taskId     : ''
  const projectId  = ('projectId'  in req) ? (req as AgentEnvRequest).projectId ?? '' : ''
  const cwd        = ('cwd'        in req) ? req.cwd ?? config.workDir : config.workDir

  const base: Record<string, string> = {
    HOME:             process.env.HOME ?? '/tmp',
    PATH:             config.toolPath ?? process.env.PATH ?? '/usr/bin:/bin',
    TERM:             'xterm-256color',
    ORCA_AGENT_CWD:   cwd,
    ORCA_ACCOUNT_ID:  accountId,
    ORCA_TASK_ID:     taskId,
    ORCA_USER_ID:     userId,
    ...(projectId ? { ORCA_PROJECT_ID: projectId } : {}),
    // Per-user GitHub/GitLab config dirs to isolate credentials across agents
    GH_CONFIG_DIR:    `${process.env.HOME ?? '/tmp'}/.config/gh/${userId}/`,
    GLAB_CONFIG_DIR:  `${process.env.HOME ?? '/tmp'}/.config/glab-cli/${userId}/`,
  }

  // Inject API key for all providers (forward resolvedApiKey to all known env vars)
  // so that multi-provider agents can use whichever they need.
  if (resolvedApiKey) {
    base['ANTHROPIC_API_KEY'] = resolvedApiKey
    base['OPENAI_API_KEY']    = resolvedApiKey
    base['GEMINI_API_KEY']    = resolvedApiKey
  } else if (spec.apiKeyEnvVar && accountId) {
    // Fallback: read Layer 1 encryptedBlob from store
    const logFn = log ?? {
      info:  () => {},
      warn:  () => {},
      error: () => {},
      debug: () => {},
    }
    const blob = await readDecryptedKey(accountId, config, logFn as AgentLogger)
    if (blob) {
      base[spec.apiKeyEnvVar] = blob
      logFn.warn?.(`buildAgentEnv: injecting Layer1 blob for ${spec.apiKeyEnvVar} — agent may fail auth if key not plaintext`)
    } else {
      logFn.warn?.(`buildAgentEnv: no credential found for accountId=${accountId} — agent will fail authentication`)
    }
  }

  // Local inference servers (Ollama, vLLM, LM Studio)
  if (spec.localInference) {
    base.OLLAMA_HOST     = process.env.OLLAMA_HOST     ?? 'http://localhost:11434'
    base.OPENAI_BASE_URL = process.env.OPENAI_BASE_URL ?? 'http://localhost:8000/v1'
  }

  // Extra env overrides from caller
  const extra = ('extraEnv' in req) ? (req as AgentEnvRequest).extraEnv ?? {} : {}
  return { ...base, ...extra }
}

// ── handleAgentSpawn (fire-and-forget) ────────────────────────────────────────

export async function handleAgentSpawn(
  id:     string | number | null,
  params: Record<string, unknown>,
  config: AgentConfig,
  log:    AgentLogger,
  ws:     WebSocket,
  _state: WireState,
): Promise<Record<string, unknown>> {
  const wireState = createWireState()

  // Accept both 'model' (from tests/new spec) and 'modelId' (legacy)
  const modelId = typeof params.model   === 'string' ? params.model
                : typeof params.modelId === 'string' ? params.modelId
                : ''

  const req: AgentSpawnRequest = {
    taskId:        typeof params.taskId      === 'string' ? params.taskId      : '',
    userId:        typeof params.userId      === 'string' ? params.userId      : '',
    modelId,
    accountId:     typeof params.accountId   === 'string' ? params.accountId   : '',
    cwd:           typeof params.cwd         === 'string' ? params.cwd         : undefined,
    resumeId:      typeof params.resumeId    === 'string' ? params.resumeId    : undefined,
    worktreePath:  typeof params.worktreePath === 'string' ? params.worktreePath : undefined,
    branchName:    typeof params.branchName   === 'string' ? params.branchName   : undefined,
  }
  // ORCH-003: Orca Server may inject pre-resolved plaintext API key
  const resolvedApiKey = typeof params.resolvedApiKey === 'string' ? params.resolvedApiKey : undefined

  const span = spawnerTracer.start({ method: 'agent.spawn', taskId: req.taskId, modelId: req.modelId })

  // ── Validation ────────────────────────────────────────────────────────────────
  const missing: string[] = []
  if (!req.modelId)  missing.push('model')
  if (!req.taskId)   missing.push('taskId')
  if (!req.userId)   missing.push('userId')
  if (!req.cwd)      missing.push('cwd')

  if (missing.length > 0) {
    span.fail(`missing ${missing.join(',')}`, { taskId: req.taskId, modelId: req.modelId })
    const errResp = {
      jsonrpc: '2.0', id,
      error: { code: AgentErrorCode.InvalidParams, message: `Missing required fields: ${missing.join(', ')}` },
    }
    try { ws.send(encodeDataFrame(wireState, JSON.stringify(errResp))) } catch { /* WS may be closed */ }
    return errResp
  }

  // Resolve spec
  const specResolved = resolveAgentSpec(req.modelId)
  if (!specResolved) {
    span.fail('unknown model', { modelId: req.modelId })
    const errResp = {
      jsonrpc: '2.0', id,
      error: { code: AgentErrorCode.InvalidParams, message: `Unknown model: ${req.modelId}` },
    }
    try { ws.send(encodeDataFrame(wireState, JSON.stringify(errResp))) } catch { /* WS may be closed */ }
    return errResp
  }

  const spawner = new SubAgentSpawner()

  try {
    spawner.transition('spawning')

    // specResolved already validated above (not null)
    const spec = specResolved
    // ORCH-003: Pass spec and resolvedApiKey — no more 'placeholder-key'
    const envBase = await buildAgentEnv(
      req,
      spec,
      config,
      resolvedApiKey ?? null,
      log,
    )
    // WT-Issue-3: Inject worktree context so agent knows which branch/path it owns
    const env: Record<string, string> = {
      ...envBase,
      ...(req.worktreePath ? { ORCA_WORKTREE_PATH: req.worktreePath } : {}),
      ...(req.branchName   ? { ORCA_WORKTREE_BRANCH: req.branchName  } : {}),
    }

    // ORCH-011: ptyId includes userId to prevent cross-user collision
    const ptyId = `pty-${req.userId}-${req.taskId}-${Date.now()}`
    // ORCH-012/004: Use buildAgentArgs (handles resume + correct args per model)
    const args = buildAgentArgs(spec, req)

    // Pre-spawn validation: check binary exists in toolPath to fail fast + synchronously
    const { existsSync: fsExistsSync } = await import('node:fs')
    const { join: pathJoin } = await import('node:path')
    const toolPathDirs = (config.toolPath ?? '').split(':').filter(Boolean)
    const binaryExists = process.platform === 'win32'
      ? true  // Windows PATH lookup is different — skip check
      : toolPathDirs.some((dir) => fsExistsSync(pathJoin(dir, spec.binary)))
        || !toolPathDirs.length  // no toolPath configured → use system PATH (always ok)

    if (!binaryExists) {
      throw new Error(
        `Agent binary '${spec.binary}' not found in toolPath '${config.toolPath ?? '(empty)'}'. ` +
        `Install it or set toolPath to the directory containing '${spec.binary}'.`
      )
    }

    // Lazy-load node-pty — only imported when a PTY spawn is actually needed.
    // This allows agent.js to start on servers without node-pty installed.
    let nodePty: typeof nodePtyTypes
    try {
      nodePty = await import('node-pty')
    } catch {
      throw new Error(
        `node-pty is not installed on this dev server. ` +
        `Run: npm install node-pty  (in ~/orca-agent/)  to enable PTY-based agent spawning.`
      )
    }

    const pty = nodePty.spawn(spec.binary, args, {
      name: 'xterm-256color',
      cols: 220, rows: 50,
      cwd:  req.cwd ?? config.workDir,
      env,
    })

    PTY_REGISTRY.set(ptyId, { pty, taskId: req.taskId, userId: req.userId })
    spawner.transition('running')
    span.step('pty-running', { ptyId, modelId: req.modelId })

    log.info(`agent.spawn: ptyId=${ptyId} model=${req.modelId}`)

    // ORCH-006: JSON-RPC 2.0 requires notifications (no id) for streaming output.
    // Sending multiple responses with the same id violates the spec.
    pty.onData((data) => {
      const notification = JSON.stringify({
        jsonrpc: '2.0',
        method: 'agent.output',
        params: { ptyId, data: Buffer.from(data).toString('base64') },
      })
      ws.send(encodeDataFrame(wireState, notification))
    })

    // ORCH-006: onExit also uses notification
    pty.onExit(({ exitCode }) => {
      PTY_REGISTRY.delete(ptyId)
      spawner.transition('stopping')
      spawner.transition('stopped')
      if (exitCode === 0) {
        span.ok({ ptyId, exitCode })
      } else {
        span.fail(`exit code ${exitCode}`, { ptyId, exitCode })
      }
      const notification = JSON.stringify({
        jsonrpc: '2.0',
        method: 'agent.exited',
        params: { ptyId, exitCode },
      })
      ws.send(encodeDataFrame(wireState, notification))
      log.info(`agent.spawn: ptyId=${ptyId} exited code=${exitCode}`)
    })

    return { jsonrpc: '2.0', id, result: { ok: true, ptyId } }

  } catch (err: unknown) {
    spawner.transition('error')
    const msg = err instanceof Error ? err.message : String(err)
    span.fail(err, { taskId: req.taskId, modelId: req.modelId })
    log.error(`agent.spawn: error ${msg}`)
    const errWireState = createWireState()
    const errResp = { jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: msg } }
    ws.send(encodeDataFrame(errWireState, JSON.stringify(errResp)))
    return errResp
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
  // ORCH-002: Respect the caller's signal choice (SIGTERM = graceful, SIGKILL = force)
  const rawSignal = typeof params.signal === 'string' ? params.signal : 'SIGTERM'
  const signal: 'SIGTERM' | 'SIGKILL' = rawSignal === 'SIGKILL' ? 'SIGKILL' : 'SIGTERM'
  const span  = spawnerTracer.start({ method: 'agent.kill', ptyId: ptyId || '(empty)', signal })

  if (!ptyId) {
    span.fail('missing ptyId', { method: 'agent.kill' })
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.InvalidParams, message: 'Missing ptyId' } }
  }

  const entry = PTY_REGISTRY.get(ptyId)
  if (!entry) {
    span.ok({ ptyId, note: 'already dead' })
    return { jsonrpc: '2.0', id, result: { ok: true, note: 'pty not found (already dead)' } }
  }

  // ORCH-002: Use the validated signal from params
  if (process.platform === 'win32') {
    entry.pty.kill()  // Windows: no signal semantics
  } else {
    entry.pty.kill(signal)
  }
  PTY_REGISTRY.delete(ptyId)
  span.ok({ ptyId, signal })
  log.info(`agent.kill: ptyId=${ptyId} ${signal} sent`)
  return { jsonrpc: '2.0', id, result: { ok: true } }
}

// ── handleAgentSendInput ──────────────────────────────────────────────────────
// ORCH-001: New handler for sending data to PTY stdin.
// Used for graceful stop (send '\x03' = Ctrl+C) and arbitrary terminal input.

export async function handleAgentSendInput(
  id:     string | number | null,
  params: Record<string, unknown>,
  _config: AgentConfig,
  log:    AgentLogger,
): Promise<object> {
  const ptyId = typeof params.ptyId === 'string' ? params.ptyId : ''
  const data  = typeof params.data  === 'string' ? params.data  : ''

  if (!ptyId) {
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.InvalidParams, message: 'Missing ptyId' } }
  }

  const entry = PTY_REGISTRY.get(ptyId)
  if (!entry) {
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.PathNotFound, message: `PTY not found: ${ptyId}` } }
  }

  try {
    entry.pty.write(data)
    log.info(`agent.sendInput: ptyId=${ptyId} bytes=${data.length}`)
    return { jsonrpc: '2.0', id, result: { ok: true } }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    log.error(`agent.sendInput failed: ${msg}`)
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: msg } }
  }
}

// ── cleanupAllPtys ───────────────────────────────────────────────────────────
// ORCH-011: Kill all PTYs in registry when the WS session closes.
// Prevents orphaned agent processes consuming resources on the Dev Server.

export function cleanupAllPtys(log: AgentLogger): void {
  if (PTY_REGISTRY.size === 0) return
  log.info(`session.stop: cleaning up ${PTY_REGISTRY.size} orphaned PTY(s)`)
  for (const [ptyId, entry] of PTY_REGISTRY.entries()) {
    try {
      if (process.platform === 'win32') {
        entry.pty.kill()
      } else {
        entry.pty.kill('SIGTERM')
      }
      log.info(`session.stop: killed PTY ${ptyId}`)
    } catch (err) {
      log.warn(`session.stop: failed to kill PTY ${ptyId}: ${err}`)
    }
  }
  PTY_REGISTRY.clear()
}
