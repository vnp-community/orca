import { callRuntimeRpc } from './runtime-rpc-client'

// Why: the crash-bundle collect/preview/upload lane (telemetry-error-tracking.md
// §User controls) is native/local-only with zero prior RPC coverage — routed
// through window.api.runtime.call for the same uniform calling convention as
// every other runtime-*-client. Not to be confused with the unrelated
// `diagnostics.memory` RPC method. Gated on `window.api.agentTrust`
// (desktop-only) rather than `window.api.runtime` — see
// runtime-computer-use-permissions-client.ts's Why; web keeps using its
// existing `window.api.diagnostics` "Unavailable on web" stub untouched.
function isDesktopElectronBridge(): boolean {
  return typeof window !== 'undefined' && Boolean(window.api?.agentTrust)
}

export type RuntimeDiagnosticsBundlePreview = {
  bundleSubmissionId: string
  bytes: number
  spanCount: number
}

export async function getRuntimeDiagnosticsStatus(): Promise<unknown> {
  if (!isDesktopElectronBridge()) {
    return null
  }
  return callRuntimeRpc({ kind: 'local' }, 'diagnostics.getStatus', undefined, {
    timeoutMs: 15_000
  })
}

export async function collectRuntimeDiagnosticsBundle(
  lookbackMinutes?: number
): Promise<RuntimeDiagnosticsBundlePreview | null> {
  if (!isDesktopElectronBridge()) {
    return null
  }
  return callRuntimeRpc<RuntimeDiagnosticsBundlePreview>(
    { kind: 'local' },
    'diagnostics.collectBundle',
    lookbackMinutes !== undefined ? { lookbackMinutes } : undefined,
    { timeoutMs: 30_000 }
  )
}

export async function openRuntimeDiagnosticsBundlePreview(
  bundleSubmissionId: string
): Promise<void> {
  if (!isDesktopElectronBridge()) {
    return
  }
  await callRuntimeRpc<void>(
    { kind: 'local' },
    'diagnostics.openBundlePreview',
    { bundleSubmissionId },
    { timeoutMs: 15_000 }
  )
}

export async function discardRuntimeDiagnosticsBundlePreview(
  bundleSubmissionId: string
): Promise<void> {
  if (!isDesktopElectronBridge()) {
    return
  }
  await callRuntimeRpc<void>(
    { kind: 'local' },
    'diagnostics.discardBundlePreview',
    { bundleSubmissionId },
    { timeoutMs: 15_000 }
  )
}

export async function uploadRuntimeDiagnosticsBundle(bundleSubmissionId: string): Promise<unknown> {
  if (!isDesktopElectronBridge()) {
    return null
  }
  return callRuntimeRpc(
    { kind: 'local' },
    'diagnostics.uploadBundle',
    { bundleSubmissionId },
    { timeoutMs: 60_000 }
  )
}

export async function deleteRuntimeDiagnosticsBundle(ticketId: string): Promise<void> {
  if (!isDesktopElectronBridge()) {
    return
  }
  await callRuntimeRpc<void>(
    { kind: 'local' },
    'diagnostics.deleteBundle',
    { ticketId },
    { timeoutMs: 15_000 }
  )
}
