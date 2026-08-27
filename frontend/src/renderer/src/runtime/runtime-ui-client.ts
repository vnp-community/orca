// Why: ui.* here covers only the request/response subset of window.api.ui —
// get/set/recordFeatureInteraction (already RPC-backed via client-ui.ts) and
// the OS-clipboard read/write actions (backed by the new ui-actions.ts RPC
// methods). Like shell.* and app.*, there's no remote-runtime-environment
// equivalent (always "this Electron process's OS clipboard" or "this local
// UI state store"), so this branches on isWebClientLocation(), not
// RuntimeClientTarget — the web build keeps its existing window.api.ui.*
// behavior (createWebUiApi in web-preload-api.ts) unchanged.
//
// Left un-migrated (window/webContents-scoped, RpcContext carries no sender
// identity to target — same carve-out app.ts documents for app:reload):
// minimize, maximize, isMaximized, requestClose, popupMenu, confirmWindowClose,
// syncTrafficLights, setMarkdownEditorFocused, setTerminalInputFocused,
// setFloatingTerminalInputFocused, setShortcutRecorderFocused,
// performNativePaste. Also un-migrated: getZoomLevel/setZoomLevel (pure local
// webFrame calls with no main-process round trip to mirror), writeClipboardFile
// (needs the full persistence Store for resolveAuthorizedPath; RpcContext only
// exposes the narrower RuntimeStore), and respondMobileMarkdownRequest /
// replyTabCreate / replyTabSetProfile / replyTabClose / replyTerminalCreate
// (reply legs of a main-initiated push request — owned by the push-event
// migration, not this one).
import type { FeatureInteractionId } from '../../../shared/feature-interaction-catalog'
import type { PersistedUIState } from '../../../shared/types'
import type { ReadClipboardTextOptions } from '../../../shared/clipboard-text'
import { isWebClientLocation } from '../lib/web-client-location'
import { callRuntimeRpc } from './runtime-rpc-client'

const LOCAL_TARGET = { kind: 'local' } as const

export async function uiGet(): Promise<PersistedUIState> {
  if (isWebClientLocation()) {
    return window.api.ui.get()
  }
  const result = await callRuntimeRpc<{ ui: PersistedUIState }>(LOCAL_TARGET, 'ui.get')
  return result.ui
}

export async function uiSet(args: Partial<PersistedUIState>): Promise<void> {
  if (isWebClientLocation()) {
    return window.api.ui.set(args)
  }
  await callRuntimeRpc(LOCAL_TARGET, 'ui.set', args)
}

export async function uiRecordFeatureInteraction(
  id: FeatureInteractionId
): Promise<PersistedUIState> {
  if (isWebClientLocation()) {
    return window.api.ui.recordFeatureInteraction(id)
  }
  const result = await callRuntimeRpc<{ ui: PersistedUIState }>(
    LOCAL_TARGET,
    'ui.recordFeatureInteraction',
    id
  )
  return result.ui
}

export async function uiReadClipboardText(options?: ReadClipboardTextOptions): Promise<string> {
  if (isWebClientLocation()) {
    return window.api.ui.readClipboardText(options)
  }
  return callRuntimeRpc(LOCAL_TARGET, 'ui.readClipboardText', options)
}

export async function uiWriteClipboardText(text: string): Promise<void> {
  if (isWebClientLocation()) {
    return window.api.ui.writeClipboardText(text)
  }
  await callRuntimeRpc(LOCAL_TARGET, 'ui.writeClipboardText', text)
}

export async function uiWriteClipboardImage(dataUrl: string): Promise<void> {
  if (isWebClientLocation()) {
    return window.api.ui.writeClipboardImage(dataUrl)
  }
  await callRuntimeRpc(LOCAL_TARGET, 'ui.writeClipboardImage', dataUrl)
}

export async function uiSaveClipboardImageAsTempFile(args?: {
  connectionId?: string | null
  runtimeEnvironmentId?: string | null
}): Promise<string | null> {
  if (isWebClientLocation()) {
    return window.api.ui.saveClipboardImageAsTempFile(args)
  }
  return callRuntimeRpc(LOCAL_TARGET, 'ui.saveClipboardImageAsTempFile', args)
}
