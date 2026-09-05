// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { render, screen, fireEvent, cleanup, waitFor, act } from '@testing-library/react'
import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest'
import { Children, createContext, isValidElement, useContext, useMemo } from 'react'
import type {
  ButtonHTMLAttributes,
  InputHTMLAttributes,
  LabelHTMLAttributes,
  ReactElement,
  ReactNode
} from 'react'
import type * as RuntimeRpcClientModule from '../../../runtime/runtime-rpc-client'
import { CreateProjectDialog } from '../CreateProjectDialog'
import { callRuntimeRpc } from '../../../runtime/runtime-rpc-client'
import { useAppStore } from '../../../store'

// The literal mocks below only ever populate `settings`/`repos`/`projects` —
// far short of the real (many-slice) AppState — so this mirrors just the
// shape these tests actually construct, in place of the full store type.
type MockAppState = {
  settings: Record<string, never>
  repos: { path: string; executionHostId: string; displayName: string }[]
  projects: { id: string; displayName: string }[]
}

vi.mock('../../../runtime/runtime-rpc-client', async () => {
  const actual = await vi.importActual<typeof RuntimeRpcClientModule>(
    '../../../runtime/runtime-rpc-client'
  )
  return {
    ...actual,
    callRuntimeRpc: vi.fn(),
    getActiveRuntimeTarget: vi.fn().mockReturnValue({ type: 'local' })
  }
})

vi.mock('../../../store', () => ({
  useAppStore: { getState: vi.fn().mockReturnValue({ settings: {}, repos: [], projects: [] }) }
}))

vi.mock('sonner', () => ({ toast: { error: vi.fn(), success: vi.fn() } }))

vi.mock('../../ui/dialog', () => ({
  Dialog: (p: { open: boolean; children: ReactNode }) =>
    p.open ? <div data-testid="dialog">{p.children}</div> : null,
  DialogContent: (p: { children: ReactNode }) => <div>{p.children}</div>,
  DialogDescription: (p: { children: ReactNode }) => <p>{p.children}</p>,
  DialogFooter: (p: { children: ReactNode }) => <div>{p.children}</div>,
  DialogHeader: (p: { children: ReactNode }) => <div>{p.children}</div>,
  DialogTitle: (p: { children: ReactNode }) => <h2>{p.children}</h2>
}))

// The real Select/SelectTrigger/SelectContent are separate sibling components (Radix
// composition) — flatten them into one native <select> here: pull id/data-testid off the
// SelectTrigger child and the <option> list off the SelectContent child's SelectItem children.
vi.mock('../../ui/select', () => {
  const SelectContent = (p: { children: ReactNode }) => <>{p.children}</>
  const SelectTrigger = (p: { children: ReactNode }) => <>{p.children}</>
  const SelectItem = (p: { value: string; children: ReactNode }) => (
    <option value={p.value}>{p.children}</option>
  )

  const Select = (p: {
    value: string
    onValueChange: (value: string) => void
    children: ReactNode
  }) => {
    const children = Children.toArray(p.children)
    const trigger = children.find((c) => isValidElement(c) && c.type === SelectTrigger) as
      | ReactElement<{ id?: string; 'data-testid'?: string }>
      | undefined
    const content = children.find((c) => isValidElement(c) && c.type === SelectContent) as
      | ReactElement<{ children?: ReactNode }>
      | undefined
    return (
      <select
        id={trigger?.props?.id}
        data-testid={trigger?.props?.['data-testid']}
        value={p.value}
        onChange={(e) => p.onValueChange(e.target.value)}
      >
        {content?.props?.children}
      </select>
    )
  }

  return { Select, SelectContent, SelectItem, SelectTrigger, SelectValue: () => null }
})

vi.mock('../../ui/button', () => ({
  Button: (p: ButtonHTMLAttributes<HTMLButtonElement>) => <button {...p} />
}))
vi.mock('../../ui/input', () => ({
  Input: (p: InputHTMLAttributes<HTMLInputElement>) => <input {...p} />
}))
vi.mock('../../ui/label', () => ({
  Label: (p: LabelHTMLAttributes<HTMLLabelElement>) => <label {...p} />
}))

