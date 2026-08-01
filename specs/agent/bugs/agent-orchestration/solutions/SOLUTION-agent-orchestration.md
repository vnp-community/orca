# SOLUTION: Agent Orchestration Domain — Fix tất cả Bugs

**Domain:** agent-orchestration  
**TDD Reference:** TDD-AG-12 (ProfileAware Agent Spawner), TDD-AG-07 (JSON-RPC Dispatch), TDD-AG-03 (Connection Modes)  
**Files cần thay đổi:** `src/relay/agent-rpc-dispatch.ts`, `src/relay/agent-spawner.ts`, `src/relay/agent-connection-relay.ts`, `src/main/agent/AgentManager.ts` (new), `src/main/agent/AgentHookParser.ts` (new), `src/main/db/migrations/XXXX_add_agent_sessions.ts` (new)  
**Tổng số bugs:** 13 (ORCH-001 → ORCH-013)

---

## Tổng quan phụ thuộc

```
ORCH-005 (AgentManager missing)
    ├── ORCH-006 (output stream handler missing)  ← phải fix trước ORCH-007, ORCH-010
    ├── ORCH-007 (AgentHookParser missing)         ← phải fix trước ORCH-010
    ├── ORCH-008 (wrong DB schema)
    ├── ORCH-009 (resume not implemented)
    └── ORCH-010 (switch account not implemented)

ORCH-001 (sendInput missing) — độc lập, fix relay side
ORCH-002 (kill signal wrong) — độc lập, fix relay side
ORCH-003 (placeholder API key) — phụ thuộc AIP-002
ORCH-004 (missing codex/opencode) — độc lập, fix relay side
ORCH-011 (PTY orphaned on disconnect) — độc lập, fix relay side
ORCH-012 (claude args invalid) — độc lập, fix relay side
ORCH-013 (hardcoded relay token) — độc lập, fix connection-relay side
```

**Thứ tự fix khuyến nghị:**  
`012 → 004 → 002 → 013 → 001 → 011 → 003 → 006 → 007 → 008 → 005 → 009 → 010`

---

## BUG-AG-ORCH-012 — Fix Claude args `--no-cache` invalid

**File:** `src/relay/agent-spawner.ts`  
**Mức độ:** 🟡 MEDIUM  
**Root cause:** `--no-cache` không phải flag hợp lệ của Claude CLI.

```typescript
// BEFORE (agent-spawner.ts ~line 81):
export function resolveAgentSpec(modelId: string): { binary: string; args: string[] } {
  if (modelId.startsWith('claude')) {
    return { binary: 'claude', args: ['--output-format', 'stream-json', '--no-cache'] }
  }
  if (modelId.startsWith('gemini')) {
    return { binary: 'gemini', args: ['--stream'] }
  }
  throw new Error(`resolveAgentSpec: unknown modelId: ${modelId}`)
}

// AFTER — Theo TDD-AG-12 §4 AGENT_BINARY_SPECS:
interface AgentBinarySpec {
  readonly binary:    string
  readonly buildArgs: (req: AgentSpawnRequest) => string[]
  readonly apiKeyEnv: string | null
  readonly localInference?: boolean
}

const AGENT_BINARY_SPECS: Record<string, AgentBinarySpec> = {
  'claude':    { binary: 'claude',    buildArgs: buildClaudeArgs,   apiKeyEnv: 'ANTHROPIC_API_KEY' },
  'claude-':   { binary: 'claude',    buildArgs: buildClaudeArgs,   apiKeyEnv: 'ANTHROPIC_API_KEY' },
  'codex':     { binary: 'codex',     buildArgs: buildCodexArgs,    apiKeyEnv: 'OPENAI_API_KEY' },
  'gpt-':      { binary: 'codex',     buildArgs: buildCodexArgs,    apiKeyEnv: 'OPENAI_API_KEY' },
  'gemini':    { binary: 'gemini',    buildArgs: buildGeminiArgs,   apiKeyEnv: 'GOOGLE_API_KEY' },
  'gemini-':   { binary: 'gemini',    buildArgs: buildGeminiArgs,   apiKeyEnv: 'GOOGLE_API_KEY' },
  'opencode':  { binary: 'opencode',  buildArgs: () => [],          apiKeyEnv: null },
  'ollama-':   { binary: 'ollama',    buildArgs: buildOllamaArgs,   apiKeyEnv: null, localInference: true },
}

function buildClaudeArgs(req: AgentSpawnRequest): string[] {
  // Claude CLI valid flags: --print, --trust, --output-format, --verbose, --resume, --init-file
  // NOTE: --no-cache là INVALID — đã xóa
  const args: string[] = ['--output-format', 'stream-json', '--verbose']
  if (req.trustPreset && req.trustPreset !== 'none') {
    args.push('--trust', req.trustPreset)
  }
  if (req.initFile) {
    args.push('--init-file', req.initFile)
  }
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

/**
 * Resolve agent binary spec từ modelId.
 * Hỗ trợ exact match và prefix match.
 * Returns undefined nếu không tìm thấy.
 */
export function resolveAgentSpec(modelId: string): AgentBinarySpec | undefined {
  if (AGENT_BINARY_SPECS[modelId]) return AGENT_BINARY_SPECS[modelId]
  for (const [key, spec] of Object.entries(AGENT_BINARY_SPECS)) {
    if (key.endsWith('-') && modelId.startsWith(key)) return spec
  }
  return undefined
}
```

