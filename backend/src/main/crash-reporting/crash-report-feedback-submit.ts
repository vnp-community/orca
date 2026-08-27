import os from 'node:os'
import { app } from 'electron'

// Why: server mode has no equivalent of desktop's ipc/feedback.ts — a search
// of backend/src/main found no existing feedback-submission code path. This
// is a minimal server-appropriate port: same two-endpoint (primary +
// legacy-fallback) upload as desktop, but using the platform's built-in
// fetch instead of Electron's `net.fetch` (desktop routes through main-process
// net to dodge a file:// origin CORS preflight — the server has no such
// origin) and with no diagnostic-log-bundle attachment, since desktop's
// bundle collector (main/observability/collectDiagnosticBundle) has no
// backend port. `includeDiagnosticLogs` is accepted for API-shape parity but
// is always a no-op here.
const FEEDBACK_API_URL = 'https://www.onorca.dev/v1/feedback'
const FEEDBACK_API_FALLBACK_URL = 'https://api.onorca.dev/v1/feedback'
const FEEDBACK_REQUEST_TIMEOUT_MS = 10_000

export type CrashFeedbackSubmissionType = 'feedback' | 'crash'

export type CrashFeedbackSubmitArgs = {
  feedback: string
  submissionType: CrashFeedbackSubmissionType
  submitAnonymously?: boolean
  githubLogin: string | null
  githubEmail: string | null
}

export type CrashFeedbackSubmitResult =
  | { ok: true }
  | { ok: false; status: number | null; error: string }

type FeedbackSubmitBody = {
  feedback: string
  submissionType: CrashFeedbackSubmissionType
  githubLogin: string | null
  githubEmail: string | null
  appVersion: string
  platform: NodeJS.Platform
  osRelease: string
  arch: string
}

function buildSubmitBody(args: CrashFeedbackSubmitArgs): FeedbackSubmitBody {
  const identity = args.submitAnonymously
    ? { githubLogin: null, githubEmail: null }
    : { githubLogin: args.githubLogin, githubEmail: args.githubEmail }

  return {
    feedback: args.feedback,
    submissionType: args.submissionType,
    ...identity,
    appVersion: app.getVersion(),
    platform: process.platform,
    osRelease: os.release(),
    arch: process.arch
  }
}

function messageFromError(error: unknown): string {
  return error instanceof Error ? error.message : String(error)
}

async function postFeedback(url: string, body: FeedbackSubmitBody): Promise<Response> {
  const controller = new AbortController()
  const timeout = setTimeout(() => controller.abort(), FEEDBACK_REQUEST_TIMEOUT_MS)
  try {
    return await fetch(url, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
      signal: controller.signal
    })
  } catch (error) {
    if (controller.signal.aborted) {
      throw new Error(`request timed out after ${FEEDBACK_REQUEST_TIMEOUT_MS / 1000} seconds`)
    }
    throw error
  } finally {
    clearTimeout(timeout)
  }
}

async function submitFallbackFeedback(
  body: FeedbackSubmitBody,
  primaryError?: unknown
): Promise<CrashFeedbackSubmitResult> {
  try {
    const fallback = await postFeedback(FEEDBACK_API_FALLBACK_URL, body)
    if (fallback.ok) {
      return { ok: true }
    }
    return { ok: false, status: fallback.status, error: `status ${fallback.status}` }
  } catch (fallbackError) {
    const message = messageFromError(fallbackError)
    if (primaryError === undefined) {
      return { ok: false, status: null, error: message }
    }
    return {
      ok: false,
      status: null,
      error: `${messageFromError(primaryError)}; fallback: ${message}`
    }
  }
}

export async function submitCrashFeedback(
  args: CrashFeedbackSubmitArgs
): Promise<CrashFeedbackSubmitResult> {
  const body = buildSubmitBody(args)
  try {
    const res = await postFeedback(FEEDBACK_API_URL, body)
    if (res.ok) {
      return { ok: true }
    }
    // Why: keep api.onorca.dev as a compatibility fallback, mirroring desktop's
    // submitFeedback — the website API owns Slack file/snippet crash delivery.
    if (res.status === 404 || res.status >= 500) {
      return submitFallbackFeedback(body)
    }
    return { ok: false, status: res.status, error: `status ${res.status}` }
  } catch (error) {
    return submitFallbackFeedback(body, error)
  }
}
