# TASK-006: OrcaProfile Types

**Phase:** 2 — Profile Hierarchy  
**Solution ref:** [SOL-V5-001](../solutions/SOL-V5-001-profile-hierarchy.md) §2.1  
**Prerequisite:** None (pure types)  
**Status:** ✅ DONE — 2026-07-28

---

## Mô tả

Tạo file types cho hệ thống Profile 3-layer. File này chứa pure interfaces, không logic.

---

## File cần tạo: `src/main/profile/OrcaProfile.ts`

```typescript
/**
 * OrcaProfile — 3-layer profile type definitions (TDD-14)
 *
 * Hierarchy: Company → Department → User
 * Merge strategy: user wins > dept wins > company (except locked sections)
 */

export interface McpServerConfig {
  name: string
  command: string
  args?: string[]
  env?: Record<string, string>
}

export interface AgentProfileSection {
  preferredModel?: string
  trustPreset?: 'minimal' | 'standard' | 'full'
  mcpServers?: McpServerConfig[]
  customInstructions?: string
  maxConcurrentAgents?: number
}

export interface EditorProfileSection {
  defaultEditor?: string
  tabSize?: number
  insertSpaces?: boolean
  theme?: string
}

export interface ShellProfileSection {
  defaultShell?: string
  /** Prepended to PATH on dev server — company + dept + user all concatenated */
  pathAdditions?: string[]
  /** Merged env vars — user overrides dept overrides company */
  envVars?: Record<string, string>
}

export interface SecurityProfileSection {
  /** Model allowlist — company admin only, locked from override */
  approvedModels?: string[]
  /** Command blocklist — applied on relay side */
  disallowedCommands?: string[]
  requireReviewBeforeCommit?: boolean
  /** Max agent session hours — company admin only */
  maxSessionHours?: number
}

export interface OrcaProfile {
  agent?: AgentProfileSection
  editor?: EditorProfileSection
  shell?: ShellProfileSection
  mcp?: { servers?: McpServerConfig[] }
  /** Locked: company-admin only, user cannot override */
  security?: SecurityProfileSection
  /** Legacy: top-level env vars */
  envVars?: Record<string, string>
}

/** Profile after 3-layer merge */
export interface ResolvedProfile extends OrcaProfile {
  /** Which layer provided each field */
  _sources: Record<string, 'company' | 'dept' | 'user'>
  /** Timestamp of resolution */
  _resolvedAt: number
}

export interface ProfileMergeOptions {
  /** Sections that company admin locks — user/dept cannot override */
  lockedSections: Array<keyof OrcaProfile>
}
```

---

## Verification

```bash
pnpm tsc --noEmit
```

## Acceptance Criteria

- [x] `src/main/profile/OrcaProfile.ts` tạo thành công
- [x] Đủ 8 exported interfaces/types
- [x] `ResolvedProfile extends OrcaProfile`
- [x] `security` section marked locked (comment)
- [x] Không TypeScript errors
