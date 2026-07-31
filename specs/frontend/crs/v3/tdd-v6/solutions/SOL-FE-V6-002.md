# SOL-FE-V6-002: Project Workspace Shell (TDD-FE-12)

**Solution ID:** SOL-FE-V6-002
**TDD Ref:** [TDD-FE-12](../../../../tdd/v5/12-project-workspace-ui.md)
**Feature:** F34, F38 | **ADR:** ADR-011 | **HLD Ref:** C3.12, C4.10
**Date:** 2026-07-30
**Status:** ✅ COMPLETED — 2026-07-30

---

## 1. Phan tich code hien co

### 1.1 Da ton tai (KHONG viet lai)

| File | Size | Nhan xet |
|------|------|---------|
| `context/WorkspaceContext.tsx` | 7757 bytes (181 lines) | DAY DU — switchProject, refreshGitStatus, refreshFileTree, emit/on |
| `components/workspace/WorkspaceLayout.tsx` | 2752 bytes (63 lines) | SKELETON — can boi sung ResizablePanel + right panel |
| `components/workspace/WorkspaceTabBar.tsx` | 1478 bytes | Co san — day du |
| `components/workspace/OfflineBanner.tsx` | 816 bytes | Co san — day du |
| `components/workspace/NoProjectSelected.tsx` | 545 bytes | Co san — day du |
| `components/workspace/WorkspaceSkeletonLoader.tsx` | 571 bytes | Co san — day du |
| `components/project/ProjectSwitcher.tsx` | 3015 bytes | Co san |
| `hooks/useWorkspace.ts` | 130 bytes | Chi re-export tu Context — OK |

### 1.2 Chua ton tai (CAN TAO MOI)

| File | Do uu tien | Ly do |
|------|-----------|-------|
| `components/project/ProjectSettings.tsx` | HIGH | Dialog settings cho project |
| `components/project/MemberManager.tsx` | MEDIUM | CRUD members |
| `components/workspace/ServerStatusBar.tsx` | MEDIUM | Health indicator |

---

## 2. Giai phap — WorkspaceLayout Upgrade

### 2.1 Van de hien tai

`WorkspaceLayout.tsx` hien tai (63 lines) co nhung van de sau:
1. **Right panel la placeholder** — chi co `{/* RightSidebar placeholder */}`
2. **Khong co ResizablePanel** — dung CSS flex co dinh thay vi drag-to-resize
3. **Khong co terminal panel** — TDD-FE-12 yeu cau terminal collapsible o bottom
4. **GitPanel duoc goi voi prop `projectId`** nhung trong TDD, GitPanel lay project tu WorkspaceContext

**Giai phap:** Boi sung WorkspaceLayout thay vi viet lai

