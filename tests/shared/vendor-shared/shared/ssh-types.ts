// ─── SSH Connection Types ───────────────────────────────────────────

export const MIN_SSH_RELAY_GRACE_PERIOD_SECONDS = 60
export const MAX_SSH_RELAY_GRACE_PERIOD_SECONDS = 7 * 24 * 60 * 60
export const LEGACY_DEFAULT_SSH_RELAY_GRACE_PERIOD_SECONDS = 3 * 60 * 60
export const DEFAULT_BOUNDED_SSH_RELAY_GRACE_PERIOD_SECONDS = 24 * 60 * 60
export const DEFAULT_SSH_RELAY_GRACE_PERIOD_SECONDS = 0
export const SSH_RELAY_CONFIGURE_GRACE_TIME_METHOD = 'relay.configureGraceTime'

export type SshTarget = {
  id: string
  label: string
  /** Internal owner for targets that Orca creates as implementation details.
   *  Owned targets are hidden from normal SSH-host management surfaces. */
  owner?: { type: 'on-demand-runtime'; runtimeId: string }
  /** Host alias to resolve through OpenSSH config (ssh -G). */
  configHost?: string
  host: string
  port: number
  username: string
  /** Path to private key file, if using key-based auth. */
  identityFile?: string
  /** SSH agent socket path from IdentityAgent, if configured. */
  identityAgent?: string
  /** Whether OpenSSH IdentitiesOnly should limit public-key auth attempts. */
  identitiesOnly?: boolean
  /** ProxyCommand from SSH config, if any. */
  proxyCommand?: string
  /** Jump host (ProxyJump), if any. */
  jumpHost?: string
  /** Where this target came from. `ssh-config` targets are kept in sync with
   *  `~/.ssh/config` on import — their config-derived fields (host, port,
   *  username, jump host, identity, proxy) are refreshed on each import.
   *  `manual` targets are never overwritten by import. Legacy persisted targets
   *  predate this field (undefined) and are adopted into config-sync on next
   *  import. */
  source?: 'ssh-config' | 'manual'
  /** Grace period in seconds before relay shuts down after disconnect.
   *  0 disables expiry. Default: 0 (until reset). Max: 604800 (7 days). */
  relayGracePeriodSeconds?: number
  /** Set to true after a successful connection that triggered a credential
   *  prompt (passphrase or password). Persisted so startup reconnect can
   *  partition targets into eager (no passphrase) vs deferred (passphrase)
   *  without attempting a connection first. */
  lastRequiredPassphrase?: boolean
  /** Port forwards to auto-restore on connect/reconnect. Persisted so
   *  forwards survive app restarts. */
  portForwards?: SavedPortForward[]
  /** Reuse a system OpenSSH connection across setup commands. Undefined means
   *  enabled; false is an explicit per-target compatibility opt-out. */
  systemSshConnectionReuse?: boolean

  // ── Fleet Management Fields ────────────────────────────────
  /** Project this server belongs to. e.g. "vnp-blc", "vnp-ai-ops", "vnp-claw" */
  project?: string
  /** Team owning this server. e.g. "backend", "frontend", "ai-platform" */
  team?: string
  /** Deployment environment */
  environment?: 'development' | 'staging' | 'production'
  /** Free-form tags for flexible grouping */
  tags?: string[]
  /** Repos available on this server */
  repos?: {
    path: string    // e.g. /srv/projects/vnp-blc
    name: string    // e.g. vnp-blc
    url?: string    // git remote URL (optional)
    branch?: string // default branch
  }[]
  /** Path to the fleet config file that imported this target */
  fleetConfigSource?: string
  /** Stable ID from fleet config (used to detect re-imports and avoid duplicates) */
  fleetId?: string
}

/** Identity of a removed SSH target, recorded so that re-adding the same host
 *  can re-point orphaned repos/worktrees from the old (deleted) target id to
 *  the new one. Repos store only the target id, so without this record the old
 *  workspaces are stranded on a dead id when the target is removed. */
export type RemovedSshTargetTombstone = {
  /** The id the removed target had — what orphaned repos/worktrees still point at. */
  oldTargetId: string
  /** ssh-config alias, if any — the most stable re-adoption key. */
  configHost?: string
  host: string
  port: number
  username: string
  label: string
  /** ms epoch when the target was removed, for pruning old tombstones. */
  removedAt: number
}

