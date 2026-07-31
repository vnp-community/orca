// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { render, screen, fireEvent, cleanup, waitFor } from '@testing-library/react'
import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest'
import { WorkspaceLayout } from '../WorkspaceLayout'
import { useWorkspace } from '../../../context/WorkspaceContext'

vi.mock('../../../context/WorkspaceContext', () => ({
  useWorkspace: vi.fn()
}))

vi.mock('../WorkspaceTabBar', () => ({
  WorkspaceTabBar: ({ activeTab, onTabChange }: any) => (
    <div data-testid="workspace-tab-bar">
      <span>{activeTab}</span>
      <button data-testid="tab-tasks" onClick={() => onTabChange('tasks')}>Tasks</button>
      <button data-testid="tab-git" onClick={() => onTabChange('git')}>Git</button>
    </div>
  )
}))
vi.mock('../OfflineBanner', () => ({ OfflineBanner: () => <div data-testid="offline-banner" /> }))
vi.mock('../NoProjectSelected', () => ({ NoProjectSelected: () => <div data-testid="no-project" /> }))
vi.mock('../WorkspaceSkeletonLoader', () => ({ WorkspaceSkeletonLoader: () => <div data-testid="skeleton-loader" /> }))

vi.mock('../../ui/resizable', () => ({
  ResizablePanelGroup: (p: any) => <div data-testid="resizable-group">{p.children}</div>,
  ResizablePanel: (p: any) => <div data-testid={p['data-testid'] || 'resizable-panel'}>{p.children}</div>,
  ResizableHandle: () => <div />
}))

// Mock lazy loaded panels
vi.mock('../ExplorerPanel', () => ({ ExplorerPanel: () => <div data-testid="explorer-panel" /> }))
vi.mock('../git/GitPanel', () => ({ GitPanel: () => <div data-testid="git-panel" /> }))
vi.mock('../../task/TaskGraphPanel', () => ({ TaskGraphPanel: () => <div data-testid="task-graph-panel" /> }))
vi.mock('../../workflow/WorkflowMonitor', () => ({ WorkflowMonitor: () => <div data-testid="workflow-monitor" /> }))

describe('WorkspaceLayout', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(useWorkspace).mockReturnValue({
      project: { id: 'p1' },
      isOffline: false,
      isInitializing: false,
      switchProject: vi.fn(),
      on: vi.fn().mockReturnValue(() => {}),
      emit: vi.fn()
    } as any)
  })

  afterEach(cleanup)

  it('renders NoProjectSelected when project=null', () => {
    vi.mocked(useWorkspace).mockReturnValue({ project: null } as any)
    render(<WorkspaceLayout />)
    expect(screen.getByTestId('no-project')).toBeInTheDocument()
  })

  it('renders WorkspaceSkeletonLoader when isInitializing=true', () => {
    vi.mocked(useWorkspace).mockReturnValue({ project: { id: 'p1' }, isInitializing: true } as any)
    render(<WorkspaceLayout />)
    expect(screen.getByTestId('skeleton-loader')).toBeInTheDocument()
  })

  it('renders OfflineBanner when isOffline=true', () => {
    vi.mocked(useWorkspace).mockReturnValue({ project: { id: 'p1' }, isOffline: true, isInitializing: false } as any)
    render(<WorkspaceLayout />)
    expect(screen.getByTestId('offline-banner')).toBeInTheDocument()
  })

  it('"git" tab active → GitPanel renders', async () => {
    render(<WorkspaceLayout />)
    await waitFor(() => {
      expect(screen.getByTestId('git-panel')).toBeInTheDocument()
    })
  })

  it('"tasks" tab click → shows task content', async () => {
    render(<WorkspaceLayout />)
    fireEvent.click(screen.getByTestId('tab-tasks'))
    await waitFor(() => {
      expect(screen.getByTestId('task-graph-panel')).toBeInTheDocument()
    })
  })

  it('"Show Terminal" toggle shows terminal panel', () => {
    render(<WorkspaceLayout />)
    expect(screen.queryByTestId('terminal-panel')).not.toBeInTheDocument()
    fireEvent.click(screen.getByTestId('toggle-terminal'))
    expect(screen.getByTestId('terminal-panel')).toBeInTheDocument()
  })
})
