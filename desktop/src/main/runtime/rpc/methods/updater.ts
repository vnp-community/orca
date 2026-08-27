import { app } from 'electron'
import { z } from 'zod'
import { defineMethod, type RpcMethod } from '../core'
import {
  checkForUpdatesFromMenu,
  dismissNudge,
  downloadUpdate,
  getUpdateStatus,
  quitAndInstall
} from '../../../updater'
import { ensureAutoUpdaterConfigured } from '../../../window/attach-main-window-services'

const CheckParams = z
  .object({
    includePrerelease: z.boolean().optional(),
    includePerfPrerelease: z.boolean().optional()
  })
  .nullish()

// Why: updater.* is native/OS-only (electron-updater) and previously
// reachable only via window.api. These wrappers call the exact same
// functions the desktop ipcMain 'updater:*' handlers call — see
// desktop/src/main/window/attach-main-window-services.ts. The push-event
// pair (updater:status / updater:clearDismissal, broadcast via
// BrowserWindow.webContents.send from desktop/src/main/updater.ts) has no
// event-bus wiring through OrcaRuntimeService the way notifications.subscribe
// does, so onStatus/onClearDismissal are intentionally left on window.api —
// see report for details.
export const UPDATER_METHODS: RpcMethod[] = [
  defineMethod({
    name: 'updater.getStatus',
    params: null,
    handler: () => getUpdateStatus()
  }),
  defineMethod({
    name: 'updater.getVersion',
    params: null,
    handler: () => app.getVersion()
  }),
  defineMethod({
    name: 'updater.check',
    params: CheckParams,
    handler: (params) => {
      ensureAutoUpdaterConfigured()
      return checkForUpdatesFromMenu(params ?? undefined)
    }
  }),
  defineMethod({
    name: 'updater.download',
    params: null,
    handler: () => downloadUpdate()
  }),
  defineMethod({
    name: 'updater.quitAndInstall',
    params: null,
    handler: () => quitAndInstall()
  }),
  defineMethod({
    name: 'updater.dismissNudge',
    params: null,
    handler: () => dismissNudge()
  })
]