---

## BUG-AG-ORCH-004 — Fix `resolveAgentSpec` missing Codex và OpenCode

Đã được cover bởi fix ORCH-012 ở trên. `AGENT_BINARY_SPECS` bao gồm đầy đủ `codex`, `gpt-*`, `opencode`.

**Bổ sung: buildResumeArgs helper:**

```typescript
export function buildResumeArgs(agentType: string, sessionId: string): string[] {
  if (agentType === 'claude') return ['--resume', sessionId]
  if (agentType === 'codex')  return ['--session-file', `~/.codex/${sessionId}.json`]
  return []
}
```

---

## BUG-AG-ORCH-002 — Fix `agent.kill` dùng SIGTERM thay vì SIGKILL

**File:** `src/relay/agent-spawner.ts`

```typescript
// BEFORE (agent-spawner.ts ~line 215):
entry.pty.kill('SIGTERM')  // ← luôn SIGTERM, bỏ qua params.signal

// AFTER:
export async function handleAgentKill(
  id: string | number | null,
  params: Record<string, unknown>,
  _config: AgentConfig,
  log: AgentLogger,
): Promise<object> {
  const ptyId  = typeof params.ptyId   === 'string' ? params.ptyId   : ''
  const signal = (params.signal === 'SIGKILL' ? 'SIGKILL' : 'SIGTERM') as 'SIGTERM' | 'SIGKILL'

  const entry = PTY_REGISTRY.get(ptyId)
  if (!entry) {
    log.warn(`agent.kill: ptyId=${ptyId} not found (already dead?)`)
    return { jsonrpc: '2.0', id, result: { ok: true, alreadyDead: true } }
  }

  try {
    entry.pty.kill(signal)  // ← Dùng signal từ params (validated)
    PTY_REGISTRY.delete(ptyId)
    log.info(`agent.kill: ptyId=${ptyId} signal=${signal}`)
    return { jsonrpc: '2.0', id, result: { ok: true } }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    return { jsonrpc: '2.0', id, error: { code: -32603, message: msg } }
  }
}
```

---

## BUG-AG-ORCH-013 — Fix relay-ws hardcoded token `'relay-secret'`

**File:** `src/relay/agent-connection-relay.ts`

