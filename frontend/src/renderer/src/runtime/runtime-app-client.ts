// Why: app.* here covers only the request/response, main-process-singleton-
// free subset of window.api.app — see the desktop app.ts RPC methods file for
// what was intentionally left off window.api-only (relaunch/restart/reload,
// startup-barrier waits, and the Store-backed floating-terminal-cwd/
// pickFloatingWorkspaceDirectory calls). Like shell.*, there's no remote-
// environment equivalent, so this branches on isWebClientLocation(), not
// RuntimeClientTarget — the web build keeps its existing window.api.app.*
// behavior unchanged.
import type { AppIdentity } from '../../../shared/app-identity'
import type { MarkdownDocument } from '../../../shared/types'
import { isWebClientLocation } from '../lib/web-client-location'
import { callRuntimeRpc } from './runtime-rpc-client'

const LOCAL_TARGET = { kind: 'local' } as const

export async function appGetIdentity(): Promise<AppIdentity> {
  if (isWebClientLocation()) {
    return window.api.app.getIdentity()
  }
  return callRuntimeRpc(LOCAL_TARGET, 'app.getIdentity')
}

export async function appGetFeatureWallAssetBaseUrl(): Promise<string> {
  if (isWebClientLocation()) {
    return window.api.app.getFeatureWallAssetBaseUrl()
  }
  return callRuntimeRpc(LOCAL_TARGET, 'app.getFeatureWallAssetBaseUrl')
}

export async function appGetKeyboardInputSourceId(): Promise<string | null> {
  if (isWebClientLocation()) {
    return window.api.app.getKeyboardInputSourceId()
  }
  return callRuntimeRpc(LOCAL_TARGET, 'app.getKeyboardInputSourceId')
}

export async function appSetUnreadDockBadgeCount(count: number): Promise<void> {
  if (isWebClientLocation()) {
    return window.api.app.setUnreadDockBadgeCount(count)
  }
  await callRuntimeRpc(LOCAL_TARGET, 'app.setUnreadDockBadgeCount', { count })
}

export async function appGetFloatingMarkdownDirectory(): Promise<string> {
  if (isWebClientLocation()) {
    return window.api.app.getFloatingMarkdownDirectory()
  }
  return callRuntimeRpc(LOCAL_TARGET, 'app.getFloatingMarkdownDirectory')
}

export async function appPickFloatingMarkdownDocument(): Promise<MarkdownDocument | null> {
  if (isWebClientLocation()) {
    return window.api.app.pickFloatingMarkdownDocument()
  }
  return callRuntimeRpc(LOCAL_TARGET, 'app.pickFloatingMarkdownDocument')
}
