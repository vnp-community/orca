// Ports desktop/src/main/runtime/rpc/methods/diagnostics-crash-bundle.ts — a
// SEPARATE lane from runtime/rpc/methods/diagnostics.ts (backend already has
// that file complete; do not confuse the two). "Diagnostic bundle" here means
// a redacted NDJSON export of recent local trace/log lines for a support
// ticket attachment — distinct from crashReports.* (backend/.../
// crash-reporting/crash-report-store.ts), which auto-captures individual
// crash/error events. The two are independent mechanisms on desktop too;
// this keeps that separation rather than merging them.
//
// Two desktop behaviors don't translate to a headless server and are adapted
// rather than 1:1 ported:
//   - openBundlePreview: desktop calls `shell.openPath()` to open the NDJSON
//     file in a native app. A server has no local file browser for the
//     browser client to receive, so this instead returns the bundle payload
//     text directly for the frontend to render as an in-app preview — same
//     "you must review before upload" gate (previewOpened), different
//     delivery mechanism. Same idiom as crashReports.copyLatestDiagnostics
//     (backend/.../rpc/methods/crash-reports.ts) adapting a native
//     clipboard/file action into a value the browser handles client-side.
//   - uploadBundle: desktop shows a native `dialog.showMessageBox` confirm
//     before sending. A server has no native dialog to show the browser
//     user; the frontend is expected to confirm client-side before calling
//     this method, so the confirmation step is simply omitted here.
import { arch as osArch, platform as osPlatform, release as osRelease } from 'node:os'
import { app } from 'electron'
import { z } from 'zod'
import { defineMethod, type RpcAnyMethod } from '../core'
import {
  collectDiagnosticBundle,
  deleteDiagnosticBundle,
  getDiagnosticsStatus,
  uploadDiagnosticBundle,
  type DiagnosticsStatus
} from '../../../observability'
import type { CollectedBundle } from '../../../observability/bundle'
import type { UploadBundleResult } from '../../../observability/diagnostic-bundle-upload'
import {
  resolveDiagnosticOrcaChannel,
  resolveDiagnosticTokenEndpoint
} from '../../../observability/diagnostic-upload-endpoint'

export type DiagnosticsBundlePreview = Omit<CollectedBundle, 'payload'>
type UploadBundleRpcResult = UploadBundleResult | { canceled: true }

const PENDING_BUNDLE_TTL_MS = 15 * 60 * 1000
const MAX_PENDING_BUNDLES = 8
const TICKET_ID_PATTERN = /^[A-Za-z0-9_-]{16,64}$/

type PendingBundle = {
  bundle: CollectedBundle
  readonly createdAtMs: number
  ttlTimer: ReturnType<typeof setTimeout>
  previewOpened: boolean
}

const pendingBundles = new Map<string, PendingBundle>()

function prunePendingBundles(now = Date.now()): void {
  for (const [id, pending] of pendingBundles) {
    if (now - pending.createdAtMs > PENDING_BUNDLE_TTL_MS) {
      deletePendingBundle(id)
    }
  }
  while (pendingBundles.size > MAX_PENDING_BUNDLES) {
    const oldest = pendingBundles.keys().next().value as string | undefined
    if (!oldest) {
      break
    }
    deletePendingBundle(oldest)
  }
}

function deletePendingBundle(bundleSubmissionId: string): void {
  const pending = pendingBundles.get(bundleSubmissionId)
  if (pending) {
    clearTimeout(pending.ttlTimer)
    pendingBundles.delete(bundleSubmissionId)
  }
}

function schedulePendingBundleExpiry(bundleSubmissionId: string): ReturnType<typeof setTimeout> {
  const timer = setTimeout(() => {
    deletePendingBundle(bundleSubmissionId)
  }, PENDING_BUNDLE_TTL_MS)
  if (typeof timer === 'object' && 'unref' in timer) {
    timer.unref()
  }
  return timer
}

function rememberBundle(bundle: CollectedBundle): void {
  deletePendingBundle(bundle.bundleSubmissionId)
  pendingBundles.set(bundle.bundleSubmissionId, {
    bundle,
    createdAtMs: Date.now(),
    ttlTimer: schedulePendingBundleExpiry(bundle.bundleSubmissionId),
    previewOpened: false
  })
  prunePendingBundles()
}

function toBundlePreview(bundle: CollectedBundle): DiagnosticsBundlePreview {
  return {
    bundleSubmissionId: bundle.bundleSubmissionId,
    bytes: bundle.bytes,
    spanCount: bundle.spanCount
  }
}

