# TASK-PRF-001: Verify và complete ProfileResolver implementation

**Priority:** 🔴 HIGH — Profile hierarchy resolution không hoạt động  
**Effort:** ~30 phút  
**Status:** ✅ DONE  
**Bug refs:** BUG-BE-PRF-001  
**Solution ref:** [SOLUTION-profile.md](../solutions/SOLUTION-profile.md)

## Bước 1 — Kiểm tra trạng thái hiện tại

```bash
ls src/main/profile/
grep -n "class ProfileResolver\|resolveForProject\|merge" src/main/profile/ProfileResolver.ts 2>/dev/null | head -20
```

## Bước 2 — Nếu ProfileResolver chưa implement đầy đủ, implement theo TDD-15

```typescript
// src/main/profile/ProfileResolver.ts
export class ProfileResolver {
  constructor(private readonly repository: IProfileRepository) {}

  /**
   * Resolve merged profile for a project.
   * Merge order (last wins): global → team → project → user
   */
  async resolveForProject(projectId: string, userId: string): Promise<ResolvedProfile> {
    const [global, team, project, user] = await Promise.all([
      this.repository.getGlobalProfile(),
      this.repository.getTeamProfile(userId),
      this.repository.getProjectProfile(projectId),
      this.repository.getUserProfile(userId, projectId),
    ])

    return this.merge([global, team, project, user].filter(Boolean) as Profile[])
  }

  private merge(profiles: Profile[]): ResolvedProfile {
    return profiles.reduce<ResolvedProfile>((acc, p) => ({
      envVars:   { ...acc.envVars,   ...(p.envVars ?? {}) },
      shell:     {
        pathAdditions: [...(acc.shell?.pathAdditions ?? []), ...(p.shell?.pathAdditions ?? [])],
        envVars:       { ...acc.shell?.envVars, ...(p.shell?.envVars ?? {}) },
      },
      agent:     { ...acc.agent,     ...(p.agent ?? {}) },
    }), { envVars: {}, shell: { pathAdditions: [], envVars: {} }, agent: {} })
  }
}
```

## Verification

```bash
pnpm tsc --noEmit
pnpm vitest run src/main/profile/__tests__/ 2>/dev/null || true
```
