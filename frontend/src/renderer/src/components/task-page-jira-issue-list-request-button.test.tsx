// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { cleanup, fireEvent, render, screen, within } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

vi.mock('../runtime/runtime-rpc-client', () => ({
  callRuntimeRpc: vi.fn(async () => []),
  getActiveRuntimeTarget: () => ({ kind: 'local' })
}))
vi.mock('../runtime/runtime-shell-client', () => ({ shellOpenUrl: vi.fn() }))

import { TooltipProvider } from '@/components/ui/tooltip'
import { useAppStore } from '@/store'
import { TaskPageJiraIssueList } from './task-page-jira-issue-list'
import type { JiraIssue } from '../../../shared/types'

const issue = {
  id: '10001',
  key: 'ORCA-7',
  title: 'Login fails on Safari',
  url: 'https://example.atlassian.net/browse/ORCA-7',
  project: { id: 'p1', key: 'ORCA', name: 'Orca' },
  issueType: { id: 't1', name: 'Bug' },
  status: { id: 's1', name: 'To Do', categoryKey: 'new' },
  labels: [],
  updatedAt: '2026-10-01T00:00:00Z',
  createdAt: '2026-10-01T00:00:00Z'
} as unknown as JiraIssue

function renderList(onStartWorkspace = vi.fn(), onOpenIssue = vi.fn()) {
  render(
    <TooltipProvider>
      <TaskPageJiraIssueList
        formatUpdatedAt={() => 'today'}
        getStatusTone={() => ''}
        issues={[issue]}
        onOpenIssue={onOpenIssue}
        onStartWorkspace={onStartWorkspace}
        selectedIssue={null}
        showSiteContext={false}
        statusOrder={null}
      />
    </TooltipProvider>
  )
  return { onStartWorkspace, onOpenIssue }
}

beforeEach(() => useAppStore.setState({ requestFlowSupport: 'supported' }))
afterEach(cleanup)

describe('Jira issue row: Create request next to Start workspace (FE-REQ-TASK-019-06)', () => {
  it('shows Create request beside Start workspace and does not start a workspace', () => {
    const { onStartWorkspace, onOpenIssue } = renderList()
    const start = screen.getByRole('button', { name: 'Start workspace from ORCA-7' })
    const actions = start.parentElement as HTMLElement
    const create = within(actions).getByRole('button', { name: 'Create request' })
    fireEvent.click(create)
    expect(onStartWorkspace).not.toHaveBeenCalled()
    expect(onOpenIssue).not.toHaveBeenCalled()
    expect(screen.getByLabelText('Title')).toHaveValue('Login fails on Safari')
  })

  it('keeps Start workspace behaviour unchanged', () => {
    const { onStartWorkspace } = renderList()
    fireEvent.click(screen.getByRole('button', { name: 'Start workspace from ORCA-7' }))
    expect(onStartWorkspace).toHaveBeenCalledWith(issue)
  })

  it('hides Create request when the runtime has no request flow', () => {
    useAppStore.setState({ requestFlowSupport: 'unsupported' })
    renderList()
    expect(screen.queryByRole('button', { name: 'Create request' })).toBeNull()
    expect(screen.getByRole('button', { name: 'Start workspace from ORCA-7' })).toBeInTheDocument()
  })
})
