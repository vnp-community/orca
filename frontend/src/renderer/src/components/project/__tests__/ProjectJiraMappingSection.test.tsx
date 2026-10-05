// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { render, screen, waitFor, fireEvent, cleanup } from '@testing-library/react'
import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest'
import { Children, isValidElement } from 'react'
import type { ReactElement, ReactNode } from 'react'
import { ProjectJiraMappingSection } from '../ProjectJiraMappingSection'
import { callRuntimeRpc } from '../../../runtime/runtime-rpc-client'

const switchProject = vi.fn().mockResolvedValue(undefined)
let currentProject: Record<string, unknown> = { id: 'p1', jiraProjectKey: '', jiraSiteId: '' }

vi.mock('../../../context/WorkspaceContext', () => ({
  useWorkspace: () => ({ project: currentProject, switchProject })
}))

vi.mock('../../../runtime/runtime-rpc-client', () => ({
  callRuntimeRpc: vi.fn(),
  getActiveRuntimeTarget: vi.fn().mockReturnValue({ type: 'local' }),
  RuntimeRpcCallError: class RuntimeRpcCallError extends Error {}
}))

const storeState = {
  settings: {},
  jiraStatus: {
    connected: true,
    viewer: null,
    sites: [
      { id: 'site-1', siteUrl: 'https://a.atlassian.net', displayName: 'Site A' },
      { id: 'site-2', siteUrl: 'https://b.atlassian.net', displayName: 'Site B' }
    ]
  },
  checkJiraConnection: vi.fn()
}
vi.mock('../../../store', () => ({
  useAppStore: Object.assign(
    (selector: (s: typeof storeState) => unknown) => selector(storeState),
    { getState: () => storeState }
  )
}))

vi.mock('sonner', () => ({ toast: { success: vi.fn(), error: vi.fn() } }))

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

describe('ProjectJiraMappingSection', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    currentProject = { id: 'p1', jiraProjectKey: '', jiraSiteId: '' }
    mockRpc.mockResolvedValue(undefined)
  })
  afterEach(cleanup)

  it('uppercases the key and saves key + site through project.update', async () => {
    render(<ProjectJiraMappingSection projectId="p1" />)
    fireEvent.change(screen.getByTestId('project-jira-key-input'), { target: { value: 'abc' } })
    fireEvent.change(screen.getByTestId('project-jira-site-select'), {
      target: { value: 'site-2' }
    })
    fireEvent.click(screen.getByTestId('project-jira-mapping-save'))
    await waitFor(() => {
      expect(mockRpc).toHaveBeenCalledWith({ type: 'local' }, 'project.update', {
        id: 'p1',
        jiraProjectKey: 'ABC',
        jiraSiteId: 'site-2'
      })
    })
    await waitFor(() => expect(switchProject).toHaveBeenCalledWith('p1'))
  })

  it('blocks an invalid key', () => {
    render(<ProjectJiraMappingSection projectId="p1" />)
    fireEvent.change(screen.getByTestId('project-jira-key-input'), { target: { value: '1x' } })
    expect(screen.getByTestId('project-jira-key-invalid')).toBeInTheDocument()
    expect(screen.getByTestId('project-jira-mapping-save')).toBeDisabled()
  })

  it('clears the mapping with empty strings', async () => {
    currentProject = { id: 'p1', jiraProjectKey: 'ABC', jiraSiteId: 'site-1' }
    render(<ProjectJiraMappingSection projectId="p1" />)
    fireEvent.change(screen.getByTestId('project-jira-key-input'), { target: { value: '' } })
    fireEvent.change(screen.getByTestId('project-jira-site-select'), {
      target: { value: '__orca_no_jira_site__' }
    })
    fireEvent.click(screen.getByTestId('project-jira-mapping-save'))
    await waitFor(() => {
      expect(mockRpc).toHaveBeenCalledWith({ type: 'local' }, 'project.update', {
        id: 'p1',
        jiraProjectKey: '',
        jiraSiteId: ''
      })
    })
  })
})
