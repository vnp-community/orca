import type {
  DeveloperPermissionId,
  DeveloperPermissionRequestResult,
  DeveloperPermissionState
} from '../../../shared/developer-permissions-types'
import { callRuntimeRpc } from './runtime-rpc-client'

// Why: developer permissions (mic/camera/screen/accessibility/etc TCC probes)
// are a native/local-only Electron feature with zero prior RPC coverage —
// routed through window.api.runtime.call for the same uniform calling
// convention as every other runtime-*-client. Gated on `window.api.agentTrust`
// (desktop-only) rather than `window.api.runtime` — see
// runtime-computer-use-permissions-client.ts's Why for the web-regression
// this avoids; web keeps using its existing `window.api.developerPermissions`
// stub untouched.
function isDesktopElectronBridge(): boolean {
  return typeof window !== 'undefined' && Boolean(window.api?.agentTrust)
}

export async function getRuntimeDeveloperPermissionStatus(): Promise<
  DeveloperPermissionState[] | null
> {
  if (!isDesktopElectronBridge()) {
    return null
  }
  return callRuntimeRpc<DeveloperPermissionState[]>(
    { kind: 'local' },
    'developerPermissions.getStatus',
    undefined,
    { timeoutMs: 15_000 }
  )
}

export async function requestRuntimeDeveloperPermission(
  id: DeveloperPermissionId
): Promise<DeveloperPermissionRequestResult | null> {
  if (!isDesktopElectronBridge()) {
    return null
  }
  return callRuntimeRpc<DeveloperPermissionRequestResult>(
    { kind: 'local' },
    'developerPermissions.request',
    { id },
    { timeoutMs: 15_000 }
  )
}

export async function openRuntimeDeveloperPermissionSettings(
  id: DeveloperPermissionId
): Promise<void> {
  if (!isDesktopElectronBridge()) {
    return
  }
  await callRuntimeRpc<void>(
    { kind: 'local' },
    'developerPermissions.openSettings',
    { id },
    { timeoutMs: 15_000 }
  )
}
