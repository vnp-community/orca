# TDD-AG-01: Architecture & Process Model (v2.1)

**Document:** TDD-AG-01
**Version:** 2.1 — Integrated into src/relay/
**Date:** 2026-07-28
**Domain:** Agent Architecture — TypeScript, esbuild, src/relay/ integration
**Source files (mới):**
- `src/relay/agent-entry.ts` ← entry point
- `src/relay/agent-config.ts` ← typed config
- `src/shared/agent-wire-protocol.ts` ← shared constants (existing)
**HLD Ref:** C3.8, C4.5
**ADR:** ADR-005

---

## 1. Tech Stack (v2.1)

| Layer | v1 (cũ) | v2.1 (mới) |
|-------|---------|----------|
| Language | CommonJS JavaScript | **TypeScript strict** |
| Module format | `require()` | TypeScript → esbuild → CJS bundle |
| Build | Không có | `pnpm run build:relay` (build-relay.mjs) |
| Test | Không có | **Vitest** (config/vitest.config.ts) |
| Type check | Không | `tsc --noEmit -p config/tsconfig.node.json` |
| Wire protocol | Inline constants | `src/shared/agent-wire-protocol.ts` (shared) |
| Code location | `deploy/dev/agent/agent.js` | **`src/relay/agent-*.ts`** |
| Build output | agent.js (raw) | **`out/relay/agent.js`** (bundled) |
| WebSocket | `ws` npm | `ws` npm (same as relay daemon) |

---

## 2. Source Directory Structure

```
src/relay/                            # Agent source (cùng dir với relay daemon)
├── agent-entry.ts                    # [NEW] Entry point — replaces agent.js main()
├── agent-config.ts                   # [NEW] Typed config from env vars
├── agent-connection-direct.ts        # [NEW] direct-websocket mode
├── agent-connection-relay.ts         # [NEW] relay-websocket mode  
├── agent-session.ts                  # [NEW] Session handler (handleSession)
├── agent-wire.ts                     # [NEW] encodeFrame/decodeFrame typed
├── agent-tool-registry.ts            # [NEW] ToolDefinition[], discoverTools()
├── agent-exec-handler.ts             # [EXTEND] runCommandCapture (already exists!)
├── agent-rpc-dispatch.ts             # [NEW] JSON-RPC router + MCP handlers
├── agent-credential-store.ts         # [NEW v5.0] AI credential AES-256-GCM
├── git-handler.ts                    # [NEW v5.0] git.exec/execStream whitelisted
├── fs-handler-file-read.ts           # [REUSE] Already exists in src/relay/
├── fs-handler-list-files.ts          # [REUSE] Already exists
├── fs-handler-rg-availability.ts     # [REUSE] Already exists
└── fs-handler-utils.ts               # [REUSE] Already exists

src/shared/
└── agent-wire-protocol.ts            # [REUSE+EXTEND] Shared types — already exists!

# Build output:
out/relay/
├── agent.js                          # Bundled agent (esbuild CJS, standalone)
└── relay.js                          # Relay daemon (existing)
```

---

## 3. agent-config.ts — Typed Configuration

```typescript
// src/relay/agent-config.ts

import { homedir } from 'node:os'
import { join } from 'node:path'

export type AgentConnectionMode = 'direct-websocket' | 'relay-websocket'

export interface AgentConfig {
  readonly mode: AgentConnectionMode
  readonly orcaUrl: string
  readonly agentToken: string
  readonly agentPort: number
  readonly devServerId: string
  readonly logLevel: 'info' | 'debug' | 'warn' | 'error'
  readonly workDir: string
  readonly toolPath: string
  readonly toolEnv: NodeJS.ProcessEnv
  readonly credentialDir: string
}

const HOME = homedir()

const TOOL_PATH = [
  `${HOME}/.local/bin`,
  `${HOME}/bin`,
  '/usr/local/bin',
  '/usr/bin',
  '/bin',
  '/usr/sbin',
  '/snap/bin',
].join(':')

export function loadAgentConfig(): AgentConfig {
  const mode = (process.env.MODE ?? 'direct-websocket') as AgentConnectionMode
  if (mode !== 'direct-websocket' && mode !== 'relay-websocket') {
    throw new Error(`Invalid MODE: ${mode}. Must be 'direct-websocket' or 'relay-websocket'`)
  }

  return {
    mode,
    orcaUrl:     process.env.ORCA_URL      ?? 'wss://b15.openledger.vn/agent',
    agentToken:  process.env.AGENT_TOKEN   ?? '',
    agentPort:   parseInt(process.env.AGENT_PORT ?? '6799', 10),
    devServerId: process.env.DEV_SERVER_ID ?? 'dev-local',
    logLevel:    (process.env.LOG_LEVEL ?? 'info') as AgentConfig['logLevel'],
    workDir:     process.env.AGENT_WORK_DIR ?? process.cwd(),
    toolPath:    TOOL_PATH,
    toolEnv: {
      ...process.env,
      PATH:              TOOL_PATH,
      HOME,
      ANTHROPIC_API_KEY: process.env.ANTHROPIC_API_KEY ?? '',
      GITHUB_TOKEN:      process.env.GITHUB_TOKEN      ?? '',
      GH_TOKEN:          process.env.GH_TOKEN          ?? '',
    },
    credentialDir: join(HOME, '.orca', 'credentials'),
  }
}
```