```typescript
// MODIFY: src/renderer/src/components/workspace/WorkspaceLayout.tsx
// Bo sung: ResizablePanelGroup, right panel, terminal panel

import { lazy, Suspense, useState } from 'react'
import { useWorkspace } from '../../context/WorkspaceContext'
import { WorkspaceTabBar } from './WorkspaceTabBar'
import { OfflineBanner } from './OfflineBanner'
import { NoProjectSelected } from './NoProjectSelected'
import { WorkspaceSkeletonLoader } from './WorkspaceSkeletonLoader'
import {
  ResizablePanelGroup,
  ResizablePanel,
  ResizableHandle,
} from '@/components/ui/resizable'

const ExplorerPanel   = lazy(() => import('./ExplorerPanel').then(m => ({ default: m.ExplorerPanel })))
const GitPanel        = lazy(() => import('./git/GitPanel').then(m => ({ default: m.GitPanel })))
const TaskGraphPanel  = lazy(() => import('../task/TaskGraphPanel').then(m => ({ default: m.TaskGraphPanel })))
const WorkflowMonitor = lazy(() => import('../workflow/WorkflowMonitor').then(m => ({ default: m.WorkflowMonitor })))

type WorkspaceTab = 'git' | 'tasks' | 'workflows' | 'agent'

export function WorkspaceLayout() {
  const { project, isOffline, isInitializing, switchProject } = useWorkspace()
  const [activeTab, setActiveTab]           = useState<WorkspaceTab>('git')
  const [rightPanelVisible, setRightPanel]  = useState(true)
  const [terminalVisible, setTerminalVisible] = useState(false)

  if (!project)       return <NoProjectSelected />
  if (isInitializing) return <WorkspaceSkeletonLoader />

  return (
    <div className="workspace-layout flex flex-col h-full" data-testid="workspace-layout">
      {isOffline && (
        <OfflineBanner
          message="Dev server unreachable — read-only mode"
          onRetry={() => switchProject(project.id)}
        />
      )}

      <WorkspaceTabBar activeTab={activeTab} onTabChange={setActiveTab} />

      <ResizablePanelGroup direction="horizontal" className="flex-1 overflow-hidden">
        {/* Left: File Explorer (always visible) */}
        <ResizablePanel defaultSize={20} minSize={15} maxSize={35}>
          <Suspense fallback={<div className="p-2 text-xs text-muted-foreground">Loading...</div>}>
            <ExplorerPanel />
          </Suspense>
        </ResizablePanel>

        <ResizableHandle />

        {/* Center: Tab content */}
        <ResizablePanel defaultSize={rightPanelVisible ? 50 : 80}>
          <Suspense fallback={<WorkspaceSkeletonLoader />}>
            {activeTab === 'git'       && <GitPanel />}
            {activeTab === 'tasks'     && <TaskGraphPanel projectId={project.id} />}
            {activeTab === 'workflows' && <WorkflowMonitor />}
          </Suspense>
        </ResizablePanel>

        {/* Right: collapsible sidebar */}
        {rightPanelVisible && (
          <>
            <ResizableHandle />
            <ResizablePanel defaultSize={30} minSize={20}>
              {/* RightSidebar — will be implemented per active tab */}
              <div className="workspace-right h-full border-l bg-muted/30 p-3 text-xs text-muted-foreground">
                {activeTab === 'git' && <span>Git details panel</span>}
                {activeTab === 'tasks' && <span>Task detail panel</span>}
              </div>
            </ResizablePanel>
          </>
        )}
      </ResizablePanelGroup>

      {/* Bottom: terminal (collapsible) */}
      {terminalVisible && (
        <div className="workspace-terminal border-t h-48">
          <div className="p-2 text-xs text-muted-foreground">Terminal placeholder</div>
        </div>
      )}

      {/* Status bar */}
      <div className="workspace-statusbar flex items-center gap-2 px-3 py-1 border-t text-xs bg-muted/50">
        <button
          onClick={() => setTerminalVisible(v => !v)}
          className="hover:text-foreground text-muted-foreground"
        >
          {terminalVisible ? 'Hide Terminal' : 'Show Terminal'}
        </button>
        <button
          onClick={() => setRightPanel(v => !v)}
          className="ml-auto hover:text-foreground text-muted-foreground"
        >
          {rightPanelVisible ? 'Hide Panel' : 'Show Panel'}
        </button>
      </div>
    </div>
  )
}
```

**Luu y quan trong:** 
- `shadcn/ui resizable` can duoc them vao neu chua co: `npx shadcn add resizable`
- Neu chua co ResizablePanel, dung alternative CSS-based approach (giu tuong tu cau truc)

---

## 3. Giai phap — ProjectSettings Dialog

**File moi:** `src/renderer/src/components/project/ProjectSettings.tsx`

