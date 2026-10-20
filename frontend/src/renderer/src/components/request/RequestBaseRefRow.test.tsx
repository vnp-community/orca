// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { act, cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

type Rpc = (method: string, params: Record<string, unknown>) => unknown
const rpc = vi.fn<Rpc>()
const toasts = vi.hoisted(() => ({
  base: vi.fn(),
  success: vi.fn(),
  warning: vi.fn(),
  info: vi.fn(),
  error: vi.fn()
}))

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
  searchRuntimeRepoBaseRefs: vi.fn(async () => ['release/1.2', 'origin/main']),
  // A gateway without repo.listBranches: the picker searches, as these tests expect.
  listRuntimeRepoBranches: vi.fn(async () => ({ status: 'unsupported' }))
}))
vi.mock('sonner', () => ({ toast: Object.assign((...a: unknown[]) => toasts.base(...a), toasts) }))

import { TooltipProvider } from '@/components/ui/tooltip'
import { listRuntimeRepoBranches } from '@/runtime/runtime-repo-client'
import { useAppStore } from '@/store'
import { resetRequestFlowCache } from '../../hooks/useRequestFlow'
import { resetRequestCooldowns } from '../../runtime/request-rate-limit'
import { RequestBaseRefRow } from './RequestBaseRefRow'
import { REQUEST_BASE_REF_FEATURE } from '../../../../shared/request-base-ref'
import type { OrcaRequest } from '../../../../shared/request-types'

const request = (over: Partial<OrcaRequest> = {}): OrcaRequest =>
  ({
    id: 'r1',
    projectId: 'p1',
    number: 1,
    title: 'T',
    type: 'change_request',
    status: 'analyzing',
    urgency: 'normal',
    baseRef: 'release/1.0',
    baseRefLocked: false,
    baseRefLockReason: '',
    version: 4,
    createdAt: '',
    updatedAt: '',
    ...over
  }) as OrcaRequest

const wire = (over: Record<string, unknown> = {}) => ({
  request: {
    id: 'r1',
    projectId: 'p1',
    type: 'change_request',
    status: 'analyzing',
    baseRef: 'release/1.2',
    baseRefLocked: false,
    baseRefLockReason: '',
    version: 5,
    ...over
  }
})

function rpcError(message: string): Error & { code: string } {
  return Object.assign(new Error(message), { code: 'runtime_error' })
}

const setCalls = () => rpc.mock.calls.filter(([m]) => m === 'request.setBaseRef')

function mount(req: OrcaRequest = request()) {
  const view = render(
    <TooltipProvider>
      <RequestBaseRefRow request={req} />
    </TooltipProvider>
  )
  return {
    ...view,
    update: (next: OrcaRequest) =>
      view.rerender(
        <TooltipProvider>
          <RequestBaseRefRow request={next} />
        </TooltipProvider>
      )
  }
}

async function chooseRef(ref: string): Promise<void> {
  fireEvent.click(await screen.findByTestId('base-ref-edit'))
  fireEvent.change(await screen.findByRole('combobox', { name: 'Search branches' }), {
    target: { value: ref.slice(0, 3) }
  })
  fireEvent.click(await screen.findByRole('option', { name: ref }))
}

beforeEach(() => {
  resetRequestFlowCache()
  resetRequestCooldowns()
  rpc.mockReset().mockImplementation((method) => {
    if (method === 'repo.list') {
      return {
        repos: [{ id: 'repo-1', url: '/work/ledger', displayName: 'ledger', devServerId: '' }]
      }
    }
    if (method === 'request.setBaseRef') {
      return wire()
    }
    if (method === 'request.get') {
      return wire({ baseRef: 'release/1.2' })
    }
    return {}
  })
  Object.values(toasts).forEach((fn) => fn.mockReset())
  useAppStore.setState({
    requestFlowFeatures: new Set([REQUEST_BASE_REF_FEATURE]),
    requestsById: {}
  })
  vi.spyOn(console, 'error').mockImplementation(() => {})
})
afterEach(() => {
  cleanup()
  vi.restoreAllMocks()
})

