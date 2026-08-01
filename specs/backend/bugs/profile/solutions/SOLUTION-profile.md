# SOLUTION: Profile Domain — Fix tất cả Bugs

**Domain:** profile  
**TDD Reference:** TDD-14 (Profile Hierarchy), TDD-08 (Agent Orchestration)  
**Files cần thay đổi:** `src/main/profile/ProfileResolver.ts`  
**Tổng số bugs:** 1 (BE-PRF-001)

---

## BUG-BE-PRF-001 — Fix ProfileResolver not implemented

**Mức độ:** 🔴 HIGH  
**Root cause:** `ProfileResolver` class chỉ là stub — logic resolve profile hierarchy chưa implement.

### Fix — Implement ProfileResolver theo TDD-14

```typescript
// src/main/profile/ProfileResolver.ts

/**
 * Profile hierarchy (TDD-14):
 *   Global Profile (system defaults)
 *     └── Org Profile (org-level overrides)
 *           └── User Profile (user-level overrides)
 *                 └── Project Profile (project-level overrides)
 *
 * Rule: Cấp thấp hơn (project) ghi đè cấp cao hơn (global).
 * Shallow merge for most fields; deep merge for allowedModels, pathAdditions.
 */

export interface OrcaProfile {
  shell: {
    defaultShell?:   string
    pathAdditions?:  string[]
    envOverrides?:   Record<string, string>
  }
  agent: {
    approvedModels?: string[]    // whitelist models user có thể dùng
    maxParallelAgents?: number
    trustPreset?:    'standard' | 'full' | 'none'
  }
  worktrees: {
    maxPerProject?:  number
    defaultBaseRef?: string      // e.g. 'main'
    autoCleanupDays?: number
  }
  automation: {
    enabled?:        boolean
    maxConcurrent?:  number
  }
}

export class ProfileResolver {
  constructor(
    private readonly repository: IProfileRepository,
    private readonly log: Logger,
  ) {}

  /**
   * Resolve merged profile cho request context.
   * Merge order: global → org → user → project
   */
  async resolve(context: {
    userId:    string
    projectId: string
    orgId?:    string
  }): Promise<OrcaProfile> {
    // Load all levels in parallel
    const [globalProfile, orgProfile, userProfile, projectProfile] = await Promise.all([
      this.repository.getGlobalProfile(),
      context.orgId ? this.repository.getOrgProfile(context.orgId) : null,
      this.repository.getUserProfile(context.userId),
      this.repository.getProjectProfile(context.projectId),
    ])

    // Build defaults
    const resolved: OrcaProfile = {
      shell: {
        defaultShell:   '/bin/bash',
        pathAdditions:  [],
        envOverrides:   {},
      },
      agent: {
        approvedModels:    ['claude-opus-4-5', 'claude-sonnet-4-5', 'gpt-4o', 'gemini-2.0-flash'],
        maxParallelAgents: 2,
        trustPreset:       'standard',
      },
      worktrees: {
        maxPerProject:   5,
        defaultBaseRef:  'main',
        autoCleanupDays: 7,
      },
      automation: {
        enabled:         true,
        maxConcurrent:   3,
      },
    }

    // Apply levels in order (each overwrites previous)
    this.applyProfile(resolved, globalProfile)
    this.applyProfile(resolved, orgProfile)
    this.applyProfile(resolved, userProfile)
    this.applyProfile(resolved, projectProfile)

    return resolved
  }

  /**
   * Deep-ish merge: arrays are concatenated (unique), objects shallow-merged.
   */
  private applyProfile(base: OrcaProfile, override: Partial<OrcaProfile> | null): void {
    if (!override) return

    // Shell
    if (override.shell) {
      if (override.shell.defaultShell !== undefined)
        base.shell.defaultShell = override.shell.defaultShell
      if (override.shell.pathAdditions?.length)
        base.shell.pathAdditions = [
          ...new Set([...(base.shell.pathAdditions ?? []), ...override.shell.pathAdditions])
        ]
      if (override.shell.envOverrides)
        base.shell.envOverrides = { ...base.shell.envOverrides, ...override.shell.envOverrides }
    }

    // Agent
    if (override.agent) {
      if (override.agent.approvedModels?.length)
        base.agent.approvedModels = override.agent.approvedModels  // replace (not merge)
      if (override.agent.maxParallelAgents !== undefined)
        base.agent.maxParallelAgents = override.agent.maxParallelAgents
      if (override.agent.trustPreset !== undefined)
        base.agent.trustPreset = override.agent.trustPreset
    }

    // Worktrees
    if (override.worktrees) Object.assign(base.worktrees, override.worktrees)

    // Automation
    if (override.automation) Object.assign(base.automation, override.automation)
  }

  /**
   * Validate rằng model được request nằm trong approvedModels của profile.
   */
  async validateModelAccess(
    modelId: string,
    context: { userId: string; projectId: string; orgId?: string }
  ): Promise<boolean> {
    const profile = await this.resolve(context)
    return profile.agent.approvedModels?.includes(modelId) ?? true
  }
}
```

---

## Tóm tắt file changes

| File | Action | Bug |
|------|--------|-----|
| `src/main/profile/ProfileResolver.ts` | Implement full profile hierarchy merge | BE-PRF-001 |
| `src/main/repositories/profile-repository.ts` | NEW — IProfileRepository interface + SQL impl | BE-PRF-001 |
| `src/main/db/migrations/0012_profiles.ts` | NEW migration (global/org/user/project profile tables) | BE-PRF-001 |
| `src/main/ipc/profile-ipc.ts` | Wire ProfileResolver to IPC handlers | BE-PRF-001 |

---

## Verification Plan

```bash
pnpm vitest run src/main/profile/__tests__/profile-resolver.test.ts

# Test cases:
# 1. Global profile only → verify defaults returned
# 2. User profile overrides global → verify merge correct
# 3. Project profile overrides user → verify project wins
# 4. approvedModels at user level → verify model whitelist enforced
# 5. pathAdditions accumulate across levels → verify union
```
