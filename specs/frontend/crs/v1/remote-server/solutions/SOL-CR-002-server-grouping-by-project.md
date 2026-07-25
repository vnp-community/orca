# SOL-CR-002 — Frontend Solution: Server Grouping by Project/Team

**CR:** CR-002 — Server Grouping by Project/Team  
**Priority:** 🟠 High  
**TDD References:** TDD-FE-02 (State Management), TDD-FE-05 (UI Components)  
**Depends on:** SOL-CR-001 (SshTarget types mở rộng)  
**Estimated effort:** 1–2 ngày frontend  
**Implementation Status:** ✅ IMPLEMENTED — 2026-07-23  
**Tasks:** TASK-002-A (SshSlice grouping), TASK-002-B (selectors), TASK-002-C (FleetFilterBar), TASK-002-D (SshTargetGroupedList), TASK-002-E (SshTargetGroupRow), TASK-002-F (SshStatusSection sidebar)

---

## 1. Tổng quan giải pháp

CR-002 yêu cầu chuyển SSH target list từ **flat list** sang **grouped view** theo project/team. Frontend cần:

1. Mở rộng **SshSlice** với selectors cho grouping
2. Thêm **FleetFilterBar** component (filter by project/team/environment)
3. Refactor **SshTargetList** thành **SshTargetGroupedList**
4. Thêm **collapsible group headers**

---

## 2. Thay đổi Store — Selectors

### 2.1 Thêm selectors vào `store/selectors.ts`

```typescript
// src/renderer/src/store/selectors.ts

// [NEW] Group SSH targets by project
export function selectSshTargetsByProject(
  state: AppState
): Record<string, SshTarget[]> {
  const targets = Object.values(state.sshConnectionStates)
    .map(s => /* get SshTarget by targetId */)

  // Lấy targets từ runtime graph (đã sync vào store)
  const allTargets = state.sshTargets ?? []

  return allTargets.reduce<Record<string, SshTarget[]>>((acc, target) => {
    const group = target.project ?? '__unassigned__'
    if (!acc[group]) acc[group] = []
    acc[group].push(target)
    return acc
  }, {})
}

// [NEW] Get unique projects from targets
export function selectUniqueProjects(state: AppState): string[] {
  const targets = state.sshTargets ?? []
  const projects = new Set(
    targets.map(t => t.project).filter(Boolean) as string[]
  )
  return Array.from(projects).sort()
}

// [NEW] Get unique teams from targets
export function selectUniqueTeams(state: AppState): string[] {
  const targets = state.sshTargets ?? []
  const teams = new Set(
    targets.map(t => t.team).filter(Boolean) as string[]
  )
  return Array.from(teams).sort()
}

// [NEW] Filter targets
export function selectFilteredSshTargets(
  state: AppState,
  filter: SshTargetFilter
): SshTarget[] {
  const targets = state.sshTargets ?? []
  return targets.filter(t => {
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

type SshTargetFilter = {
  project?: string
  team?: string
  environment?: 'development' | 'staging' | 'production'
  search?: string
}
```

### 2.2 Thêm `sshTargets` vào SshSlice

```typescript
// src/renderer/src/store/slices/ssh.ts
// SshTarget list cần được sync từ RuntimeSyncWindowGraph

type SshSlice = {
  // ...existing...

  // [NEW] Full list synced from backend
  sshTargets: SshTarget[]
  setSshTargets: (targets: SshTarget[]) => void

  // [NEW] Collapsed groups (persisted)
  collapsedSshGroups: Record<string, boolean>
  toggleSshGroupCollapsed: (groupKey: string) => void
}
```

---

## 3. UI Components

### 3.1 `FleetFilterBar` — Filter panel

