import type {
  BrowserCookieImportResult,
  BrowserSessionProfile,
  GlobalSettings
} from '../../../shared/types'
import type {
  BrowserDetectedInfo,
  BrowserDetectProfilesResult,
  BrowserProfileClearDefaultCookiesResult,
  BrowserProfileCreateResult,
  BrowserProfileDeleteResult,
  BrowserProfileImportFromBrowserResult,
  BrowserProfileListResult
} from '../../../shared/runtime-types'
import { callRuntimeRpc, getActiveRuntimeTarget } from './runtime-rpc-client'

export type RuntimeBrowserSettings =
  | Pick<GlobalSettings, 'activeRuntimeEnvironmentId'>
  | null
  | undefined

// Why: browser session profiles (cookie isolation) are owned by whichever
// browser engine is actually running the pages — the local Electron process
// or the remote runtime host — so listing/creating/deleting profiles must
// follow the active runtime target like every other hybrid RPC client.

export async function browserProfileList(
  settings: RuntimeBrowserSettings
): Promise<BrowserSessionProfile[]> {
  const target = getActiveRuntimeTarget(settings)
  if (target.kind === 'environment') {
    const result = await callRuntimeRpc<BrowserProfileListResult>(
      target,
      'browser.profileList',
      undefined,
      { timeoutMs: 15_000 }
    )
    return result.profiles
  }
  return window.api.browser.sessionListProfiles() as Promise<BrowserSessionProfile[]>
}

export async function browserProfileCreate(
  settings: RuntimeBrowserSettings,
  scope: BrowserSessionProfileScope,
  label: string
): Promise<BrowserSessionProfile | null> {
  const target = getActiveRuntimeTarget(settings)
  if (target.kind === 'environment') {
    const result = await callRuntimeRpc<BrowserProfileCreateResult>(
      target,
      'browser.profileCreate',
      { scope, label },
      { timeoutMs: 15_000 }
    )
    return result.profile
  }
  return window.api.browser.sessionCreateProfile({ scope, label }) as Promise<
    BrowserSessionProfile | null
  >
}

export async function browserProfileDelete(
  settings: RuntimeBrowserSettings,
  profileId: string
): Promise<boolean> {
  const target = getActiveRuntimeTarget(settings)
  if (target.kind === 'environment') {
    const result = await callRuntimeRpc<BrowserProfileDeleteResult>(
      target,
      'browser.profileDelete',
      { profileId },
      { timeoutMs: 15_000 }
    )
    return result.deleted
  }
  return window.api.browser.sessionDeleteProfile({ profileId })
}

export async function browserProfileDetectBrowsers(
  settings: RuntimeBrowserSettings
): Promise<BrowserDetectedInfo[]> {
  const target = getActiveRuntimeTarget(settings)
  if (target.kind === 'environment') {
    const result = await callRuntimeRpc<BrowserDetectProfilesResult>(
      target,
      'browser.profileDetectBrowsers',
      undefined,
      { timeoutMs: 15_000 }
    )
    return result.browsers
  }
  return window.api.browser.sessionDetectBrowsers() as Promise<BrowserDetectedInfo[]>
}

export async function browserProfileImportFromBrowser(
  settings: RuntimeBrowserSettings,
  args: { profileId: string; browserFamily: string; browserProfile?: string }
): Promise<BrowserProfileImportFromBrowserResult> {
  const target = getActiveRuntimeTarget(settings)
  if (target.kind === 'environment') {
    return callRuntimeRpc<BrowserProfileImportFromBrowserResult>(
      target,
      'browser.profileImportFromBrowser',
      args,
      { timeoutMs: 30_000 }
    )
  }
  return window.api.browser.sessionImportFromBrowser(args) as Promise<BrowserCookieImportResult>
}

export async function browserProfileClearDefaultCookies(
  settings: RuntimeBrowserSettings
): Promise<boolean> {
  const target = getActiveRuntimeTarget(settings)
  if (target.kind === 'environment') {
    const result = await callRuntimeRpc<BrowserProfileClearDefaultCookiesResult>(
      target,
      'browser.profileClearDefaultCookies',
      undefined,
      { timeoutMs: 15_000 }
    )
    return result.cleared
  }
  return window.api.browser.sessionClearDefaultCookies()
}

// Why: manual cookie-file import opens a native OS file picker attached to
// the local Electron window (see desktop `pickCookieFile`) — there is no
// remote-runtime equivalent, so callers must gate this on the active target
// themselves before invoking it (never call while a remote runtime is active).
export async function browserProfileImportCookiesFromFile(
  profileId: string
): Promise<BrowserCookieImportResult> {
  return window.api.browser.sessionImportCookies({ profileId })
}