```typescript
// BEFORE (agent-connection-relay.ts:26):
const token = config.agentToken || 'relay-secret'

// AFTER:
export async function listenRelay(
  config: AgentConfig,
  tools: ToolDefinition[],
  log: AgentLogger
): Promise<never> {
  const token = config.agentToken?.trim()
  if (!token) {
    log.error('AGENT_TOKEN is required for relay-websocket mode.')
    log.error('Set ORCA_AGENT_TOKEN environment variable on the Dev Server.')
    log.error('Example: export ORCA_AGENT_TOKEN=$(openssl rand -hex 32)')
    process.exit(1)
  }

  return new Promise<never>((_, reject) => {
    const wss = new WebSocketServer({ port: config.agentPort, path: '/orca-relay' })

    wss.once('listening', () => {
      log.info(`Ready: ws://0.0.0.0:${config.agentPort}/orca-relay`)
      // SECURITY: Không log token ra console
      log.info(`In Orca UI: Type=relay-websocket  URL=ws://${config.devServerId}:${config.agentPort}/orca-relay`)
    })

    wss.on('connection', (ws: WebSocket, req: IncomingMessage) => {
      if (!authenticate(ws, req, token, log)) return
      log.info(`Orca connected from ${req.socket.remoteAddress}`)
      const session = createSession(config, tools, log)
      session.start(ws)
      ws.once('close', () => session.stop())
    })

    wss.once('error', (err) => { reject(err) })
  })
}
```

---

## BUG-AG-ORCH-001 — Fix `agent.sendInput` missing trong RPC dispatch

**Files:** `src/relay/agent-rpc-dispatch.ts`, `src/relay/agent-spawner.ts`

### agent-spawner.ts — thêm handleAgentSendInput:

```typescript
export async function handleAgentSendInput(
  id: string | number | null,
  params: Record<string, unknown>,
  _config: AgentConfig,
  log: AgentLogger,
): Promise<object> {
  const ptyId = typeof params.ptyId === 'string' ? params.ptyId : ''
  const data  = typeof params.data  === 'string' ? params.data  : ''

  if (!ptyId) {
    return { jsonrpc: '2.0', id, error: { code: -32602, message: 'Missing ptyId' } }
  }

  const entry = PTY_REGISTRY.get(ptyId)
  if (!entry) {
    return { jsonrpc: '2.0', id, error: { code: -32004, message: `PTY not found: ${ptyId}` } }
  }

  try {
    entry.pty.write(data)
    log.info(`agent.sendInput: ptyId=${ptyId} bytes=${data.length}`)
    return { jsonrpc: '2.0', id, result: { ok: true } }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    return { jsonrpc: '2.0', id, error: { code: -32603, message: msg } }
  }
}
```

### agent-rpc-dispatch.ts — thêm case:

```typescript
case 'agent.sendInput': {
  try {
    const { handleAgentSendInput } = await import('./agent-spawner')
    return (await handleAgentSendInput(rpc.id, rpc.params ?? {}, config, log)) as JsonRpcResponse
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    return makeError(rpc.id, AgentErrorCode.ServerError, `agent.sendInput unavailable: ${msg}`)
  }
}
```

---

## BUG-AG-ORCH-011 — Fix PTY_REGISTRY orphaned PTYs khi WS disconnect

**Files:** `src/relay/agent-spawner.ts`, `src/relay/agent-session.ts`

### agent-spawner.ts — export cleanup + fix ptyId isolation:

```typescript
// Cleanup function:
export function cleanupSessionPtys(sessionId: string, log: AgentLogger): void {
  const ptyIds = [...PTY_REGISTRY.keys()]
  for (const ptyId of ptyIds) {
    try {
      const entry = PTY_REGISTRY.get(ptyId)!
      entry.pty.kill('SIGTERM')
      PTY_REGISTRY.delete(ptyId)
      log.info(`Session ${sessionId} closed: killed orphaned PTY ${ptyId}`)
    } catch (err) {
      log.warn(`Failed to kill PTY ${ptyId}: ${err}`)
    }
  }
}

// Fix isolation: ptyId phải include userId
export function generatePtyId(userId: string, taskId: string): string {
  return `pty-${userId}-${taskId}-${Date.now()}`
}
```

### agent-session.ts — gọi cleanup trong stop():

```typescript
import { cleanupSessionPtys } from './agent-spawner'

// Trong stop():
stop(): void {
  if (keepaliveTimer) {
    clearInterval(keepaliveTimer)
    keepaliveTimer = null
  }
  // Cleanup tất cả PTYs khi session kết thúc
  cleanupSessionPtys(config.devServerId, log)
},
```

---

## BUG-AG-ORCH-003 — Fix `buildAgentEnv` dùng `'placeholder-key'`

**File:** `src/relay/agent-spawner.ts`, `src/relay/agent-credential-store.ts`

### AiCredStore.readDecrypted() — method mới:

```typescript
export class AiCredStore {
  constructor(private readonly config: AgentConfig) {}

