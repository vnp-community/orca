# TASK-002-D — Tạo SshTargetGroupedList Component

**Task ID:** TASK-002-D  
**CR:** CR-002 — Server Grouping by Project/Team  
**Solution Ref:** SOL-CR-002, Section 3.2  
**Dependencies:** TASK-002-A, TASK-002-B, TASK-002-C  
**Estimated:** 1–2 giờ  
**Status:** ✅ DONE

---

## Mục tiêu

Tạo `SshTargetGroupedList` — container component kết hợp FilterBar + grouped/collapsible SSH target list. Đây là component thay thế flat list hiện tại.

---

## File cần tạo

`src/renderer/src/components/settings/ssh/SshTargetGroupedList.tsx`

---

## Bước thực thi

### Bước 1: Kiểm tra list hiện tại

```bash
find src/renderer/src/components -name "*SshTarget*" -o -name "*ssh-target*" | head -10
```

### Bước 2: Tạo SshTargetGroupedList.tsx

```typescript
// src/renderer/src/components/settings/ssh/SshTargetGroupedList.tsx
import { useMemo, useState } from 'react'
import { useAppStore } from '@/store'
import { translate } from '@/i18n/i18n'
import {
  selectUniqueProjects,
  selectUniqueTeams,
  selectFilteredSshTargets,
  type SshTargetFilter,
} from '@/store/selectors'
import { FleetFilterBar } from './FleetFilterBar'
import { SshTargetGroup } from './SshTargetGroup'

export function SshTargetGroupedList() {
  const [filter, setFilter] = useState<SshTargetFilter>({})

  const sshTargets = useAppStore((s) => s.sshTargets ?? [])
  const connectionStates = useAppStore((s) => s.sshConnectionStates)
  const collapsedGroups = useAppStore((s) => s.collapsedSshGroups)
  const toggleGroup = useAppStore((s) => s.toggleSshGroupCollapsed)
  const projects = useAppStore(selectUniqueProjects)
  const teams = useAppStore(selectUniqueTeams)

  // Apply filter
  const filteredTargets = useMemo(
    () => selectFilteredSshTargets({ sshTargets }, filter),
    [sshTargets, filter]
  )

  // Group by project
  const groupedTargets = useMemo(() => {
    return filteredTargets.reduce<Record<string, typeof filteredTargets>>(
      (acc, t) => {
        const key = t.project ?? '__unassigned__'
        if (!acc[key]) acc[key] = []
        acc[key].push(t)
        return acc
      },
      {}
    )
  }, [filteredTargets])

  // Sort: named projects first, __unassigned__ last
  const groupKeys = Object.keys(groupedTargets).sort((a, b) => {
    if (a === '__unassigned__') return 1
    if (b === '__unassigned__') return -1
    return a.localeCompare(b)
  })

  return (
    <div className="flex flex-col gap-1">
      <FleetFilterBar
        filter={filter}
        onFilterChange={setFilter}
        projects={projects}
        teams={teams}
      />

      <div className="mt-3 space-y-2">
        {groupKeys.map((groupKey) => (
          <SshTargetGroup
            key={groupKey}
            label={
              groupKey === '__unassigned__'
                ? translate('fleet.group.unassigned', 'Unassigned')
                : groupKey
            }
            targets={groupedTargets[groupKey]}
            connectionStates={connectionStates}
            isCollapsed={collapsedGroups[groupKey] ?? false}
            onToggleCollapse={() => toggleGroup(groupKey)}
          />
        ))}

        {groupKeys.length === 0 && (
          <div className="py-8 text-center text-sm text-muted-foreground">
            {translate('fleet.empty', 'No SSH hosts match your filter.')}
          </div>
        )}
      </div>
    </div>
  )
}
```

### Bước 3: Verify

```bash
npx tsc --noEmit 2>&1 | grep "SshTargetGrouped" | head -10
```

---

## Acceptance Criteria

- [x] Component render đúng với filter + grouped logic
- [x] Empty state khi không có server nào match filter
- [x] `Unassigned` group xuất hiện cuối cùng
- [x] Collapsible state từ store
- [x] TypeScript compile clean

---

## Notes cho AI

- `SshTargetGroup` sẽ được tạo trong TASK-002-E — nếu chạy task này trước, có thể tạo placeholder
- `useAppStore(selectUniqueProjects)` — selector function truyền trực tiếp vào hook
- `collapsedSshGroups[groupKey] ?? false` — default là expanded (không collapsed)

---

## Implementation Notes

> **Completed:** 2026-07-23 | `SshTargetGroupedList.tsx`: FleetFilterBar + grouped collapsible list via SshTargetGroup, empty state when no matches, Unassigned rendered last, collapsed state from store. TypeScript: ✅ 0 errors.
