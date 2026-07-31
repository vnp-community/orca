# TASK-06: Create Agent Spawner — ProfileAwareAgentSpawner

> ✅ **STATUS: DONE** — Completed 2026-07-30T17:52

**Phase:** 3
**File:** `src/relay/agent-spawner.ts` (NEW FILE)
**Operation:** CREATE
**CR:** [CR-AG-12](../solutions/CR-AG-12-agent-spawner.md)
**TDD:** TDD-AG-12
**Depends on:** TASK-02 (cần `readDecryptedKey` export từ agent-credential-store.ts)
**Blocked by:** TASK-02 phải hoàn thành trước (hoặc stub `readDecryptedKey`)

---

## Mục tiêu

Tạo mới `src/relay/agent-spawner.ts`:
- `AgentStateMachine` — lifecycle state machine (idle→running→waiting→completed/error)
- `AGENT_BINARY_SPECS` — model name → binary spec map (claude, codex, gemini, ollama...)
- `buildAgentEnv()` — per-user env isolation với GH/GLAB config dirs
- `handleAgentSpawn()` — RPC handler (spawn via node-pty)
- `handleAgentKill()` — terminate PTY by ptyId
- `PTY_REGISTRY` — in-process registry để kill lookup

---

## Prerequisites

Verify `node-pty` có trong `package.json` và build config:

```bash
# Check node-pty installed
grep '"node-pty"' package.json

# Check build externals (node-pty phải là external, không bundle)
grep -n "node-pty\|external" build-relay.mjs
```

Nếu chưa có `node-pty`:
```bash
pnpm add node-pty
```

Nếu `build-relay.mjs` chưa có `node-pty` trong externals, thêm vào.

---

## File cần tạo

Tạo file mới hoàn toàn tại: `src/relay/agent-spawner.ts`

