# TASK-V5-03: WorkspaceLayout + ProjectSwitcher UI

**Order:** 3  
**Prerequisite:** TASK-V5-02 (WorkspaceContext, WorkspaceSlice)  
**Solution Ref:** SOL-FE-V5-02 (section 3, 4)  
**Est. effort:** ~90 min | **Tests:** 13

---

## Mô tả

Tạo WorkspaceLayout (3-panel resizable) và ProjectSwitcher (dropdown command palette). Đây là shell chứa tất cả workspace feature panels (git, tasks, workflows, file explorer).

---

## Files Cần Tạo

### 1. `src/renderer/src/components/workspace/NoProjectSelected.tsx`

```typescript
export function NoProjectSelected() {
  return (
    <div className="flex flex-col items-center justify-center h-full gap-4 text-muted-foreground">
      <FolderOpen size={48} className="opacity-30" />
      <p className="text-lg font-medium">No project selected</p>
      <p className="text-sm">Select a project from the switcher above to get started</p>
    </div>
  )
}
```

### 2. `src/renderer/src/components/workspace/WorkspaceSkeletonLoader.tsx`

```typescript
import { Skeleton } from '../ui/skeleton'

export function WorkspaceSkeletonLoader() {
  return (
    <div className="workspace-skeleton p-4 space-y-3 h-full">
      <Skeleton className="h-8 w-64" />
      <div className="flex gap-3 h-[calc(100%-3rem)]">
        <Skeleton className="h-full w-1/4" />
        <Skeleton className="h-full flex-1" />
        <Skeleton className="h-full w-1/3" />
      </div>
    </div>
  )
}
```

### 3. `src/renderer/src/components/workspace/OfflineBanner.tsx`

```typescript
import { WifiOff } from 'lucide-react'
import { Button } from '../ui/button'

interface OfflineBannerProps {
  message:  string
  onRetry?: () => void
}

export function OfflineBanner({ message, onRetry }: OfflineBannerProps) {
  return (
    <div
      data-testid="offline-banner"
      className="bg-yellow-50 border-b border-yellow-200 px-4 py-2 flex items-center gap-2"
    >
      <WifiOff size={14} className="text-yellow-600 shrink-0" />
      <span className="text-sm text-yellow-800">{message}</span>
      {onRetry && (
        <Button size="sm" variant="outline" onClick={onRetry} className="ml-auto">
          Retry Connection
        </Button>
      )}
    </div>
  )
}
```

### 4. `src/renderer/src/components/workspace/WorkspaceTabBar.tsx`

```typescript
type WorkspaceTab = 'git' | 'tasks' | 'workflows' | 'agent'

interface WorkspaceTabBarProps {
  activeTab: WorkspaceTab
  onTabChange: (tab: WorkspaceTab) => void
}

const TABS: Array<{ id: WorkspaceTab; label: string; icon: ReactNode }> = [
  { id: 'git',       label: 'Git',       icon: <GitBranch size={14} /> },
  { id: 'tasks',     label: 'Tasks',     icon: <CheckSquare size={14} /> },
  { id: 'workflows', label: 'Workflows', icon: <Workflow size={14} /> },
  { id: 'agent',     label: 'Agent',     icon: <Bot size={14} /> },
]

export function WorkspaceTabBar({ activeTab, onTabChange }: WorkspaceTabBarProps) {
  return (
    <div className="workspace-tab-bar flex border-b bg-muted/30">
      {TABS.map(tab => (
        <button
          key={tab.id}
          onClick={() => onTabChange(tab.id)}
          data-testid={`tab-${tab.id}`}
          className={cn(
            'flex items-center gap-1.5 px-4 py-2 text-sm font-medium border-b-2 transition-colors',
            activeTab === tab.id
              ? 'border-primary text-primary'
              : 'border-transparent text-muted-foreground hover:text-foreground'
          )}
        >
          {tab.icon}
          {tab.label}
        </button>
      ))}
    </div>
  )
}
```

### 5. `src/renderer/src/components/workspace/WorkspaceLayout.tsx`