/** Exact repo ownership changes made while re-adopting a removed SSH host. */
export type SshRepoReadoption = {
  oldTargetId: string
  newTargetId: string
  repoIds: string[]
}

export type SshTargetAddResult = {
  target: SshTarget
  repoReadoptions: SshRepoReadoption[]
}

export type SshConfigImportResult = {
  targets: SshTarget[]
  repoReadoptions: SshRepoReadoption[]
}

export type SavedPortForward = {
  localPort: number
  remoteHost: string
  remotePort: number
  label?: string
}

export type SshConnectionStatus =
  | 'disconnected'
  | 'connecting'
  | 'auth-failed'
  | 'deploying-relay'
  | 'connected'
  | 'reconnecting'
  | 'reconnection-failed'
  | 'error'

export type SshRemotePlatform = 'linux' | 'darwin' | 'win32'

export type SshConnectionState = {
  targetId: string
  status: SshConnectionStatus
  error: string | null
  /** Number of reconnection attempts since last disconnect. */
  reconnectAttempt: number
  /** Remote OS detected by the SSH relay once available. */
  remotePlatform?: SshRemotePlatform
}

export type SshRemotePtyLeaseState = 'attached' | 'detached' | 'terminated' | 'expired'

export type SshRemotePtyLease = {
  targetId: string
  ptyId: string
  worktreeId?: string
  tabId?: string
  leafId?: string
  state: SshRemotePtyLeaseState
  createdAt: number
  updatedAt: number
  lastAttachedAt?: number
  lastDetachedAt?: number
}

// ─── Port Forwarding Types ─────────────────────────────────────────

export type PortForwardEntry = {
  id: string
  connectionId: string
  localPort: number
  remoteHost: string
  remotePort: number
  label?: string
  /** Origin captured from terminal output for this remote port (e.g. a Vite
   *  banner printed inside an SSH-hosted PTY). The renderer rewrites the port
   *  to the local forward and trusts the user has DNS for the custom host. */
  advertisedUrl?: string
  /** Protocol parsed from the advertised URL — used to upgrade HTTP guesses
   *  to HTTPS even when the advertised host can't be reused locally. */
  advertisedProtocol?: 'http' | 'https'
}

/** A listening port detected on the remote host by the relay.
 *  Keep in sync with src/relay/port-scan-handler.ts — DetectedPort.
 *  The relay is deployed as a standalone bundle and cannot import from shared. */
export type DetectedPort = {
  port: number
  host: string
  pid?: number
  processName?: string
}

/** A detected SSH port after the main process has mapped terminal-advertised
 *  URLs onto the raw relay scan row for IPC/UI consumption. */
export type EnrichedDetectedPort = DetectedPort & {
  advertisedUrl?: string
  advertisedProtocol?: 'http' | 'https'
}

// ─── Fleet Grouping ────────────────────────────────────────────

/** A group of SSH targets belonging to the same project */
export type SshTargetGroup = {
  key: string           // project name or '__unassigned__'
  label: string         // Display label (project name or 'Unassigned')
  targets: SshTarget[]
  isUnassigned: boolean
}

/**
 * Group a flat list of SshTargets by project.
 * Named projects appear first (sorted alphabetically), 'Unassigned' last.
 * Pure function — safe to run in renderer process without IPC.
 */
export function groupSshTargetsByProject(targets: SshTarget[]): SshTargetGroup[] {
  const map = new Map<string, SshTarget[]>()

  for (const target of targets) {
    const key = target.project ?? '__unassigned__'
    const group = map.get(key) ?? []
    group.push(target)
    map.set(key, group)
  }

  const groups: SshTargetGroup[] = []

  // Named projects first, sorted alphabetically
  const projectKeys = [...map.keys()].filter((k) => k !== '__unassigned__').sort()
  for (const key of projectKeys) {
    groups.push({
      key,
      label: key,
      targets: map.get(key)!,
      isUnassigned: false,
    })
  }

  // Unassigned servers last
  const unassigned = map.get('__unassigned__')
  if (unassigned && unassigned.length > 0) {
    groups.push({
      key: '__unassigned__',
      label: 'Unassigned',
      targets: unassigned,
      isUnassigned: true,
    })
  }

  return groups
}
