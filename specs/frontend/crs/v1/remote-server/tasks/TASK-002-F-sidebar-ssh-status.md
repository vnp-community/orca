# TASK-002-F — Tạo SshStatusSection trong Sidebar

**Task ID:** TASK-002-F  
**CR:** CR-002 — Server Grouping by Project/Team  
**Solution Ref:** SOL-CR-002, Section 4  
**Dependencies:** TASK-002-A  
**Estimated:** 1 giờ  
**Status:** ✅ DONE

---

## Mục tiêu

Tạo `SshStatusSection` — compact summary view hiển thị per-project SSH status trong left sidebar của Orca.

---

## Files

| File | Action |
|------|--------|
| `src/renderer/src/components/sidebar/SshStatusSection.tsx` | CREATE |
| `src/renderer/src/components/sidebar/Sidebar.tsx` (hoặc tương đương) | MODIFY |

---

## Bước 1: Khám phá Sidebar cấu trúc

```bash
find src/renderer/src/components/sidebar -type f | head -20
grep -n "SshStatus\|ssh.*status\|SSH" src/renderer/src/components/sidebar/Sidebar.tsx 2>/dev/null | head -10
```

## Bước 2: Tạo SshStatusSection.tsx

```typescript
// src/renderer/src/components/sidebar/SshStatusSection.tsx
import { useMemo } from 'react'
import { cn } from '@/lib/utils'
import { useAppStore } from '@/store'
import { translate } from '@/i18n/i18n'

export function SshStatusSection() {
  const sshTargets = useAppStore((s) => s.sshTargets ?? [])
  const connectionStates = useAppStore((s) => s.sshConnectionStates)

  // Compute per-project summary
  const projectSummary = useMemo(() => {
    return Object.entries(
      sshTargets.reduce<Record<string, { total: number; connected: number }>>(
        (acc, t) => {
          const key = t.project ?? '__unassigned__'
          if (!acc[key]) acc[key] = { total: 0, connected: 0 }
          acc[key].total++
          if (connectionStates[t.id]?.status === 'connected') {
            acc[key].connected++
          }
          return acc
        },
        {}
      )
    ).sort(([a], [b]) => {
      if (a === '__unassigned__') return 1
      if (b === '__unassigned__') return -1
      return a.localeCompare(b)
    })
  }, [sshTargets, connectionStates])

  if (sshTargets.length === 0) return null

  return (
    <div className="px-2 py-1.5">
      <p className="mb-1 text-xs font-semibold uppercase tracking-wider text-muted-foreground/70">
        {translate('sidebar.sshServers', 'SSH Servers')}
      </p>
      <div className="space-y-0.5">
        {projectSummary.map(([project, counts]) => {
          const allConnected = counts.connected === counts.total
          const someConnected = counts.connected > 0 && !allConnected
          const noneConnected = counts.connected === 0

          return (
            <div
              key={project}
              className="flex items-center gap-1.5 text-xs"
            >
              {/* Status dot */}
              <span
                className={cn('h-1.5 w-1.5 flex-shrink-0 rounded-full', {
                  'bg-green-500': allConnected,
                  'bg-yellow-500': someConnected,
                  'bg-muted-foreground/40': noneConnected,
                })}
              />
              {/* Project label */}
              <span className="flex-1 truncate text-muted-foreground">
                {project === '__unassigned__'
                  ? translate('sidebar.sshUnassigned', 'Other')
                  : project}
              </span>
              {/* Count */}
              <span className="tabular-nums text-muted-foreground/60">
                {counts.connected}/{counts.total}
              </span>
            </div>
          )
        })}
      </div>
    </div>
  )
}
```

## Bước 3: Tích hợp vào Sidebar

Tìm vị trí SSH connection status hiện tại trong Sidebar (nếu có) và thay bằng `SshStatusSection`, hoặc thêm mới vào đúng vị trí:

```typescript
// Trong Sidebar.tsx, thêm import:
import { SshStatusSection } from './SshStatusSection'

// Trong JSX, ở khu vực SSH status (tìm bằng grep):
<SshStatusSection />
```

## Bước 4: Verify

```bash
npx tsc --noEmit 2>&1 | grep "SshStatus" | head -10
```

---

## Acceptance Criteria

- [x] `SshStatusSection` hiển thị trong left sidebar
- [x] Mỗi project: dot indicator + project name + connected/total count
- [x] Dot: xanh = all connected, vàng = partial, xám = none
- [x] `__unassigned__` projects hiển thị là "Other"
- [x] Component ẩn hoàn toàn khi không có SSH targets
- [x] Compact layout, không chiếm quá nhiều space

---

## Notes cho AI

- Nếu Sidebar đã có SSH status indicators, xem xét replace để tránh duplicate
- `tabular-nums` font feature giúp count align đẹp
- Component cần re-render khi `connectionStates` thay đổi (đã handle qua Zustand subscription)

---

## Implementation Notes

> **Completed:** 2026-07-23 | `sidebar/SshStatusSection.tsx`: per-project dot (green=all/yellow=partial/gray=none) + label + count, __unassigned__ shown as 'Other', returns null when no targets, compact text-xs layout. TypeScript: ✅ 0 errors.
