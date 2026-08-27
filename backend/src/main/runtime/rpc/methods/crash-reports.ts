import { z } from 'zod'
import { defineMethod, type RpcAnyMethod } from '../core'
import {
  type CrashReportBreadcrumbData,
  type CrashReportCopyDiagnosticsArgs,
  type CrashReportRecord,
  type CrashReportSubmitResult,
  type ReactErrorBoundaryReportResult,
  formatCrashReportText,
  formatUncapturedCrashReportText,
  sanitizeCrashReportDetails
} from '../../../../shared/crash-reporting'
import {
  assertClipboardTextWriteWithinLimit,
  isClipboardTextWriteTooLargeError
} from '../../../../shared/clipboard-text'
import {
  getCrashBreadcrumbSnapshot,
  recordCrashBreadcrumb
} from '../../../crash-reporting/crash-breadcrumb-store'
import {
  buildServerUncapturedCrashReportContext,
  getSharedCrashReportStore,
  serverCrashReportContext,
  type CrashReportStore
} from '../../../crash-reporting/crash-report-store'
import { formatCrashReportCopyText } from '../../../crash-reporting/crash-report-copy-text'
import { submitCrashFeedback } from '../../../crash-reporting/crash-report-feedback-submit'

// Why: server mode has no Electron crashReporter, so there is no native
// pending-crash equivalent of desktop's CrashReportStore capture path (see
// crash-report-store.ts's header comment). This file still matches desktop's
// crash-reports.ts method-for-method (see
// frontend/src/renderer/src/runtime/runtime-crash-reports-client.ts for the
// exact contract) — reports just come from three sources instead of native
// crash dumps: explicit React error-boundary submissions
// (recordRendererError), best-effort process.on('uncaughtException')/
// 'unhandledRejection' capture (installServerCrashReportProcessHandlers,
// wired in server-bootstrap.ts), and the crashReports.submit "uncaptured"
// path for manual feedback-menu reports.

const ReactErrorBoundarySurface = z.enum([
  'app-root',
  'web-root',
  'workspace-shell',
  'sidebar',
  'terminal-workbench',
  'right-sidebar',
  'page',
  'modal',
  'overlay',
  'rich-markdown-editor'
])

const RecordRendererError = z.object({
  boundaryId: z.string().trim().min(1).max(120),
  surface: ReactErrorBoundarySurface,
  errorName: z.string().trim().min(1).max(120).optional(),
  errorMessage: z.string().trim().min(1).max(1_000).optional(),
  errorStack: z.string().max(8_000).optional(),
  componentStack: z.string().max(8_000).optional(),
  activeView: z.string().max(80).optional(),
  activeModal: z.string().max(80).nullable().optional(),
  activeTabType: z.string().max(80).optional(),
  activeRightSidebarTab: z.string().max(80).optional(),
  hasActiveWorktree: z.boolean().optional()
})

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

function sanitizeBreadcrumbData(value: unknown): CrashReportBreadcrumbData | undefined {
  if (!value || typeof value !== 'object' || Array.isArray(value)) {
    return undefined
  }
  const primitiveData: Record<string, unknown> = {}
  for (const [key, entry] of Object.entries(value as Record<string, unknown>)) {
    if (typeof entry === 'string' || typeof entry === 'boolean' || entry === null) {
      primitiveData[key] = entry
    } else if (typeof entry === 'number' && Number.isFinite(entry)) {
      primitiveData[key] = entry
    }
  }
  const sanitized = sanitizeCrashReportDetails(primitiveData)
  return Object.keys(sanitized).length > 0 ? sanitized : undefined
}

// Why: mirrors desktop's ipc/crash-reporting.ts getRequestedCrashReport — a
// caller that omits the whole args object (copyLatestDiagnostics with no
// params) wants "whatever is pending right now"; a caller that passes args
// without a reportId (crashReports.submit's manual feedback-menu path, or a
// diagnostics-copy call scoped to an already-known-absent report) means "no
// captured report" and must not silently substitute a different pending one.
async function getRequestedCrashReport(
  store: CrashReportStore,
  args?: { reportId?: string }
): Promise<CrashReportRecord | null> {
  if (args?.reportId) {
    return store.getById(args.reportId)
  }
  return args ? null : store.getLatestPending()
}

