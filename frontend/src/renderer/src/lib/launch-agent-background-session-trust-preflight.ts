import { TUI_AGENT_CONFIG } from '../../../shared/tui-agent-config'
import type { TuiAgent } from '../../../shared/types'
import { markRuntimeAgentTrusted } from '@/runtime/runtime-agent-trust-client'

// Why: trust-gated agents (cursor-agent, copilot) consume the bracketed paste
// as menu input on first launch. Pre-write the trust artifact before any
// terminal spawns. Best-effort — the worktree already exists, so a failure
// here must not strand the launch.
export async function markAgentBackgroundSessionTrusted(
  agent: TuiAgent,
  workspacePath: string | undefined
): Promise<void> {
  const preflight = TUI_AGENT_CONFIG[agent].preflightTrust
  if (!preflight || !workspacePath) {
    return
  }
  try {
    await markRuntimeAgentTrusted({ preset: preflight, workspacePath })
  } catch {
    // Best-effort: continue with launch. The user can still accept the trust menu.
  }
}