describe('visibility', () => {
  it('shows the branch with an edit button once the repo is known', async () => {
    mount()
    expect(screen.getByText('Starting branch')).toBeInTheDocument()
    expect(screen.getByText('Worktrees start from release/1.0')).toBeInTheDocument()
    expect(screen.queryByTestId('base-ref-edit')).toBeNull()
    expect(
      await screen.findByRole('button', { name: 'Change starting branch' })
    ).toBeInTheDocument()
    expect(screen.queryByTestId('base-ref-lock')).toBeNull()
  })

  it('says the default branch applies for an empty ref', () => {
    mount(request({ baseRef: '' }))
    expect(screen.getByText('Worktrees start from the default branch')).toBeInTheDocument()
  })

  it('is hidden when the gateway cannot store the ref or sent no baseRef', () => {
    useAppStore.setState({ requestFlowFeatures: new Set() })
    const { container, update } = mount()
    expect(container).toBeEmptyDOMElement()
    useAppStore.setState({ requestFlowFeatures: new Set([REQUEST_BASE_REF_FEATURE]) })
    update(request({ baseRef: undefined }))
    expect(container).toBeEmptyDOMElement()
  })

  it.each(['question', 'spike'] as const)(
    'is hidden for %s: its flow has no execution and no plan',
    (type) => {
      const { container } = mount(request({ type }))
      expect(container).toBeEmptyDOMElement()
    }
  )

  it.each(['change_request', 'bug', 'hotfix', 'unknown'] as const)('is shown for %s', (type) => {
    mount(request({ type }))
    expect(screen.getByTestId('request-base-ref-row')).toBeInTheDocument()
  })
})

describe('lock', () => {
  const reasons = [
    ['proposal', /analysis already created a worktree/],
    ['task', /task worktree already exists/],
    ['execution', /Execution has started/],
    ['terminal', /request is finished/],
    ['unknown', /Worktrees already exist/]
  ] as const
  it.each(reasons)('locked by %s: lock with its reason, no edit button', (reason, text) => {
    mount(request({ baseRefLocked: true, baseRefLockReason: reason }))
    const lock = screen.getByTestId('base-ref-lock')
    expect(lock).toHaveTextContent('Locked')
    expect(lock.getAttribute('aria-label')).toMatch(text)
    expect(screen.queryByTestId('base-ref-edit')).toBeNull()
  })

  it('follows a realtime change: another person locking it removes the button, the value updates', () => {
    const { update } = mount()
    update(
      request({
        baseRef: 'release/2.0',
        baseRefLocked: true,
        baseRefLockReason: 'task',
        version: 6
      })
    )
    expect(screen.getByText('Worktrees start from release/2.0')).toBeInTheDocument()
    expect(screen.queryByTestId('base-ref-edit')).toBeNull()
    expect(screen.getByTestId('base-ref-lock')).toBeInTheDocument()
  })

  it('shows the not-checked notice for a ref the host could not confirm', () => {
    mount(request({ baseRefUnverified: true }))
    expect(screen.getByTestId('base-ref-unverified')).toHaveTextContent(
      "We couldn't reach the host to check this branch"
    )
  })
})

describe('saving', () => {
  it('sends id, baseRef and expectedVersion, then shows the new branch and version', async () => {
    mount()
    await chooseRef('release/1.2')
    await waitFor(() =>
      expect(screen.getByText('Worktrees start from release/1.2')).toBeInTheDocument()
    )
    expect(setCalls()).toHaveLength(1)
    expect(setCalls()[0][1]).toEqual({ id: 'r1', baseRef: 'release/1.2', expectedVersion: 4 })
    const stored = useAppStore.getState().requestsById.r1
    expect(stored).toMatchObject({ baseRef: 'release/1.2', version: 5, title: 'T' })
    expect(toasts.success).toHaveBeenCalledWith('Starting branch updated')
  })

  it('picking the default branch by name stores the empty ref (follow the default)', async () => {
    rpc.mockImplementation((method) =>
      method === 'request.setBaseRef'
        ? wire({ baseRef: '' })
        : { repos: [{ id: 'repo-1', url: '/w', displayName: 'ledger', devServerId: '' }] }
    )
    mount()
    await chooseRef('origin/main')
    await waitFor(() => expect(setCalls()).toHaveLength(1))
    expect(setCalls()[0][1]).toMatchObject({ baseRef: '' })
    await screen.findByText('Worktrees start from the default branch')
  })

  it('saves nothing when the chosen branch is the one already in effect', async () => {
    mount(request({ baseRef: 'release/1.2' }))
    await chooseRef('release/1.2')
    await act(async () => {})
    expect(setCalls()).toHaveLength(0)
  })

  it('shows the not-checked notice when the host could not confirm the saved ref', async () => {
    rpc.mockImplementation((method) =>
      method === 'request.setBaseRef'
        ? { ...wire(), baseRefUnverified: true }
        : { repos: [{ id: 'repo-1', url: '/w', displayName: 'ledger', devServerId: '' }] }
    )
    mount()
    await chooseRef('release/1.2')
    expect(await screen.findByTestId('base-ref-unverified')).toBeInTheDocument()
  })

  it('does not claim success when the answer does not carry the branch (a gateway that ignored it)', async () => {
    rpc.mockImplementation((method) =>
      method === 'request.setBaseRef'
        ? wire({ baseRef: undefined })
        : { repos: [{ id: 'repo-1', url: '/w', displayName: 'ledger', devServerId: '' }] }
    )
    mount()
    await chooseRef('release/1.2')
    expect(
      await screen.findByText("This server didn't apply the starting branch.")
    ).toBeInTheDocument()
    expect(screen.getByText('Worktrees start from release/1.0')).toBeInTheDocument()
    expect(toasts.success).not.toHaveBeenCalled()
  })

  it('omits expectedVersion when the version is unknown', async () => {
    mount(request({ version: undefined }))
    await chooseRef('release/1.2')
    await waitFor(() => expect(setCalls()).toHaveLength(1))
    expect(setCalls()[0][1]).toEqual({ id: 'r1', baseRef: 'release/1.2' })
  })
})

