// Why: feedback submission is proxied through the desktop main process (see
// SidebarFeedbackDialog's CORS note) — always local, no remote-environment
// routing.
import { callRuntimeRpc } from './runtime-rpc-client'

const LOCAL_TARGET = { kind: 'local' } as const

export type RuntimeFeedbackSubmitResult = { ok: true } | { ok: false; status: number | null; error: string }

export function submitRuntimeFeedback(args: {
  feedback: string
  submitAnonymously?: boolean
  githubLogin: string | null
  githubEmail: string | null
}): Promise<RuntimeFeedbackSubmitResult> {
  return callRuntimeRpc<RuntimeFeedbackSubmitResult>(LOCAL_TARGET, 'feedback.submit', args)
}
