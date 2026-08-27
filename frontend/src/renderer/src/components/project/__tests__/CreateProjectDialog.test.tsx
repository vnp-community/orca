// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { render, screen, fireEvent, cleanup, waitFor, act } from '@testing-library/react'
import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest'
import { Children, isValidElement } from 'react'
import { CreateProjectDialog } from '../CreateProjectDialog'
import { callRuntimeRpc } from '../../../runtime/runtime-rpc-client'

vi.mock('../../../runtime/runtime-rpc-client', async () => {
  const actual = await vi.importActual<typeof import('../../../runtime/runtime-rpc-client')>(
    '../../../runtime/runtime-rpc-client'
  )
  return {
    ...actual,
    callRuntimeRpc: vi.fn(),
    getActiveRuntimeTarget: vi.fn().mockReturnValue({ type: 'local' })
  }
})

vi.mock('../../../store', () => ({
  useAppStore: { getState: vi.fn().mockReturnValue({ settings: {} }) }
}))

vi.mock('../../ui/dialog', () => ({
  Dialog: (p: any) => (p.open ? <div data-testid="dialog">{p.children}</div> : null),
  DialogContent: (p: any) => <div>{p.children}</div>,
  DialogDescription: (p: any) => <p>{p.children}</p>,
  DialogFooter: (p: any) => <div>{p.children}</div>,
  DialogHeader: (p: any) => <div>{p.children}</div>,
  DialogTitle: (p: any) => <h2>{p.children}</h2>
}))

// The real Select/SelectTrigger/SelectContent are separate sibling components (Radix
// composition) — flatten them into one native <select> here: pull id/data-testid off the
// SelectTrigger child and the <option> list off the SelectContent child's SelectItem children.
vi.mock('../../ui/select', () => {
  const SelectContent = (p: any) => <>{p.children}</>
  const SelectTrigger = (p: any) => <>{p.children}</>
  const SelectItem = (p: any) => <option value={p.value}>{p.children}</option>

  const Select = (p: any) => {
    const children = Children.toArray(p.children)
    const trigger = children.find(c => isValidElement(c) && c.type === SelectTrigger) as any
    const content = children.find(c => isValidElement(c) && c.type === SelectContent) as any
    return (
      <select
        id={trigger?.props?.id}
        data-testid={trigger?.props?.['data-testid']}
        value={p.value}
        onChange={e => p.onValueChange(e.target.value)}
      >
        {content?.props?.children}
      </select>
    )
  }

  return { Select, SelectContent, SelectItem, SelectTrigger, SelectValue: () => null }
})

vi.mock('../../ui/button', () => ({ Button: (p: any) => <button {...p} /> }))
vi.mock('../../ui/input', () => ({ Input: (p: any) => <input {...p} /> }))
vi.mock('../../ui/label', () => ({ Label: (p: any) => <label {...p} /> }))

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
    vi.mocked(callRuntimeRpc).mockImplementation(async (_target, method) => {
      if (method === 'devServer.list') {return devServers}
      if (method === 'project.create') {return { id: 'new-p', name: 'New Project', devServerId: 'ds-1' }}
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

    await act(async () => { fireEvent.click(submit) })

    expect(callRuntimeRpc).toHaveBeenCalledWith(expect.anything(), 'project.create', {
      name: 'My Project',
      description: undefined,
      devServerId: 'ds-1',
      repoPath: '/home/user/repo',
      visibility: 'private'
    })
    expect(onCreated).toHaveBeenCalledWith({ id: 'new-p', name: 'New Project', devServerId: 'ds-1' })
    expect(onOpenChange).toHaveBeenCalledWith(false)
  })

  it('shows an error message and keeps the dialog open when project.create rejects', async () => {
    vi.mocked(callRuntimeRpc).mockImplementation(async (_target, method) => {
      if (method === 'devServer.list') {return devServers}
      if (method === 'project.create') {throw new Error('repo path already registered')}
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
    await act(async () => { fireEvent.click(submit) })

    expect(screen.getByText('repo path already registered')).toBeInTheDocument()
    expect(onCreated).not.toHaveBeenCalled()
    expect(onOpenChange).not.toHaveBeenCalledWith(false)
  })
})