describe('branch list in the edit popover', () => {
  const list = {
    status: 'ok' as const,
    branches: [
      { name: 'main', remote: false },
      { name: 'release/1.0', remote: false },
      { name: 'release/1.2', remote: false },
      { name: 'origin/hotfix/x', remote: true }
    ],
    truncated: false
  }

  it('opens on the list with the current branch ticked and saves the picked one', async () => {
    vi.mocked(listRuntimeRepoBranches).mockResolvedValueOnce(list)
    mount()
    fireEvent.click(await screen.findByTestId('base-ref-edit'))
    await screen.findByRole('listbox')
    expect(screen.getAllByRole('option').map((o) => o.getAttribute('aria-label'))).toEqual([
      'origin/main (default)',
      'main',
      'release/1.0',
      'release/1.2',
      'origin/hotfix/x (remote only)'
    ])
    expect(screen.getByRole('option', { name: 'release/1.0' })).toHaveAttribute(
      'aria-selected',
      'true'
    )
    fireEvent.click(screen.getByRole('option', { name: 'release/1.2' }))
    await waitFor(() => expect(setCalls()).toHaveLength(1))
    expect(setCalls()[0][1]).toMatchObject({ baseRef: 'release/1.2' })
  })

  it('choosing the pinned default stores the empty ref', async () => {
    vi.mocked(listRuntimeRepoBranches).mockResolvedValueOnce(list)
    rpc.mockImplementation((method) =>
      method === 'request.setBaseRef'
        ? wire({ baseRef: '' })
        : { repos: [{ id: 'repo-1', url: '/w', displayName: 'ledger', devServerId: '' }] }
    )
    mount()
    fireEvent.click(await screen.findByTestId('base-ref-edit'))
    fireEvent.click(await screen.findByRole('option', { name: 'origin/main (default)' }))
    await waitFor(() => expect(setCalls()).toHaveLength(1))
    expect(setCalls()[0][1]).toMatchObject({ baseRef: '' })
  })
})