```typescript
// NEW: src/renderer/src/components/project/ProjectSettings.tsx
import { useState } from 'react'
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Tabs, TabsList, TabsTrigger, TabsContent } from '@/components/ui/tabs'
import { MemberManager } from './MemberManager'
import { useAppStore } from '@/store'

interface ProjectSettingsProps {
  projectId: string
  open: boolean
  onClose: () => void
}

export function ProjectSettings({ projectId, open, onClose }: ProjectSettingsProps) {
  const project = useAppStore(s => s.projects?.find(p => p.id === projectId))
  const [activeTab, setActiveTab] = useState('general')

  return (
    <Dialog open={open} onOpenChange={onClose}>
      <DialogContent className="max-w-2xl">
        <DialogHeader>
          <DialogTitle>Project Settings — {project?.name ?? projectId}</DialogTitle>
        </DialogHeader>
        <Tabs value={activeTab} onValueChange={setActiveTab}>
          <TabsList>
            <TabsTrigger value="general">General</TabsTrigger>
            <TabsTrigger value="members">Members</TabsTrigger>
          </TabsList>
          <TabsContent value="general">
            <ProjectGeneralForm project={project} onClose={onClose} />
          </TabsContent>
          <TabsContent value="members">
            <MemberManager projectId={projectId} />
          </TabsContent>
        </Tabs>
      </DialogContent>
    </Dialog>
  )
}

function ProjectGeneralForm({ project, onClose }: { project: any; onClose: () => void }) {
  // Basic form: name, description
  return (
    <div className="space-y-3 py-2">
      <p className="text-sm text-muted-foreground">
        General project settings (name, description, bindings).
      </p>
      {/* Form fields se duoc implement sau */}
    </div>
  )
}
```

---

## 4. Giai phap — MemberManager

**File moi:** `src/renderer/src/components/project/MemberManager.tsx`

```typescript
// NEW: src/renderer/src/components/project/MemberManager.tsx
// Tai su dung: shadcn Table, Button, Badge, Select
import { useState, useEffect, useCallback } from 'react'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { callRuntimeRpc, getActiveRuntimeTarget } from '@/runtime/runtime-rpc-client'
import { useAppStore } from '@/store'
import { toast } from 'sonner'
import { Trash2 } from 'lucide-react'

type ProjectRole = 'developer' | 'lead' | 'admin'

interface ProjectMember {
  userId: string
  displayName: string
  email: string
  role: ProjectRole
  joinedAt: Date
}

export function MemberManager({ projectId }: { projectId: string }) {
  const [members, setMembers] = useState<ProjectMember[]>([])
  const [isLoading, setIsLoading] = useState(true)

  const loadMembers = useCallback(async () => {
    try {
      const target = getActiveRuntimeTarget(useAppStore.getState().settings)
      const result = await callRuntimeRpc<ProjectMember[]>(target, 'projects.listMembers', { projectId })
      setMembers(result)
    } catch {
      toast.error('Failed to load members')
    } finally {
      setIsLoading(false)
    }
  }, [projectId])

  useEffect(() => { loadMembers() }, [loadMembers])

  const updateRole = async (userId: string, role: ProjectRole) => {
    const target = getActiveRuntimeTarget(useAppStore.getState().settings)
    await callRuntimeRpc(target, 'projects.updateMemberRole', { projectId, userId, role })
    setMembers(prev => prev.map(m => m.userId === userId ? { ...m, role } : m))
    toast.success('Role updated')
  }

  const removeMember = async (userId: string) => {
    const target = getActiveRuntimeTarget(useAppStore.getState().settings)
    await callRuntimeRpc(target, 'projects.removeMember', { projectId, userId })
    setMembers(prev => prev.filter(m => m.userId !== userId))
    toast.success('Member removed')
  }

  if (isLoading) return <div className="p-4 text-sm text-muted-foreground">Loading members...</div>

  return (
    <div className="member-manager">
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>Member</TableHead>
            <TableHead>Role</TableHead>
            <TableHead className="w-10"></TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {members.map(member => (
            <TableRow key={member.userId}>
              <TableCell>
                <div>
                  <p className="font-medium text-sm">{member.displayName}</p>
                  <p className="text-xs text-muted-foreground">{member.email}</p>
                </div>
              </TableCell>
              <TableCell>
                <Select
                  value={member.role}
                  onValueChange={(role) => updateRole(member.userId, role as ProjectRole)}
                >
                  <SelectTrigger className="w-32 h-7 text-xs">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="developer">Developer</SelectItem>
                    <SelectItem value="lead">Lead</SelectItem>
                    <SelectItem value="admin">Admin</SelectItem>
                  </SelectContent>
                </Select>
              </TableCell>
              <TableCell>
                <Button variant="ghost" size="icon" className="h-7 w-7" onClick={() => removeMember(member.userId)}>
                  <Trash2 size={12} />
                </Button>
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  )
}
```

