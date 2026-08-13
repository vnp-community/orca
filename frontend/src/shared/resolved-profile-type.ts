/**
 * ResolvedProfile — type-only mirror of `backend/src/main/profile/OrcaProfile.ts`
 *
 * `backend/src/main/profile/OrcaProfile.ts` is the canonical, live implementation
 * (`ProfileResolver.merge()` produces this shape for real). This file exists only
 * because `frontend`'s TS project boundaries don't allow `src/shared/` to reach
 * into `src/main/` (or into another package's `src/` at all), so the type can't
 * be imported directly — keep this in sync by hand whenever the backend shape
 * changes (docs/guides/user-profile-team-department-rbac.md §4).
 *
 * @module shared/resolved-profile-type
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
  pathAdditions?: string[]
  envVars?: Record<string, string>
}

export type SecurityProfileSection = {
  approvedModels?: string[]
  disallowedCommands?: string[]
  requireReviewBeforeCommit?: boolean
  maxSessionHours?: number
  require2FA?: boolean
}

export type OrcaProfile = {
  agent?: AgentProfileSection
  editor?: EditorProfileSection
  shell?: ShellProfileSection
  mcp?: { servers?: McpServerConfig[] }
  security?: SecurityProfileSection
  envVars?: Record<string, string>
}

/** Which layer provided a merged field — Team carries the winning team's id. */
export type ProfileSourceLayer = 'company' | 'dept' | 'user' | `team:${string}`

/** Profile after cascade merge (Company → Department → Team → User) */
export type ResolvedProfile = {
  _sources: Record<string, ProfileSourceLayer>
  _resolvedAt: number
} & OrcaProfile
