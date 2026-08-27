import {
  sanitizeCrashReportString,
  type CrashReportCopySubmissionFailure
} from '../../shared/crash-reporting'

// Why: pure text formatting, ported verbatim from
// desktop/src/main/crash-reporting/crash-report-copy-text.ts (it has no
// Electron dependency — see crash-reports.ts's copyLatestDiagnostics handler
// for why server mode returns this text instead of writing it anywhere).
function sanitizedNonEmptyString(value: unknown): string | null {
  if (typeof value !== 'string') {
    return null
  }
  const sanitized = sanitizeCrashReportString(value.replace(/[\r\n]+/g, ' ')).trim()
  return sanitized || null
}

function normalizeSubmissionFailure(value: unknown): CrashReportCopySubmissionFailure | null {
  if (!value || typeof value !== 'object' || Array.isArray(value)) {
    return null
  }
  const record = value as Record<string, unknown>
  const error = sanitizedNonEmptyString(record.error)
  if (!error) {
    return null
  }

  const rawContext = record.diagnosticContext
  if (!rawContext || typeof rawContext !== 'object' || Array.isArray(rawContext)) {
    return { error }
  }
  const context = rawContext as Record<string, unknown>
  if (context.status === 'uploaded') {
    const ticketId = sanitizedNonEmptyString(context.ticketId)
    return ticketId ? { error, diagnosticContext: { status: 'uploaded', ticketId } } : { error }
  }
  if (context.status === 'not_uploaded') {
    const reason = sanitizedNonEmptyString(context.reason)
    return reason ? { error, diagnosticContext: { status: 'not_uploaded', reason } } : { error }
  }
  return { error }
}

export function formatCrashReportCopyText(baseText: string, submissionFailure: unknown): string {
  const failure = normalizeSubmissionFailure(submissionFailure)
  if (!failure) {
    return baseText
  }

  const lines = [baseText, '', 'Submission failure:', `- Report error: ${failure.error}`]
  if (failure.diagnosticContext?.status === 'uploaded') {
    lines.push(`- Diagnostic ticket uploaded but not linked: ${failure.diagnosticContext.ticketId}`)
  } else if (failure.diagnosticContext?.status === 'not_uploaded') {
    lines.push(`- Diagnostic logs not uploaded: ${failure.diagnosticContext.reason}`)
  }
  return lines.join('\n')
}