---

## 4. agent-entry.ts — Entry Point

```typescript
// src/relay/agent-entry.ts
// Agent process entry point — replaces deploy/dev/agent/agent.js

import { loadAgentConfig } from './agent-config'
import { discoverTools } from './agent-tool-registry'
import { connectDirect } from './agent-connection-direct'
import { listenRelay } from './agent-connection-relay'
import { createAgentLogger } from './agent-logger'

const config = loadAgentConfig()
const log = createAgentLogger(config.logLevel)

async function main(): Promise<void> {
  log.info('Orca Dev Agent v2.1.0')
  log.info(`Mode: ${config.mode}  |  DevServerId: ${config.devServerId}  |  WorkDir: ${config.workDir}`)
  log.info('Discovering tools...')

  const tools = await discoverTools(config)
  log.info(`Tool discovery complete: ${tools.length} tools available`)

  process.on('SIGINT',  () => { log.info('Shutting down (SIGINT)');  process.exit(0) })
  process.on('SIGTERM', () => { log.info('Shutting down (SIGTERM)'); process.exit(0) })

  if (config.mode === 'relay-websocket') {
    await listenRelay(config, tools, log)
  } else {
    await connectDirect(config, tools, log)
  }
}

main().catch(err => {
  console.error('[agent:FATAL]', err)
  process.exit(1)
})
```

---

## 5. Build Integration — build-relay.mjs

Thêm `agent-entry.ts` vào `config/scripts/build-relay.mjs`:

```javascript
// config/scripts/build-relay.mjs (EXTEND)

const AGENT_ENTRY = join(ROOT, 'src', 'relay', 'agent-entry.ts')

// Existing relay build:
await build({ entryPoints: [RELAY_ENTRY], outfile: 'out/relay/relay.js', ... })

// NEW: Agent build
await build({
  entryPoints: [AGENT_ENTRY],
  outfile: 'out/relay/agent.js',
  bundle: true,
  platform: 'node',
  target: 'node22',
  format: 'cjs',
  external: ['node-pty'],  // Agent không dùng node-pty
  minify: false,           // Readable output for debugging on dev server
})
```

**pnpm scripts** (extend `package.json`):
```json
{
  "build:agent": "node config/scripts/build-relay.mjs --agent-only",
  "build:relay": "node config/scripts/build-relay.mjs"
}
```

---

## 6. TypeScript Config

Agent code nằm trong `src/relay/` → tự động được cover bởi `config/tsconfig.node.json`:

```json
// config/tsconfig.node.json (existing, covers src/relay/)
{
  "include": ["src/main/**/*", "src/relay/**/*", "src/shared/**/*", "src/server/**/*"]
}
```

Không cần tsconfig riêng cho agent.

---

## 7. Test Setup (Vitest)

```typescript
// src/relay/__tests__/agent-config.test.ts
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { loadAgentConfig } from '../agent-config'

describe('loadAgentConfig', () => {
  beforeEach(() => {
    vi.stubEnv('MODE', 'direct-websocket')
    vi.stubEnv('DEV_SERVER_ID', 'test-server')
  })

  it('loads direct-websocket mode', () => {
    const cfg = loadAgentConfig()
    expect(cfg.mode).toBe('direct-websocket')
    expect(cfg.devServerId).toBe('test-server')
  })

  it('throws on invalid MODE', () => {
    vi.stubEnv('MODE', 'invalid-mode')
    expect(() => loadAgentConfig()).toThrow('Invalid MODE')
  })

  it('defaults workDir to process.cwd()', () => {
    vi.stubEnv('AGENT_WORK_DIR', '')
    const cfg = loadAgentConfig()
    expect(cfg.workDir).toBe(process.cwd())
  })
})
```

---

## 8. Migration Guide

| Cũ (v1) | Mới (v2.1) |
|---------|-----------|
| `deploy/dev/agent/agent.js` | `src/relay/agent-entry.ts` |
| `deploy/dev/agent/package.json` | Dùng chung root `package.json` |
| `node agent.js` (direct) | `node out/relay/agent.js` (bundled) |
| Không có build step | `pnpm run build:relay` |
| Không có tests | `vitest run` (chạy cùng backend tests) |
| CommonJS `require()` | TypeScript `import` |
| Inline constants | `src/shared/agent-wire-protocol.ts` |
| `FRAME_TYPE.PING = 0x09` (magic) | `MessageType.KeepAlive` (typed enum) |
| `discoverTools()` global | `discoverTools(config: AgentConfig)` (dependency injection) |

**Deploy vẫn giống nhau**: copy `out/relay/agent.js` lên dev server, chạy với `node agent.js`.
