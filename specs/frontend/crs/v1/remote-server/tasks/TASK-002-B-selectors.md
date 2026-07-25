# TASK-002-B — Tạo Selectors cho SSH Target Grouping/Filtering

**Task ID:** TASK-002-B  
**CR:** CR-002 — Server Grouping by Project/Team  
**Solution Ref:** SOL-CR-002, Section 2.1  
**Dependencies:** TASK-002-A  
**Estimated:** 1 giờ  
**Status:** ✅ DONE

---

## Mục tiêu

Thêm 4 selector functions vào `src/renderer/src/store/selectors.ts` để grouping, filtering, và lấy danh sách unique projects/teams.

---

## Bước thực thi

### Bước 1: Tìm selectors file

```bash
ls src/renderer/src/store/selectors.ts
# Nếu không có, tạo mới
```

### Bước 2: Thêm SshTargetFilter type

```typescript
// Trong selectors.ts hoặc store/types.ts:
export type SshTargetFilter = {
  project?: string
  team?: string
  environment?: 'development' | 'staging' | 'production'
  search?: string
}
```

### Bước 3: Thêm 4 selectors

```typescript
// src/renderer/src/store/selectors.ts

import type { AppState } from '@/store/types'
import type { SshTarget } from 'src/shared/ssh-types'

// Selector 1: Group SSH targets by project
export function selectSshTargetsByProject(
  state: AppState
): Record<string, SshTarget[]> {
  const allTargets = state.sshTargets ?? []
  return allTargets.reduce<Record<string, SshTarget[]>>((acc, target) => {
    const group = target.project ?? '__unassigned__'
    if (!acc[group]) acc[group] = []
    acc[group].push(target)
    return acc
  }, {})
}

// Selector 2: Get unique projects
export function selectUniqueProjects(state: AppState): string[] {
  const targets = state.sshTargets ?? []
  const projects = new Set(
    targets.map((t) => t.project).filter(Boolean) as string[]
  )
  return Array.from(projects).sort()
}

// Selector 3: Get unique teams
export function selectUniqueTeams(state: AppState): string[] {
  const targets = state.sshTargets ?? []
  const teams = new Set(
    targets.map((t) => t.team).filter(Boolean) as string[]
  )
  return Array.from(teams).sort()
}

// Selector 4: Filter targets
export function selectFilteredSshTargets(
  state: Pick<AppState, 'sshTargets'>,
  filter: SshTargetFilter
): SshTarget[] {
  const targets = state.sshTargets ?? []
  return targets.filter((t) => {
    if (filter.project && t.project !== filter.project) return false
    if (filter.team && t.team !== filter.team) return false
    if (filter.environment && t.environment !== filter.environment) return false
    if (filter.search) {
      const q = filter.search.toLowerCase()
      return (
        t.label.toLowerCase().includes(q) ||
        t.host.toLowerCase().includes(q)
      )
    }
    return true
  })
}
```

### Bước 4: Export check

Đảm bảo tất cả 4 functions được export và có thể import:

```bash
npx tsc --noEmit 2>&1 | grep "selector\|selectSsh\|selectUnique\|selectFiltered" | head -10
```

---

## Acceptance Criteria

- [x] `selectSshTargetsByProject` trả về `Record<string, SshTarget[]>` đúng
- [x] Servers không có `project` → key `'__unassigned__'`
- [x] `selectUniqueProjects` trả về sorted string array
- [x] `selectUniqueTeams` trả về sorted string array
- [x] `selectFilteredSshTargets` filter đúng theo project, team, environment, search
- [x] `SshTargetFilter` type được export
- [x] TypeScript compile clean

---

## Notes cho AI

- Nếu `selectors.ts` đã có các selectors khác, chỉ THÊM vào cuối file
- Selectors là pure functions, không có side effects
- `SshTarget` import từ shared types — tìm đúng path
- `selectFilteredSshTargets` nhận `Pick<AppState, 'sshTargets'>` để dùng được cả trong test

---

## Implementation Notes

> **Completed:** 2026-07-23 | `store/selectors.ts`: selectSshTargetsByProject (key='__unassigned__' for no project), selectUniqueProjects (sorted), selectUniqueTeams (sorted), selectFilteredSshTargets (project/team/env/search filter), SshTargetFilter type exported. TypeScript: ✅ 0 errors.
