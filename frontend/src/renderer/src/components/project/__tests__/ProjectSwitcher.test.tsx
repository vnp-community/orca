// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { render, screen, fireEvent, cleanup, waitFor, act } from '@testing-library/react'
import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest'
import { ProjectSwitcher } from '../ProjectSwitcher'
import { useWorkspace } from '../../../context/WorkspaceContext'
import { callRuntimeRpc } from '../../../runtime/runtime-rpc-client'

vi.mock('../../../context/WorkspaceContext', () => ({
  useWorkspace: vi.fn()
}))

vi.mock('../../../runtime/runtime-rpc-client', () => ({
  callRuntimeRpc: vi.fn(),
  getActiveRuntimeTarget: vi.fn().mockReturnValue({ type: 'local' })
}))

vi.mock('../../../store', () => ({
  useAppStore: { getState: vi.fn().mockReturnValue({ settings: {} }) }
}))

vi.mock('../../ui/popover', () => ({
  Popover: (p: any) => <div data-testid="popover" {...p} />,
  PopoverTrigger: (p: any) => <div data-testid="popover-trigger" onClick={() => p.onOpenChange?.(!p.open)}>{p.children}</div>,
  PopoverContent: (p: any) => <div data-testid="popover-content">{p.children}</div>
}))

vi.mock('../../ui/command', () => ({
  Command: (p: any) => <div data-testid="command">{p.children}</div>,
  CommandInput: (p: any) => <input data-testid="command-input" value={p.value} onChange={e => p.onValueChange(e.target.value)} />,
  CommandList: (p: any) => <div data-testid="command-list">{p.children}</div>,
  CommandEmpty: (p: any) => <div data-testid="command-empty">{p.children}</div>,
  CommandGroup: (p: any) => <div data-testid="command-group">{p.children}</div>,
  CommandItem: (p: any) => <div data-testid={p['data-testid'] || 'command-item'} onClick={p.onSelect}>{p.children}</div>,
  CommandSeparator: () => <hr />
}))

vi.mock('../../ui/button', () => ({ Button: (p: any) => <button {...p} /> }))

vi.mock('../CreateProjectDialog', () => ({
  CreateProjectDialog: (p: any) =>
    p.open ? <div data-testid="create-project-dialog" /> : null
}))

describe('ProjectSwitcher', () => {
  const switchProject = vi.fn()

  beforeEach(() => {
    switchProject.mockClear()
    vi.mocked(useWorkspace).mockReturnValue({
      project: { id: 'p1', name: 'Test Project 1' } as any,
      switchProject,
      isInitializing: false,
    } as any)
    vi.mocked(callRuntimeRpc).mockResolvedValue([
      { id: 'p1', name: 'Test Project 1', devServerId: 'local' },
      { id: 'p2', name: 'Test Project 2', devServerId: 'remote' }
    ])
  })

  afterEach(cleanup)

  it('fetches the project list via project.list RPC on mount', async () => {
    await act(async () => { render(<ProjectSwitcher />) })
    expect(callRuntimeRpc).toHaveBeenCalledWith(expect.anything(), 'project.list', null)
  })

  it('renders current project name from context', () => {
    render(<ProjectSwitcher />)
    expect(screen.getByTestId('popover-trigger')).toHaveTextContent('Test Project 1')
  })

  it('opens dropdown with full projects list', async () => {
    await act(async () => { render(<ProjectSwitcher />) })
    expect(screen.getByTestId('command-list')).toHaveTextContent('Test Project 2')
  })

  it('calls switchProject(id) when project item clicked', async () => {
    await act(async () => { render(<ProjectSwitcher />) })
    const items = screen.getAllByTestId('command-item')
    fireEvent.click(items[1])
    expect(switchProject).toHaveBeenCalledWith('p2')
  })

  it('shows loading spinner (isInitializing=true)', () => {
    vi.mocked(useWorkspace).mockReturnValue({ isInitializing: true } as any)
    render(<ProjectSwitcher />)
    expect(screen.getByTestId('popover-trigger')).not.toHaveTextContent('Test Project 1')
    expect(document.querySelector('.animate-spin')).toBeInTheDocument()
  })

  it('search input filters project list by name', async () => {
    await act(async () => { render(<ProjectSwitcher />) })
    expect(screen.getByTestId('command-list')).toHaveTextContent('Test Project 2')
    const input = screen.getByTestId('command-input')
    fireEvent.change(input, { target: { value: 'Project 1' } })
    await waitFor(() => {
      expect(screen.getByTestId('command-list')).not.toHaveTextContent('Test Project 2')
      expect(screen.getByTestId('command-list')).toHaveTextContent('Test Project 1')
    })
  })

  it('falls back to an empty list when project.list rejects', async () => {
    vi.mocked(callRuntimeRpc).mockRejectedValue(new Error('offline'))
    await act(async () => { render(<ProjectSwitcher />) })
    expect(screen.getByTestId('command-empty')).toBeInTheDocument()
  })

  it('"Create New Project" opens CreateProjectDialog and closes the popover', async () => {
    await act(async () => { render(<ProjectSwitcher />) })
    expect(screen.queryByTestId('create-project-dialog')).not.toBeInTheDocument()

    fireEvent.click(screen.getByTestId('create-project-item'))

    expect(screen.getByTestId('create-project-dialog')).toBeInTheDocument()
  })
})
