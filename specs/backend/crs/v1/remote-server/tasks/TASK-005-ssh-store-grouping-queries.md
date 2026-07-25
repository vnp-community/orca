# TASK-005: Thêm Grouping Query Methods vào `SshConnectionStore`

**Source:** SOL-002  
**Phase:** 1 | **Effort:** S (30–90 min)  
**Depends on:** TASK-001

---

## Objective

Thêm 5 query methods vào `SshConnectionStore` để support grouping, filtering, và listing:
1. `listTargetsByProject()` → `Map<string, SshTarget[]>`
2. `listTargetsByProjectFilter(project)` → `SshTarget[]`
3. `listTargetsByTeam(team)` → `SshTarget[]`
4. `listProjects()` → `string[]`
5. `listTeams()` → `string[]`
6. `filterTargets(criteria)` → `SshTarget[]`

---

## File to modify

**`src/main/ssh/ssh-connection-store.ts`**

---

## Implementation

Thêm các methods sau vào class `SshConnectionStore`:

```typescript
  // ── Fleet Grouping & Filtering (NEW) ──────────────────────

  /**
   * Group all targets by project.
   * Targets without a project are grouped under 'unassigned'.
   */
  listTargetsByProject(): Map<string, SshTarget[]> {
    const groups = new Map<string, SshTarget[]>()
    for (const target of this.listTargets()) {
      const key = target.project ?? 'unassigned'
      const group = groups.get(key) ?? []
      group.push(target)
      groups.set(key, group)
    }
    return groups
  }

  /**
   * Filter targets by exact project name.
   */
  listTargetsByProjectFilter(project: string): SshTarget[] {
    return this.listTargets().filter(t => t.project === project)
  }

  /**
   * Filter targets by team name.
   */
  listTargetsByTeam(team: string): SshTarget[] {
    return this.listTargets().filter(t => t.team === team)
  }

  /**
   * Get distinct sorted list of project names across all targets.
   */
  listProjects(): string[] {
    const projects = this.listTargets()
      .map(t => t.project)
      .filter((p): p is string => typeof p === 'string' && p.length > 0)
    return [...new Set(projects)].sort()
  }

  /**
   * Get distinct sorted list of team names across all targets.
   */
  listTeams(): string[] {
    const teams = this.listTargets()
      .map(t => t.team)
      .filter((t): t is string => typeof t === 'string' && t.length > 0)
    return [...new Set(teams)].sort()
  }

  /**
   * Multi-criteria filter. All provided criteria must match (AND logic).
   */
  filterTargets(criteria: SshTargetFilterCriteria): SshTarget[] {
    return this.listTargets().filter(target => {
      if (criteria.project !== undefined && target.project !== criteria.project) return false
      if (criteria.team !== undefined && target.team !== criteria.team) return false
      if (criteria.environment !== undefined && target.environment !== criteria.environment) return false
      if (criteria.tags?.length) {
        const targetTags = target.tags ?? []
        if (!criteria.tags.some(tag => targetTags.includes(tag))) return false
      }
      if (criteria.search) {
        const q = criteria.search.toLowerCase()
        const haystack = `${target.label} ${target.host}`.toLowerCase()
        if (!haystack.includes(q)) return false
      }
      return true
    })
  }
```

### Add type (top-level, before class or after imports)

```typescript
export type SshTargetFilterCriteria = {
  project?: string
  team?: string
  environment?: 'development' | 'staging' | 'production'
  tags?: string[]           // match if target has ANY of these tags
  search?: string           // substring match on label + host
}
```

---

## Verification

```bash
npx tsc --noEmit 2>&1 | grep ssh-connection-store | head -20
```

---

## Done criteria

- [x] `listTargetsByProject()` returns `Map<string, SshTarget[]>`
- [x] `filterTargets(criteria)` supports project + team + environment + tags + search
- [x] `listProjects()` returns sorted unique list
- [x] `SshTargetFilterCriteria` type exported
- [x] TypeScript compile: no errors

**Status: ✅ DONE** — `ssh-connection-store.ts` updated. 5 methods added: `listTargetsByProject()`, `listTargetsByProjectFilter()`, `listProjects()`, `listTeams()`, `filterTargets()`. `SshTargetFilterCriteria` type exported. IPC handlers for all 5 also added (covers TASK-006).
