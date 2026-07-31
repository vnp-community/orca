// src/relay/agent-config.ts
// Typed configuration for Orca Dev Agent, loaded from environment variables.
// All config is read once at startup via loadAgentConfig().
// Use vi.stubEnv() in tests to override individual variables.

import { homedir } from 'node:os'
import { join } from 'node:path'

export type AgentConnectionMode = 'direct-websocket' | 'relay-websocket'
export type AgentLogLevel = 'info' | 'debug' | 'warn' | 'error'

export interface AgentConfig {
  readonly mode: AgentConnectionMode
  /** WebSocket URL for direct-websocket mode (ORCA_URL env var) */
  readonly orcaUrl: string
  /** One-time bearer token for direct-ws, long-lived secret for relay-ws */
  readonly agentToken: string
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
 * Load and validate agent configuration from process.env.
 * Throws on invalid MODE to fail fast at startup — not at connection time.
 */
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

  return {
    mode,
    orcaUrl:     process.env.ORCA_URL      || 'wss://b15.openledger.vn/agent',
    agentToken:  process.env.AGENT_TOKEN   ?? '',
    agentPort:   parseInt(process.env.AGENT_PORT || '6799', 10),
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