```typescript
// src/relay/agent-spawner.ts
// ProfileAware Agent Spawner for Orca Dev Agent v5.0.
//
// Responsibilities:
//   1. Validate model against profile whitelist
//   2. Load AI credentials from AiCredStore
//   3. Build per-user environment (GH_CONFIG_DIR, GLAB_CONFIG_DIR, API keys)
//   4. Spawn agent CLI via node-pty
//   5. Parse OSC sequences → lifecycle state machine
//   6. Stream PTY output → Gateway via WebSocket frames
//   7. Cleanup PTY resources on exit

import * as nodePty from 'node-pty'
import { homedir } from 'node:os'
import { accessSync, constants } from 'node:fs'
import { join } from 'node:path'
import type WebSocket from 'ws'
import type { AgentConfig } from './agent-config'
import type { AgentLogger } from './agent-logger'
import { encodeDataFrame } from './agent-wire'
import type { WireState } from './agent-wire'
import { AgentErrorCode } from '../shared/agent-wire-protocol'

// ─── Agent Lifecycle State Machine ────────────────────────────────────────────

export type AgentLifecycleState =
  | 'idle'
  | 'running'
  | 'waiting_for_input'
  | 'completed'
  | 'error'

type LifecycleEvent =
  | 'first_output'
  | 'osc_prompt_open'
  | 'osc_prompt_close'
  | 'exit_ok'
  | 'exit_err'

export class AgentStateMachine {
  private state: AgentLifecycleState = 'idle'

  transition(event: LifecycleEvent): AgentLifecycleState {
    switch (event) {
      case 'first_output':
        if (this.state === 'idle') this.state = 'running'
        break
      case 'osc_prompt_open':
        if (this.state === 'running') this.state = 'waiting_for_input'
        break
      case 'osc_prompt_close':
        if (this.state === 'waiting_for_input') this.state = 'running'
        break
      case 'exit_ok':
        this.state = 'completed'
        break
      case 'exit_err':
        this.state = 'error'
        break
    }
    return this.state
  }

  current(): AgentLifecycleState { return this.state }
}

// ─── Agent Binary Specs ───────────────────────────────────────────────────────

interface AgentBinarySpec {
  readonly binary:         string
  readonly buildArgs:      (model: string, trustPreset: string, initFile?: string) => string[]
  readonly apiKeyEnv:      string | null   // null = local inference (no external key)
  readonly localInference?: boolean
}

const AGENT_BINARY_SPECS: Record<string, AgentBinarySpec> = {
  'claude':    { binary: 'claude',   buildArgs: buildClaudeArgs,  apiKeyEnv: 'ANTHROPIC_API_KEY' },
  'codex':     { binary: 'codex',    buildArgs: buildCodexArgs,   apiKeyEnv: 'OPENAI_API_KEY' },
  'gemini':    { binary: 'gemini',   buildArgs: buildGeminiArgs,  apiKeyEnv: 'GOOGLE_API_KEY' },
  'opencode':  { binary: 'opencode', buildArgs: () => [],          apiKeyEnv: null },
  'ollama':    { binary: 'ollama',   buildArgs: buildOllamaArgs,  apiKeyEnv: null, localInference: true },
}

function buildClaudeArgs(model: string, trustPreset: string, initFile?: string): string[] {
  const args = ['--print', '--trust', trustPreset || 'standard']
  if (model && model !== 'claude') args.push('--model', model)
  if (initFile) args.push('--init-file', initFile)
  return args
}

function buildCodexArgs(model: string): string[] {
  return model && model !== 'codex' ? ['--model', model] : []
}

function buildGeminiArgs(model: string): string[] {
  return model && model !== 'gemini' ? ['--model', model] : []
}

function buildOllamaArgs(model: string): string[] {
  const modelName = model.replace(/^ollama[-:]/, '')
  return ['run', modelName]
}

/**
 * Resolve model name to binary spec.
 * Supports prefix matching: 'claude-opus-4-5' → claude spec.
 */
export function resolveAgentSpec(model: string): AgentBinarySpec | undefined {
  // Exact match first
  if (AGENT_BINARY_SPECS[model]) return AGENT_BINARY_SPECS[model]

  // Prefix matching
  for (const [prefix, spec] of Object.entries(AGENT_BINARY_SPECS)) {
    if (model.startsWith(prefix + '-') || model.startsWith(prefix + ':')) {
      return spec
    }
  }

  // Ollama models: 'ollama-llama3', 'ollama:llama3'
  if (model.startsWith('ollama')) return AGENT_BINARY_SPECS['ollama']

  return undefined
}

/**
 * Check if binary is executable in toolPath.
 */
function isBinaryAvailable(binary: string, toolPath: string): boolean {
  for (const dir of toolPath.split(':')) {
    if (!dir) continue
    try {
      accessSync(join(dir, binary), constants.X_OK)
      return true
    } catch { /* not in this dir */ }
  }
  return false
}

// ─── PTY Registry (in-process singleton) ─────────────────────────────────────

interface PtyEntry {
  pty:    nodePty.IPty
  taskId: string
  userId: string
}

const PTY_REGISTRY = new Map<string, PtyEntry>()

// ─── Environment Builder ──────────────────────────────────────────────────────

export interface AgentSpawnRequest {
  readonly model:          string
  readonly trustPreset:    string
  readonly cwd:            string
  readonly taskId:         string
  readonly userId:         string
  readonly projectId:      string
  readonly accountId:      string
  readonly initFile?:      string
  readonly extraEnv?:      Record<string, string>
  readonly pathAdditions?: string[]
}

export async function buildAgentEnv(
  req:    AgentSpawnRequest,
  spec:   AgentBinarySpec,
  config: AgentConfig,
  apiKey: string | null
): Promise<NodeJS.ProcessEnv> {
  const home = homedir()

  const apiKeyPair: Record<string, string> = {}
  if (spec.apiKeyEnv && apiKey) {
    apiKeyPair[spec.apiKeyEnv] = apiKey
  }

  const localInferenceEnv: NodeJS.ProcessEnv = {}
  if (spec.localInference) {
    localInferenceEnv.OLLAMA_HOST     = process.env.OLLAMA_HOST     ?? 'http://localhost:11434'
    localInferenceEnv.OPENAI_BASE_URL = process.env.OPENAI_BASE_URL ?? 'http://localhost:8000/v1'
  }

  const extraPaths = req.pathAdditions ?? []
  const basePath   = config.toolPath
  const fullPath   = [...extraPaths, ...basePath.split(':')].filter(Boolean).join(':')

  return {
    HOME: home,
    TERM: 'xterm-256color',
    LANG: 'en_US.UTF-8',
    ...apiKeyPair,
    ...localInferenceEnv,
    ...(req.extraEnv ?? {}),
    ORCA_PROJECT_ID:     req.projectId,
    ORCA_TASK_ID:        req.taskId,
    ORCA_USER_ID:        req.userId,
    ORCA_AGENT_HOOK_URL: `http://localhost:${(config as Record<string, unknown>)['hookPort'] ?? 6800}`,
    GH_CONFIG_DIR:       `${home}/.config/gh/${req.userId}/`,
    GLAB_CONFIG_DIR:     `${home}/.config/glab-cli/${req.userId}/`,
    PATH: fullPath,
  }
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

