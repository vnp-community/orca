import { is } from '@electron-toolkit/utils'
import { z } from 'zod'
import { defineMethod, type RpcMethod } from '../core'
import type { AppIdentity } from '../../../../shared/app-identity'
import { getDevInstanceIdentity } from '../../../startup/dev-instance-identity'
import { setUnreadDockBadgeCount } from '../../../dock/unread-badge'
import { ensureDefaultFloatingWorkspacePath } from '../../../ipc/floating-workspace-directory'
import {
  getFeatureWallAssetBaseUrl,
  pickFloatingMarkdownDocument,
  readKeyboardInputSourceId
} from '../../../ipc/app'

const SetUnreadDockBadgeCountParams = z.object({ count: z.number() })

// Why: app.* is native/OS-only and previously reachable only via window.api.
// These wrappers call the exact same functions the desktop ipcMain 'app:*'
// handlers already call — see desktop/src/main/ipc/app.ts. Methods needing a
// specific BrowserWindow/webContents sender (app:reload) or main-process
// singleton state that RpcContext does not carry (app:relaunch/app:restart's
// onBeforeRelaunch cleanup, app:awaitFirstWindowStartupServices,
// app:startupDiagnostic, app:getFloatingTerminalCwd and
// app:pickFloatingWorkspaceDirectory's Store dependency) are intentionally
// not migrated — see report for details.
export const APP_METHODS: RpcMethod[] = [
  defineMethod({
    name: 'app.getIdentity',
    params: null,
    handler: (): AppIdentity => {
      const identity = getDevInstanceIdentity(is.dev)
      return {
        name: identity.name,
        isDev: identity.isDev,
        devLabel: identity.devLabel,
        devBranch: identity.devBranch,
        devWorktreeName: identity.devWorktreeName,
        devRepoRoot: identity.devRepoRoot,
        dockBadgeLabel: identity.dockBadgeLabel
      }
    }
  }),
  defineMethod({
    name: 'app.getFeatureWallAssetBaseUrl',
    params: null,
    handler: (): string => getFeatureWallAssetBaseUrl()
  }),
  defineMethod({
    name: 'app.getKeyboardInputSourceId',
    params: null,
    handler: async (): Promise<string | null> => {
      if (process.platform !== 'darwin') {
        return null
      }
      try {
        const stdout = await readKeyboardInputSourceId()
        const trimmed = stdout?.trim() ?? ''
        return trimmed.length > 0 ? trimmed : null
      } catch {
        return null
      }
    }
  }),
  defineMethod({
    name: 'app.setUnreadDockBadgeCount',
    params: SetUnreadDockBadgeCountParams,
    handler: (params) => {
      setUnreadDockBadgeCount(Number.isFinite(params.count) ? params.count : 0)
    }
  }),
  defineMethod({
    name: 'app.getFloatingMarkdownDirectory',
    params: null,
    handler: () => ensureDefaultFloatingWorkspacePath()
  }),
  defineMethod({
    name: 'app.pickFloatingMarkdownDocument',
    params: null,
    // Why: no BrowserWindow to parent the dialog to over RPC (no
    // IpcMainInvokeEvent sender) — same no-parent fallback the ipc handler
    // already takes when BrowserWindow.fromWebContents(event.sender) is null.
    handler: () => pickFloatingMarkdownDocument(null)
  })
]
