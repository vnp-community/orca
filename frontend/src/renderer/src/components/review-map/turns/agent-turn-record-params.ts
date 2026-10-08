/**
 * agent-turn-record-params.ts — FE-CV-TASK-089-01
 *
 * Pure builder for `quality.turn.record` params (CONTRACT ui-api 3.2, camelCase).
 * Privacy: only digests and counts leave the renderer; the raw prompt,
 * toolInput and lastAssistantMessage are never copied into the output.
 *
 * @module components/review-map/turns/agent-turn-record-params
 */

import { buildFilesDigest, normalizePrompt, sha256Hex } from '../../../lib/agent-turn-digest'
import type { AgentTurnCommandsSummary } from './agent-tool-use-command-summarizer'

const PROMPT_EXCERPT_MAX = 160

export type AgentTurnRecordParams = {
  projectId: string
  worktreeId: string
  clientTurnId: string
  agentType: string
  endedAt: string
  startedAt?: string
  interrupted?: boolean
  endHeadCommit: string
  treeDirtyEnd: boolean
  filesChangedCount: number
  filesDigest: string
  promptDigest: string
  promptExcerpt?: string
  commandsSummary?: AgentTurnCommandsSummary
}

export type AgentTurnRecordInput = {
  projectId: string | null | undefined
  worktreeId: string | null | undefined
  entry: {
    paneKey: string
    agentType?: string
    prompt: string
    /** ms epoch of the `done` transition that ended the turn */
    doneAt: number
    stateHistory: readonly { state: string; startedAt: number }[]
    interrupted?: boolean
  }
  headOid: string | null | undefined
  treeDirty: boolean
  fileIdentities: readonly string[]
  commands: AgentTurnCommandsSummary | null
  /** Settings.tenant.agentTurnStorePromptExcerpt */
  storePromptExcerpt: boolean
  /** Client-side secret masker; absent today, so excerpts are never sent. */
  maskSensitiveText?: ((text: string) => string) | null
}

// Floating terminals and folder workspaces have no repo binding on the backend.
function isRecordableWorktree(worktreeId: string): boolean {
  return !worktreeId.startsWith('::workspace:') && !worktreeId.startsWith('folder:')
}

/** Returns null when the turn cannot be attributed (missing project/worktree/HEAD). */
export function buildAgentTurnRecordParams(input: AgentTurnRecordInput): AgentTurnRecordParams | null {
  const { projectId, worktreeId, headOid, entry } = input
  if (!projectId || !worktreeId || !headOid || !isRecordableWorktree(worktreeId)) {
    return null
  }

  const lastWorking = [...entry.stateHistory].toReversed().find((h) => h.state === 'working')
  const params: AgentTurnRecordParams = {
    projectId,
    worktreeId,
    clientTurnId: `${entry.paneKey}:${entry.doneAt}`,
    agentType: entry.agentType ?? 'unknown',
    endedAt: new Date(entry.doneAt).toISOString(),
    endHeadCommit: headOid,
    treeDirtyEnd: input.treeDirty,
    filesChangedCount: input.fileIdentities.length,
    filesDigest: buildFilesDigest(input.fileIdentities),
    promptDigest: sha256Hex(normalizePrompt(entry.prompt))
  }
  if (lastWorking) {
    params.startedAt = new Date(lastWorking.startedAt).toISOString()
  }
  if (entry.interrupted) {
    params.interrupted = true
  }
  if (input.commands) {
    params.commandsSummary = input.commands
  }
  // Why: the excerpt needs both the tenant flag and a masker; without a masker it is never sent.
  if (input.storePromptExcerpt && input.maskSensitiveText && entry.prompt) {
    params.promptExcerpt = input.maskSensitiveText(normalizePrompt(entry.prompt)).slice(
      0,
      PROMPT_EXCERPT_MAX
    )
  }
  return params
}