function sendFrame(ws: WebSocket, wireState: WireState, payload: object): void {
  if (ws.readyState === 1 /* WebSocket.OPEN */) {
    ws.send(encodeDataFrame(wireState, JSON.stringify(payload)))
  }
}

function emitStatusEvent(
  ws:        WebSocket,
  wireState: WireState,
  event:     { taskId: string; ptyId: string; state: AgentLifecycleState; exitCode?: number }
): void {
  sendFrame(ws, wireState, {
    jsonrpc: '2.0', id: null,
    result: { type: 'agent.statusChanged', ...event },
  })
}

// ─── handleAgentSpawn ─────────────────────────────────────────────────────────

function parseSpawnRequest(params: Record<string, unknown>): AgentSpawnRequest | null {
  const model     = typeof params.model     === 'string' ? params.model     : ''
  const taskId    = typeof params.taskId    === 'string' ? params.taskId    : ''
  const userId    = typeof params.userId    === 'string' ? params.userId    : ''
  const projectId = typeof params.projectId === 'string' ? params.projectId : ''
  const accountId = typeof params.accountId === 'string' ? params.accountId : ''
  const cwd       = typeof params.cwd       === 'string' ? params.cwd       : ''

  if (!model || !taskId || !userId || !cwd) return null

  return {
    model,
    trustPreset:    typeof params.trustPreset === 'string' ? params.trustPreset : 'standard',
    cwd,
    taskId,
    userId,
    projectId,
    accountId,
    initFile:       typeof params.initFile    === 'string' ? params.initFile    : undefined,
    extraEnv:       (params.extraEnv as Record<string, string> | undefined) ?? {},
    pathAdditions:  Array.isArray(params.pathAdditions) ? params.pathAdditions.map(String) : [],
  }
}