function requireBundleSubmissionId(value: unknown): string {
  if (typeof value !== 'string' || !TICKET_ID_PATTERN.test(value)) {
    throw new Error('bundleSubmissionId has invalid format')
  }
  return value
}

function getPendingBundleForUpload(bundleSubmissionId: unknown): {
  readonly bundle: CollectedBundle
  readonly payload: string
} {
  const id = requireBundleSubmissionId(bundleSubmissionId)
  prunePendingBundles()
  const pending = pendingBundles.get(id)
  if (!pending) {
    throw new Error('review file has expired; create a new one before sending')
  }
  if (!pending.previewOpened) {
    throw new Error('open the review file before sending')
  }
  return { bundle: pending.bundle, payload: pending.bundle.payload }
}

const CollectBundle = z.object({ lookbackMinutes: z.number().finite().optional() }).optional()
const BundleSubmissionId = z.object({ bundleSubmissionId: z.string() })
const TicketId = z.object({ ticketId: z.string() })

export const DIAGNOSTICS_CRASH_BUNDLE_METHODS: readonly RpcAnyMethod[] = [
  defineMethod({
    name: 'diagnostics.getStatus',
    params: null,
    handler: (): DiagnosticsStatus => getDiagnosticsStatus()
  }),
  defineMethod({
    name: 'diagnostics.collectBundle',
    params: CollectBundle,
    handler: (params): DiagnosticsBundlePreview => {
      const status = getDiagnosticsStatus()
      if (!status.bundleEnabled) {
        throw new Error('creating review files is disabled')
      }
      const lookbackMinutesIn = params?.lookbackMinutes
      const lookbackMinutes =
        typeof lookbackMinutesIn === 'number' && Number.isFinite(lookbackMinutesIn)
          ? Math.max(1, Math.min(30 * 24 * 60, Math.floor(lookbackMinutesIn)))
          : undefined
      const bundle = collectDiagnosticBundle({
        appVersion: app.getVersion(),
        platform: osPlatform(),
        arch: osArch(),
        osRelease: osRelease(),
        orcaChannel: resolveDiagnosticOrcaChannel(),
        ...(lookbackMinutes !== undefined ? { lookbackMinutes } : {})
      })
      rememberBundle(bundle)
      return toBundlePreview(bundle)
    }
  }),
  // Why: see module header — returns preview text instead of opening a
  // native path. Frontend renders this as the "review before sending" view.
  defineMethod({
    name: 'diagnostics.openBundlePreview',
    params: BundleSubmissionId,
    handler: (params): { payload: string } => {
      const id = requireBundleSubmissionId(params.bundleSubmissionId)
      prunePendingBundles()
      const pending = pendingBundles.get(id)
      if (!pending) {
        throw new Error('review file has expired; create a new one before opening')
      }
      pending.previewOpened = true
      return { payload: pending.bundle.payload }
    }
  }),
  defineMethod({
    name: 'diagnostics.discardBundlePreview',
    params: BundleSubmissionId,
    handler: (params): void => {
      deletePendingBundle(requireBundleSubmissionId(params.bundleSubmissionId))
    }
  }),
  defineMethod({
    name: 'diagnostics.uploadBundle',
    params: BundleSubmissionId,
    handler: async (params): Promise<UploadBundleRpcResult> => {
      // Why: no confirmBundleUpload() gate here — see module header. The
      // getPendingBundleForUpload() call still enforces the "reviewed before
      // send" invariant via previewOpened.
      const { bundle, payload } = getPendingBundleForUpload(params.bundleSubmissionId)
      if (!getDiagnosticsStatus().bundleEnabled) {
        throw new Error('sending diagnostics is disabled')
      }
      const tokenEndpoint = resolveDiagnosticTokenEndpoint()
      if (!tokenEndpoint) {
        throw new Error('sending diagnostics is not configured for this build')
      }
      const result = await uploadDiagnosticBundle({
        tokenEndpoint,
        payload,
        bundleSubmissionId: bundle.bundleSubmissionId
      })
      if (pendingBundles.has(bundle.bundleSubmissionId)) {
        deletePendingBundle(bundle.bundleSubmissionId)
      }
      return result
    }
  }),
  defineMethod({
    name: 'diagnostics.deleteBundle',
    params: TicketId,
    handler: async (params): Promise<void> => {
      if (!TICKET_ID_PATTERN.test(params.ticketId)) {
        throw new Error('ticketId has invalid format')
      }
      const tokenEndpoint = resolveDiagnosticTokenEndpoint()
      if (!tokenEndpoint) {
        throw new Error('diagnostic upload endpoint is not configured for this build')
      }
      await deleteDiagnosticBundle({ tokenEndpoint, ticketId: params.ticketId })
    }
  })
]
