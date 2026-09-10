// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { render, screen, fireEvent, cleanup, waitFor, act } from '@testing-library/react'
import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest'
import type { ButtonHTMLAttributes, ReactNode } from 'react'
import { ProjectSwitcher } from '../ProjectSwitcher'
import { useWorkspace } from '../../../context/WorkspaceContext'
import { callRuntimeRpc } from '../../../runtime/runtime-rpc-client'
import type { OrcaProject } from '../../../types/workspace-types'

// Mocked useWorkspace() only ever returns this narrow slice (never the full
// WorkspaceContext value, and not always every field of it), so this is what
// the mock's return type reflects.
type MockWorkspace = {
  project?: Pick<OrcaProject, 'id' | 'name'> | null
  switchProject?: (projectId: string) => Promise<void>
  isInitializing?: boolean
}

const toastMocks = vi.hoisted(() => ({ error: vi.fn() }))
vi.mock('sonner', () => ({ toast: { error: toastMocks.error } }))

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
  Popover: (p: { children: ReactNode }) => <div data-testid="popover" {...p} />,
  PopoverTrigger: (p: {
    open?: boolean
    onOpenChange?: (open: boolean) => void
    children: ReactNode
  }) => (
    <div data-testid="popover-trigger" onClick={() => p.onOpenChange?.(!p.open)}>
      {p.children}
    </div>
  ),
  PopoverContent: (p: { children: ReactNode }) => (
    <div data-testid="popover-content">{p.children}</div>
  )
}))

vi.mock('../../ui/command', () => ({
  Command: (p: { children: ReactNode }) => <div data-testid="command">{p.children}</div>,
  CommandInput: (p: { value: string; onValueChange: (value: string) => void }) => (
    <input
      data-testid="command-input"
      value={p.value}
      onChange={(e) => p.onValueChange(e.target.value)}
    />
  ),
  CommandList: (p: { children: ReactNode }) => <div data-testid="command-list">{p.children}</div>,
  CommandEmpty: (p: { children: ReactNode }) => <div data-testid="command-empty">{p.children}</div>,
  CommandGroup: (p: { children: ReactNode }) => <div data-testid="command-group">{p.children}</div>,
  CommandItem: (p: { children: ReactNode; onSelect?: () => void; 'data-testid'?: string }) => (
    <div data-testid={p['data-testid'] || 'command-item'} onClick={p.onSelect}>
      {p.children}
    </div>
  ),
  CommandSeparator: () => <hr />
}))

vi.mock('../../ui/button', () => ({
  Button: (p: ButtonHTMLAttributes<HTMLButtonElement>) => <button {...p} />
}))

vi.mock('../CreateProjectDialog', () => ({
  CreateProjectDialog: (p: {
    open: boolean
    onCreated: (project: { id: string; name: string; devServerId: string }) => void
  }) =>
    p.open ? (
      <div data-testid="create-project-dialog">
        <button
          data-testid="fire-on-created"
          onClick={() => p.onCreated({ id: 'p3', name: 'New Project', devServerId: 'local' })}
        />
      </div>
    ) : null
}))

// ProjectSettings has its own test file — shallow-mock it here so this
// file's tests aren't coupled to its internals (dialog/tabs/MemberManager).
vi.mock('../ProjectSettings', () => ({
  ProjectSettings: (p: { open: boolean }) =>
    p.open ? <div data-testid="project-settings-dialog-mock" /> : null
}))