  async readDecrypted(accountId: string): Promise<string | null> {
    const credFile = join(this.config.credentialDir, `${accountId}.enc`)
    if (!existsSync(credFile)) return null

    const CRED_KEY = process.env.ORCA_AI_CREDENTIAL_KEY
    if (!CRED_KEY) {
      throw new Error('ORCA_AI_CREDENTIAL_KEY not set — cannot decrypt credential')
    }

    try {
      const stored  = JSON.parse(readFileSync(credFile, 'utf8'))
      const salt    = Buffer.from(stored.salt,    'base64')
      const iv2     = Buffer.from(stored.iv2,     'base64')
      const authTag = Buffer.from(stored.authTag, 'base64')
      const data    = Buffer.from(stored.data,    'base64')

      const key      = scryptSync(CRED_KEY, salt, 32)
      const decipher = createDecipheriv('aes-256-gcm', key, iv2)
      decipher.setAuthTag(authTag)
      const decrypted = Buffer.concat([decipher.update(data), decipher.final()])
      const payload   = JSON.parse(decrypted.toString('utf8'))

      return payload.apiKey ?? null
    } catch (err) {
      throw new Error(`Failed to decrypt credential for ${accountId}: ${err}`)
    }
  }
}
```

### buildAgentEnv — sửa từ placeholder sang real credential:

```typescript
// BEFORE:
const env = await buildAgentEnv(req.accountId, 'placeholder-key', req.cwd ?? config.workDir)

// AFTER (theo TDD-AG-12 §5):
export async function buildAgentEnv(
  req: AgentSpawnRequest,
  spec: AgentBinarySpec,
  config: AgentConfig,
  credStore: AiCredStore
): Promise<NodeJS.ProcessEnv> {
  const home = homedir()

  const baseEnv: NodeJS.ProcessEnv = {
    HOME: home, TERM: 'xterm-256color', LANG: 'en_US.UTF-8',
  }

  let apiKeyPair: Record<string, string> = {}
  if (spec.apiKeyEnv && req.accountId) {
    const apiKey = await credStore.readDecrypted(req.accountId)
    if (apiKey) {
      apiKeyPair[spec.apiKeyEnv] = apiKey
    } else {
      log.warn(`buildAgentEnv: no credential found for accountId=${req.accountId}`)
    }
  }

  const localInferenceEnv: NodeJS.ProcessEnv = {}
  if (spec.localInference) {
    localInferenceEnv.OLLAMA_HOST     = process.env.OLLAMA_HOST     ?? 'http://localhost:11434'
    localInferenceEnv.OPENAI_BASE_URL = process.env.OPENAI_BASE_URL ?? 'http://localhost:8000/v1'
  }

  const extraPaths = req.pathAdditions ?? []
  const fullPath   = [...extraPaths, ...config.toolPath.split(':')].join(':')

  return {
    ...baseEnv,
    ...apiKeyPair,
    ...localInferenceEnv,
    ...(req.extraEnv ?? {}),
    ORCA_PROJECT_ID:     req.projectId,
    ORCA_TASK_ID:        req.taskId,
    ORCA_USER_ID:        req.userId,
    ORCA_AGENT_HOOK_URL: `http://localhost:${config.hookPort ?? 6800}`,
    GH_CONFIG_DIR:       `${home}/.config/gh/${req.userId}/`,
    GLAB_CONFIG_DIR:     `${home}/.config/glab-cli/${req.userId}/`,
    PATH:                fullPath,
  }
}
```

---

## BUG-AG-ORCH-006 — Fix `agent.output` stream handler missing

**Root cause:** PTY output được gửi dưới dạng JSON-RPC response (có `id`) vi phạm JSON-RPC 2.0.

### Dev Server side — agent-spawner.ts:

```typescript
// Thay thế pty.onData() handler — dùng notification thay response:
pty.onData((data: string) => {
  clearTimeout(hungTimer)
  hungTimer = setTimeout(() => { pty.kill('SIGTERM') }, HUNG_TIMEOUT_MS)

  if (firstOutput) {
    firstOutput = false
    stateMachine.transition('first_output')
    sendNotification(ws, wireState, 'agent.statusChanged', {
      ptyId, taskId: req.taskId, state: 'running',
    })
  }

  if (data.includes('\x1b]133;A')) {
    stateMachine.transition('osc_prompt_open')
    sendNotification(ws, wireState, 'agent.statusChanged', {
      ptyId, taskId: req.taskId, state: 'waiting_for_input',
    })
  }
  if (data.includes('\x1b]133;B')) {
    stateMachine.transition('osc_prompt_close')
    sendNotification(ws, wireState, 'agent.statusChanged', {
      ptyId, taskId: req.taskId, state: 'running',
    })
  }

  if (RATE_LIMIT_RE.test(data)) {
    sendNotification(ws, wireState, 'agent.rateLimited', {
      ptyId, taskId: req.taskId, pattern: data.slice(0, 200),
    })
  }

  // Stream output via notification (không có id)
  sendNotification(ws, wireState, 'agent.output', {
    ptyId,
    data: Buffer.from(data).toString('base64'),
  })
})