// Real Radix Tabs doesn't reliably switch active tab under fireEvent.click in
// happy-dom (same reason other Tabs-driven components in this codebase mock
// ui/tabs in tests, e.g. ProfileEditor.test.tsx) — provide a minimal
// controlled mock instead, so TabsContent visibility actually follows value.
vi.mock('../../ui/tabs', () => {
  const TabsCtx = createContext<{ value: string; onValueChange: (v: string) => void }>({
    value: '',
    onValueChange: () => {}
  })
  const Tabs = (p: { value: string; onValueChange: (v: string) => void; children: ReactNode }) => {
    // p.value/p.onValueChange are the props driving this render — memoizing on
    // them keeps the context value referentially stable across unrelated re-renders.
    const ctxValue = useMemo(
      () => ({ value: p.value, onValueChange: p.onValueChange }),
      [p.value, p.onValueChange]
    )
    return (
      <TabsCtx.Provider value={ctxValue}>
        <div>{p.children}</div>
      </TabsCtx.Provider>
    )
  }
  const TabsList = (p: { children: ReactNode }) => <div>{p.children}</div>
  const TabsTrigger = (p: { value: string; children: ReactNode; 'data-testid'?: string }) => {
    const ctx = useContext(TabsCtx)
    return (
      <button
        type="button"
        data-testid={p['data-testid']}
        onClick={() => ctx.onValueChange(p.value)}
      >
        {p.children}
      </button>
    )
  }
  const TabsContent = (p: { value: string; children: ReactNode }) => {
    const ctx = useContext(TabsCtx)
    return ctx.value === p.value ? <div>{p.children}</div> : null
  }
  return { Tabs, TabsList, TabsTrigger, TabsContent }
})

const devServers = [
  { id: 'ds-1', name: 'MacBook Pro M3', status: 'connected' },
  { id: 'ds-2', name: 'Dev Box', status: 'connected' }
]

