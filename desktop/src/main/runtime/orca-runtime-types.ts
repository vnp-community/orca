// frontend/src/main/runtime/orca-runtime-types.ts
// Why (BUG-FE-BIGFILE-002 / TASK-BIGFILE-009): pure type declarations
// extracted from orca-runtime.ts. No runtime logic — safe standalone
// module. Re-exported from orca-runtime.ts so existing external importers
// (e.g. main/runtime/rpc/methods/terminal.ts's `DriverState` import) are
// unaffected; all 14 types are also imported back into orca-runtime.ts
// because OrcaRuntimeService itself uses them as parameter/return types
// throughout its body.
import type {
  AutomationCreateInput,
  AutomationUpdateInput,
  AutomationWorkspaceMode
} from '../../shared/automations-types'
import type { ParsedAgentStatusPayload } from '../../shared/agent-status-types'
import type { PtyProcessInfo } from '../providers/types'
import type { RateLimitState } from '../../shared/rate-limit-types'
import type { RuntimeTerminalDriverState } from '../../shared/runtime-types'
import type {
  ClaudeRateLimitAccountsState,
  CodexRateLimitAccountsState,
  WorktreeStartupLaunch
} from '../../shared/types'

export type RemoteFetchResult = { ok: true } | { ok: false; errorKind: 'git_error' }

export type RemoteTrackingBase = {
  remote: string
  branch: string
  ref: string
  base: string
}

export type AccountsSnapshot = {
  claude: ClaudeRateLimitAccountsState
  codex: CodexRateLimitAccountsState
  rateLimits: RateLimitState
}

export type RuntimeAutomationCreateInput = Omit<
  AutomationCreateInput,
  'projectId' | 'workspaceId' | 'workspaceMode' | 'timezone'
> & {
  repo?: string
  workspace?: string
  workspaceMode?: AutomationWorkspaceMode
  timezone?: string
}

export type RuntimeAutomationUpdateInput = Omit<
  AutomationUpdateInput,
  'projectId' | 'workspaceId'
> & {
  repo?: string
  workspace?: string
}

export type RuntimeTerminalAgentStatusEvent = {
  ptyId: string
  source: 'mounted-leaf' | 'pty-record'
  paneKey: string
  tabId?: string
  worktreeId?: string
  connectionId?: string | null
  payload: ParsedAgentStatusPayload
}

export type RuntimePtyController = {
  spawn?(opts: {
    cols: number
    rows: number
    cwd?: string
    command?: string
    commandDelivery?: 'renderer' | 'provider'
    startupCommandDelivery?: WorktreeStartupLaunch['startupCommandDelivery']
    env?: Record<string, string>
    envToDelete?: string[]
    telemetry?: WorktreeStartupLaunch['telemetry']
    connectionId?: string | null
    worktreeId?: string
    preAllocatedHandle?: string
    tabId?: string
    leafId?: string
    sessionId?: string
    persistHostSessionBinding?: boolean
  }): Promise<{ id: string }>
  write(ptyId: string, data: string): boolean
  kill(ptyId: string): boolean
  stopAndWait?(ptyId: string, opts?: { keepHistory?: boolean }): Promise<boolean>
  getCwd?(ptyId: string): Promise<string | null>
  getForegroundProcess(ptyId: string): Promise<string | null>
  confirmForegroundProcess?(ptyId: string): Promise<string | null>
  hasChildProcesses?(ptyId: string): Promise<boolean>
  clearBuffer?(ptyId: string): Promise<void>
  resize?(ptyId: string, cols: number, rows: number): boolean
  listProcesses?(): Promise<PtyProcessInfo[]>
  serializeBuffer?(
    ptyId: string,
    opts?: { scrollbackRows?: number; altScreenForcesZeroRows?: boolean }
  ): Promise<{ data: string; cols: number; rows: number; lastTitle?: string } | null>
  // Why: synchronous probe used by maybeHydrateHeadlessFromRenderer to skip
  // hydration when no renderer is authoritative for this PTY. See
  // docs/mobile-prefer-renderer-scrollback.md.
  hasRendererSerializer?(ptyId: string): boolean
  getSize?(ptyId: string): { cols: number; rows: number } | null
}

export type MobileNotificationDispatchEvent = {
  type: 'notification'
  source: 'agent-task-complete' | 'terminal-bell' | 'test'
  title: string
  body: string
  worktreeId?: string
  notificationId?: string
}

export type MobileNotificationDismissEvent = {
  type: 'dismiss'
  notificationId: string
}

export type MobileNotificationEvent =
  | MobileNotificationDispatchEvent
  | MobileNotificationDismissEvent

// Why: presence-based driver state for the mobile-presence lock. Exactly one
// driver per PTY at any moment. See docs/mobile-presence-lock.md.
//   - `idle`: no mobile subscribers; desktop input flows freely
//   - `desktop`: at least one mobile client subscribed but desktop reclaimed
//      (or all mobile clients are passive `desktop`-mode watchers); desktop
//      input flows freely
//   - `mobile{clientId}`: a mobile client is the active driver; desktop
//      input/resize are dropped server-side and the lock banner is mounted.
//      `clientId` is the most recent mobile actor for this PTY.
export type DriverState = RuntimeTerminalDriverState

// Why: per-PTY layout target — what the PTY *should* be at right now.
// `desktop` ⇒ runs at the desktop renderer's pane geometry; mobile passive
// watchers (mode='desktop') still receive scrollback. `phone` ⇒ runs at
// `ownerClientId`'s viewport; the desktop renderer's auto-fit is suppressed.
// See docs/mobile-terminal-layout-state-machine.md.
export type PtyLayoutTarget =
  | { kind: 'desktop'; cols: number; rows: number }
  | { kind: 'phone'; cols: number; rows: number; ownerClientId: string }
  | { kind: 'remote-desktop'; cols: number; rows: number; ownerSubscriptionKey: string }

// Why: authoritative layout state with monotonic seq. Bumped on every
// applyLayout success; emitted on mobile subscribe-stream events so clients
// drop stale events that arrive after a newer transition.
export type PtyLayoutState = PtyLayoutTarget & {
  seq: number
  appliedAt: number
}

// Why: applyLayout result discriminator. Callers (especially RPC handlers)
// need to distinguish "shipped a new state at seq N" from "no-op — caller
// should not claim a seq it didn't produce." `pty-exited` is terminal;
// `resize-failed` is transient and the caller may retry.
export type ApplyLayoutResult =
  | { ok: true; state: PtyLayoutState }
  | { ok: false; reason: 'pty-exited' | 'resize-failed' }
