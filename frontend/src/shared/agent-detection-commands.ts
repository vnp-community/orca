// ─── Agent Detection Commands (web/dev-server relay) ──────────────────────
// Mirrors desktop/src/shared/agent-detection-commands.ts exactly — same
// pure shared shape, same "minimal fields the relay needs" rationale — but
// lives in frontend/ since the web build (no Electron desktop process) is
// what actually calls onboarding.detectAgents with this catalog attached
// (see web-preload-api.ts's createOnboardingApi). Kept as its own file
// rather than folded into tui-agent-config.ts so that file stays UI-config-
// agnostic of the relay wire shape.

import { TUI_AGENT_CONFIG } from './tui-agent-config'

export type AgentDetectionCommand = {
  id: string
  /** Primary executable name to probe on PATH */
  cmd: string
  /** Additional commands that must also be present on PATH for the agent to count */
  requiredCommands?: readonly string[]
  /** Runtimes on which this agent is not detectable. */
  unsupportedRuntimes?: readonly (NodeJS.Platform | 'wsl')[]
}

/**
 * Builds the minimal agent detection command list from the TUI agent
 * catalog — the same {commands} shape the real agent-side
 * preflight.detectAgents RPC expects (specs/agent/api/agent-rpc-catalog-
 * runtime.md), passed through verbatim by the backend's onboarding.detectAgents
 * wscompat channel (no catalog duplicated server-side).
 */
export function buildAgentDetectionCommands(): AgentDetectionCommand[] {
  return Object.entries(TUI_AGENT_CONFIG).map(([id, config]) => ({
    id,
    cmd: config.detectCmd,
    ...(config.detectRequiredCommands ? { requiredCommands: config.detectRequiredCommands } : {}),
    ...(config.detectUnsupportedRuntimes
      ? { unsupportedRuntimes: config.detectUnsupportedRuntimes }
      : {})
  }))
}
