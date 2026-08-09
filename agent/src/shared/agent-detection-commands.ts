// ─── Agent Detection Commands ─────────────────────────────────────────────────
// Pure shared module — no UI-only fields (label, description, icon, installUrl).
// Used by the relay side to identify installed agents on a remote dev server.

import { TUI_AGENT_CONFIG } from './tui-agent-config'

export type AgentDetectionCommand = {
  id: string
  /** Primary executable name to probe on PATH */
  cmd: string
  /** Additional commands that must also be present on PATH for the agent to count */
  requiredCommands?: readonly string[]
  /**
   * Runtimes on which this agent is not detectable.
   * Mirrors `TuiAgentConfig.detectUnsupportedRuntimes`.
   */
  unsupportedRuntimes?: readonly (NodeJS.Platform | 'wsl')[]
}

/**
 * Builds the minimal agent detection command list from the TUI agent catalog.
 *
 * Only maps fields needed by the relay — intentionally omits UI-only fields
 * (label, description, icon, installUrl, launchCmd, promptInjectionMode, etc.)
 * so the relay bundle stays lean and self-describing.
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
