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
      <button data-testid="tab-agent" onClick={() => onTabChange('agent')}>Agent</button>
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
vi.mock('../AgentPanel', () => ({
  AgentPanel: ({ worktreeId }: { worktreeId: string }) => (
    <div data-testid="agent-panel" data-worktree-id={worktreeId} />
  )
}))
vi.mock('../WorkspaceTerminalPanel', () => ({
  WorkspaceTerminalPanel: ({ worktreeId }: { worktreeId: string }) => (
    <div data-testid="workspace-terminal-panel" data-worktree-id={worktreeId} />
  )
}))
vi.mock('../../status-bar/SshStatusSegment', () => ({
  SshStatusSegment: () => <div data-testid="ssh-status-segment" />
}))

describe('WorkspaceLayout', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(useWorkspace).mockReturnValue({
      project: { id: 'p1' },
      isOffline: false,
      isInitializing: false,
      currentWorktree: null,
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

  it('"agent" tab with no currentWorktree → shows NoWorktreeSelected empty state', async () => {
    render(<WorkspaceLayout />)
    fireEvent.click(screen.getByTestId('tab-agent'))
    await waitFor(() => {
      expect(screen.getByTestId('no-worktree-selected')).toBeInTheDocument()
    })
    expect(screen.queryByTestId('agent-panel')).not.toBeInTheDocument()
  })

  it('"agent" tab with currentWorktree → AgentPanel renders with worktreeId', async () => {
    vi.mocked(useWorkspace).mockReturnValue({
      project: { id: 'p1' },
      isOffline: false,
      isInitializing: false,
      currentWorktree: { id: 'wt-1', path: '/tmp/wt-1', branch: 'main', isMain: true },
      switchProject: vi.fn(),
      on: vi.fn().mockReturnValue(() => {}),
      emit: vi.fn()
    } as unknown as ReturnType<typeof useWorkspace>)
    render(<WorkspaceLayout />)
    fireEvent.click(screen.getByTestId('tab-agent'))
    await waitFor(() => {
      expect(screen.getByTestId('agent-panel')).toHaveAttribute('data-worktree-id', 'wt-1')
    })
  })

  it('terminal panel with no currentWorktree → prompts to select a worktree, no PTY mount', () => {
    render(<WorkspaceLayout />)
    fireEvent.click(screen.getByTestId('toggle-terminal'))
    expect(screen.getByTestId('terminal-panel')).toHaveTextContent('Select a worktree')
    expect(screen.queryByTestId('workspace-terminal-panel')).not.toBeInTheDocument()
  })

  it('terminal panel with currentWorktree → mounts WorkspaceTerminalPanel with worktreeId', () => {
    vi.mocked(useWorkspace).mockReturnValue({
      project: { id: 'p1' },
      isOffline: false,
      isInitializing: false,
      currentWorktree: { id: 'wt-1', path: '/tmp/wt-1', branch: 'main', isMain: true },
      switchProject: vi.fn(),
      on: vi.fn().mockReturnValue(() => {}),
      emit: vi.fn()
    } as unknown as ReturnType<typeof useWorkspace>)
    render(<WorkspaceLayout />)
    fireEvent.click(screen.getByTestId('toggle-terminal'))
    expect(screen.getByTestId('workspace-terminal-panel')).toHaveAttribute('data-worktree-id', 'wt-1')
  })
})
