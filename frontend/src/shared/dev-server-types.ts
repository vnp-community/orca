// ─── Dev Server Types ───────────────────────────────────────────────────────
// Types for the DevServer subsystem (SOL-002, CR-OB-002).
// Pure type file — no imports from other project modules.

export type DevServerConnectionType =
  | 'relay-ssh' // Orca SSH → deploy relay → stdin/stdout
  // Direction fix: these two were previously commented the other way
  // around — see agent/src/relay/agent-connection-relay.ts/
  // agent-connection-direct.ts's own header comments, which this now
  // matches, plus DevServerStep.tsx's DEV_SERVER_CONNECTION_TYPE_LABELS.
  | 'relay-websocket' // Orca connects WS → dev server (dev server only listens)
  | 'direct-websocket' // Dev server connects WS → Orca (reverse)

export type DevServerStatus = 'connected' | 'disconnected' | 'connecting' | 'error'

export type DevServer = {
  id: string // 'ds-<uuid>'
  name: string // Human label: "MacBook Pro M3"
  connectionType: DevServerConnectionType
  // relay-ssh specific:
  sshTargetId?: string // Links to existing SshTarget
  // relay-websocket / direct-websocket specific:
  wsUrl?: string // ws://devserver.local:6799
  // Runtime (not persisted — populated after handshake):
  status: DevServerStatus
  platform: NodeJS.Platform | null
  arch: string | null // 'arm64' | 'x64'
  nodeVersion: string | null
  lastConnectedAt: number | null
  lastError: string | null
  workspaceDir: string | null // Remote default workspace directory
  addedAt: number
  /** Capabilities the agent advertised at handshake (e.g. 'pty', 'pty.stream',
   *  'fs.watch'). Null until a successful handshake has completed at least once. */
  capabilities: readonly string[] | null
  // CR-DS-006: admin-approval + grouping. Optional since older cached/local
  // DevServer objects (pre-migration) won't carry these — callers should
  // treat a missing approvalStatus as 'pending_approval' (fail closed).
  approvalStatus?: DevServerApprovalStatus
  groupId?: string
}

export type DevServerInput = {
  name: string
  connectionType: DevServerConnectionType
  sshTargetId?: string
  wsUrl?: string
}

export type ConnectionTestResult =
  | { ok: true; platform: NodeJS.Platform; nodeVersion: string }
  | { ok: false; error: string; hint?: string }

/**
 * Persisted subset of DevServer.
 * Runtime-only fields (status, platform, arch, nodeVersion, lastConnectedAt, lastError)
 * are NOT persisted — they are reconstructed on startup from the live relay.
 */
export type PersistedDevServer = {
  id: string
  name: string
  connectionType: DevServerConnectionType
  sshTargetId?: string
  wsUrl?: string
  workspaceDir: string | null
  addedAt: number
}

// ─── Remote Preflight Status ──────────────────────────────────────────────────

/** Result of running preflight checks (gh, git) on a remote dev server */
export type RemotePreflightStatus = {
  devServerId: string
  platform: NodeJS.Platform
  checkedAt: number
  gh: {
    installed: boolean
    authenticated: boolean
    version?: string
  }
  // Why: GitLab CLI (glab) check via relay. Optional for backward compatibility
  // with older relay versions that don't report glab status (FE-TASK-08 / FE-SOL-04).
  glab?: {
    installed: boolean
    authenticated: boolean
    version?: string
  }
  git: {
    installed: boolean
    version?: string
    hasUserName: boolean
    hasUserEmail: boolean
  }
}

// ─── Per-Server Onboarding Checklist ─────────────────────────────────────────

/** Checklist state tracked per dev server (keyed by devServerId) */
export type PerServerChecklistState = {
  addedRepo?: boolean
  ranFirstAgent?: boolean
  ranSecondAgentOnSameTask?: boolean
  reviewedDiff?: boolean
  openedPr?: boolean
  addedFolder?: boolean
  openedFile?: boolean
  ranAgentOnFile?: boolean
}

// ─── Windows Terminal Capabilities ───────────────────────────────────────────

/** Windows-specific terminal capabilities detected from remote dev server */
export type WindowsTerminalCapabilities = {
  wslAvailable: boolean
  wslDistros: string[]
  pwshAvailable: boolean
  pwshVersion?: string
  gitBashAvailable: boolean
  gitBashPath?: string
  /** Platform of the relay host (always 'win32' for Windows servers). */
  hostPlatform?: NodeJS.Platform | null
}

/**
 * Payload emitted by DevServerRelayBridge when a direct-websocket token is generated.
 * Shared between main process (IPC sender), preload, and renderer (IPC receiver).
 */
export type AgentTokenInfo = {
  /** ID of the DevServer this token belongs to */
  devServerId: string
  /** One-time token for agent to authenticate: format "agt-<devServerId>-<timestamp>" */
  agentToken: string
  /** Orca WebSocket URL the agent should connect to: "ws://<host>:6768/agent" */
  orcaUrl: string
}

// ─── Dev Server Access Control (CR-DS-006/007/008) ───────────────────────────
// docs/crs/v2/dev-server/CR-DS-006-dev-server-approval-and-grouping.md et seq.

export type DevServerApprovalStatus = 'pending_approval' | 'approved' | 'rejected'

export type DevServerGroup = {
  id: string
  tenantId: string
  name: string
  /** Empty = root of the tree. */
  parentGroupId: string
}

export type DevServerGranteeKind = 'department' | 'team'

export type DevServerGroupGrant = {
  id: string
  tenantId: string
  devServerGroupId: string
  granteeKind: DevServerGranteeKind
  granteeId: string
}

export type DevServerAccessRequestStatus = 'pending' | 'approved' | 'rejected'

export type DevServerAccessRequest = {
  id: string
  tenantId: string
  userId: string
  devServerGroupId: string
  status: DevServerAccessRequestStatus
  message: string
  createdAtUnixMs: number
}
