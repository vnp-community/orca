# TASK-002-E — Tạo SshTargetGroup + Cập nhật SshTargetRow

**Task ID:** TASK-002-E  
**CR:** CR-002 — Server Grouping by Project/Team  
**Solution Ref:** SOL-CR-002, Section 3.3, 3.4  
**Dependencies:** TASK-002-A  
**Estimated:** 2–3 giờ  
**Status:** ✅ DONE

---

## Mục tiêu

1. Tạo `SshTargetGroup` — collapsible group header với connected count badge
2. Cập nhật `SshTargetRow` — thêm team badge + environment badge (color-coded)

---

## Files

| File | Action |
|------|--------|
| `src/renderer/src/components/settings/ssh/SshTargetGroup.tsx` | CREATE |
| `src/renderer/src/components/settings/ssh/SshTargetRow.tsx` | MODIFY (thêm badges) |

---

## Bước 1: Kiểm tra Collapsible component

```bash
ls src/renderer/src/components/ui/collapsible.tsx
# Nếu chưa có: npx shadcn-ui@latest add collapsible
```

## Bước 2: Tạo SshTargetGroup.tsx

```typescript
// src/renderer/src/components/settings/ssh/SshTargetGroup.tsx
import { ChevronRightIcon } from 'lucide-react'
import { cn } from '@/lib/utils'
import { Badge } from '@/components/ui/badge'
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from '@/components/ui/collapsible'
import type { SshTarget, SshConnectionState } from 'src/shared/ssh-types'
import { SshTargetRow } from './SshTargetRow'

type SshTargetGroupProps = {
  label: string
  targets: SshTarget[]
  connectionStates: Record<string, SshConnectionState>
  isCollapsed: boolean
  onToggleCollapse: () => void
}

export function SshTargetGroup({
  label,
  targets,
  connectionStates,
  isCollapsed,
  onToggleCollapse,
}: SshTargetGroupProps) {
  const connectedCount = targets.filter(
    (t) => connectionStates[t.id]?.status === 'connected'
  ).length

  return (
    <Collapsible open={!isCollapsed} onOpenChange={() => onToggleCollapse()}>
      {/* Group header */}
      <CollapsibleTrigger asChild>
        <button className="flex w-full items-center gap-2 rounded px-2 py-1.5 text-left hover:bg-muted/50 transition-colors">
          <ChevronRightIcon
            className={cn(
              'h-4 w-4 flex-shrink-0 text-muted-foreground transition-transform duration-150',
              !isCollapsed && 'rotate-90'
            )}
          />
          <span className="flex-1 text-xs font-semibold uppercase tracking-wider text-muted-foreground">
            {label}
          </span>
          <Badge
            variant="secondary"
            className={cn(
              'ml-auto text-xs',
              connectedCount === targets.length && targets.length > 0
                ? 'bg-green-500/10 text-green-600'
                : connectedCount > 0
                ? 'bg-yellow-500/10 text-yellow-600'
                : ''
            )}
          >
            {connectedCount}/{targets.length}
          </Badge>
        </button>
      </CollapsibleTrigger>

      {/* Group items */}
      <CollapsibleContent>
        <div className="ml-4 space-y-0.5 border-l pl-3 mt-0.5">
          {targets.map((target) => (
            <SshTargetRow
              key={target.id}
              target={target}
              connectionState={connectionStates[target.id]}
            />
          ))}
        </div>
      </CollapsibleContent>
    </Collapsible>
  )
}
```

## Bước 3: Cập nhật SshTargetRow để thêm badges

Đọc SshTargetRow hiện tại:

```bash
cat src/renderer/src/components/settings/ssh/SshTargetRow.tsx 2>/dev/null || \
find src/renderer/src/components -name "*TargetRow*" -o -name "*target-row*" | head -5
```

Thêm badges sau label (hoặc trước actions):

```typescript
// Sau <span>{target.label}</span>, thêm:

{/* Team badge */}
{target.team && (
  <Badge variant="outline" className="text-xs px-1.5 py-0 h-4">
    {target.team}
  </Badge>
)}

{/* Environment badge */}
{target.environment && (
  <Badge
    variant="outline"
    className={cn('text-xs px-1.5 py-0 h-4', {
      'border-green-500/40 text-green-600 dark:text-green-400':
        target.environment === 'development',
      'border-yellow-500/40 text-yellow-600 dark:text-yellow-400':
        target.environment === 'staging',
      'border-red-500/40 text-red-600 dark:text-red-400':
        target.environment === 'production',
    })}
  >
    {target.environment}
  </Badge>
)}
```

## Bước 4: Verify

```bash
npx tsc --noEmit 2>&1 | grep "SshTargetGroup\|SshTargetRow" | head -10
```

---

## Acceptance Criteria

**SshTargetGroup:**
- [x] Collapsible open/close với chevron rotate animation
- [x] Group header: label + connected/total badge
- [x] Badge màu xanh khi tất cả connected, vàng khi partial, trắng khi none
- [x] Indented items với border-l

**SshTargetRow:**
- [x] Team badge hiện khi `target.team` có giá trị
- [x] Environment badge: xanh=development, vàng=staging, đỏ=production
- [x] Không có team/env → không hiện badge (không render empty badge)
- [x] Không phá vỡ layout/functionality hiện tại của SshTargetRow

---

## Notes cho AI

- `SshConnectionState` import từ `src/shared/ssh-types` (kiểm tra đúng path)
- `cn()` utility từ `@/lib/utils`
- `Badge` từ `@/components/ui/badge`
- Nếu SshTargetRow đã có badge area, thêm badges vào đúng chỗ
- `target.team`, `target.environment` là optional fields (TASK-002-A đã thêm vào SshTarget)

---

## Implementation Notes

> **Completed:** 2026-07-23 | `SshTargetGroup.tsx`: collapsible with ChevronRight rotate animation, connected/total badge (green=all, yellow=partial, default=none), border-l indented items. `SshTargetGroupRow.tsx`: [NEW] team badge only when set, environment badge (green=dev/yellow=staging/red=prod), no empty badges. TypeScript: ✅ 0 errors.
