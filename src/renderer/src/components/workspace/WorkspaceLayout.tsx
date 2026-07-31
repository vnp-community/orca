// WorkspaceLayout.tsx — 3-panel resizable workspace shell (TDD-FE-12)
// TASK-FE-003: Upgraded with ResizablePanelGroup + status bar + terminal toggle
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
} from '../ui/resizable'

// Lazy loaded panels (heavy components)
const ExplorerPanel   = lazy(() => import('./ExplorerPanel').then(m => ({ default: m.ExplorerPanel })))
const GitPanel        = lazy(() => import('./git/GitPanel').then(m => ({ default: m.GitPanel })))
const TaskGraphPanel  = lazy(() => import('../task/TaskGraphPanel').then(m => ({ default: m.TaskGraphPanel })))
const WorkflowMonitor = lazy(() => import('../workflow/WorkflowMonitor').then(m => ({ default: m.WorkflowMonitor })))

type WorkspaceTab = 'git' | 'tasks' | 'workflows' | 'agent'

export function WorkspaceLayout() {
  const { project, isOffline, isInitializing, switchProject } = useWorkspace()
  const [activeTab, setActiveTab]             = useState<WorkspaceTab>('git')
  const [rightPanelVisible, setRightPanel]    = useState(true)
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

      <ResizablePanelGroup orientation="horizontal" className="flex-1 overflow-hidden">
        {/* Left: File Explorer (always visible) */}
        <ResizablePanel defaultSize={20} minSize={15} maxSize={35} data-testid="panel-explorer">
          <Suspense fallback={<div className="p-2 text-xs text-muted-foreground">Loading...</div>}>
            <ExplorerPanel />
          </Suspense>
        </ResizablePanel>

        <ResizableHandle />

        {/* Center: Tab content */}
        <ResizablePanel defaultSize={rightPanelVisible ? 50 : 80} data-testid="panel-center">
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
            <ResizablePanel defaultSize={30} minSize={20} data-testid="panel-right">
              <div className="workspace-right h-full border-l bg-muted/30 overflow-y-auto p-3 text-xs text-muted-foreground">
                {activeTab === 'git' && <span>Git details</span>}
                {activeTab === 'tasks' && <span>Task detail</span>}
              </div>
            </ResizablePanel>
          </>
        )}
      </ResizablePanelGroup>

      {/* Bottom: terminal (collapsible) */}
      {terminalVisible && (
        <div className="workspace-terminal border-t h-48 overflow-auto" data-testid="terminal-panel">
          <div className="p-2 text-xs text-muted-foreground">Terminal — coming soon</div>
        </div>
      )}

      {/* Status bar */}
      <div className="workspace-statusbar flex items-center gap-2 px-3 py-1 border-t text-xs bg-muted/50" data-testid="status-bar">
        <button
          onClick={() => setTerminalVisible(v => !v)}
          className="hover:text-foreground text-muted-foreground"
          data-testid="toggle-terminal"
        >
          {terminalVisible ? 'Hide Terminal' : 'Show Terminal'}
        </button>
        <button
          onClick={() => setRightPanel(v => !v)}
          className="ml-auto hover:text-foreground text-muted-foreground"
          data-testid="toggle-right-panel"
        >
          {rightPanelVisible ? 'Hide Panel' : 'Show Panel'}
        </button>
      </div>
    </div>
  )
}