const RATE_LIMIT_RE = /rate.?limit(ed)?|you've reached your (daily|weekly|monthly) limit|usage is limited/i

function sendNotification(
  ws: WebSocket,
  wireState: WireState,
  method: string,
  params: Record<string, unknown>,
): void {
  const notification = JSON.stringify({ jsonrpc: '2.0', method, params })
  ws.send(encodeDataFrame(wireState, notification))
}
```

### Orca Server side — dev-server-relay-bridge.ts:

```typescript
// Khi nhận message từ Agent WS, phân biệt notification vs response:
ws.on('message', (data: Buffer) => {
  const frame = decodeFrame(wireState, data)
  if (!frame) return
  const msg = JSON.parse(frame.payload.toString('utf8'))

  // Notification: có method, không có id
  if (msg.method && msg.id === undefined) {
    switch (msg.method) {
      case 'agent.output':
        this.emit('agent:output', msg.params)
        break
      case 'agent.statusChanged':
        this.emit('agent:statusChanged', msg.params)
        break
      case 'agent.rateLimited':
        this.emit('agent:rateLimited', msg.params)
        break
    }
    return
  }

  // Response: có id
  if (msg.id !== undefined) {
    this.resolveRequest(msg.id, msg)
  }
})
```

---

## BUG-AG-ORCH-007 — Implement `AgentHookParser`

**File:** `src/main/agent/AgentHookParser.ts` (NEW)

```typescript
import { EventEmitter } from 'node:events'

export type AgentStatus =
  | 'idle' | 'running' | 'waiting_for_input'
  | 'completed' | 'error' | 'rate_limited'

const RATE_LIMIT_PATTERNS = [
  /rate.?limit(ed)?\.?\s*please try again/i,
  /you've reached your (daily|weekly|monthly) limit/i,
  /claude usage is rate limited/i,
  /too many requests/i,
  /quota exceeded/i,
]

