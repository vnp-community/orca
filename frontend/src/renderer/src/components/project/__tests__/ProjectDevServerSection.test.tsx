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

const { switchProject, workspaceProject } = vi.hoisted(() => ({
  switchProject: vi.fn().mockResolvedValue(undefined),
  workspaceProject: { id: 'p1', devServerId: 'ds-1' }
}))

vi.mock('../../../context/WorkspaceContext', () => ({
  useWorkspace: vi.fn().mockReturnValue({
    project: workspaceProject,
    switchProject
  })
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

describe('ProjectDevServerSection', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    switchProject.mockResolvedValue(undefined)
    mockRpc.mockImplementation((_target, method) => {
      if (method === 'devServer.list') {
        return Promise.resolve([
          { id: 'ds-1', name: 'dev-01', status: 'healthy' },
          { id: 'ds-2', name: 'dev-ai', status: 'healthy' }
        ])
      }
      return Promise.resolve(undefined)
    })
  })

  afterEach(cleanup)

  it('lists dev servers via devServer.list and preselects the project’s current one', async () => {
    render(<ProjectDevServerSection projectId="p1" />)

    await waitFor(() => {
      expect(mockRpc).toHaveBeenCalledWith({ type: 'local' }, 'devServer.list', null)
    })
    expect(screen.getByTestId('project-dev-server-select')).toHaveValue('ds-1')
    // Save starts disabled since the picker already matches the current value.
    expect(screen.getByTestId('project-dev-server-save')).toBeDisabled()
  })

  it('calls project.rebindDevServer with the picked dev server and refreshes the project', async () => {
    render(<ProjectDevServerSection projectId="p1" />)
    await waitFor(() => {
      expect(mockRpc).toHaveBeenCalledWith({ type: 'local' }, 'devServer.list', null)
    })

    fireEvent.change(screen.getByTestId('project-dev-server-select'), {
      target: { value: 'ds-2' }
    })
    expect(screen.getByTestId('project-dev-server-save')).not.toBeDisabled()
    fireEvent.click(screen.getByTestId('project-dev-server-save'))

    await waitFor(() => {
      expect(mockRpc).toHaveBeenCalledWith({ type: 'local' }, 'project.rebindDevServer', {
        projectId: 'p1',
        newDevServerId: 'ds-2'
      })
      expect(switchProject).toHaveBeenCalledWith('p1')
    })
  })
})
