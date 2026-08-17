import { z } from 'zod'
import { defineMethod, type RpcMethod } from '../core'
import { submitFeedback } from '../../../ipc/feedback'

const FeedbackSubmitArgs = z.object({
  feedback: z.string(),
  submitAnonymously: z.boolean().optional(),
  githubLogin: z.string().nullable(),
  githubEmail: z.string().nullable()
})

export const FEEDBACK_METHODS: RpcMethod[] = [
  defineMethod({
    name: 'feedback.submit',
    params: FeedbackSubmitArgs,
    // Why: crash submissions carry a diagnostic bundle and are sent main-only
    // (crash-reporting.ts calls submitFeedback in-process). A renderer-reachable
    // channel — ipc or RPC — must not be trusted to select that lane itself.
    handler: (params) => submitFeedback({ ...params, submissionType: 'feedback' })
  })
]
