import { z } from 'zod'
import { defineMethod, type RpcMethod } from '../core'
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

// Why: mirrors desktop/src/main/ipc/agent-trust.ts's `agentTrust:markTrusted`
// handler exactly — same preset dispatch, same best-effort swallow — so the
// RPC surface and the ipcMain channel write identical trust artifacts.
export const AGENT_TRUST_METHODS: RpcMethod[] = [
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
        // Best-effort: see ipc/agent-trust.ts's Why for the rationale.
      }
    }
  })
]