describe('ProjectSwitcher', () => {
  const switchProject = vi.fn()

  beforeEach(() => {
    switchProject.mockClear()
    vi.mocked(useWorkspace).mockReturnValue({
      project: { id: 'p1', name: 'Test Project 1' },
      switchProject,
      isInitializing: false
    } as MockWorkspace)
    vi.mocked(callRuntimeRpc).mockImplementation((_target, channel: string) => {
      if (channel === 'devServer.list') {
        return Promise.resolve([{ id: 'remote', name: 'dev-01' }])
      }
      return Promise.resolve([
        { id: 'p1', name: 'Test Project 1', devServerId: 'local' },
        { id: 'p2', name: 'Test Project 2', devServerId: 'remote' }
      ])
    })
  })

  afterEach(cleanup)

  it('fetches the project list via project.list RPC on mount', async () => {
    await act(async () => {
      render(<ProjectSwitcher />)
    })
    expect(callRuntimeRpc).toHaveBeenCalledWith(expect.anything(), 'project.list', null)
  })

  it('renders current project name from context', () => {
    render(<ProjectSwitcher />)
    expect(screen.getByTestId('popover-trigger')).toHaveTextContent('Test Project 1')
  })

  it('opens dropdown with full projects list', async () => {
    await act(async () => {
      render(<ProjectSwitcher />)
    })
    expect(screen.getByTestId('command-list')).toHaveTextContent('Test Project 2')
  })

  // Regression guard: the list used to render p.devServerId (a raw uuid)
  // verbatim next to each project name — resolved to devServer.list's own
  // human label instead.
  it("resolves devServerId to the dev server's name via devServer.list", async () => {
    await act(async () => {
      render(<ProjectSwitcher />)
    })
    await waitFor(() => {
      expect(screen.getByTestId('command-list')).toHaveTextContent('dev-01')
    })
    expect(screen.getByTestId('command-list')).not.toHaveTextContent('remote')
  })

  it('calls switchProject(id) when project item clicked', async () => {
    await act(async () => {
      render(<ProjectSwitcher />)
    })
    const items = screen.getAllByTestId('command-item')
    fireEvent.click(items[1])
    expect(switchProject).toHaveBeenCalledWith('p2')
  })

  it('shows loading spinner (isInitializing=true)', () => {
    vi.mocked(useWorkspace).mockReturnValue({ isInitializing: true } as MockWorkspace)
    render(<ProjectSwitcher />)
    expect(screen.getByTestId('popover-trigger')).not.toHaveTextContent('Test Project 1')
    expect(document.querySelector('.animate-spin')).toBeInTheDocument()
  })

  it('search input filters project list by name', async () => {
    await act(async () => {
      render(<ProjectSwitcher />)
    })
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
    await act(async () => {
      render(<ProjectSwitcher />)
    })
    expect(screen.getByTestId('command-empty')).toBeInTheDocument()
  })

  it('"Create New Project" opens CreateProjectDialog and closes the popover', async () => {
    await act(async () => {
      render(<ProjectSwitcher />)
    })
    expect(screen.queryByTestId('create-project-dialog')).not.toBeInTheDocument()

    fireEvent.click(screen.getByTestId('create-project-item'))

    expect(screen.getByTestId('create-project-dialog')).toBeInTheDocument()
  })

  it('switches to the newly created project on onCreated', async () => {
    switchProject.mockResolvedValue(undefined)
    await act(async () => {
      render(<ProjectSwitcher />)
    })
    fireEvent.click(screen.getByTestId('create-project-item'))

    await act(async () => {
      fireEvent.click(screen.getByTestId('fire-on-created'))
    })

    expect(switchProject).toHaveBeenCalledWith('p3')
    expect(toastMocks.error).not.toHaveBeenCalled()
  })

  // Regression guard: onCreated used to `void switchProject(created.id)` with
  // no catch — a rejection (e.g. the live PROJECT_MEMBERSHIP_LOOKUP_FAILED
  // bug) surfaced as a raw unhandled promise rejection instead of a
  // user-visible message.
  it('shows an error toast, not an unhandled rejection, when switching to the new project fails', async () => {
    switchProject.mockRejectedValue(new Error('PROJECT_NOT_AUTHORIZED'))
    await act(async () => {
      render(<ProjectSwitcher />)
    })
    fireEvent.click(screen.getByTestId('create-project-item'))

    await act(async () => {
      fireEvent.click(screen.getByTestId('fire-on-created'))
    })
    await waitFor(() => {
      expect(toastMocks.error).toHaveBeenCalledWith(expect.stringContaining('could not switch'))
    })
  })

  // Regression guard: ProjectSettings/MemberManager were fully built and
  // wired to real RPCs, but nothing in the app ever rendered a trigger to
  // open them — this button is that trigger.
  it('settings-gear trigger opens ProjectSettings for the current project', () => {
    render(<ProjectSwitcher />)
    expect(screen.queryByTestId('project-settings-dialog-mock')).not.toBeInTheDocument()

    fireEvent.click(screen.getByTestId('project-settings-trigger'))

    expect(screen.getByTestId('project-settings-dialog-mock')).toBeInTheDocument()
  })

  it('settings-gear trigger is disabled when no project is selected', () => {
    vi.mocked(useWorkspace).mockReturnValue({
      project: null,
      switchProject,
      isInitializing: false
    } as MockWorkspace)
    render(<ProjectSwitcher />)
    expect(screen.getByTestId('project-settings-trigger')).toBeDisabled()
  })
})
