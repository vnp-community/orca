// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { render, screen, waitFor, fireEvent, cleanup } from '@testing-library/react'
import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest'
import { Children, isValidElement } from 'react'
import type { ReactElement, ReactNode } from 'react'
import { ProjectDevServerSection } from '../ProjectDevServerSection'
import { callRuntimeRpc } from '../../../runtime/runtime-rpc-client'

vi.mock('../../../runtime/runtime-rpc-client', () => ({
  callRuntimeRpc: vi.fn(),
  getActiveRuntimeTarget: vi.fn().mockReturnValue({ type: 'local' }),
  RuntimeRpcCallError: class RuntimeRpcCallError extends Error {}
}))

vi.mock('../../../store', () => ({
  useAppStore: { getState: vi.fn().mockReturnValue({ settings: {} }) }
}))

vi.mock('sonner', () => ({ toast: { success: vi.fn(), error: vi.fn() } }))

// Same flatten-Radix-Select-into-a-native-<select> mock as
// CreateProjectDialog.test.tsx — see that file's own comment for why.
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
      | ReactElement<{ 'data-testid'?: string }>
      | undefined
    const content = children.find((c) => isValidElement(c) && c.type === SelectContent) as
      | ReactElement<{ children?: ReactNode }>
      | undefined
    return (
      <select
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

const mockRpc = vi.mocked(callRuntimeRpc)

const oneRepo = [
  {
    id: 'r1',
    projectId: 'p1',
    url: 'https://x',
    displayName: 'my-repo',
    position: 0,
    devServerId: 'ds-1'
  }
]

describe('ProjectDevServerSection', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockRpc.mockImplementation((_target, method) => {
      if (method === 'devServer.list') {
        return Promise.resolve([
          { id: 'ds-1', name: 'dev-01', status: 'healthy' },
          { id: 'ds-2', name: 'dev-ai', status: 'healthy' }
        ])
      }
      if (method === 'repo.list') {
        return Promise.resolve({ repos: [] })
      }
      return Promise.resolve(undefined)
    })
  })

  afterEach(cleanup)

  it('shows a no-repos note when the project has no repos yet', async () => {
    render(<ProjectDevServerSection projectId="p1" />)

    await waitFor(() => {
      expect(mockRpc).toHaveBeenCalledWith({ type: 'local' }, 'repo.list', { projectId: 'p1' })
    })
    expect(
      screen.getByText('Add a repo from the Repos tab to set its dev server.')
    ).toBeInTheDocument()
  })

  it('lists a per-repo dev-server select via repo.list, preselecting the repo’s current binding', async () => {
    mockRpc.mockImplementation((_target, method) => {
      if (method === 'devServer.list') {
        return Promise.resolve([
          { id: 'ds-1', name: 'dev-01', status: 'healthy' },
          { id: 'ds-2', name: 'dev-ai', status: 'healthy' }
        ])
      }
      if (method === 'repo.list') {
        return Promise.resolve({ repos: oneRepo })
      }
      return Promise.resolve(undefined)
    })

    render(<ProjectDevServerSection projectId="p1" />)

    await waitFor(() => {
      expect(screen.getByTestId('repo-dev-server-select-r1')).toHaveValue('ds-1')
    })
    // Save starts disabled since the picker already matches the current value.
    expect(screen.getByTestId('repo-dev-server-save-r1')).toBeDisabled()
  })

  it('calls repo.rebindDevServer with the picked dev server and refreshes the repo list', async () => {
    mockRpc.mockImplementation((_target, method) => {
      if (method === 'devServer.list') {
        return Promise.resolve([
          { id: 'ds-1', name: 'dev-01', status: 'healthy' },
          { id: 'ds-2', name: 'dev-ai', status: 'healthy' }
        ])
      }
      if (method === 'repo.list') {
        return Promise.resolve({ repos: oneRepo })
      }
      return Promise.resolve(undefined)
    })

    render(<ProjectDevServerSection projectId="p1" />)
    await waitFor(() => {
      expect(screen.getByTestId('repo-dev-server-select-r1')).toHaveValue('ds-1')
    })

    fireEvent.change(screen.getByTestId('repo-dev-server-select-r1'), {
      target: { value: 'ds-2' }
    })
    expect(screen.getByTestId('repo-dev-server-save-r1')).not.toBeDisabled()
    fireEvent.click(screen.getByTestId('repo-dev-server-save-r1'))

    await waitFor(() => {
      expect(mockRpc).toHaveBeenCalledWith({ type: 'local' }, 'repo.rebindDevServer', {
        repoId: 'r1',
        newDevServerId: 'ds-2'
      })
    })
  })
})