describe('refusals', () => {
  function failSet(message: string): void {
    rpc.mockImplementation((method) => {
      if (method === 'request.setBaseRef') {
        throw rpcError(message)
      }
      if (method === 'request.get') {
        return wire({
          baseRef: 'release/1.0',
          baseRefLocked: true,
          baseRefLockReason: 'task',
          version: 6
        })
      }
      return { repos: [{ id: 'repo-1', url: '/w', displayName: 'ledger', devServerId: '' }] }
    })
  }

  it('INVALID: message under the row, the branch and the edit button stay', async () => {
    failSet('REQUEST_BASE_REF_INVALID: bad')
    mount()
    await chooseRef('release/1.2')
    expect(
      await screen.findByText('This is not a valid branch or tag name. Check it and try again.')
    ).toBeInTheDocument()
    expect(screen.getByText('Worktrees start from release/1.0')).toBeInTheDocument()
    expect(screen.getByTestId('base-ref-edit')).toBeEnabled()
  })

  it('NOT_FOUND: message with suggestions; picking a suggestion saves it', async () => {
    failSet('REQUEST_BASE_REF_NOT_FOUND: nope {"suggestions":["release/1.3"]}')
    mount()
    await chooseRef('release/1.2')
    await screen.findByText('This branch or tag was not found in the repository. Pick another one.')
    rpc.mockImplementation((method) =>
      method === 'request.setBaseRef' ? wire({ baseRef: 'release/1.3' }) : { repos: [] }
    )
    fireEvent.click(screen.getByRole('button', { name: 'release/1.3' }))
    await waitFor(() => expect(setCalls().pop()?.[1]).toMatchObject({ baseRef: 'release/1.3' }))
    await screen.findByText('Worktrees start from release/1.3')
    expect(screen.queryByText(/was not found/)).toBeNull()
  })

  it('LOCKED: neutral toast, the request is reloaded and the edit button is replaced by the lock', async () => {
    failSet('REQUEST_BASE_REF_LOCKED: locked {"reason":"task"}')
    mount()
    await chooseRef('release/1.2')
    await waitFor(() => expect(toasts.base).toHaveBeenCalled())
    expect(toasts.base.mock.calls[0][0]).toMatch(/task worktree already exists/)
    expect(toasts.error).not.toHaveBeenCalled()
    await waitFor(() => expect(screen.queryByTestId('base-ref-edit')).toBeNull())
    expect(screen.getByTestId('base-ref-lock').getAttribute('aria-label')).toMatch(/task worktree/)
  })

  it('FORBIDDEN: shows the standard permission error and removes the edit button', async () => {
    failSet('REQUEST_FORBIDDEN: no')
    mount()
    await chooseRef('release/1.2')
    expect(await screen.findByText('You do not have permission to do this.')).toBeInTheDocument()
    expect(screen.queryByTestId('base-ref-edit')).toBeNull()
  })

  it('stale version: reloads, asks, and "Apply my change" retries with the new version', async () => {
    rpc.mockImplementation((method) => {
      if (method === 'request.setBaseRef') {
        throw rpcError('REQUEST_VERSION_CONFLICT: stale')
      }
      if (method === 'request.get') {
        return wire({ baseRef: 'hotfix/9', version: 7 })
      }
      return { repos: [{ id: 'repo-1', url: '/w', displayName: 'ledger', devServerId: '' }] }
    })
    mount()
    await chooseRef('release/1.2')
    const question = await screen.findByTestId('base-ref-conflict')
    expect(question).toHaveTextContent(
      'It now starts from hotfix/9. Your change was to release/1.2.'
    )
    expect(screen.getByText('Worktrees start from hotfix/9')).toBeInTheDocument()

    rpc.mockImplementation((method) =>
      method === 'request.setBaseRef' ? wire({ baseRef: 'release/1.2', version: 8 }) : { repos: [] }
    )
    fireEvent.click(screen.getByRole('button', { name: 'Apply my change' }))
    await waitFor(() => expect(setCalls()).toHaveLength(2))
    expect(setCalls()[1][1]).toEqual({ id: 'r1', baseRef: 'release/1.2', expectedVersion: 7 })
    await screen.findByText('Worktrees start from release/1.2')
    expect(screen.queryByTestId('base-ref-conflict')).toBeNull()
  })

  it('stale version: "Keep the current one" drops the question and changes nothing', async () => {
    rpc.mockImplementation((method) => {
      if (method === 'request.setBaseRef') {
        throw rpcError('REQUEST_VERSION_CONFLICT: stale')
      }
      return method === 'request.get'
        ? wire({ baseRef: 'hotfix/9', version: 7 })
        : { repos: [{ id: 'repo-1', url: '/w', displayName: 'ledger', devServerId: '' }] }
    })
    mount()
    await chooseRef('release/1.2')
    await screen.findByTestId('base-ref-conflict')
    fireEvent.click(screen.getByRole('button', { name: 'Keep the current one' }))
    expect(screen.queryByTestId('base-ref-conflict')).toBeNull()
    expect(setCalls()).toHaveLength(1)
  })

  it('stale version where the reload shows the same branch: no question', async () => {
    rpc.mockImplementation((method) => {
      if (method === 'request.setBaseRef') {
        throw rpcError('REQUEST_VERSION_CONFLICT: stale')
      }
      return method === 'request.get'
        ? wire({ baseRef: 'release/1.2', version: 7 })
        : { repos: [{ id: 'repo-1', url: '/w', displayName: 'ledger', devServerId: '' }] }
    })
    mount()
    await chooseRef('release/1.2')
    await screen.findByText('Worktrees start from release/1.2')
    expect(screen.queryByTestId('base-ref-conflict')).toBeNull()
  })
})
