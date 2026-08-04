# TDD-AG-12: ProfileAware Agent Spawner — AI Agent CLI Host (v5.0)

**Document:** TDD-AG-12 (NEW — v5.0)
**Version:** 1.0
**Date:** 2026-07-30
**Domain:** AI Agent CLI spawn, lifecycle management, OSC state machine, env isolation
**Feature:** F04, F36, F37
**ADR:** ADR-009
**HLD Ref:** C3.9, C3.11, §11 (dev-server-architecture.md)
**Related TDD:** TDD-AG-09 (AiCredStore), TDD-AG-02 (Wire Protocol)

> **Status: 🚧 In-Progress** — v5.0 new module

---

## 1. Vai trò trong hệ thống

`ProfileAwareAgentSpawner` là thành phần **trung tâm** của Dev Server Agent — nơi AI agent CLIs thực sự được spawn và quản lý:

- Nhận lệnh từ Gateway (`agent.spawn` RPC)
- Validate model whitelist theo profile
- Load encrypted credentials từ `AiCredStore`
- Build per-user environment isolation
- Spawn agent CLI via `node-pty`
- Parse OSC sequences → state machine
- Stream PTY output về Gateway
- Cleanup resources khi agent exit

> **Lưu ý — 2 hệ thống PTY khác nhau (2026-08-03):** `agent-spawner.ts` (module này) spawns and manages PTYs for **AI agent CLIs** (`agent.spawn`/`agent.kill`, exports `cleanupAllPtys`). This is a *separate* concern from the plain **terminal PTYs** created via `pty.create`/`pty.write`/`pty.scrollback`/etc, which live in `src/relay/pty-agent-bridge.ts` and export `cleanupAgentPtys` (note the similar but distinct name). Both are called from `agent-session.ts`'s `stop()` — see TDD-AG-04 §8. Do not conflate the two: this doc (TDD-AG-12) covers AI-agent PTYs only; terminal PTY streaming is covered in TDD-AG-07 §9.

---

## 2. Source File

```
src/relay/
└── agent-spawner.ts              ← [NEW v5.0] ProfileAwareAgentSpawner
```

---

## 3. AgentSpawnRequest — RPC params

```typescript
// src/relay/agent-spawner.ts

export interface AgentSpawnRequest {
  readonly model:       string          // e.g. 'claude-opus-4-5', 'gpt-4o', 'gemini-2.0-flash'
  readonly trustPreset: 'standard' | 'full' | 'none'
  readonly cwd:         string          // absolute path on dev server
  readonly taskId:      string          // ORCA_TASK_ID
  readonly userId:      string          // ORCA_USER_ID
  readonly projectId:   string          // ORCA_PROJECT_ID
  readonly accountId:   string          // AiCredStore account to load
  readonly initFile?:   string          // --init-file path (Claude Code)
  readonly extraEnv?:   Record<string, string>  // profile.shell.envOverrides
  readonly pathAdditions?: string[]     // profile.shell.pathAdditions
}
```

---

## 4. Supported AI Agent CLIs

```typescript
// src/relay/agent-spawner.ts

interface AgentBinarySpec {
  readonly binary:    string           // execFile binary name (must be in PATH)
  readonly buildArgs: (req: AgentSpawnRequest) => string[]
  readonly apiKeyEnv: string | null    // null = local inference (no key needed)
  readonly localInference?: boolean    // Ollama/vLLM — OLLAMA_HOST env
}

const AGENT_BINARY_SPECS: Record<string, AgentBinarySpec> = {
  'claude':       { binary: 'claude',    buildArgs: buildClaudeArgs,   apiKeyEnv: 'ANTHROPIC_API_KEY' },
  'claude-*':     { binary: 'claude',    buildArgs: buildClaudeArgs,   apiKeyEnv: 'ANTHROPIC_API_KEY' },
  'codex':        { binary: 'codex',     buildArgs: buildCodexArgs,    apiKeyEnv: 'OPENAI_API_KEY' },
  'gpt-*':        { binary: 'codex',     buildArgs: buildCodexArgs,    apiKeyEnv: 'OPENAI_API_KEY' },
  'gemini':       { binary: 'gemini',    buildArgs: buildGeminiArgs,   apiKeyEnv: 'GOOGLE_API_KEY' },
  'gemini-*':     { binary: 'gemini',    buildArgs: buildGeminiArgs,   apiKeyEnv: 'GOOGLE_API_KEY' },
  'opencode':     { binary: 'opencode',  buildArgs: () => [],          apiKeyEnv: null },
  'ollama-*':     { binary: 'ollama',    buildArgs: buildOllamaArgs,   apiKeyEnv: null, localInference: true },
}

function buildClaudeArgs(req: AgentSpawnRequest): string[] {
  const args = ['--print', '--trust', req.trustPreset]
  if (req.initFile) args.push('--init-file', req.initFile)
  return args
}

function buildCodexArgs(req: AgentSpawnRequest): string[] {
  return ['--model', req.model]
}

function buildGeminiArgs(req: AgentSpawnRequest): string[] {
  return ['--model', req.model]
}

function buildOllamaArgs(req: AgentSpawnRequest): string[] {
  const modelName = req.model.replace('ollama-', '')
  return ['run', modelName]
}
```

