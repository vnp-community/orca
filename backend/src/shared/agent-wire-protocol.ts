// src/shared/agent-wire-protocol.ts
// Agent Wire Protocol v1 — Language-agnostic constants and types.
//
// Frame format (13-byte header, SAME AS relay-protocol.ts):
//   [TYPE u8][SEQ u32 BE][ACK u32 BE][LENGTH u32 BE][PAYLOAD bytes]
//
// TYPE values:
//   0x01 = Regular (JSON-RPC 2.0 payload)
//   0x09 = KeepAlive (empty payload, LENGTH=0)
//
// Keepalive: sender emits every 5000ms if no Regular frame sent.
// Timeout: if no frame received in 20000ms → close connection.
//
// Kept in src/shared/ so both server and relay code can import it
// without circular dependencies.

export const AGENT_PROTOCOL_VERSION = '1'
export const AGENT_HANDSHAKE_METHOD = 'agent.handshake'

// Keepalive & timeout — same values as relay-protocol.ts
export const AGENT_KEEPALIVE_INTERVAL_MS = 5_000
export const AGENT_TIMEOUT_MS = 20_000

// direct-websocket: how long Orca waits for agent to connect after slot registration
export const AGENT_CONNECT_TIMEOUT_MS = 60_000

// WS endpoint path for direct-websocket connections (Agent → Orca)
export const AGENT_WS_PATH = '/agent'

// Minimum agent version Orca accepts
export const AGENT_MIN_VERSION = '1.0.0'

// ─── Error Codes ──────────────────────────────────────────────────────────────

export const AgentErrorCode = {
  // JSON-RPC 2.0 standard
  ParseError:           -32700,
  InvalidRequest:       -32600,
  MethodNotFound:       -32601,
  InvalidParams:        -32602,
  ServerError:          -32000,
  // Agent-specific
  CommandNotFound:      -33001,
  PermissionDenied:     -33002,
  PathNotFound:         -33003,
  PtyAllocationFailed:  -33004,
  DiskFull:             -33005,
  TooManyStreams:       -33006,
  StreamProtocolError:  -33007,
  HandshakeFailed:      -33100,  // Version mismatch, protocol violation
  AuthFailed:           -33101,  // Invalid or missing agent token
} as const

export type AgentErrorCodeValue = typeof AgentErrorCode[keyof typeof AgentErrorCode]

// ─── Capabilities ────────────────────────────────────────────────────────────

export type AgentCapability = 'pty' | 'fs' | 'git' | 'preflight'

// ─── Handshake Types ─────────────────────────────────────────────────────────

/** Params agent sends in agent.handshake — first message after WS connected */
export type AgentHandshakeParams = {
  agentVersion: string
  platform: string           // 'linux' | 'darwin' | 'win32'
  arch: string               // 'x64' | 'arm64' | ...
  nodeVersion?: string       // e.g. 'v20.11.0' or 'python-3.12' or 'go-1.22'
  capabilities: AgentCapability[]
  /** Bearer token for direct-websocket auth. Not sent in relay-websocket mode. */
  agentToken?: string
}

/** Result Orca returns on successful handshake */
export type AgentHandshakeResult = {
  ok: true
  orcaVersion: string
  sessionId: string
}

// ─── Token helpers ───────────────────────────────────────────────────────────

/**
 * Generate a unique agent token for direct-websocket mode.
 * Format: agt-<devServerId>-<timestamp>
 *
 * The token is stored in AgentWebSocketServer.pendingSlots and validated
 * during the agent.handshake params check.
 */
export function generateAgentToken(devServerId: string): string {
  return `agt-${devServerId}-${Date.now()}`
}

/**
 * Minimal semver "less than" comparison (major.minor.patch only — no
 * pre-release/build metadata support, agent versions don't use them).
 * Returns true if `version` is strictly older than `minVersion`.
 * Non-numeric or malformed segments are treated as 0 (fail open toward
 * "too old" rather than silently accepting garbage as compatible).
 */
export function isAgentVersionBelowMinimum(version: string, minVersion: string): boolean {
  const parse = (v: string): [number, number, number] => {
    const [maj, min, pat] = v.trim().split('.')
    return [Number(maj) || 0, Number(min) || 0, Number(pat) || 0]
  }
  const [aMaj, aMin, aPat] = parse(version)
  const [bMaj, bMin, bPat] = parse(minVersion)
  if (aMaj !== bMaj) return aMaj < bMaj
  if (aMin !== bMin) return aMin < bMin
  return aPat < bPat
}