export class AgentHookParser extends EventEmitter {
  parse(ptyId: string, taskId: string, rawData: string): void {
    // OSC 133;A → command prompt (agent waiting for input)
    if (rawData.includes('\x1b]133;A')) {
      this.emit('agent:statusChanged', { ptyId, taskId, status: 'waiting_for_input' })
    }
    // OSC 133;C → command started
    if (rawData.includes('\x1b]133;C')) {
      this.emit('agent:statusChanged', { ptyId, taskId, status: 'running' })
    }
    // OSC 133;D;<exitCode> → command finished
    const oscDMatch = rawData.match(/\x1b\]133;D;(\d+)/)
    if (oscDMatch) {
      const exitCode = parseInt(oscDMatch[1], 10)
      this.emit('agent:statusChanged', {
        ptyId, taskId,
        status: exitCode === 0 ? 'completed' : 'error',
        exitCode,
      })
    }
    // Text patterns
    if (/waiting for input/i.test(rawData)) {
      this.emit('agent:statusChanged', { ptyId, taskId, status: 'waiting_for_input' })
    }
    if (/task completed|done\./i.test(rawData)) {
      this.emit('agent:statusChanged', { ptyId, taskId, status: 'completed' })
    }
    // Rate limit detection
    for (const pattern of RATE_LIMIT_PATTERNS) {
      if (pattern.test(rawData)) {
        const resetAt = new Date()
        resetAt.setHours(resetAt.getHours() + 1)
        this.emit('agent:rateLimited', { ptyId, taskId, pattern: rawData.slice(0, 200), resetAt })
        this.emit('agent:statusChanged', { ptyId, taskId, status: 'error', detail: 'rate_limited' })
        break
      }
    }
  }
}
```

---

## BUG-AG-ORCH-008 — Fix `orca_sessions` wrong schema

**File:** `src/main/db/migrations/0010_add_agent_sessions.ts` (NEW)

```typescript
import type { Kysely } from 'kysely'

export async function up(db: Kysely<unknown>): Promise<void> {
  await db.schema
    .createTable('orca_agent_sessions')
    .ifNotExists()
    .addColumn('id',            'text',    col => col.primaryKey().notNull())
    .addColumn('worktree_id',   'text',    col => col.notNull())
    .addColumn('agent_type',    'text',    col => col.notNull())
    .addColumn('dev_server_id', 'text',    col => col.notNull())
    .addColumn('user_id',       'text',    col => col.notNull())
    .addColumn('pty_id',        'text')
    .addColumn('status',        'text',    col => col.defaultTo('idle').notNull())
    .addColumn('started_at',    'integer', col => col.notNull())
    .addColumn('stopped_at',    'integer')
    .execute()

  await db.schema
    .createIndex('idx_agent_sessions_worktree')
    .on('orca_agent_sessions')
    .columns(['worktree_id', 'started_at'])
    .execute()
}

export async function down(db: Kysely<unknown>): Promise<void> {
  await db.schema.dropIndex('idx_agent_sessions_worktree').execute()
  await db.schema.dropTable('orca_agent_sessions').execute()
}
```

---

## BUG-AG-ORCH-005 — Implement `AgentManager`

**File:** `src/main/agent/AgentManager.ts` (NEW)

```typescript
import { randomUUID } from 'node:crypto'
import type { Kysely } from 'kysely'
import type { Database } from '../db/schema'
import type { RelayConnectionPool } from '../dev-server/relay-connection-pool'

export class AgentManager {
  constructor(
    private readonly relayPool: RelayConnectionPool,
    private readonly db:        Kysely<Database>,
  ) {}

  async start(opts: {
    userId: string; worktreeId: string; agentType: string
    trustPreset: 'standard' | 'full' | 'none'; accountId?: string
    projectId: string; cwd: string; devServerId: string
  }): Promise<{ sessionId: string; ptyId: string }> {
    const sessionId = randomUUID()
    const taskId    = randomUUID()

    const conn = this.relayPool.getConnection(opts.devServerId)
    if (!conn) throw new Error(`Dev Server ${opts.devServerId} not connected`)

    await this.db.insertInto('orca_agent_sessions').values({
      id: sessionId, worktree_id: opts.worktreeId, agent_type: opts.agentType,
      dev_server_id: opts.devServerId, user_id: opts.userId,
      pty_id: null, status: 'idle', started_at: Date.now(), stopped_at: null,
    }).execute()

    const spawnResult = await conn.call('agent.spawn', {
      model: opts.agentType, trustPreset: opts.trustPreset,
      cwd: opts.cwd, taskId, userId: opts.userId,
      projectId: opts.projectId, accountId: opts.accountId ?? '',
    })

    const ptyId = spawnResult.ptyId as string
    await this.db.updateTable('orca_agent_sessions')
      .set({ pty_id: ptyId, status: 'running' })
      .where('id', '=', sessionId)
      .execute()

    return { sessionId, ptyId }
  }