describe('CreateProjectDialog', () => {
  const onOpenChange = vi.fn()
  const onCreated = vi.fn()

  beforeEach(() => {
    onOpenChange.mockClear()
    onCreated.mockClear()
    vi.mocked(useAppStore.getState).mockReturnValue({
      settings: {},
      repos: [],
      projects: []
    } as MockAppState)
    vi.mocked(callRuntimeRpc).mockImplementation(async (_target, method) => {
      if (method === 'devServer.list') {
        return devServers
      }
      if (method === 'project.list') {
        return []
      }
      if (method === 'project.create') {
        return { id: 'new-p', name: 'New Project' }
      }
      if (method === 'repo.add') {
        return { id: 'repo-1' }
      }
      if (method === 'orcaProjects.linkSourceProject') {
        return { success: true }
      }
      return null
    })
  })

  afterEach(cleanup)

  it('renders nothing when closed', () => {
    render(<CreateProjectDialog open={false} onOpenChange={onOpenChange} onCreated={onCreated} />)
    expect(screen.queryByTestId('dialog')).not.toBeInTheDocument()
  })

  it('fetches devServer.list when opened', async () => {
    await act(async () => {
      render(<CreateProjectDialog open onOpenChange={onOpenChange} onCreated={onCreated} />)
    })
    expect(callRuntimeRpc).toHaveBeenCalledWith(expect.anything(), 'devServer.list', null)
    await waitFor(() => {
      expect(screen.getByText('MacBook Pro M3')).toBeInTheDocument()
    })
  })

  it('submit is disabled until name, dev server, and repo path are filled', async () => {
    await act(async () => {
      render(<CreateProjectDialog open onOpenChange={onOpenChange} onCreated={onCreated} />)
    })
    const submit = screen.getByRole('button', { name: /create project/i })
    expect(submit).toBeDisabled()
  })

  it('calls project.create and onCreated with the new project on submit', async () => {
    await act(async () => {
      render(<CreateProjectDialog open onOpenChange={onOpenChange} onCreated={onCreated} />)
    })

    fireEvent.change(screen.getByLabelText('Name'), { target: { value: 'My Project' } })
    fireEvent.change(screen.getByLabelText('Dev Server'), { target: { value: 'ds-1' } })
    fireEvent.change(screen.getByLabelText('Repo Path'), { target: { value: '/home/user/repo' } })

    const submit = screen.getByRole('button', { name: /create project/i })
    await waitFor(() => expect(submit).not.toBeDisabled())

    await act(async () => {
      fireEvent.click(submit)
    })

    // Why project.create's args have neither devServerId nor repoPath:
    // CreateProjectRequest (project.proto) has no such fields — both used
    // to be sent here and were silently dropped by the wscompat handler.
    expect(callRuntimeRpc).toHaveBeenCalledWith(expect.anything(), 'project.create', {
      name: 'My Project',
      description: undefined,
      visibility: 'private'
    })
    // The repo attaches via a follow-up repo.add call, with the shape the Go
    // handler actually decodes (projectId/url/displayName) — Phase 10 gave
    // AddRepoRequest its own devServerId, so the dev server binds directly on
    // this same call now instead of a separate project.rebindDevServer step.
    expect(callRuntimeRpc).toHaveBeenCalledWith(expect.anything(), 'repo.add', {
      projectId: 'new-p',
      url: '/home/user/repo',
      displayName: 'My Project',
      devServerId: 'ds-1'
    })
    expect(callRuntimeRpc).not.toHaveBeenCalledWith(
      expect.anything(),
      'project.rebindDevServer',
      expect.anything()
    )
    expect(onCreated).toHaveBeenCalledWith({ id: 'new-p', name: 'New Project' })
    expect(onOpenChange).toHaveBeenCalledWith(false)
  })

  it('still creates and closes when the repo.add follow-up fails, but shows a toast', async () => {
    const { toast } = await import('sonner')
    vi.mocked(callRuntimeRpc).mockImplementation(async (_target, method) => {
      if (method === 'devServer.list') {
        return devServers
      }
      if (method === 'project.create') {
        return { id: 'new-p', name: 'New Project' }
      }
      if (method === 'repo.add') {
        throw new Error('repo path already registered')
      }
      return null
    })

    await act(async () => {
      render(<CreateProjectDialog open onOpenChange={onOpenChange} onCreated={onCreated} />)
    })

    fireEvent.change(screen.getByLabelText('Name'), { target: { value: 'My Project' } })
    fireEvent.change(screen.getByLabelText('Dev Server'), { target: { value: 'ds-1' } })
    fireEvent.change(screen.getByLabelText('Repo Path'), { target: { value: '/home/user/repo' } })

    const submit = screen.getByRole('button', { name: /create project/i })
    await waitFor(() => expect(submit).not.toBeDisabled())
    await act(async () => {
      fireEvent.click(submit)
    })

    // The project row already exists at this point — a repo.add failure is
    // "couldn't fully set it up", not "creation failed".
    expect(toast.error).toHaveBeenCalledWith(expect.stringContaining('repo could not be added'))
    expect(onCreated).toHaveBeenCalledWith({ id: 'new-p', name: 'New Project' })
    expect(onOpenChange).toHaveBeenCalledWith(false)
  })

  it('shows an error message and keeps the dialog open when project.create rejects', async () => {
    vi.mocked(callRuntimeRpc).mockImplementation(async (_target, method) => {
      if (method === 'devServer.list') {
        return devServers
      }
      if (method === 'project.create') {
        throw new Error('repo path already registered')
      }
      return null
    })

    await act(async () => {
      render(<CreateProjectDialog open onOpenChange={onOpenChange} onCreated={onCreated} />)
    })

    fireEvent.change(screen.getByLabelText('Name'), { target: { value: 'My Project' } })
    fireEvent.change(screen.getByLabelText('Dev Server'), { target: { value: 'ds-1' } })
    fireEvent.change(screen.getByLabelText('Repo Path'), { target: { value: '/home/user/repo' } })

    const submit = screen.getByRole('button', { name: /create project/i })
    await waitFor(() => expect(submit).not.toBeDisabled())
    await act(async () => {
      fireEvent.click(submit)
    })

    expect(screen.getByText('repo path already registered')).toBeInTheDocument()
    expect(onCreated).not.toHaveBeenCalled()
    expect(onOpenChange).not.toHaveBeenCalledWith(false)
  })

  // BUG-FE-PW-001
  describe('duplicate repo warning', () => {
    it('warns when repoPath + devServerId already exist in the sidebar', async () => {
      vi.mocked(useAppStore.getState).mockReturnValue({
        settings: {},
        repos: [
          { path: '/home/user/repo', executionHostId: 'devServer:ds-1', displayName: 'my-repo' }
        ],
        projects: []
      } as MockAppState)

      await act(async () => {
        render(<CreateProjectDialog open onOpenChange={onOpenChange} onCreated={onCreated} />)
      })
      fireEvent.change(screen.getByLabelText('Dev Server'), { target: { value: 'ds-1' } })
      fireEvent.change(screen.getByLabelText('Repo Path'), { target: { value: '/home/user/repo' } })

      expect(await screen.findByTestId('cp-duplicate-repo-warning')).toHaveTextContent('my-repo')
    })

    it('does not warn when the path matches but the dev server differs', async () => {
      vi.mocked(useAppStore.getState).mockReturnValue({
        settings: {},
        repos: [
          { path: '/home/user/repo', executionHostId: 'devServer:ds-2', displayName: 'my-repo' }
        ],
        projects: []
      } as MockAppState)

      await act(async () => {
        render(<CreateProjectDialog open onOpenChange={onOpenChange} onCreated={onCreated} />)
      })
      fireEvent.change(screen.getByLabelText('Dev Server'), { target: { value: 'ds-1' } })
      fireEvent.change(screen.getByLabelText('Repo Path'), { target: { value: '/home/user/repo' } })

      expect(screen.queryByTestId('cp-duplicate-repo-warning')).not.toBeInTheDocument()
    })

    it('does not warn, and submits normally, when there is no matching repo', async () => {
      await act(async () => {
        render(<CreateProjectDialog open onOpenChange={onOpenChange} onCreated={onCreated} />)
      })
      fireEvent.change(screen.getByLabelText('Name'), { target: { value: 'My Project' } })
      fireEvent.change(screen.getByLabelText('Dev Server'), { target: { value: 'ds-1' } })
      fireEvent.change(screen.getByLabelText('Repo Path'), { target: { value: '/home/user/repo' } })

      expect(screen.queryByTestId('cp-duplicate-repo-warning')).not.toBeInTheDocument()
      const submit = screen.getByRole('button', { name: /create project/i })
      await waitFor(() => expect(submit).not.toBeDisabled())
    })
  })

  // BUG-FE-PW-002 (dialog-side wiring)
  describe('link existing project mode', () => {
    it('submit is disabled in link mode until name and a project are chosen', async () => {
      // The link picker must list real OrcaProjects (project.list, real
      // project.projects UUIDs) — NOT useAppStore's `projects` (the
      // client-only Project Host Setup projection). See DialogMode's doc
      // comment for why: sending that projection's id as linkSourceProject's
      // projectId makes the backend's UUID-column lookup throw instead of
      // cleanly denying — confirmed live on b15.openledger.vn.
      vi.mocked(callRuntimeRpc).mockImplementation(async (_target, method) => {
        if (method === 'devServer.list') {
          return devServers
        }
        if (method === 'project.list') {
          return [{ id: 'p1', name: 'Backend' }]
        }
        return null
      })

      await act(async () => {
        render(<CreateProjectDialog open onOpenChange={onOpenChange} onCreated={onCreated} />)
      })
      fireEvent.click(screen.getByTestId('cp-mode-link'))

      const submit = screen.getByRole('button', { name: /create project/i })
      expect(submit).toBeDisabled()

      fireEvent.change(screen.getByLabelText('Name'), { target: { value: 'Shared Backend' } })
      expect(submit).toBeDisabled() // still no project chosen

      await waitFor(() => expect(screen.getByText('Backend')).toBeInTheDocument())
      fireEvent.change(screen.getByTestId('cp-link-project-select'), { target: { value: 'p1' } })
      await waitFor(() => expect(submit).not.toBeDisabled())
    })

    it('link mode calls project.create then orcaProjects.linkSourceProject — not repo.add/rebindDevServer', async () => {
      // callRuntimeRpc's call history isn't cleared between tests in this
      // file (only onOpenChange/onCreated are) — clear it here so the
      // "not called with repo.add/rebindDevServer" assertions below aren't
      // polluted by earlier 'new-repo' mode tests in this same file.
      vi.mocked(callRuntimeRpc).mockClear()
      vi.mocked(callRuntimeRpc).mockImplementation(async (_target, method) => {
        if (method === 'devServer.list') {
          return devServers
        }
        if (method === 'project.list') {
          return [{ id: 'p1', name: 'Backend' }]
        }
        if (method === 'project.create') {
          return { id: 'new-p', name: 'New Project' }
        }
        if (method === 'orcaProjects.linkSourceProject') {
          return { success: true }
        }
        return null
      })

      await act(async () => {
        render(<CreateProjectDialog open onOpenChange={onOpenChange} onCreated={onCreated} />)
      })
      fireEvent.click(screen.getByTestId('cp-mode-link'))
      fireEvent.change(screen.getByLabelText('Name'), { target: { value: 'Shared Backend' } })
      await waitFor(() => expect(screen.getByText('Backend')).toBeInTheDocument())
      fireEvent.change(screen.getByTestId('cp-link-project-select'), { target: { value: 'p1' } })

      const submit = screen.getByRole('button', { name: /create project/i })
      await waitFor(() => expect(submit).not.toBeDisabled())
      await act(async () => {
        fireEvent.click(submit)
      })

      expect(callRuntimeRpc).toHaveBeenCalledWith(expect.anything(), 'project.create', {
        name: 'Shared Backend',
        description: undefined,
        visibility: 'private'
      })
      expect(callRuntimeRpc).toHaveBeenCalledWith(
        expect.anything(),
        'orcaProjects.linkSourceProject',
        {
          orcaProjectId: 'new-p',
          projectId: 'p1'
        }
      )
      expect(callRuntimeRpc).not.toHaveBeenCalledWith(
        expect.anything(),
        'repo.add',
        expect.anything()
      )
      expect(callRuntimeRpc).not.toHaveBeenCalledWith(
        expect.anything(),
        'project.rebindDevServer',
        expect.anything()
      )
      expect(onCreated).toHaveBeenCalledWith({ id: 'new-p', name: 'New Project' })
      expect(onOpenChange).toHaveBeenCalledWith(false)
    })

    it('shows an error and keeps the dialog open when linkSourceProject rejects', async () => {
      vi.mocked(callRuntimeRpc).mockImplementation(async (_target, method) => {
        if (method === 'devServer.list') {
          return devServers
        }
        if (method === 'project.list') {
          return [{ id: 'p1', name: 'Backend' }]
        }
        if (method === 'project.create') {
          return { id: 'new-p', name: 'New Project' }
        }
        if (method === 'orcaProjects.linkSourceProject') {
          throw new Error('FORBIDDEN: not owner')
        }
        return null
      })

      await act(async () => {
        render(<CreateProjectDialog open onOpenChange={onOpenChange} onCreated={onCreated} />)
      })
      fireEvent.click(screen.getByTestId('cp-mode-link'))
      fireEvent.change(screen.getByLabelText('Name'), { target: { value: 'Shared Backend' } })
      await waitFor(() => expect(screen.getByText('Backend')).toBeInTheDocument())
      fireEvent.change(screen.getByTestId('cp-link-project-select'), { target: { value: 'p1' } })

      const submit = screen.getByRole('button', { name: /create project/i })
      await waitFor(() => expect(submit).not.toBeDisabled())
      await act(async () => {
        fireEvent.click(submit)
      })

      expect(
        screen.getByText('You do not have permission to create a project.')
      ).toBeInTheDocument()
      expect(onCreated).not.toHaveBeenCalled()
      expect(onOpenChange).not.toHaveBeenCalledWith(false)
    })
  })
})