---

## 5. Environment Building (per-userId isolation)

```typescript
// src/relay/agent-spawner.ts

export async function buildAgentEnv(
  req: AgentSpawnRequest,
  spec: AgentBinarySpec,
  config: AgentConfig,
  credStore: AiCredStore
): Promise<NodeJS.ProcessEnv> {
  const home = homedir()

  // Base env từ process.env (exclude sensitive vars)
  const baseEnv: NodeJS.ProcessEnv = {
    HOME:    home,
    TERM:    'xterm-256color',
    LANG:    'en_US.UTF-8',
    // PATH: thêm ở dưới
  }

  // AI credential (chỉ load nếu spec yêu cầu)
  let apiKeyPair: Record<string, string> = {}
  if (spec.apiKeyEnv && req.accountId) {
    const cred = await credStore.readDecrypted(req.accountId)  // returns plaintext key
    if (cred) {
      apiKeyPair[spec.apiKeyEnv] = cred
    }
  }

  // Local inference (Ollama/vLLM)
  const localInferenceEnv: NodeJS.ProcessEnv = {}
  if (spec.localInference) {
    localInferenceEnv.OLLAMA_HOST = process.env.OLLAMA_HOST ?? 'http://localhost:11434'
    localInferenceEnv.OPENAI_BASE_URL = process.env.OPENAI_BASE_URL ?? 'http://localhost:8000/v1'
  }

  // PATH với pathAdditions từ profile
  const extraPaths = req.pathAdditions ?? []
  const basePath = config.toolPath
  const fullPath = [...extraPaths, ...basePath.split(':')].join(':')

  return {
    ...baseEnv,
    ...apiKeyPair,
    ...localInferenceEnv,
    ...req.extraEnv,
    // Orca context
    ORCA_PROJECT_ID:       req.projectId,
    ORCA_TASK_ID:          req.taskId,
    ORCA_USER_ID:          req.userId,
    ORCA_AGENT_HOOK_URL:   `http://localhost:${config.hookPort ?? 6800}`,
    // Per-user GitHub/GitLab isolation
    GH_CONFIG_DIR:         `${home}/.config/gh/${req.userId}/`,
    GLAB_CONFIG_DIR:       `${home}/.config/glab-cli/${req.userId}/`,
    // PATH
    PATH: fullPath,
  }
}
```

---

## 6. Agent Lifecycle State Machine

```typescript
// src/relay/agent-spawner.ts

export type AgentLifecycleState =
  | 'idle'               // PTY allocated, không có output
  | 'running'            // Agent đang chạy, có output
  | 'waiting_for_input'  // Agent chờ user input (OSC 133 prompt)
  | 'completed'          // Agent exit code 0
  | 'error'              // Agent exit code ≠ 0

export interface AgentStatusEvent {
  readonly taskId:   string
  readonly ptyId:    string
  readonly state:    AgentLifecycleState
  readonly exitCode?: number
  readonly message?:  string
}

export class AgentStateMachine {
  private state: AgentLifecycleState = 'idle'

  transition(event: 'first_output' | 'osc_prompt_open' | 'osc_prompt_close' | 'exit_ok' | 'exit_err'): AgentLifecycleState {
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
```

---

## 7. handleAgentSpawn() — RPC Handler

```typescript
// src/relay/agent-spawner.ts

export async function handleAgentSpawn(
  id: string | number | null,
  params: Record<string, unknown>,
  config: AgentConfig,
  log: AgentLogger,
  ws: WebSocket,
  wireState: WireState
): Promise<object> {
  const req = parseSpawnRequest(params)
  if (!req) {
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.InvalidParams, message: 'Invalid agent.spawn params' } }
  }