  async stop(opts: { sessionId: string; force?: boolean }): Promise<void> {
    const session = await this.db
      .selectFrom('orca_agent_sessions').selectAll()
      .where('id', '=', opts.sessionId).executeTakeFirst()

    if (!session?.pty_id) return

    const conn = this.relayPool.getConnection(session.dev_server_id)
    if (!conn) throw new Error(`Dev Server not connected`)

    if (opts.force) {
      await conn.call('agent.kill', { ptyId: session.pty_id, signal: 'SIGKILL' })
    } else {
      await conn.call('agent.sendInput', { ptyId: session.pty_id, data: '\x03' })
      await new Promise(r => setTimeout(r, 10_000))
      const current = await this.db
        .selectFrom('orca_agent_sessions').select('status')
        .where('id', '=', opts.sessionId).executeTakeFirst()
      if (current?.status === 'running' || current?.status === 'waiting_for_input') {
        await conn.call('agent.kill', { ptyId: session.pty_id, signal: 'SIGKILL' })
      }
    }

    await this.db.updateTable('orca_agent_sessions')
      .set({ status: 'stopped', stopped_at: Date.now() })
      .where('id', '=', opts.sessionId).execute()
  }

  async resume(opts: { worktreeId: string; userId: string; devServerId: string; cwd: string }): Promise<{ sessionId: string; ptyId: string }> {
    const lastSession = await this.db
      .selectFrom('orca_agent_sessions').selectAll()
      .where('worktree_id', '=', opts.worktreeId)
      .where('user_id', '=', opts.userId)
      .orderBy('started_at', 'desc').executeTakeFirst()

    if (!lastSession) throw new Error(`No previous session for worktree ${opts.worktreeId}`)

    const conn = this.relayPool.getConnection(opts.devServerId)
    if (!conn) throw new Error(`Dev Server not connected`)

    const sessionId = randomUUID()
    const taskId    = randomUUID()

    await this.db.insertInto('orca_agent_sessions').values({
      id: sessionId, worktree_id: opts.worktreeId, agent_type: lastSession.agent_type,
      dev_server_id: opts.devServerId, user_id: opts.userId,
      pty_id: null, status: 'idle', started_at: Date.now(), stopped_at: null,
    }).execute()

    const spawnResult = await conn.call('agent.spawn', {
      model: lastSession.agent_type, trustPreset: 'standard',
      cwd: opts.cwd, taskId, userId: opts.userId,
      projectId: '', accountId: '',
      resumeSessionId: lastSession.id,
    })

    const ptyId = spawnResult.ptyId as string
    await this.db.updateTable('orca_agent_sessions')
      .set({ pty_id: ptyId, status: 'running' })
      .where('id', '=', sessionId).execute()

    return { sessionId, ptyId }
  }

  async switchAccount(opts: { sessionId: string; newAccountId: string; cwd: string }): Promise<{ sessionId: string; ptyId: string }> {
    const session = await this.db
      .selectFrom('orca_agent_sessions').selectAll()
      .where('id', '=', opts.sessionId).executeTakeFirst()

    if (!session) throw new Error(`Session ${opts.sessionId} not found`)

    await this.stop({ sessionId: opts.sessionId, force: true })

    return this.start({
      userId: session.user_id, worktreeId: session.worktree_id,
      agentType: session.agent_type, trustPreset: 'standard',
      accountId: opts.newAccountId, projectId: '',
      cwd: opts.cwd, devServerId: session.dev_server_id,
    })
  }
}
```

---

## BUG-AG-ORCH-009 — Fix Resume Session (relay side)

Resume args được inject vào AgentSpawnRequest `resumeSessionId` field, agent-spawner cần xử lý:

```typescript
// agent-spawner.ts — sửa AgentSpawnRequest:
export interface AgentSpawnRequest {
  readonly model:            string
  readonly trustPreset:      'standard' | 'full' | 'none'
  readonly cwd:              string
  readonly taskId:           string
  readonly userId:           string
  readonly projectId:        string
  readonly accountId:        string
  readonly initFile?:        string
  readonly extraEnv?:        Record<string, string>
  readonly pathAdditions?:   string[]
  readonly resumeSessionId?: string   // NEW: nếu có, inject --resume flag
}