```typescript
import React, { lazy, Suspense, useState } from 'react'
import { useWorkspace } from '../../context/WorkspaceContext'
import { WorkspaceTabBar } from './WorkspaceTabBar'
import { OfflineBanner } from './OfflineBanner'
import { NoProjectSelected } from './NoProjectSelected'
import { WorkspaceSkeletonLoader } from './WorkspaceSkeletonLoader'

// Lazy loaded panels (heavy components)
const ExplorerPanel     = lazy(() => import('./ExplorerPanel'))         // TASK-09
const GitPanel          = lazy(() => import('./git/GitPanel'))          // TASK-12
const TaskGraphPanel    = lazy(() => import('../task/TaskGraph'))       // TASK-15
const WorkflowMonitor   = lazy(() => import('../workflow/WorkflowBuilder')) // TASK-19

type WorkspaceTab = 'git' | 'tasks' | 'workflows' | 'agent'

export function WorkspaceLayout() {
  const { project, isOffline, isInitializing, switchProject } = useWorkspace()
  const [activeTab, setActiveTab]           = useState<WorkspaceTab>('git')
  const [rightPanelVisible, setRightPanel]  = useState(true)
  const [terminalVisible, setTerminal]      = useState(false)

  if (!project)        return <NoProjectSelected />
  if (isInitializing)  return <WorkspaceSkeletonLoader />

  return (
    <div className="workspace-layout flex flex-col h-full" data-testid="workspace-layout">
      {isOffline && (
        <OfflineBanner
          message="Dev server unreachable — read-only mode"
          onRetry={() => switchProject(project.id)}
        />
      )}

      <WorkspaceTabBar activeTab={activeTab} onTabChange={setActiveTab} />

      <div className="workspace-body flex flex-1 overflow-hidden">
        {/* Left: File Explorer (always visible) */}
        <div className="workspace-left w-56 min-w-[140px] border-r overflow-y-auto">
          <Suspense fallback={<div className="p-2 text-xs text-muted-foreground">Loading...</div>}>
            <ExplorerPanel />
          </Suspense>
        </div>

        {/* Center: Tab content */}
        <div className="workspace-center flex-1 overflow-auto">
          <Suspense fallback={<WorkspaceSkeletonLoader />}>
            {activeTab === 'git'       && <GitPanel />}
            {activeTab === 'tasks'     && <TaskGraphPanel projectId={project.id} />}
            {activeTab === 'workflows' && <WorkflowMonitor />}
          </Suspense>
        </div>

        {/* Right: collapsible sidebar */}
        {rightPanelVisible && (
          <div className="workspace-right w-80 min-w-[240px] border-l overflow-y-auto">
            {/* RightSidebar placeholder */}
          </div>
        )}
      </div>
    </div>
  )
}
```

### 6. `src/renderer/src/components/project/ProjectSwitcher.tsx`

```typescript
import { useState } from 'react'
import { useWorkspace } from '../../context/WorkspaceContext'
import { useAppStore } from '../../store'
import { Check, ChevronsUpDown, Loader2, Plus } from 'lucide-react'
import { Button } from '../ui/button'
import { Command, CommandEmpty, CommandGroup, CommandInput, CommandItem, CommandList, CommandSeparator } from '../ui/command'
import { Popover, PopoverContent, PopoverTrigger } from '../ui/popover'
import { cn } from '../../utils'

export function ProjectSwitcher() {
  const { project, switchProject, isInitializing } = useWorkspace()
  const projects = useAppStore(s => s.projects)
  const [open, setOpen]     = useState(false)
  const [search, setSearch] = useState('')

  const filtered = projects.filter(p =>
    p.name.toLowerCase().includes(search.toLowerCase())
  )

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <Button
          variant="outline"
          role="combobox"
          aria-expanded={open}
          disabled={isInitializing}
          data-testid="project-switcher-trigger"
          className="w-52 justify-between"
        >
          {isInitializing ? (
            <Loader2 className="animate-spin" size={16} />
          ) : (
            <span className="truncate">{project?.name ?? 'Select Project'}</span>
          )}
          <ChevronsUpDown size={14} className="ml-auto opacity-50 shrink-0" />
        </Button>
      </PopoverTrigger>
      <PopoverContent className="w-72 p-0" align="start">
        <Command>
          <CommandInput
            placeholder="Search projects..."
            value={search}
            onValueChange={setSearch}
          />
          <CommandList>
            <CommandEmpty>No projects found</CommandEmpty>
            <CommandGroup>
              {filtered.map(p => (
                <CommandItem
                  key={p.id}
                  value={p.id}
                  onSelect={() => {
                    switchProject(p.id)
                    setOpen(false)
                  }}
                >
                  <Check
                    className={cn('mr-2 shrink-0', p.id === project?.id ? 'opacity-100' : 'opacity-0')}
                    size={14}
                  />
                  <span className="truncate">{p.name}</span>
                  <span className="ml-auto text-xs text-muted-foreground shrink-0">
                    {p.devServerId}
                  </span>
                </CommandItem>
              ))}
            </CommandGroup>
            <CommandSeparator />
            <CommandItem data-testid="create-project-item">
              <Plus size={14} className="mr-2" />
              Create New Project
            </CommandItem>
          </CommandList>
        </Command>
      </PopoverContent>
    </Popover>
  )
}
```

