/* oxlint-disable max-lines -- Why: mirrors the multi-handler shape of
   desktop/src/main/ipc/crash-reporting.ts one-for-one; splitting further would
   separate closely related submission/copy flows that share the same guards. */
import { z } from 'zod'
import { clipboard } from 'electron'
import { defineMethod, type RpcMethod } from '../core'
import {
  type CrashReportBreadcrumbData,
  type CrashReportCopyDiagnosticsArgs,
  type CrashReportSubmitArgs,
  type CrashReportSubmitResult,
  formatCrashReportText
} from '../../../../shared/crash-reporting'
import {
  assertClipboardTextWriteWithinLimit,
  isClipboardTextWriteTooLargeError
} from '../../../../shared/clipboard-text'
import { submitFeedback } from '../../../ipc/feedback'
import { recordCrashBreadcrumb } from '../../../crash-reporting/crash-breadcrumb-store'
import {
  diagnosticBundleForReportOnlyRetry,
  prepareCrashDiagnosticBundle,
  resolveSubmittedDiagnosticBundle
} from '../../../crash-reporting/crash-feedback-diagnostic-bundle'
import { formatCrashReportCopyText } from '../../../crash-reporting/crash-report-copy-text'
import {
  buildUncapturedCrashReportText,
  clearCrashReportSubmissionInFlight,
  getCrashReportStoreForRpc,
  getLatestPendingReport,
  getLatestSendableReport,
  getRequestedCrashReport,
  isCrashReportAlreadySubmitted,
  isCrashReportSubmissionInFlight,
  markCrashReportSubmissionInFlight,
  recordRendererBreadcrumbTrace,
  recordRendererErrorReport,
  rememberSubmittedReportId,
  sanitizeRendererBreadcrumbData
} from '../../../ipc/crash-reporting'
import type { CrashReportStore } from '../../../crash-reporting/crash-report-store'

function requireStore(): CrashReportStore {
  const store = getCrashReportStoreForRpc()
  if (!store) {
    throw new Error('runtime_unavailable')
  }
  return store
}

const Dismiss = z.object({ reportId: z.string().min(1) })
const RecordBreadcrumb = z.object({ name: z.string().min(1), data: z.unknown().optional() })
const Submit = z.object({
  reportId: z.string().min(1).optional(),
  notes: z.string().optional(),
  includeDiagnosticLogs: z.boolean().optional(),
  submitAnonymously: z.boolean().optional(),
  githubLogin: z.string().nullable(),
  githubEmail: z.string().nullable()
})
const CopyLatestDiagnostics = z
  .object({
    reportId: z.string().min(1).optional(),
    notes: z.string().optional(),
    submissionFailure: z.unknown().optional()
  })
  .optional()

