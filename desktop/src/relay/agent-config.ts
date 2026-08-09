// src/relay/agent-config.ts
// Typed configuration for Orca Dev Agent, loaded from environment variables.
// All config is read once at startup via loadAgentConfig().
// Use vi.stubEnv() in tests to override individual variables.

import { homedir } from 'node:os'
import { join } from 'node:path'

export type AgentConnectionMode = 'direct-websocket' | 'relay-websocket'
export type AgentLogLevel = 'info' | 'debug' | 'warn' | 'error'

export type AgentConfig = {
  readonly mode: AgentConnectionMode
  /** WebSocket URL for direct-websocket mode (ORCA_URL env var) */
  readonly orcaUrl: string
  /** HTTP base URL of the Orca Server for token API calls, e.g. http://172.20.2.39:6769.
   *  Derived from ORCA_HTTP_URL env var or auto-converted from ORCA_URL (wss→https, ws→http). */
  readonly orcaHttpUrl: string
  /** One-time bearer token for direct-ws, long-lived secret for relay-ws */
  readonly agentToken: string
  /** API secret for POST /api/agent-token (ORCA_AGENT_API_SECRET). Required for token renewal. */
  readonly apiSecret: string
  /** Port the agent listens on in relay-websocket mode (AGENT_PORT) */
  readonly agentPort: number
  /** Identifier for this dev server (DEV_SERVER_ID) */
  readonly devServerId: string
  readonly logLevel: AgentLogLevel
  /** Working directory for tool execution (AGENT_WORK_DIR or process.cwd()) */
  readonly workDir: string
  /** Colon-separated PATH for tool binary discovery */
  readonly toolPath: string
  /** Merged process.env with expanded PATH and AI provider tokens */
  readonly toolEnv: NodeJS.ProcessEnv
  /** Directory for storing encrypted AI credentials (~/.orca/credentials/) */
  readonly credentialDir: string
  /** Whether to enforce TLS certificate validation */
  readonly tlsRejectUnauthorized: boolean
}

/**
 * Build colon-separated tool PATH — expands ~/.local/bin and other common
 * bin dirs that may not be in process.env.PATH when running under systemd.
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
 * Derive an HTTP base URL from the WS URL for token API calls.
 * wss://host/agent → https://host
 * ws://host/agent  → http://host
 */
function deriveHttpUrl(wsUrl: string): string {
  try {
    const u = new URL(wsUrl)
    u.protocol = u.protocol === 'wss:' ? 'https:' : 'http:'
    u.pathname = '/'
    u.search   = ''
    return u.origin
  } catch {
    return 'http://localhost:6769'
  }
}

export function loadAgentConfig(): AgentConfig {
  // Use || instead of ?? so that empty string env vars also fall back to defaults.
  const rawMode = process.env.MODE || 'direct-websocket'
  if (rawMode !== 'direct-websocket' && rawMode !== 'relay-websocket') {
    throw new Error(
      `Invalid MODE="${rawMode}". Must be "direct-websocket" or "relay-websocket".`
    )
  }
  const mode = rawMode as AgentConnectionMode

  const home = homedir()
  const toolPath = buildToolPath(home)

  const orcaUrl = process.env.ORCA_URL || 'wss://b15.openledger.vn/agent'
  // ORCA_HTTP_URL can be set explicitly (e.g. http://172.20.2.39:6769) for
  // environments where the HTTP and WS endpoints are on different addresses.
  const orcaHttpUrl = process.env.ORCA_HTTP_URL || deriveHttpUrl(orcaUrl)

  return {
    mode,
    orcaUrl,
    orcaHttpUrl,
    agentToken:  process.env.AGENT_TOKEN        ?? '',
    apiSecret:   process.env.ORCA_AGENT_API_SECRET ?? '',
    agentPort:   Number.parseInt(process.env.AGENT_PORT || '6799', 10),
    devServerId: process.env.DEV_SERVER_ID || 'dev-local',
    logLevel:    (process.env.LOG_LEVEL    || 'info') as AgentLogLevel,
    workDir:     process.env.AGENT_WORK_DIR || process.cwd(),
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