// buildClaudeArgs — handle resume:
function buildClaudeArgs(req: AgentSpawnRequest): string[] {
  if (req.resumeSessionId) {
    return ['--resume', req.resumeSessionId]
  }
  const args: string[] = ['--output-format', 'stream-json', '--verbose']
  if (req.trustPreset && req.trustPreset !== 'none') args.push('--trust', req.trustPreset)
  if (req.initFile) args.push('--init-file', req.initFile)
  return args
}

// buildCodexArgs — handle resume:
function buildCodexArgs(req: AgentSpawnRequest): string[] {
  if (req.resumeSessionId) {
    return ['--session-file', `~/.codex/${req.resumeSessionId}.json`, '--model', req.model]
  }
  return ['--model', req.model]
}
```

---

## BUG-AG-ORCH-010 — Fix Switch Account (wire rate-limit → AgentManager)

```typescript
// Trong dev-server-relay-bridge.ts hoặc AgentWebSocketServer:
hookParser.on('agent:rateLimited', async (event) => {
  // Notify browser
  wsRouter.emit('agent:rateLimited', event)

  // Auto-switch nếu có fallback account configured
  const session = await db.selectFrom('orca_agent_sessions')
    .selectAll()
    .where('pty_id', '=', event.ptyId)
    .executeTakeFirst()

  if (!session) return

  const fallbackAccountId = await aiProviderSvc.getFallbackAccount(
    session.user_id, session.agent_type
  )
  if (fallbackAccountId) {
    await agentManager.switchAccount({
      sessionId:    session.id,
      newAccountId: fallbackAccountId,
      cwd:          process.env.HOME ?? '/home/ubuntu',
    })
  }
})
```

---

## Tóm tắt file changes

| File | Action | Bugs fixed |
|------|--------|------------|
| `src/relay/agent-spawner.ts` | MODIFY | ORCH-001, 002, 003, 004, 009, 011, 012 |
| `src/relay/agent-rpc-dispatch.ts` | MODIFY | ORCH-001 (`agent.sendInput` case) |
| `src/relay/agent-connection-relay.ts` | MODIFY | ORCH-013 |
| `src/relay/agent-config.ts` | MODIFY | ORCH-013 (validateConfig) |
| `src/relay/agent-credential-store.ts` | MODIFY | ORCH-003 (readDecrypted method) |
| `src/relay/agent-session.ts` | MODIFY | ORCH-011 (stop() cleanup PTYs) |
| `src/main/agent/AgentManager.ts` | **NEW** | ORCH-005, 009, 010 |
| `src/main/agent/AgentHookParser.ts` | **NEW** | ORCH-007 |
| `src/main/dev-server/dev-server-relay-bridge.ts` | MODIFY | ORCH-006 (notification handler) |
| `src/main/db/migrations/0010_add_agent_sessions.ts` | **NEW** | ORCH-008 |
| `src/main/db/schema.ts` | MODIFY | ORCH-008 (OrcaAgentSessionTable) |

---

## Verification Plan

```bash
# 1. Type check
pnpm tsc --noEmit -p config/tsconfig.node.json

# 2. Unit tests
pnpm vitest run src/relay/__tests__/agent-spawner.test.ts
pnpm vitest run src/relay/__tests__/agent-connection-relay.test.ts
pnpm vitest run src/relay/__tests__/agent-session.test.ts
pnpm vitest run src/main/__tests__/agent/AgentHookParser.test.ts

# 3. Integration tests (manual):
# - Start relay với ORCA_AGENT_TOKEN="" → expect process.exit(1)
# - Spawn claude → verify args do not contain --no-cache
# - Spawn codex/opencode → verify no throw from resolveAgentSpec
# - Send agent.sendInput → verify Ctrl+C reaches PTY
# - Kill with signal:SIGKILL → verify SIGKILL sent (not SIGTERM)
# - Disconnect WS → verify no orphaned PTY processes remain
```

---

## ✅ Implementation Status (2026-08-01)

8/13 bugs FIXED. ORCH-001,002,003,004,006,011,012,013 DONE. ORCH-005,007,008,009,010 DEFERRED (Phase 3).