```typescript
// src/renderer/src/components/settings/ssh/FleetFilterBar.tsx

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
  return (
    <div className="flex flex-wrap gap-2 pb-3 border-b">
      {/* Search input */}
      <div className="relative flex-1 min-w-[180px]">
        <SearchIcon className="absolute left-2 top-2.5 h-4 w-4 text-muted-foreground" />
        <Input
          placeholder={translate('fleet.filter.search', 'Search hosts...')}
          value={filter.search ?? ''}
          onChange={e => onFilterChange({ ...filter, search: e.target.value })}
          className="pl-8 h-9"
        />
      </div>

      {/* Project filter */}
      {projects.length > 0 && (
        <Select
          value={filter.project ?? 'all'}
          onValueChange={v =>
            onFilterChange({ ...filter, project: v === 'all' ? undefined : v })
          }
        >
          <SelectTrigger className="h-9 w-[160px]">
            <SelectValue placeholder={translate('fleet.filter.project', 'All projects')} />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">
              {translate('fleet.filter.allProjects', 'All projects')}
            </SelectItem>
            {projects.map(p => (
              <SelectItem key={p} value={p}>{p}</SelectItem>
            ))}
          </SelectContent>
        </Select>
      )}

      {/* Environment filter */}
      <Select
        value={filter.environment ?? 'all'}
        onValueChange={v =>
          onFilterChange({
            ...filter,
            environment: v === 'all' ? undefined : v as any,
          })
        }
      >
        <SelectTrigger className="h-9 w-[140px]">
          <SelectValue placeholder={translate('fleet.filter.env', 'All envs')} />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value="all">{translate('fleet.filter.allEnvs', 'All envs')}</SelectItem>
          <SelectItem value="development">Development</SelectItem>
          <SelectItem value="staging">Staging</SelectItem>
          <SelectItem value="production">Production</SelectItem>
        </SelectContent>
      </Select>

      {/* Clear filters */}
      {(filter.project || filter.team || filter.environment || filter.search) && (
        <Button
          variant="ghost"
          size="sm"
          onClick={() => onFilterChange({})}
          className="h-9 text-muted-foreground"
        >
          {translate('fleet.filter.clear', 'Clear')}
        </Button>
      )}
    </div>
  )
}
```

### 3.2 `SshTargetGroupedList` — Grouped view

