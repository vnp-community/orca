// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

type Rpc = (method: string, params: Record<string, unknown>) => unknown
const rpc = vi.fn<Rpc>()

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
vi.mock('sonner', () => ({
  toast: Object.assign(vi.fn(), {
    success: vi.fn(),
    warning: vi.fn(),
    info: vi.fn(),
    error: vi.fn()
  })
}))

import { TooltipProvider } from '@/components/ui/tooltip'
import { useAppStore } from '@/store'
import { resetRequestFlowCache } from '../../hooks/useRequestFlow'
import { resetRequestCooldowns } from '../../runtime/request-rate-limit'
import { RequestBaseRefRow } from './RequestBaseRefRow'
import { REQUEST_BASE_REF_FEATURE } from '../../../../shared/request-base-ref'
import type { OrcaRequest } from '../../../../shared/request-types'

const request = {
  id: 'r1',
  projectId: 'p1',
  type: 'change_request',
  status: 'analyzing',
  baseRef: 'release/1.0',
  baseRefLocked: false,
  baseRefLockReason: '',
  version: 4
} as OrcaRequest

const wire = (over: Record<string, unknown> = {}) => ({
  request: {
    id: 'r1',
    projectId: 'p1',
    type: 'change_request',
    status: 'analyzing',
    baseRef: 'release/1.2',
    version: 5,
    ...over
  }
})

function failWith(code: string, message: string): Error & { code: string } {
  return Object.assign(new Error(`${code}: ${message}`), { code: 'runtime_error' })
}

async function chooseRef(ref: string): Promise<void> {
  fireEvent.click(await screen.findByTestId('base-ref-edit'))
  fireEvent.change(await screen.findByRole('combobox', { name: 'Search branches' }), {
    target: { value: ref.slice(0, 3) }
  })
  fireEvent.click(await screen.findByRole('option', { name: ref }))
}

const onChanged = vi.fn()

function mount() {
  return render(
    <TooltipProvider>
      <RequestBaseRefRow request={request} onChanged={onChanged} />
    </TooltipProvider>
  )
}

function answer(over: Record<string, () => unknown> = {}): void {
  rpc.mockReset().mockImplementation((method) => {
    if (over[method]) {
      return over[method]()
    }
    if (method === 'repo.list') {
      return {
        repos: [{ id: 'repo-1', url: '/work/ledger', displayName: 'ledger', devServerId: '' }]
      }
    }
    return method === 'request.setBaseRef' || method === 'request.get' ? wire() : {}
  })
}

beforeEach(() => {
  resetRequestFlowCache()
  resetRequestCooldowns()
  answer()
  useAppStore.setState({
    requestFlowFeatures: new Set([REQUEST_BASE_REF_FEATURE]),
    requestsById: {}
  })
  vi.spyOn(console, 'error').mockImplementation(() => {})
})
afterEach(() => {
  cleanup()
  vi.restoreAllMocks()
  onChanged.mockReset()
})

describe('RequestBaseRefRow onChanged', () => {
  it('tells the pane to refetch after a saved branch', async () => {
    mount()
    await chooseRef('release/1.2')
    await waitFor(() => expect(onChanged).toHaveBeenCalledTimes(1))
  })

  it('tells the pane to refetch after a lock refusal reloaded the Request', async () => {
    answer({
      'request.setBaseRef': () => {
        throw failWith('REQUEST_BASE_REF_LOCKED', '{"reason":"task"}')
      }
    })
    mount()
    await chooseRef('release/1.2')
    await waitFor(() => expect(onChanged).toHaveBeenCalledTimes(1))
  })

  it('tells the pane to refetch after a stale-version reload', async () => {
    answer({
      'request.setBaseRef': () => {
        throw failWith('REQUEST_VERSION_CONFLICT', 'stale')
      }
    })
    mount()
    await chooseRef('release/1.2')
    await waitFor(() => expect(onChanged).toHaveBeenCalledTimes(1))
  })

  it('does not refetch when the branch was refused and nothing changed', async () => {
    answer({
      'request.setBaseRef': () => {
        throw failWith('REQUEST_BASE_REF_INVALID', 'bad ref')
      }
    })
    mount()
    await chooseRef('release/1.2')
    await screen.findByText('This is not a valid branch or tag name. Check it and try again.')
    expect(onChanged).not.toHaveBeenCalled()
  })
})
