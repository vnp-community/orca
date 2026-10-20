// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { act, cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const rpc = vi.fn<(method: string, params: Record<string, unknown>) => unknown>()

vi.mock('../../runtime/runtime-rpc-client', () => ({
  callRuntimeRpc: (_target: unknown, method: string, params: Record<string, unknown>) =>
    Promise.resolve().then(() => rpc(method, params)),
  getActiveRuntimeTarget: () => ({ kind: 'local' }),
  subscribeRuntimeStreamChannel: vi.fn()
}))
vi.mock('@/runtime/runtime-repo-client', () => ({
  getRuntimeRepoBaseRefDefault: vi.fn(async () => ({
    defaultBaseRef: 'origin/main',
    remoteCount: 1
  })),
  searchRuntimeRepoBaseRefs: vi.fn(async () => ['release/1.2']),
  // A gateway without repo.listBranches: the picker searches, as these tests expect.
  listRuntimeRepoBranches: vi.fn(async () => ({ status: 'unsupported' }))
}))
vi.mock('sonner', () => ({
  toast: Object.assign(vi.fn(), {
    warning: vi.fn(),
    info: vi.fn(),
    success: vi.fn(),
    error: vi.fn()
  })
}))

import { TooltipProvider } from '@/components/ui/tooltip'
import { useAppStore } from '@/store'
import { CreateRequestIssueButton } from './CreateRequestIssueButton'
import { REQUEST_BASE_REF_FEATURE } from '../../../../shared/request-base-ref'
import type { JiraIssue } from '../../../../shared/jira-types'
import type { GitHubWorkItem } from '../../../../shared/types'

const jira = {
  key: 'LED-7',
  title: 'Rounding bug',
  url: 'https://j/LED-7',
  description: 'cents'
} as JiraIssue
const github = (type: 'issue' | 'pr') =>
  ({
    type,
    number: 12,
    title: 'Rounding bug',
    url: 'https://github.com/o/r/issues/12'
  }) as GitHubWorkItem

beforeEach(() => {
  rpc.mockReset().mockImplementation((method) => {
    if (method === 'project.list') {
      return [{ id: 'p1', name: 'Ledger' }]
    }
    if (method === 'repo.list') {
      return {
        repos: [{ id: 'repo-1', url: '/work/ledger', displayName: 'ledger', devServerId: '' }]
      }
    }
    if (method === 'request.create') {
      return {
        request: { id: 'r1', projectId: 'p1', baseRef: 'release/1.2', version: 1 },
        created: true
      }
    }
    return {}
  })
  useAppStore.setState({
    requestFlowSupport: 'supported',
    requestFlowFeatures: new Set([REQUEST_BASE_REF_FEATURE])
  })
  vi.spyOn(console, 'error').mockImplementation(() => {})
})
afterEach(() => {
  cleanup()
  vi.restoreAllMocks()
})

const wrap = (ui: React.ReactElement) => render(<TooltipProvider>{ui}</TooltipProvider>)

async function pickReleaseAndCreate(): Promise<void> {
  fireEvent.click(await screen.findByRole('button', { name: /^Starting branch:/ }))
  fireEvent.change(await screen.findByRole('combobox', { name: 'Search branches' }), {
    target: { value: 'rel' }
  })
  fireEvent.click(await screen.findByRole('option', { name: 'release/1.2' }))
  fireEvent.click(screen.getByRole('button', { name: 'Create' }))
  await act(async () => {})
}

describe('Start from field in the issue flow', () => {
  it('a Jira issue opens the same dialog with the field and sends the picked branch with the source', async () => {
    wrap(<CreateRequestIssueButton issue={jira} />)
    fireEvent.click(screen.getByRole('button', { name: 'Create request' }))
    expect(screen.getByLabelText('Title')).toHaveValue('Rounding bug')
    await pickReleaseAndCreate()
    await waitFor(() => expect(rpc).toHaveBeenCalledWith('request.create', expect.anything()))
    const params = rpc.mock.calls.find(([m]) => m === 'request.create')?.[1]
    expect(params).toMatchObject({
      baseRef: 'release/1.2',
      source: { provider: 'jira', ref: 'LED-7' }
    })
  })

  it('a GitHub issue opens the dialog with the field and sends nothing when the default is kept', async () => {
    wrap(
      <CreateRequestIssueButton item={github('issue')} repoIdentity={{ owner: 'o', repo: 'r' }} />
    )
    fireEvent.click(screen.getByRole('button', { name: 'Create request' }))
    expect(await screen.findByTestId('start-from-field')).toBeInTheDocument()
    await waitFor(() => expect(screen.getByRole('button', { name: 'Create' })).toBeEnabled())
    fireEvent.click(screen.getByRole('button', { name: 'Create' }))
    await waitFor(() => expect(rpc).toHaveBeenCalledWith('request.create', expect.anything()))
    const params = rpc.mock.calls.find(([m]) => m === 'request.create')?.[1] ?? {}
    expect(params).not.toHaveProperty('baseRef')
    expect(params).toMatchObject({ source: { provider: 'github', ref: 'o/r#12' } })
  })

  it('a pull request still has no create button, so no field either', () => {
    const { container } = wrap(
      <CreateRequestIssueButton item={github('pr')} repoIdentity={{ owner: 'o', repo: 'r' }} />
    )
    expect(container).toBeEmptyDOMElement()
  })
})
