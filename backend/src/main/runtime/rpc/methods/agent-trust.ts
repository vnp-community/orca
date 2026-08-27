import { z } from 'zod'
import { defineMethod, type RpcAnyMethod } from '../core'
import {
  type AgentTrustPreset,
  markCodexProjectTrusted,
  markCopilotFolderTrusted,
  markCursorWorkspaceTrusted
} from '../../../agent-trust-presets'
import { markRemoteAgentWorkspaceTrusted } from '../../../remote-agent-trust-presets'

const MarkTrusted = z.object({
  preset: z.enum(['cursor', 'copilot', 'codex']),
  workspacePath: z.string().min(1),
  connectionId: z.string().min(1).optional()
})

// Why: mirrors desktop/src/main/runtime/rpc/methods/agent-trust.ts's
// `agentTrust.markTrusted` handler exactly — same preset dispatch, same
// best-effort swallow, same optional `connectionId` (present for repos/folder
// workspaces backed by an SSH remote, absent for repos hosted directly on
// this backend host — see repo.ts/folder-workspace.ts's own optional
// `connectionId` params) — so the two runtimes write identical trust
// artifacts regardless of which one handles the request.
export const AGENT_TRUST_METHODS: readonly RpcAnyMethod[] = [
  defineMethod({
    name: 'agentTrust.markTrusted',
    params: MarkTrusted,
    handler: async (params) => {
      try {
        const preset: AgentTrustPreset = params.preset
        if (params.connectionId) {
          await markRemoteAgentWorkspaceTrusted({
            preset,
            connectionId: params.connectionId,
            workspacePath: params.workspacePath
          })
        } else if (preset === 'cursor') {
          markCursorWorkspaceTrusted(params.workspacePath)
        } else if (preset === 'copilot') {
          markCopilotFolderTrusted(params.workspacePath)
        } else if (preset === 'codex') {
          markCodexProjectTrusted(params.workspacePath)
        }
      } catch {
        // Best-effort: see desktop's agent-trust.ts's Why for the rationale —
        // trust pre-marking is a UX nicety, not correctness-critical.
      }
    }
  })
]
