import type {
  CrashReportBreadcrumbData,
  CrashReportCopyDiagnosticsArgs,
  CrashReportRecord,
  CrashReportSubmitArgs,
  CrashReportSubmitResult,
  ReactErrorBoundaryReportArgs,
  ReactErrorBoundaryReportResult
} from '../../../shared/crash-reporting'
import { callRuntimeRpc } from './runtime-rpc-client'

// Why: crash reporting is a native/local-only Electron feature (writes to
// the on-disk CrashReportStore) with zero prior RPC coverage — routed
// through window.api.runtime.call for the same uniform calling convention as
// every other runtime-*-client. Gated on `window.api.agentTrust`
// (desktop-only) rather than `window.api.runtime` — see
// runtime-computer-use-permissions-client.ts's Why; web keeps using its
// existing `window.api.crashReports` "Unavailable on web" stub untouched.
function isDesktopElectronBridge(): boolean {
  return typeof window !== 'undefined' && Boolean(window.api?.agentTrust)
}

export async function getRuntimeLatestPendingCrashReport(): Promise<CrashReportRecord | null> {
  if (!isDesktopElectronBridge()) {
    return null
  }
  return callRuntimeRpc<CrashReportRecord | null>(
    { kind: 'local' },
    'crashReports.getLatestPending',
    undefined,
    { timeoutMs: 15_000 }
  )
}

export async function getRuntimeLatestCrashReport(): Promise<CrashReportRecord | null> {
  if (!isDesktopElectronBridge()) {
    return null
  }
  return callRuntimeRpc<CrashReportRecord | null>(
    { kind: 'local' },
    'crashReports.getLatestReport',
    undefined,
    { timeoutMs: 15_000 }
  )
}

export async function dismissRuntimeCrashReport(
  reportId: string
): Promise<CrashReportRecord | null> {
  if (!isDesktopElectronBridge()) {
    return null
  }
  return callRuntimeRpc<CrashReportRecord | null>(
    { kind: 'local' },
    'crashReports.dismiss',
    { reportId },
    { timeoutMs: 15_000 }
  )
}

export async function recordRuntimeCrashReportRendererError(
  args: ReactErrorBoundaryReportArgs
): Promise<ReactErrorBoundaryReportResult | null> {
  if (!isDesktopElectronBridge()) {
    return null
  }
  return callRuntimeRpc<ReactErrorBoundaryReportResult>(
    { kind: 'local' },
    'crashReports.recordRendererError',
    args,
    { timeoutMs: 15_000 }
  )
}

export async function recordRuntimeCrashReportBreadcrumb(args: {
  name: string
  data?: CrashReportBreadcrumbData
}): Promise<void> {
  if (!isDesktopElectronBridge()) {
    return
  }
  await callRuntimeRpc<void>({ kind: 'local' }, 'crashReports.recordBreadcrumb', args, {
    timeoutMs: 15_000
  })
}

export async function submitRuntimeCrashReport(
  args: CrashReportSubmitArgs
): Promise<CrashReportSubmitResult | null> {
  if (!isDesktopElectronBridge()) {
    return null
  }
  return callRuntimeRpc<CrashReportSubmitResult>({ kind: 'local' }, 'crashReports.submit', args, {
    timeoutMs: 60_000
  })
}

export async function copyRuntimeCrashReportLatestDiagnostics(
  args?: CrashReportCopyDiagnosticsArgs
): Promise<{ ok: true } | { ok: false; error: string } | null> {
  if (!isDesktopElectronBridge()) {
    return null
  }
  return callRuntimeRpc<{ ok: true } | { ok: false; error: string }>(
    { kind: 'local' },
    'crashReports.copyLatestDiagnostics',
    args,
    { timeoutMs: 15_000 }
  )
}