  // 1. Resolve agent binary spec
  const spec = resolveAgentSpec(req.model)
  if (!spec) {
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.InvalidParams, message: `Unknown model: ${req.model}` } }
  }

  // 2. Validate binary in PATH
  if (!isBinaryAvailable(spec.binary, config.toolPath)) {
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: `${spec.binary} not found in PATH` } }
  }

  // 3. Load credential
  const credStore = new AiCredStore(config)
  let env: NodeJS.ProcessEnv
  try {
    env = await buildAgentEnv(req, spec, config, credStore)
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.PermissionDenied, message: `Credential error: ${msg}` } }
  }

  // 4. Spawn via node-pty
  const args = spec.buildArgs(req)
  const ptyId = `pty-${req.taskId}-${Date.now()}`

  try {
    const pty = nodePty.spawn(spec.binary, args, {
      name: 'xterm-256color',
      cols: 220, rows: 50,
      cwd: req.cwd,
      env,
    })

    const stateMachine = new AgentStateMachine()
    let firstOutput = true
    const HUNG_TIMEOUT_MS = 5 * 60 * 1000  // 5 min

    let hungTimer = setTimeout(() => {
      log.warn(`agent hung (no output 5min) ptyId=${ptyId} — SIGTERM`)
      pty.kill('SIGTERM')
    }, HUNG_TIMEOUT_MS)

    // 5. Stream output → Gateway
    pty.onData((data: string) => {
      clearTimeout(hungTimer)
      hungTimer = setTimeout(() => { pty.kill('SIGTERM') }, HUNG_TIMEOUT_MS)

      if (firstOutput) {
        firstOutput = false
        stateMachine.transition('first_output')
        emitStatusEvent(ws, wireState, { taskId: req.taskId, ptyId, state: 'running' })
      }

      // Detect OSC 133 prompt boundaries
      if (data.includes('\x1b]133;A')) stateMachine.transition('osc_prompt_open')
      if (data.includes('\x1b]133;B')) stateMachine.transition('osc_prompt_close')

      // Stream PTY output
      sendFrame(ws, wireState, {
        jsonrpc: '2.0', id: null,
        result: { type: 'pty.output', ptyId, data: Buffer.from(data).toString('base64') }
      })
    })

    // 6. Handle exit
    pty.onExit(({ exitCode }) => {
      clearTimeout(hungTimer)
      const state = exitCode === 0 ? 'completed' : 'error'
      stateMachine.transition(exitCode === 0 ? 'exit_ok' : 'exit_err')
      emitStatusEvent(ws, wireState, { taskId: req.taskId, ptyId, state, exitCode })
      log.info(`agent exit: ptyId=${ptyId} exitCode=${exitCode} state=${state}`)
    })

    log.info(`agent.spawn: model=${req.model} binary=${spec.binary} ptyId=${ptyId} cwd=${req.cwd}`)
    return { jsonrpc: '2.0', id, result: { ok: true, ptyId } }

  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    log.error(`agent.spawn failed: ${msg}`)
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: msg } }
  }
}
```

---

## 8. RPC Method Registration

```typescript
// src/relay/agent-rpc-dispatch.ts (extend)

// Thêm case:
case 'agent.spawn': {
  try {
    const { handleAgentSpawn } = await import('./agent-spawner')
    return (await handleAgentSpawn(rpc.id, rpc.params ?? {}, config, log, ws, state)) as JsonRpcResponse
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    return makeError(rpc.id, AgentErrorCode.ServerError, `agent.spawn unavailable: ${msg}`)
  }
}

case 'agent.kill': {
  try {
    const { handleAgentKill } = await import('./agent-spawner')
    return (await handleAgentKill(rpc.id, rpc.params ?? {}, config, log)) as JsonRpcResponse
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    return makeError(rpc.id, AgentErrorCode.ServerError, `agent.kill unavailable: ${msg}`)
  }
}
```

---

## 9. Dependencies

```typescript
// package.json (thêm nếu chưa có)
{
  "dependencies": {
    "node-pty": "^1.0.0"   // PTY binding — cần native addon
  }
}

// tsconfig: cần "types": ["node"]
// esbuild: exclude node-pty khỏi bundle (native addon)
//   → external: ['node-pty'] trong build-relay.mjs
```

---

## 10. Tests

```typescript
// src/relay/__tests__/agent-spawner.test.ts