```typescript
// src/renderer/src/components/settings/ssh/SshTargetGroupedList.tsx

export function SshTargetGroupedList() {
  const [filter, setFilter] = useState<SshTargetFilter>({})

  const sshTargets = useAppStore(s => s.sshTargets ?? [])
  const connectionStates = useAppStore(s => s.sshConnectionStates)
  const collapsedGroups = useAppStore(s => s.collapsedSshGroups)
  const toggleGroup = useAppStore(s => s.toggleSshGroupCollapsed)

  const projects = useAppStore(selectUniqueProjects)
  const teams = useAppStore(selectUniqueTeams)

  // Apply filter
  const filteredTargets = useMemo(
    () => selectFilteredSshTargets({ sshTargets } as any, filter),
    [sshTargets, filter]
  )

  // Group by project
  const groupedTargets = useMemo(() => {
    return filteredTargets.reduce<Record<string, SshTarget[]>>((acc, t) => {
      const key = t.project ?? '__unassigned__'
      if (!acc[key]) acc[key] = []
      acc[key].push(t)
      return acc
    }, {})
  }, [filteredTargets])

  const groupKeys = Object.keys(groupedTargets).sort((a, b) => {
    // Unassigned luôn cuối
    if (a === '__unassigned__') return 1
    if (b === '__unassigned__') return -1
    return a.localeCompare(b)
  })

  return (
    <div className="space-y-1">
      <FleetFilterBar
        filter={filter}
        onFilterChange={setFilter}
        projects={projects}
        teams={teams}
      />

      <div className="mt-3 space-y-2">
        {groupKeys.map(groupKey => {
          const targets = groupedTargets[groupKey]
          const isCollapsed = collapsedGroups[groupKey] ?? false
          const label =
            groupKey === '__unassigned__'
              ? translate('fleet.group.unassigned', 'Unassigned')
              : groupKey

          return (
            <SshTargetGroup
              key={groupKey}
              label={label}
              targets={targets}
              connectionStates={connectionStates}
              isCollapsed={isCollapsed}
              onToggleCollapse={() => toggleGroup(groupKey)}
            />
          )
        })}

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

### 3.3 `SshTargetGroup` — Collapsible group

```typescript
// src/renderer/src/components/settings/ssh/SshTargetGroup.tsx

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
  // Summary: connected count
  const connectedCount = targets.filter(
    t => connectionStates[t.id]?.status === 'connected'
  ).length

  return (
    <Collapsible open={!isCollapsed} onOpenChange={() => onToggleCollapse()}>
      {/* Group header */}
      <CollapsibleTrigger asChild>
        <button className="flex w-full items-center gap-2 rounded px-2 py-1.5 text-left hover:bg-muted/50">
          <ChevronRightIcon
            className={cn(
              'h-4 w-4 text-muted-foreground transition-transform',
              !isCollapsed && 'rotate-90'
            )}
          />
          <span className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
            {label}
          </span>
          <Badge variant="secondary" className="ml-auto text-xs">
            {connectedCount}/{targets.length}
          </Badge>
        </button>
      </CollapsibleTrigger>

      {/* Group items */}
      <CollapsibleContent>
        <div className="ml-4 space-y-0.5 border-l pl-3">
          {targets.map(target => (
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

### 3.4 `SshTargetRow` — Row với metadata badges

```typescript
// src/renderer/src/components/settings/ssh/SshTargetRow.tsx
// Cập nhật: hiển thị team + environment badges

export function SshTargetRow({
  target,
  connectionState,
}: {
  target: SshTarget
  connectionState?: SshConnectionState
}) {
  return (
    <div className="flex items-center gap-2 rounded px-2 py-1.5 hover:bg-muted/30">
      {/* Connection status dot */}
      <SshConnectionStatusDot status={connectionState?.status ?? 'disconnected'} />

      {/* Main info */}
      <div className="flex-1 min-w-0">
        <div className="flex items-center gap-1.5">
          <span className="text-sm font-medium truncate">{target.label}</span>

          {/* Team badge */}
          {target.team && (
            <Badge variant="outline" className="text-xs px-1.5 py-0">
              {target.team}
            </Badge>
          )}

          {/* Environment badge */}
          {target.environment && (
            <Badge
              variant="outline"
              className={cn('text-xs px-1.5 py-0', {
                'border-green-500/50 text-green-600': target.environment === 'development',
                'border-yellow-500/50 text-yellow-600': target.environment === 'staging',
                'border-red-500/50 text-red-600': target.environment === 'production',
              })}
            >
              {target.environment}
            </Badge>
          )}
        </div>
        <p className="text-xs text-muted-foreground truncate">
          {target.username}@{target.host}:{target.port}
        </p>
      </div>

      {/* Actions */}
      <SshTargetRowActions target={target} connectionState={connectionState} />
    </div>
  )
}
```

---

## 4. Sidebar Integration

SSH status trong Sidebar cũng cần grouped view:

```typescript
// src/renderer/src/components/sidebar/SshStatusSection.tsx
// Hiển thị compact grouped SSH status trong left sidebar

export function SshStatusSection() {
  const sshTargets = useAppStore(s => s.sshTargets ?? [])
  const connectionStates = useAppStore(s => s.sshConnectionStates)

  // Compact view: chỉ hiện project + connected count
  const projectSummary = useMemo(() => {
    return Object.entries(
      sshTargets.reduce<Record<string, { total: number; connected: number }>>((acc, t) => {
        const key = t.project ?? '__unassigned__'
        if (!acc[key]) acc[key] = { total: 0, connected: 0 }
        acc[key].total++
        if (connectionStates[t.id]?.status === 'connected') acc[key].connected++
        return acc
      }, {})
    )
  }, [sshTargets, connectionStates])

  return (
    <div className="px-2 py-1 space-y-0.5">
      {projectSummary.map(([project, counts]) => (
        <div key={project} className="flex items-center gap-1 text-xs">
          <span className={cn(
            'h-1.5 w-1.5 rounded-full',
            counts.connected === counts.total ? 'bg-green-500' :
            counts.connected > 0 ? 'bg-yellow-500' : 'bg-muted-foreground'
          )} />
          <span className="text-muted-foreground truncate">{project}</span>
          <span className="ml-auto text-muted-foreground">
            {counts.connected}/{counts.total}
          </span>
        </div>
      ))}
    </div>
  )
}
```

---

## 5. File mới cần tạo

| File | Loại | Mô tả |
|------|------|-------|
| `src/renderer/src/components/settings/ssh/FleetFilterBar.tsx` | [NEW] | Filter bar cho SSH targets |
| `src/renderer/src/components/settings/ssh/SshTargetGroupedList.tsx` | [NEW] | Grouped list (thay flat list) |
| `src/renderer/src/components/settings/ssh/SshTargetGroup.tsx` | [NEW] | Collapsible group header |
| `src/renderer/src/components/sidebar/SshStatusSection.tsx` | [NEW] | Compact SSH status trong sidebar |

## 6. File cần chỉnh sửa

| File | Thay đổi |
|------|---------|
| `src/renderer/src/store/slices/ssh.ts` | Thêm `sshTargets`, `collapsedSshGroups`, `toggleSshGroupCollapsed` |
| `src/renderer/src/store/selectors.ts` | Thêm `selectSshTargetsByProject`, `selectFilteredSshTargets`, v.v. |
| `src/renderer/src/components/settings/ssh/SshTargetRow.tsx` | Thêm team/environment badges |
| `src/renderer/src/components/sidebar/Sidebar.tsx` | Tích hợp `SshStatusSection` |
| `src/renderer/src/runtime/sync-runtime-graph.ts` | Sync `sshTargets` array vào store |

---

## 7. Acceptance Criteria (Frontend)

- [x] SSH Hosts sidebar hiển thị servers gom theo PROJECT header
- [x] Group headers collapsible (click để ẩn/hiện)
- [x] Filter bar: lọc theo project, team, environment, hoặc search text
- [x] Mỗi server row hiển thị: status dot + label + team badge + env badge
- [x] Servers không có project metadata → hiển thị ở nhóm "Unassigned"
- [x] Import từ fleet config → tự điền project/team/environment
- [x] Backward compatible: servers cũ vẫn hiển thị
- [x] Sidebar compact view: tóm tắt per-project connected/total

## 8. Implementation Notes

> **Implemented 2026-07-23**
>
> - `src/renderer/src/store/slices/ssh.ts`: Added `sshTargets: SshTarget[]`, `collapsedSshGroups`, `setSshTargets`, `toggleSshGroupCollapsed` actions.
> - `src/renderer/src/store/selectors.ts`: Added `selectSshTargetsByProject`, `selectFilteredSshTargets`, `SshTargetFilter`, `SshGroupedTargets`.
> - `src/renderer/src/components/settings/ssh/FleetFilterBar.tsx`: [NEW] Filter bar with project/team/env dropdowns + search.
> - `src/renderer/src/components/settings/ssh/SshTargetGroupedList.tsx`: [NEW] Grouped list component.
> - `src/renderer/src/components/settings/ssh/SshTargetGroupRow.tsx`: [NEW] Individual row with status dot + badges.
> - `src/renderer/src/components/settings/ssh/SshStatusSection.tsx`: [NEW] Compact sidebar SSH status section.
> - `src/renderer/src/components/settings/SshPane.tsx`: Integrated `SshTargetGroupedList` and `FleetFilterBar`.
> - **TypeScript:** ✅ 0 new errors.