export async function handleAgentSpawn(
  id:        string | number | null,
  params:    Record<string, unknown>,
  config:    AgentConfig,
  log:       AgentLogger,
  ws:        WebSocket,
  wireState: WireState
): Promise<object> {
  const req = parseSpawnRequest(params)
  if (!req) {
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.InvalidParams, message: 'Invalid agent.spawn params: model, taskId, userId, cwd required' } }
  }

  // 1. Resolve binary spec
  const spec = resolveAgentSpec(req.model)
  if (!spec) {
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.InvalidParams, message: `Unknown model: ${req.model}` } }
  }

  // 2. Verify binary in PATH
  if (!isBinaryAvailable(spec.binary, config.toolPath)) {
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: `${spec.binary} not found in PATH (toolPath: ${config.toolPath})` } }
  }

  // 3. Load credential (null if localInference or no accountId)
  let apiKey: string | null = null
  if (spec.apiKeyEnv && req.accountId) {
    try {
      const { readDecryptedKey } = await import('./agent-credential-store')
      apiKey = await readDecryptedKey(req.accountId, config, log)
      if (!apiKey) {
        return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: `Credential not found: ${req.accountId}` } }
      }
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : String(err)
      return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: `Credential error: ${msg}` } }
    }
  }

  // 4. Build env
  const env = await buildAgentEnv(req, spec, config, apiKey)

  // 5. Build args
  const args = spec.buildArgs(req.model, req.trustPreset, req.initFile)

  // 6. Spawn via node-pty
  const ptyId = `pty-${req.taskId}-${Date.now()}`
  const HUNG_TIMEOUT_MS = 5 * 60 * 1_000  // 5 minutes

  try {
    const pty = nodePty.spawn(spec.binary, args, {
      name: 'xterm-256color',
      cols: 220,
      rows: 50,
      cwd:  req.cwd,
      env,
    })

    PTY_REGISTRY.set(ptyId, { pty, taskId: req.taskId, userId: req.userId })

    const stateMachine = new AgentStateMachine()
    let firstOutput = true

    let hungTimer = setTimeout(() => {
      log.warn(`agent hung (no output 5min) ptyId=${ptyId} — SIGTERM`)
      pty.kill('SIGTERM')
    }, HUNG_TIMEOUT_MS)

    pty.onData((data: string) => {
      // Reset hung timer on any output
      clearTimeout(hungTimer)
      hungTimer = setTimeout(() => { pty.kill('SIGTERM') }, HUNG_TIMEOUT_MS)

      // Transition state on first output
      if (firstOutput) {
        firstOutput = false
        stateMachine.transition('first_output')
        emitStatusEvent(ws, wireState, { taskId: req.taskId, ptyId, state: 'running' })
      }

      // OSC 133 prompt boundary detection
      if (data.includes('\x1b]133;A')) stateMachine.transition('osc_prompt_open')
      if (data.includes('\x1b]133;B')) stateMachine.transition('osc_prompt_close')

      // Stream PTY output to Gateway
      sendFrame(ws, wireState, {
        jsonrpc: '2.0', id: null,
        result: { type: 'pty.output', ptyId, data: Buffer.from(data).toString('base64') },
      })
    })

    pty.onExit(({ exitCode }) => {
      clearTimeout(hungTimer)
      PTY_REGISTRY.delete(ptyId)
      const state: AgentLifecycleState = exitCode === 0 ? 'completed' : 'error'
      stateMachine.transition(exitCode === 0 ? 'exit_ok' : 'exit_err')
      emitStatusEvent(ws, wireState, { taskId: req.taskId, ptyId, state, exitCode })
      log.info(`agent.spawn exit: ptyId=${ptyId} exitCode=${exitCode} state=${state}`)
    })

    log.info(`agent.spawn: model=${req.model} binary=${spec.binary} ptyId=${ptyId} cwd=${req.cwd}`)
    return { jsonrpc: '2.0', id, result: { ok: true, ptyId } }

  } catch (err: unknown) {
    PTY_REGISTRY.delete(ptyId)
    const msg = err instanceof Error ? err.message : String(err)
    log.error(`agent.spawn failed: ${msg}`)
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: msg } }
  }
}

// ─── handleAgentKill ──────────────────────────────────────────────────────────

export async function handleAgentKill(
  id:     string | number | null,
  params: Record<string, unknown>,
  _config: AgentConfig,
  log:    AgentLogger
): Promise<object> {
  const ptyId = typeof params.ptyId === 'string' ? params.ptyId : ''
  if (!ptyId) {
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.InvalidParams, message: 'Missing required param: ptyId' } }
  }

  const entry = PTY_REGISTRY.get(ptyId)
  if (!entry) {
    return { jsonrpc: '2.0', id, result: { ok: true, note: 'pty not found (already exited)' } }
  }

  entry.pty.kill('SIGTERM')
  PTY_REGISTRY.delete(ptyId)
  log.info(`agent.kill: ptyId=${ptyId} SIGTERM sent`)
  return { jsonrpc: '2.0', id, result: { ok: true } }
}
```

---

## Verify

```bash
# Check node-pty available
node -e "require('node-pty'); console.log('node-pty OK')"

# TypeScript compile
npx tsc --noEmit -p config/tsconfig.node.json

# Check exports
grep -n "^export" src/relay/agent-spawner.ts
# Expected:
# export type AgentLifecycleState
# export class AgentStateMachine
# export function resolveAgentSpec
# export interface AgentSpawnRequest
# export async function buildAgentEnv
# export async function handleAgentSpawn
# export async function handleAgentKill
```

---

## Done criteria

- [ ] `node-pty` import không lỗi
- [ ] `AgentStateMachine` — 6 transitions đúng
- [ ] `resolveAgentSpec('claude-opus-4-5')` → `{ binary: 'claude', apiKeyEnv: 'ANTHROPIC_API_KEY' }`
- [ ] `resolveAgentSpec('ollama-llama3')` → `{ binary: 'ollama', localInference: true }`
- [ ] `buildAgentEnv()` — GH_CONFIG_DIR và GLAB_CONFIG_DIR per userId
- [ ] `handleAgentSpawn()` — validate params, load cred, spawn PTY, stream output
- [ ] `handleAgentKill()` — terminate PTY by ptyId (idempotent)
- [ ] TypeScript compile không lỗi