// Why: mirrors desktop/src/main/ipc/crash-reporting.ts's seven ipcMain
// handlers — same dedupe/in-flight guards (exported from that module), same
// formatting and upload helpers — so a report submitted via RPC is
// indistinguishable in the CrashReportStore from one submitted via the
// renderer's crash-report dialog.
export const CRASH_REPORTS_METHODS: RpcMethod[] = [
  defineMethod({
    name: 'crashReports.getLatestPending',
    params: null,
    handler: () => getLatestPendingReport(requireStore())
  }),
  defineMethod({
    name: 'crashReports.getLatestReport',
    params: null,
    handler: () => getLatestSendableReport(requireStore())
  }),
  defineMethod({
    name: 'crashReports.dismiss',
    params: Dismiss,
    handler: async (params) => {
      const store = requireStore()
      if (isCrashReportSubmissionInFlight(params.reportId)) {
        return store.getById(params.reportId)
      }
      if (isCrashReportAlreadySubmitted(params.reportId)) {
        const report = await store.getById(params.reportId)
        return report ? { ...report, status: 'sent' as const } : null
      }
      return store.dismiss(params.reportId)
    }
  }),
  defineMethod({
    name: 'crashReports.recordRendererError',
    params: z.unknown(),
    handler: async (params) => {
      try {
        return await recordRendererErrorReport(requireStore(), params)
      } catch (error) {
        console.error('[crash-reports rpc] Failed to record renderer error report:', error)
        return { ok: false, error: 'Failed to record renderer error report.' }
      }
    }
  }),
  defineMethod({
    name: 'crashReports.recordBreadcrumb',
    params: RecordBreadcrumb,
    handler: (params): void => {
      const data = sanitizeRendererBreadcrumbData(params.data) as CrashReportBreadcrumbData | undefined
      recordCrashBreadcrumb(params.name, data)
      recordRendererBreadcrumbTrace(params.name, data)
    }
  }),
  defineMethod({
    name: 'crashReports.submit',
    params: Submit,
    handler: async (paramsIn): Promise<CrashReportSubmitResult> => {
      const store = requireStore()
      const args = paramsIn as CrashReportSubmitArgs
      const report = await getRequestedCrashReport(store, args)
      if (!report) {
        const diagnosticUpload = prepareCrashDiagnosticBundle(args.includeDiagnosticLogs !== false)
        const diagnosticBundle = diagnosticUpload.diagnosticBundle
        const reportOnlyDiagnosticBundle = diagnosticBundleForReportOnlyRetry(diagnosticUpload)
        const result = await submitFeedback({
          feedback: buildUncapturedCrashReportText(args.notes, diagnosticBundle),
          submissionType: 'crash',
          submitAnonymously: args.submitAnonymously,
          githubLogin: args.githubLogin,
          githubEmail: args.githubEmail,
          ...(diagnosticUpload.feedbackDiagnosticBundle
            ? {
                diagnosticBundle: diagnosticUpload.feedbackDiagnosticBundle,
                feedbackWithoutDiagnosticBundle: buildUncapturedCrashReportText(
                  args.notes,
                  reportOnlyDiagnosticBundle
                )
              }
            : {})
        })
        const submittedDiagnosticBundle = resolveSubmittedDiagnosticBundle(diagnosticUpload, result)
        return result.ok
          ? { ok: true, report: null, diagnosticBundle: submittedDiagnosticBundle }
          : {
              ok: false,
              status: result.status,
              error: result.error,
              report: null,
              diagnosticBundle: submittedDiagnosticBundle
            }
      }
      const canSubmitDismissedReport = Boolean(args.reportId && report.status === 'dismissed')
      if (
        (!canSubmitDismissedReport && report.status !== 'pending') ||
        isCrashReportAlreadySubmitted(report.id)
      ) {
        return {
          ok: true,
          report: isCrashReportAlreadySubmitted(report.id) ? { ...report, status: 'sent' } : report
        }
      }
      if (isCrashReportSubmissionInFlight(report.id)) {
        return {
          ok: false,
          status: null,
          error: 'Crash report submission already in progress.',
          report
        }
      }

      markCrashReportSubmissionInFlight(report.id)
      try {
        const diagnosticUpload = prepareCrashDiagnosticBundle(args.includeDiagnosticLogs !== false)
        const diagnosticBundle = diagnosticUpload.diagnosticBundle
        const reportOnlyDiagnosticBundle = diagnosticBundleForReportOnlyRetry(diagnosticUpload)
        const result = await submitFeedback({
          feedback: formatCrashReportText(report, args.notes, diagnosticBundle),
          submissionType: 'crash',
          submitAnonymously: args.submitAnonymously,
          githubLogin: args.githubLogin,
          githubEmail: args.githubEmail,
          ...(diagnosticUpload.feedbackDiagnosticBundle
            ? {
                diagnosticBundle: diagnosticUpload.feedbackDiagnosticBundle,
                feedbackWithoutDiagnosticBundle: formatCrashReportText(
                  report,
                  args.notes,
                  reportOnlyDiagnosticBundle
                )
              }
            : {})
        })
        const submittedDiagnosticBundle = resolveSubmittedDiagnosticBundle(diagnosticUpload, result)
        if (!result.ok) {
          return {
            ok: false,
            status: result.status,
            error: result.error,
            report,
            diagnosticBundle: submittedDiagnosticBundle
          }
        }
        rememberSubmittedReportId(report.id)
        if (report.status === 'dismissed') {
          try {
            const sent = await store.markDismissedSent(report.id)
            return {
              ok: true,
              report: sent ?? { ...report, status: 'sent' },
              diagnosticBundle: submittedDiagnosticBundle
            }
          } catch (error) {
            console.error(
              '[crash-reports rpc] Failed to mark dismissed crash report sent:',
              error
            )
            return {
              ok: true,
              report: { ...report, status: 'sent' },
              diagnosticBundle: submittedDiagnosticBundle
            }
          }
        }
        try {
          const sent = await store.markSent(report.id)
          return {
            ok: true,
            report: sent ?? { ...report, status: 'sent' },
            diagnosticBundle: submittedDiagnosticBundle
          }
        } catch (error) {
          console.error('[crash-reports rpc] Failed to mark crash report sent:', error)
          return {
            ok: true,
            report: { ...report, status: 'sent' },
            diagnosticBundle: submittedDiagnosticBundle
          }
        }
      } finally {
        clearCrashReportSubmissionInFlight(report.id)
      }
    }
  }),
  defineMethod({
    name: 'crashReports.copyLatestDiagnostics',
    params: CopyLatestDiagnostics,
    handler: async (paramsIn) => {
      const store = requireStore()
      const args = paramsIn as CrashReportCopyDiagnosticsArgs | undefined
      const report = await getRequestedCrashReport(store, args)
      const baseText = report
        ? formatCrashReportText(report, args?.notes)
        : buildUncapturedCrashReportText(args?.notes)
      try {
        clipboard.writeText(
          assertClipboardTextWriteWithinLimit(
            formatCrashReportCopyText(baseText, args?.submissionFailure)
          )
        )
      } catch (error) {
        if (isClipboardTextWriteTooLargeError(error)) {
          return { ok: false as const, error: 'Crash diagnostics are too large to copy safely.' }
        }
        throw error
      }
      return { ok: true as const }
    }
  })
]
