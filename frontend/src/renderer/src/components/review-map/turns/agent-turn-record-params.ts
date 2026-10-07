/**
 * agent-turn-record-params.ts — FE-CV-TASK-089-01
 *
 * Builds the parameters for `quality.turn.record` RPC calls.
 * Rules:
 * - Returns null when headOid/projectId/worktreeId is missing or floating
 * - No raw prompt/toolInput/lastAssistantMessage in output
 * - promptExcerpt omitted by default (requires tenant flag + maskSensitiveText)
 * - filesDigest is sha256 of sorted file identifiers
 *
 * @module components/review-map/turns/agent-turn-record-params
 */

import { buildFilesDigest } from '../../../lib/agent-turn-digest'

// ---------------------------------------------------------------------------
// Input types (renderer-available subset of AgentTurn)
// ---------------------------------------------------------------------------

export type AgentTurnStateEntry = {
  state: 'working' | 'idle' | 'complete' | 'error'
  startedAt: string
}

export type AgentTurnInput = {
  paneKey: string
  projectId: string | null | undefined
  worktreeId: string | null | undefined
  headOid: string | null | undefined
  stateStartedAt: string | null
  endedAt: string
  stateHistory: AgentTurnStateEntry[]
  /** File identifiers touched in this turn */
  fileIdentifiers: string[]
  /** Tenant flag: only set promptExcerpt when true AND maskFn is provided */
  promptExcerptEnabled?: boolean
  /**
   * A masking function — if not provided (e.g., maskSensitiveText not implemented),
   * promptExcerpt is not included in output.
   */
  maskSensitiveText?: ((text: string) => string) | null
  /** Raw prompt text — only used to produce excerpt if flag+mask are present */
  rawPrompt?: string
}

// ---------------------------------------------------------------------------
// Output type
// ---------------------------------------------------------------------------

export type AgentTurnRecordParams = {
  clientTurnId: string
  projectId: string
  worktreeId: string
  headOid: string
  startedAt: string
  endedAt: string
  filesDigest: string
  fileCount: number
  promptExcerpt?: string
}

// ---------------------------------------------------------------------------
// Builder
// ---------------------------------------------------------------------------

/**
 * Build quality.turn.record params.
 * Returns null when required fields are missing.
 */
export async function buildAgentTurnRecordParams(
  input: AgentTurnInput
): Promise<AgentTurnRecordParams | null> {
  const { paneKey, projectId, worktreeId, headOid, stateStartedAt, endedAt, stateHistory, fileIdentifiers } = input

  // Required fields check
  if (!projectId || !worktreeId || !headOid) return null

  // Reject floating worktrees (::workspace: or folder: prefixes)
  if (worktreeId.startsWith('::workspace:') || worktreeId.startsWith('folder:')) return null

  // clientTurnId: paneKey + stateStartedAt
  const clientTurnId = stateStartedAt
    ? `${paneKey}:${stateStartedAt}`
    : `${paneKey}:${endedAt}`

  // startedAt: most recent 'working' entry in stateHistory
  const workingEntry = [...stateHistory].reverse().find((e) => e.state === 'working')
  const startedAt = workingEntry?.startedAt ?? endedAt

  // filesDigest: sha256 of sorted file identifiers
  const filesDigest = await buildFilesDigest(fileIdentifiers)

  const params: AgentTurnRecordParams = {
    clientTurnId,
    projectId,
    worktreeId,
    headOid,
    startedAt,
    endedAt,
    filesDigest,
    fileCount: fileIdentifiers.length
  }

  // promptExcerpt: only if tenant flag enabled AND masking function exists
  if (input.promptExcerptEnabled && input.maskSensitiveText && input.rawPrompt) {
    const masked = input.maskSensitiveText(input.rawPrompt)
    params.promptExcerpt = masked.slice(0, 200)
  }
  // else: omit — no raw prompt in output

  return params
}
