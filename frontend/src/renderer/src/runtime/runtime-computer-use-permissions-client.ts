import type {
  ComputerUsePermissionId,
  ComputerUsePermissionResetResult,
  ComputerUsePermissionSetupResult,
  ComputerUsePermissionStatusResult
} from '../../../shared/computer-use-permissions-types'
import { callRuntimeRpc } from './runtime-rpc-client'

// Why: computer-use permissions are a native/local-only (macOS) Electron
// feature with zero prior RPC coverage — routed through window.api.runtime.call
// for the same uniform calling convention as every other runtime-*-client.
// Gated on `window.api.agentTrust` (desktop-only preload key, absent from
// web-preload-api.ts) rather than `window.api.runtime` — the latter exists on
// web too and would route into the backend registry, which has no
// `computerUsePermissions.*` methods; web keeps using its existing
// `window.api.computerUsePermissions` stub untouched.
function isDesktopElectronBridge(): boolean {
  return typeof window !== 'undefined' && Boolean(window.api?.agentTrust)
}

export async function getRuntimeComputerUsePermissionStatus(): Promise<ComputerUsePermissionStatusResult | null> {
  if (!isDesktopElectronBridge()) {
    return null
  }
  return callRuntimeRpc<ComputerUsePermissionStatusResult>(
    { kind: 'local' },
    'computerUsePermissions.getStatus',
    undefined,
    { timeoutMs: 15_000 }
  )
}

export async function openRuntimeComputerUsePermissionSetup(
  id?: ComputerUsePermissionId
): Promise<ComputerUsePermissionSetupResult | null> {
  if (!isDesktopElectronBridge()) {
    return null
  }
  return callRuntimeRpc<ComputerUsePermissionSetupResult>(
    { kind: 'local' },
    'computerUsePermissions.openSetup',
    id ? { id } : undefined,
    { timeoutMs: 15_000 }
  )
}

export async function resetRuntimeComputerUsePermissions(): Promise<ComputerUsePermissionResetResult | null> {
  if (!isDesktopElectronBridge()) {
    return null
  }
  return callRuntimeRpc<ComputerUsePermissionResetResult>(
    { kind: 'local' },
    'computerUsePermissions.reset',
    undefined,
    { timeoutMs: 15_000 }
  )
}