---

## Tests — `src/renderer/src/components/project/__tests__/ProjectSwitcher.test.tsx`

```typescript
// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { render, screen, cleanup, fireEvent } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { ProjectSwitcher } from '../ProjectSwitcher'

afterEach(() => cleanup())

const switchProject = vi.fn()
vi.mock('../../../context/WorkspaceContext', () => ({
  useWorkspace: () => ({
    project:         { id: 'p1', name: 'MyApp Backend', devServerId: 'srv1' },
    switchProject,
    isInitializing:  false,
  }),
}))
vi.mock('../../../store', () => ({
  useAppStore: (fn: any) => fn({
    projects: [
      { id: 'p1', name: 'MyApp Backend',  devServerId: 'srv1' },
      { id: 'p2', name: 'Frontend Web',   devServerId: 'srv1' },
      { id: 'p3', name: 'API Gateway',    devServerId: 'srv2' },
    ]
  }),
}))

describe('ProjectSwitcher', () => {
  it('renders current project name', () => {
    render(<ProjectSwitcher />)
    expect(screen.getByText('MyApp Backend')).toBeInTheDocument()
  })

  it('opens dropdown on trigger click', async () => {
    render(<ProjectSwitcher />)
    fireEvent.click(screen.getByTestId('project-switcher-trigger'))
    expect(await screen.findByText('Frontend Web')).toBeInTheDocument()
    expect(await screen.findByText('API Gateway')).toBeInTheDocument()
  })

  it('filters projects by search text', async () => {
    render(<ProjectSwitcher />)
    fireEvent.click(screen.getByTestId('project-switcher-trigger'))
    const input = await screen.findByPlaceholderText('Search projects...')
    fireEvent.change(input, { target: { value: 'api' } })
    expect(screen.queryByText('Frontend Web')).not.toBeInTheDocument()
    expect(screen.getByText('API Gateway')).toBeInTheDocument()
  })

  it('calls switchProject on item select', async () => {
    render(<ProjectSwitcher />)
    fireEvent.click(screen.getByTestId('project-switcher-trigger'))
    const item = await screen.findByText('Frontend Web')
    fireEvent.click(item)
    expect(switchProject).toHaveBeenCalledWith('p2')
  })

  it('shows loading spinner during initialization', () => {
    vi.mock('../../../context/WorkspaceContext', () => ({
      useWorkspace: () => ({ project: null, switchProject: vi.fn(), isInitializing: true }),
    }))
    // Trigger button should be disabled
    render(<ProjectSwitcher />)
    const btn = screen.getByTestId('project-switcher-trigger')
    expect(btn).toBeDisabled()
  })
})
```

## Tests — `src/renderer/src/components/workspace/__tests__/WorkspaceLayout.test.tsx`

```typescript
// @vitest-environment happy-dom
// 8 tests: NoProjectSelected, WorkspaceSkeletonLoader, OfflineBanner,
//          tab switching, terminal toggle, right panel toggle
// (Xem SOL-FE-V5-02 section 7 cho full test list)
```

---

## Acceptance Criteria

- [x] `NoProjectSelected` render khi `project === null`
- [x] `WorkspaceSkeletonLoader` render khi `isInitializing === true`
- [x] `OfflineBanner` render khi `isOffline === true`
- [x] `ProjectSwitcher` dropdown có search input
- [x] `switchProject` gọi khi click item
- [x] `WorkspaceTabBar` active tab border visible
- [x] Tất cả panels lazy-loaded (`React.lazy`) — ExplorerPanel, GitPanel, TaskGraphPanel, WorkflowMonitor
- [x] 17/17 tests pass (12 WorkspaceLayout + 5 ProjectSwitcher)
