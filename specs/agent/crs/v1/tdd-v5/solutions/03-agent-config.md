# SOL-03: agent-config.ts — Typed Configuration

**TDD Ref:** TDD-AG-01  
**File:** `src/relay/agent-config.ts` [NEW]  
**Mức độ:** 🟢 Đơn giản  
**Thời gian ước tính:** 30m

---

## Vấn đề

Agent v1 dùng module-level `const` globals (`MODE`, `ORCA_URL`, `TOOL_ENV`, ...).  
Agent v2.1 cần `AgentConfig` interface — injectable, testable với `vi.stubEnv()`.

---

## Giải pháp — Full Implementation

```typescript
// src/relay/agent-config.ts

import { homedir } from 'node:os'
import { join } from 'node:path'

export type AgentConnectionMode = 'direct-websocket' | 'relay-websocket'
export type AgentLogLevel = 'info' | 'debug' | 'warn' | 'error'

export interface AgentConfig {
  readonly mode: AgentConnectionMode
  readonly orcaUrl: string
  readonly agentToken: string
  readonly agentPort: number
  readonly devServerId: string
  readonly logLevel: AgentLogLevel
  readonly workDir: string
  readonly toolPath: string
  readonly toolEnv: NodeJS.ProcessEnv
  readonly credentialDir: string
  readonly tlsRejectUnauthorized: boolean
}

/**
 * Build tool PATH — expands ~/.local/bin and other common bin dirs
 * that may not be in process.env.PATH when running under systemd.
 */
function buildToolPath(home: string): string {
  return [
    `${home}/.local/bin`,
    `${home}/bin`,
    '/usr/local/bin',
    '/usr/bin',
    '/bin',
    '/usr/sbin',
    '/snap/bin',
  ].join(':')
}

/**
 * Load config from environment variables.
 * Throws on invalid MODE (fail fast at startup, not at connection time).
 */
export function loadAgentConfig(): AgentConfig {
  const mode = (process.env.MODE ?? 'direct-websocket') as AgentConnectionMode

  if (mode !== 'direct-websocket' && mode !== 'relay-websocket') {
    throw new Error(
      `Invalid MODE="${mode}". Must be "direct-websocket" or "relay-websocket".`
    )
  }

  const home = homedir()
  const toolPath = buildToolPath(home)

  return {
    mode,
    orcaUrl:     process.env.ORCA_URL      ?? 'wss://b15.openledger.vn/agent',
    agentToken:  process.env.AGENT_TOKEN   ?? '',
    agentPort:   parseInt(process.env.AGENT_PORT ?? '6799', 10),
    devServerId: process.env.DEV_SERVER_ID ?? 'dev-local',
    logLevel:    (process.env.LOG_LEVEL    ?? 'info') as AgentLogLevel,
    workDir:     process.env.AGENT_WORK_DIR ? process.env.AGENT_WORK_DIR : process.cwd(),
    toolPath,
    toolEnv: {
      ...process.env,
      PATH:              toolPath,
      HOME:              home,
      ANTHROPIC_API_KEY: process.env.ANTHROPIC_API_KEY ?? '',
      GITHUB_TOKEN:      process.env.GITHUB_TOKEN      ?? '',
      GH_TOKEN:          process.env.GH_TOKEN          ?? '',
    },
    credentialDir: join(home, '.orca', 'credentials'),
    tlsRejectUnauthorized: process.env.NODE_TLS_REJECT_UNAUTHORIZED !== '0',
  }
}
```

---

## Logger Factory (companion file)

```typescript
// src/relay/agent-logger.ts [NEW]

export interface AgentLogger {
  info(...args: unknown[]): void
  warn(...args: unknown[]): void
  error(...args: unknown[]): void
  debug(...args: unknown[]): void
}

export function createAgentLogger(level: string): AgentLogger {
  const ts = () => new Date().toISOString()
  return {
    info:  (...a) => console.log(`[agent] ${ts()} ${a.join(' ')}`),
    warn:  (...a) => console.warn(`[agent:WARN] ${a.join(' ')}`),
    error: (...a) => console.error(`[agent:ERROR] ${a.join(' ')}`),
    debug: (...a) => { if (level === 'debug') console.log(`[agent:DEBUG] ${a.join(' ')}`) },
  }
}
```

---

## Test File

```typescript
// src/relay/__tests__/agent-config.test.ts
import { describe, it, expect, vi, afterEach } from 'vitest'
import { loadAgentConfig } from '../agent-config'

describe('loadAgentConfig', () => {
  afterEach(() => { vi.unstubAllEnvs() })

  it('defaults to direct-websocket mode', () => {
    vi.stubEnv('MODE', '')
    const cfg = loadAgentConfig()
    expect(cfg.mode).toBe('direct-websocket')
  })

  it('loads relay-websocket mode', () => {
    vi.stubEnv('MODE', 'relay-websocket')
    const cfg = loadAgentConfig()
    expect(cfg.mode).toBe('relay-websocket')
  })

  it('throws on invalid MODE', () => {
    vi.stubEnv('MODE', 'ssh')
    expect(() => loadAgentConfig()).toThrow('Invalid MODE')
  })

  it('parses AGENT_PORT as number', () => {
    vi.stubEnv('AGENT_PORT', '7799')
    const cfg = loadAgentConfig()
    expect(cfg.agentPort).toBe(7799)
  })

  it('defaults workDir to process.cwd()', () => {
    vi.stubEnv('AGENT_WORK_DIR', '')
    const cfg = loadAgentConfig()
    expect(cfg.workDir).toBe(process.cwd())
  })

  it('toolEnv contains expanded PATH', () => {
    const cfg = loadAgentConfig()
    expect(cfg.toolEnv.PATH).toContain('.local/bin')
  })

  it('tlsRejectUnauthorized=false when NODE_TLS_REJECT_UNAUTHORIZED=0', () => {
    vi.stubEnv('NODE_TLS_REJECT_UNAUTHORIZED', '0')
    const cfg = loadAgentConfig()
    expect(cfg.tlsRejectUnauthorized).toBe(false)
  })
})
```

---

## Definition of Done

- [x] `src/relay/agent-config.ts` created
- [x] `src/relay/agent-logger.ts` created
- [x] `tsc` passes
- [x] `src/relay/__tests__/agent-config.test.ts` — ≥ 8 tests pass
