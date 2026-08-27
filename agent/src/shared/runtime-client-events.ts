import type {
  CreateWorktreeResult,
  WorktreeBaseStatusEvent,
  WorktreeDefaultTabsLaunch,
  WorktreeRemoteBranchConflictEvent,
  WorktreeSetupLaunch,
  WorktreeStartupLaunch
} from './types'
import type { SshConnectionState } from './ssh-types'

// Why: menu-bar clicks and global keyboard shortcuts both resolve to the same
// command surface (createMainWindow.ts's before-input-event dispatch and
// index.ts's App Menu click handlers funnel into identical webContents.send
// channels) — one command-id payload covers both trigger paths instead of a
// distinct event type per command. Scoped to commands that are (a) pure
// store/view navigation or layout toggles, with no dependency on a native
// Menu/webview object or local-device-only IPC data, and (b) safe to replay
// against a paired client's own local state (excludes destructive actions
// like delete-workspace, whose "current" target is inherently per-device —
// see notifyMenuCommand's callers for the full command surface and the
// excluded commands' reasoning).
export type MenuCommandPayload =
  | { command: 'openSettings' }
  | { command: 'openSetupGuide' }
  | { command: 'openFeatureTour' }
  | { command: 'toggleLeftSidebar' }
  | { command: 'toggleRightSidebar' }
  | { command: 'toggleWorktreePalette' }
  | { command: 'toggleFloatingTerminal' }
  | { command: 'toggleStatusBar' }
  | { command: 'toggleQuickCommandsMenu' }
  | { command: 'openQuickOpen' }
  | { command: 'openNewWorkspace' }
  | { command: 'openWorkspaceBoard' }
  | { command: 'openTasks' }
  | { command: 'switchRecentTab' }
  | { command: 'jumpToWorktreeIndex'; index: number }
  | { command: 'jumpToTabIndex'; index: number }
  | { command: 'worktreeHistoryNavigate'; direction: 'back' | 'forward' }

// Why: BrowserWindow maximize/unmaximize/fullscreen transitions and the
// powerMonitor 'resume' wake signal are the window-state subset of the
// ui.on* push events — coarse-grained on purpose (one discriminant) since
// callers only ever care about the transition, not a payload.
export type WindowStateChangedState =
  | 'maximized'
  | 'unmaximized'
  | 'enteredFullScreen'
  | 'leftFullScreen'
  | 'systemResumed'

export type RuntimeClientEvent =
  | { type: 'reposChanged' }
  | { type: 'worktreesChanged'; repoId: string }
  // Why: SSH connections live on the runtime host; paired clients have no IPC
  // channel for ssh:state-changed, so without this event their reconnect
  // overlays never learn the host connected (STA-1468).
  | { type: 'sshStateChanged'; targetId: string; state: SshConnectionState }
  | {
      type: 'linearLinkedIssueUpdated'
      worktreeId: string
      identifier: string
      workspaceId: string
    }
  | {
      type: 'activateWorktree'
      repoId: string
      worktreeId: string
      setup?: WorktreeSetupLaunch
      startup?: WorktreeStartupLaunch
      defaultTabs?: WorktreeDefaultTabsLaunch
    }
  // Why: desktop's base-status drift reconciliation and remote-branch-conflict
  // detection only reached the Electron notifier (IPC) before this — paired
  // web/mobile runtime clients never saw them. Bridge onto the same
  // client-events stream so runtime targets get the same live updates.
  | ({ type: 'worktreeBaseStatus' } & WorktreeBaseStatusEvent)
  | ({ type: 'worktreeRemoteBranchConflict' } & WorktreeRemoteBranchConflictEvent)
  | { type: 'worktreeCreateProgress'; creationId?: string; phase: 'fetching' | 'creating' }
  // Why: extends the same bridge to the menu/shortcut/window-state subset of
  // ui.on* push events so a paired client mirroring this host's session sees
  // the same navigation/layout commands and window transitions.
  | ({ type: 'menuCommand' } & MenuCommandPayload)
  | { type: 'windowStateChanged'; state: WindowStateChangedState }

export type RuntimeClientEventStreamMessage =
  | ({ type: 'ready'; subscriptionId: string } & {
      snapshot?: {
        // Reserved for future hydration. Current clients refresh through the
        // existing repo/worktree RPCs after receiving server events.
        repos?: unknown[]
      }
    })
  | RuntimeClientEvent
  | { type: 'end' }

export type RuntimeActivateWorktreeEvent = Extract<RuntimeClientEvent, { type: 'activateWorktree' }>

export function toRuntimeActivateWorktreeEvent(
  repoId: string,
  worktreeId: string,
  setup?: CreateWorktreeResult['setup'],
  startup?: WorktreeStartupLaunch,
  defaultTabs?: CreateWorktreeResult['defaultTabs']
): RuntimeActivateWorktreeEvent {
  return {
    type: 'activateWorktree',
    repoId,
    worktreeId,
    ...(setup ? { setup } : {}),
    ...(startup ? { startup } : {}),
    ...(defaultTabs ? { defaultTabs } : {})
  }
}