describe('AgentStateMachine', () => {
  it('starts in idle state', () => {
    const sm = new AgentStateMachine()
    expect(sm.current()).toBe('idle')
  })

  it('transitions idle → running on first_output', () => {
    const sm = new AgentStateMachine()
    sm.transition('first_output')
    expect(sm.current()).toBe('running')
  })

  it('transitions running → waiting_for_input on osc_prompt_open', () => {
    const sm = new AgentStateMachine()
    sm.transition('first_output')
    sm.transition('osc_prompt_open')
    expect(sm.current()).toBe('waiting_for_input')
  })

  it('transitions waiting → running on osc_prompt_close', () => {
    const sm = new AgentStateMachine()
    sm.transition('first_output')
    sm.transition('osc_prompt_open')
    sm.transition('osc_prompt_close')
    expect(sm.current()).toBe('running')
  })

  it('transitions to completed on exit_ok', () => {
    const sm = new AgentStateMachine()
    sm.transition('first_output')
    sm.transition('exit_ok')
    expect(sm.current()).toBe('completed')
  })

  it('transitions to error on exit_err', () => {
    const sm = new AgentStateMachine()
    sm.transition('first_output')
    sm.transition('exit_err')
    expect(sm.current()).toBe('error')
  })
})

describe('resolveAgentSpec', () => {
  it('resolves claude-opus-4-5 → claude binary', () => {
    const spec = resolveAgentSpec('claude-opus-4-5')
    expect(spec?.binary).toBe('claude')
    expect(spec?.apiKeyEnv).toBe('ANTHROPIC_API_KEY')
  })

  it('resolves gpt-4o → codex binary', () => {
    const spec = resolveAgentSpec('gpt-4o')
    expect(spec?.binary).toBe('codex')
  })

  it('resolves ollama-llama3 → ollama binary (local inference)', () => {
    const spec = resolveAgentSpec('ollama-llama3')
    expect(spec?.binary).toBe('ollama')
    expect(spec?.localInference).toBe(true)
    expect(spec?.apiKeyEnv).toBeNull()
  })

  it('returns undefined for unknown model', () => {
    expect(resolveAgentSpec('unknown-model-xyz')).toBeUndefined()
  })
})

describe('buildAgentEnv', () => {
  it('sets per-user GH_CONFIG_DIR', async () => {
    // mock AiCredStore
    const env = await buildAgentEnv(
      { ...baseReq, userId: 'user123' },
      claudeSpec, config, mockCredStore
    )
    expect(env.GH_CONFIG_DIR).toContain('user123')
    expect(env.GLAB_CONFIG_DIR).toContain('user123')
  })

  it('sets ORCA_TASK_ID and ORCA_PROJECT_ID', async () => {
    const env = await buildAgentEnv(
      { ...baseReq, taskId: 'task-abc', projectId: 'proj-xyz' },
      claudeSpec, config, mockCredStore
    )
    expect(env.ORCA_TASK_ID).toBe('task-abc')
    expect(env.ORCA_PROJECT_ID).toBe('proj-xyz')
  })

  it('sets OLLAMA_HOST for local inference models', async () => {
    const env = await buildAgentEnv(
      { ...baseReq }, ollamaSpec, config, mockCredStore
    )
    expect(env.OLLAMA_HOST).toBeDefined()
    expect(env.OPENAI_BASE_URL).toBeDefined()
  })

  it('does not expose API key if apiKeyEnv is null', async () => {
    const env = await buildAgentEnv(
      { ...baseReq, accountId: 'acc1' }, ollamaSpec, config, mockCredStore
    )
    expect(env.ANTHROPIC_API_KEY).toBeUndefined()
    expect(env.OPENAI_API_KEY).toBeUndefined()
  })
})
```

**Target: ≥ 20 tests**

---

## 11. Implementation Checklist

- [ ] `src/relay/agent-spawner.ts` — tạo file mới
- [ ] Interface: `AgentSpawnRequest`, `AgentBinarySpec`, `AgentStatusEvent`
- [ ] `AGENT_BINARY_SPECS` map với arg builders cho claude/codex/gemini/ollama
- [ ] `buildAgentEnv()` — per-user env isolation
- [ ] `AgentStateMachine` class — 6 state transitions
- [ ] `handleAgentSpawn()` — RPC handler (node-pty spawn)
- [ ] `handleAgentKill()` — terminate PTY by ptyId
- [ ] `src/relay/agent-rpc-dispatch.ts` — thêm `agent.spawn`, `agent.kill` cases
- [ ] `src/relay/__tests__/agent-spawner.test.ts` — tạo test file
- [ ] `package.json` — verify `node-pty` dependency
- [ ] `build-relay.mjs` — add `node-pty` to externals
