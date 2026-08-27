// Why: updater.* is native/OS-only (electron-updater) with no remote-runtime-
// environment equivalent, so — like shell.*/app.* — this branches on
// isWebClientLocation(), not RuntimeClientTarget; the web build keeps its
// existing window.api.updater.* behavior (no-op/unsupported) unchanged.
import type { UpdateCheckOptions, UpdateStatus } from '../../../shared/types'
import {
  ORCA_EDITOR_PREPARE_HOT_EXIT_EVENT,
  type EditorPrepareHotExitDetail
} from '../../../shared/editor-save-events'
import {
  ORCA_UPDATER_QUIT_AND_INSTALL_ABORTED_EVENT,
  ORCA_UPDATER_QUIT_AND_INSTALL_STARTED_EVENT
} from '../../../shared/updater-renderer-events'
import { isWebClientLocation } from '../lib/web-client-location'
import { callRuntimeRpc } from './runtime-rpc-client'

const LOCAL_TARGET = { kind: 'local' } as const

export async function updaterGetStatus(): Promise<UpdateStatus> {
  if (isWebClientLocation()) {
    return window.api.updater.getStatus()
  }
  return callRuntimeRpc(LOCAL_TARGET, 'updater.getStatus')
}

export async function updaterGetVersion(): Promise<string> {
  if (isWebClientLocation()) {
    return window.api.updater.getVersion()
  }
  return callRuntimeRpc(LOCAL_TARGET, 'updater.getVersion')
}

export async function updaterCheck(options?: UpdateCheckOptions): Promise<void> {
  if (isWebClientLocation()) {
    return window.api.updater.check(options)
  }
  await callRuntimeRpc(LOCAL_TARGET, 'updater.check', options ?? null)
}

export async function updaterDownload(): Promise<void> {
  if (isWebClientLocation()) {
    return window.api.updater.download()
  }
  await callRuntimeRpc(LOCAL_TARGET, 'updater.download')
}

export async function updaterDismissNudge(): Promise<void> {
  if (isWebClientLocation()) {
    return window.api.updater.dismissNudge()
  }
  await callRuntimeRpc(LOCAL_TARGET, 'updater.dismissNudge')
}

// Why: mirrors preload's requestEditorHotExitBackup — dispatched to whichever
// editor-autosave controller has mounted so terminal/editor buffers flush
// before the update installs and quits the process. Preload isn't importable
// from renderer code, so this small DOM-only dance (no Node/Electron API) is
// duplicated here against the same shared event name/type preload uses.
function requestEditorHotExitBackup(): Promise<void> {
  return new Promise<void>((resolvePromise, rejectPromise) => {
    let claimed = false
    window.dispatchEvent(
      new CustomEvent<EditorPrepareHotExitDetail>(ORCA_EDITOR_PREPARE_HOT_EXIT_EVENT, {
        detail: {
          claim: () => {
            claimed = true
          },
          resolve: resolvePromise,
          reject: (message) => rejectPromise(new Error(message))
        }
      })
    )
    if (!claimed) {
      resolvePromise()
    }
  })
}

export async function updaterQuitAndInstall(): Promise<void> {
  if (isWebClientLocation()) {
    return window.api.updater.quitAndInstall()
  }
  window.dispatchEvent(new Event(ORCA_UPDATER_QUIT_AND_INSTALL_STARTED_EVENT))
  try {
    await requestEditorHotExitBackup()
  } catch (error) {
    window.dispatchEvent(new Event(ORCA_UPDATER_QUIT_AND_INSTALL_ABORTED_EVENT))
    throw error
  }
  window.dispatchEvent(new Event('beforeunload'))
  try {
    await callRuntimeRpc(LOCAL_TARGET, 'updater.quitAndInstall')
  } catch (error) {
    window.dispatchEvent(new Event(ORCA_UPDATER_QUIT_AND_INSTALL_ABORTED_EVENT))
    throw error
  }
}
