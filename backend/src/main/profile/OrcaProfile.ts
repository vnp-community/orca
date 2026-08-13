/**
 * OrcaProfile — 3-layer profile type definitions (TDD-14)
 *
 * Hierarchy: Company → Department → User
 * Merge strategy: user wins > dept wins > company (except locked sections)
 *
 * @module main/profile/OrcaProfile
 */

export type McpServerConfig = {
  name: string
  command: string
  args?: string[]
  env?: Record<string, string>
}

export type AgentProfileSection = {
  preferredModel?: string
  trustPreset?: 'minimal' | 'standard' | 'full'
  mcpServers?: McpServerConfig[]
  customInstructions?: string
  maxConcurrentAgents?: number
}

export type EditorProfileSection = {
  defaultEditor?: string
  tabSize?: number
  insertSpaces?: boolean
  theme?: string
}

export type ShellProfileSection = {
  defaultShell?: string
  /** Prepended to PATH on dev server — company + dept + user all concatenated */
  pathAdditions?: string[]
  /** Merged env vars — user overrides dept overrides company */
  envVars?: Record<string, string>
}

export type SecurityProfileSection = {
  /** Model allowlist — company admin only, locked from override */
  approvedModels?: string[]
  /** Command blocklist — applied on relay side */
  disallowedCommands?: string[]
  requireReviewBeforeCommit?: boolean
  /** Max agent session hours — company admin only */
  maxSessionHours?: number
  /** Require 2FA for agent sessions — company admin only */
  require2FA?: boolean
}

export type OrcaProfile = {
  agent?: AgentProfileSection
  editor?: EditorProfileSection
  shell?: ShellProfileSection
  mcp?: { servers?: McpServerConfig[] }
  /** Locked: company-admin only, user cannot override */
  security?: SecurityProfileSection
  /** Legacy: top-level env vars (merged into shell.envVars on resolve) */
  envVars?: Record<string, string>
}

/** Profile after 3-layer merge */
export type ResolvedProfile = {
  /** Which layer provided each field */
  _sources: Record<string, 'company' | 'dept' | 'user'>
  /** Unix timestamp of resolution */
  _resolvedAt: number
} & OrcaProfile

export type ProfileMergeOptions = {
  /** Sections that company admin locks — user/dept cannot override */
  lockedSections: (keyof OrcaProfile)[]
}