---

## 5. WorkspaceContext — Verification

**Cac diem can kiem tra trong WorkspaceContext.tsx (da co 181 lines):**

1. **`switchProject()` flow** — Da implement dung pattern:
   - `setIsInitializing(true)`
   - `Promise.all([project.get, git.status, workspace.listFiles, profile.getResolved])`
   - Set state va emit `project.switched`
   - `setIsOffline(true)` khi `DEV_SERVER_UNREACHABLE`

2. **`WorkspaceContextValue` interface** — Can kiem tra co day du:
   - `currentWorktree`, `availableWorktrees`, `setCurrentWorktree`
   - `activeAgentSessionId`, `setActiveAgentSession`

   **Gap:** WorkspaceContext hien chi co `fileTree: FileNode | null` nhung TDD yeu cau `fileTree: FileNode[]` (mang) + `currentWorktree: Worktree | null`

3. **`on()` / `emit()` event bus** — Da implement day du

---

## 6. Store Verification

**Slice `workspace-slice.ts` can kiem tra:**

```typescript
// Can co cac fields:
export type WorkspaceSlice = {
  projects: OrcaProject[]     // danh sach projects
  activeProjectId: string | null
  setProjects: (p: OrcaProject[]) => void
  setActiveProject: (id: string | null) => void
  addMember: (projectId: string, member: any) => void
}
```

---

## 7. Test Plan

**Target:** >= 25 tests

```
src/renderer/src/components/project/__tests__/
├── ProjectSwitcher.test.tsx         (5+ tests)
│   ├── renders current project name
│   ├── opens dropdown with project list
│   ├── calls switchProject on item select
│   ├── shows loading spinner during initialization
│   └── filters by search text
├── ProjectSettings.test.tsx         (4+ tests)
│   ├── renders General + Members tabs
│   ├── closes on Escape
│   └── renders project name in title

src/renderer/src/components/workspace/__tests__/
├── WorkspaceLayout.test.tsx         (6+ tests)
│   ├── renders NoProjectSelected when no project
│   ├── renders WorkspaceSkeletonLoader when initializing
│   ├── renders OfflineBanner when isOffline
│   ├── shows GitPanel when git tab active
│   ├── shows TaskGraphPanel when tasks tab active
│   └── toggles terminal panel visibility

src/renderer/src/context/__tests__/
└── WorkspaceContext.test.tsx        (8+ tests)
    ├── switchProject loads data and sets state
    ├── switchProject: DEV_SERVER_UNREACHABLE => isOffline=true
    ├── refreshGitStatus updates gitStatus
    ├── refreshFileTree updates fileTree
    ├── emit + on: handler receives event
    ├── on returns cleanup function that removes handler
    ├── agent.complete event triggers refreshGitStatus listener
    └── isInitializing resets to false after switchProject
```

---

## 8. Phu thuoc va Thu tu

**Prerequisite:** shadcn/ui `resizable` component phai co san

**Cach kiem tra:** `ls src/renderer/src/components/ui/resizable.tsx`

**Neu chua co, install:**
```bash
npx shadcn add resizable
```

**Cac file phu thuoc vao SOL-FE-V6-002:**
- SOL-FE-V6-005 (TaskGraph), SOL-FE-V6-006 (Git), SOL-FE-V6-007 (Explorer) deu render trong WorkspaceLayout