export const CRASH_REPORT_METHODS: readonly RpcAnyMethod[] = [
  defineMethod({
    name: 'crashReports.getLatestPending',
    params: null,
    handler: () => getSharedCrashReportStore().getLatestPending()
  }),
  defineMethod({
    name: 'crashReports.getLatestReport',
    params: null,
    handler: async () => {
      const reports = await getSharedCrashReportStore().listRecent()
      return reports.find((report) => report.status === 'pending' || report.status === 'dismissed') ?? null
    }
  }),
  defineMethod({
    name: 'crashReports.dismiss',
    params: Dismiss,
    handler: async (params) => getSharedCrashReportStore().dismiss(params.reportId)
  }),
  defineMethod({
    name: 'crashReports.recordRendererError',
    params: z.unknown(),
    handler: async (paramsIn): Promise<ReactErrorBoundaryReportResult> => {
      const parsed = RecordRendererError.safeParse(paramsIn)
      if (!parsed.success) {
        return { ok: false, error: 'Invalid renderer error report.' }
      }
      const args = parsed.data
      try {
        // Why: unlike desktop, this does not dedupe repeated reports from the
        // same boundary within a time window — MAX_REPORTS already caps
        // on-disk growth, and a looping boundary is a rarer failure mode
        // server-side than in Electron's crashReporter path this replaces.
        const report = await getSharedCrashReportStore().record({
          source: 'renderer',
          processType: 'react-render',
          reason: 'react-error-boundary',
          exitCode: null,
          ...serverCrashReportContext(),
          details: {
            boundary_id: args.boundaryId,
            surface: args.surface,
            error_name: args.errorName ?? 'Error',
            error_message: args.errorMessage ?? 'Unknown render error',
            ...(args.errorStack ? { error_stack: args.errorStack } : {}),
            ...(args.componentStack ? { component_stack: args.componentStack } : {}),
            ...(args.activeView ? { active_view: args.activeView } : {}),
            ...(args.activeModal !== undefined ? { active_modal: args.activeModal } : {}),
            ...(args.activeTabType ? { active_tab_type: args.activeTabType } : {}),
            ...(args.activeRightSidebarTab
              ? { right_sidebar_tab: args.activeRightSidebarTab }
              : {}),
            ...(args.hasActiveWorktree !== undefined
              ? { has_active_worktree: args.hasActiveWorktree }
              : {})
          },
          breadcrumbs: getCrashBreadcrumbSnapshot()
        })
        return { ok: true, report, deduped: false }
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
      recordCrashBreadcrumb(params.name, sanitizeBreadcrumbData(params.data))
    }
  }),
  defineMethod({
    name: 'crashReports.submit',
    params: Submit,
    handler: async (args): Promise<CrashReportSubmitResult> => {
      const store = getSharedCrashReportStore()
      const report = await getRequestedCrashReport(store, args)

      if (!report) {
        const text = formatUncapturedCrashReportText(
          buildServerUncapturedCrashReportContext(),
          args.notes
        )
        const result = await submitCrashFeedback({
          feedback: text,
          submissionType: 'crash',
          submitAnonymously: args.submitAnonymously,
          githubLogin: args.githubLogin,
          githubEmail: args.githubEmail
        })
        return result.ok
          ? { ok: true, report: null }
          : { ok: false, status: result.status, error: result.error, report: null }
      }

      const canSubmitDismissedReport = Boolean(args.reportId && report.status === 'dismissed')
      if (!canSubmitDismissedReport && report.status !== 'pending') {
        return { ok: true, report }
      }

      const text = formatCrashReportText(report, args.notes)
      const result = await submitCrashFeedback({
        feedback: text,
        submissionType: 'crash',
        submitAnonymously: args.submitAnonymously,
        githubLogin: args.githubLogin,
        githubEmail: args.githubEmail
      })
      if (!result.ok) {
        return { ok: false, status: result.status, error: result.error, report }
      }

      if (report.status === 'dismissed') {
        const sent = await store.markDismissedSent(report.id)
        return { ok: true, report: sent ?? { ...report, status: 'sent' } }
      }
      const sent = await store.markSent(report.id)
      return { ok: true, report: sent ?? { ...report, status: 'sent' } }
    }
  }),
  defineMethod({
    name: 'crashReports.copyLatestDiagnostics',
    params: CopyLatestDiagnostics,
    handler: async (paramsIn) => {
      const store = getSharedCrashReportStore()
      const args = paramsIn as CrashReportCopyDiagnosticsArgs | undefined
      const report = await getRequestedCrashReport(store, args)
      const baseText = report
        ? formatCrashReportText(report, args?.notes)
        : formatUncapturedCrashReportText(buildServerUncapturedCrashReportContext(), args?.notes)
      const text = formatCrashReportCopyText(baseText, args?.submissionFailure)
      try {
        // Why: server mode has no OS clipboard to write to — the RPC caller
        // is a browser tab, not the process's own desktop session. Return the
        // formatted text so the frontend copies it via the browser's own
        // navigator.clipboard instead of a server-side clipboard write (see
        // module comment). Still enforce the same size guard desktop uses
        // before a clipboard write, so an oversized payload fails the same way.
        assertClipboardTextWriteWithinLimit(text)
      } catch (error) {
        if (isClipboardTextWriteTooLargeError(error)) {
          return { ok: false as const, error: 'Crash diagnostics are too large to copy safely.' }
        }
        throw error
      }
      return { ok: true as const, text }
    }
  })
]
