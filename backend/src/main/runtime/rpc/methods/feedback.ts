import { z } from 'zod'
import { defineMethod, type RpcAnyMethod } from '../core'
import { submitCrashFeedback } from '../../../crash-reporting/crash-report-feedback-submit'

const FeedbackSubmitArgs = z.object({
  feedback: z.string(),
  submitAnonymously: z.boolean().optional(),
  githubLogin: z.string().nullable(),
  githubEmail: z.string().nullable()
})

// Why: a DIFFERENT namespace from crashReports.submit (already ported) —
// desktop's ipc/feedback.ts is general product feedback, not a crash report.
// Desktop's submitFeedback() additionally supports a diagnostic-bundle
// multipart attachment used only by the crash-reporting caller; the plain
// feedback.submit RPC method never passes one, so its behavior is already
// identical to backend's submitCrashFeedback (built earlier this session for
// crashReports.submit, same two-endpoint POST, same body shape) called with
// submissionType: 'feedback' — reusing it here avoids standing up a second,
// near-identical HTTP client for the same /v1/feedback endpoint.
export const FEEDBACK_METHODS: readonly RpcAnyMethod[] = [
  defineMethod({
    name: 'feedback.submit',
    params: FeedbackSubmitArgs,
    handler: (params) => submitCrashFeedback({ ...params, submissionType: 'feedback' })
  })
]
