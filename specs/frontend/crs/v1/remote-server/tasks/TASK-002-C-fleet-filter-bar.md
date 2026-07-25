# TASK-002-C — Tạo FleetFilterBar Component

**Task ID:** TASK-002-C  
**CR:** CR-002 — Server Grouping by Project/Team  
**Solution Ref:** SOL-CR-002, Section 3.1  
**Dependencies:** TASK-002-B (SshTargetFilter type)  
**Estimated:** 1–2 giờ  
**Status:** ✅ DONE

---

## Mục tiêu

Tạo component `FleetFilterBar` — thanh filter với search input, project dropdown, environment dropdown, và clear button.

---

## File cần tạo

`src/renderer/src/components/settings/ssh/FleetFilterBar.tsx`

---

## Bước thực thi

### Bước 1: Kiểm tra shadcn components cần thiết

```bash
ls src/renderer/src/components/ui/select.tsx   # Select/SelectTrigger
ls src/renderer/src/components/ui/input.tsx    # Input
ls src/renderer/src/components/ui/button.tsx   # Button
```

### Bước 2: Tạo FleetFilterBar.tsx

```typescript
// src/renderer/src/components/settings/ssh/FleetFilterBar.tsx
import { SearchIcon } from 'lucide-react'
import { Input } from '@/components/ui/input'
import { Button } from '@/components/ui/button'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { translate } from '@/i18n/i18n'
import type { SshTargetFilter } from '@/store/selectors'

type FleetFilterBarProps = {
  filter: SshTargetFilter
  onFilterChange: (filter: SshTargetFilter) => void
  projects: string[]
  teams: string[]
}

export function FleetFilterBar({
  filter,
  onFilterChange,
  projects,
  teams,
}: FleetFilterBarProps) {
  const hasActiveFilter =
    !!filter.project || !!filter.team || !!filter.environment || !!filter.search

  return (
    <div className="flex flex-wrap items-center gap-2 pb-3 border-b">
      {/* Search input */}
      <div className="relative flex-1 min-w-[180px]">
        <SearchIcon className="absolute left-2 top-2.5 h-4 w-4 text-muted-foreground pointer-events-none" />
        <Input
          placeholder={translate('fleet.filter.search', 'Search hosts...')}
          value={filter.search ?? ''}
          onChange={(e) =>
            onFilterChange({ ...filter, search: e.target.value || undefined })
          }
          className="pl-8 h-9"
        />
      </div>

      {/* Project filter */}
      {projects.length > 0 && (
        <Select
          value={filter.project ?? 'all'}
          onValueChange={(v) =>
            onFilterChange({
              ...filter,
              project: v === 'all' ? undefined : v,
            })
          }
        >
          <SelectTrigger className="h-9 w-[160px]">
            <SelectValue
              placeholder={translate('fleet.filter.allProjects', 'All projects')}
            />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">
              {translate('fleet.filter.allProjects', 'All projects')}
            </SelectItem>
            {projects.map((p) => (
              <SelectItem key={p} value={p}>
                {p}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      )}

      {/* Environment filter */}
      <Select
        value={filter.environment ?? 'all'}
        onValueChange={(v) =>
          onFilterChange({
            ...filter,
            environment:
              v === 'all'
                ? undefined
                : (v as 'development' | 'staging' | 'production'),
          })
        }
      >
        <SelectTrigger className="h-9 w-[140px]">
          <SelectValue
            placeholder={translate('fleet.filter.allEnvs', 'All envs')}
          />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value="all">
            {translate('fleet.filter.allEnvs', 'All envs')}
          </SelectItem>
          <SelectItem value="development">Development</SelectItem>
          <SelectItem value="staging">Staging</SelectItem>
          <SelectItem value="production">Production</SelectItem>
        </SelectContent>
      </Select>

      {/* Clear all filters */}
      {hasActiveFilter && (
        <Button
          variant="ghost"
          size="sm"
          onClick={() => onFilterChange({})}
          className="h-9 text-muted-foreground hover:text-foreground"
        >
          {translate('fleet.filter.clear', 'Clear')}
        </Button>
      )}
    </div>
  )
}
```

### Bước 3: Verify TypeScript

```bash
npx tsc --noEmit 2>&1 | grep "FleetFilter" | head -10
```

---

## Acceptance Criteria

- [x] Search input: typing updates `filter.search`
- [x] Project dropdown: chọn project filter by project, "All projects" = clear
- [x] Environment dropdown: development/staging/production/all
- [x] Clear button: chỉ hiện khi có active filter, click → `onFilterChange({})`
- [x] Projects dropdown ẩn khi `projects.length === 0`
- [x] TypeScript compile clean

---

## Notes cho AI

- `SshTargetFilter` type từ `@/store/selectors` (hoặc `@/store/types` nếu export từ đó)
- Dùng controlled Select (value + onValueChange), không uncontrolled
- Search: dùng `|| undefined` để không lưu empty string
- Teams filter có thể bỏ qua nếu UX phức tạp (project filter là đủ cho Phase 1)

---

## Implementation Notes

> **Completed:** 2026-07-23 | `FleetFilterBar.tsx`: Search input (updates filter.search), project dropdown (controlled, shows when projects>0, 'All'=clear), environment dropdown (4 options), Clear button (only when filter active, click resets to {}). TypeScript: ✅ 0 errors.
