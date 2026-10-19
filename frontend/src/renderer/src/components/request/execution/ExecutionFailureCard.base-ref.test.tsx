// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

type Rpc = (method: string, params: Record<string, unknown>) => unknown
const rpc = vi.fn<Rpc>()

vi.mock('../../../runtime/runtime-rpc-client', () => ({
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
  searchRuntimeRepoBaseRefs: vi.fn(async () => ['release/1.2'])
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
import { ExecutionFailureCard } from './ExecutionFailureCard'
import { failureCauseAdvice, failureCauseLabel } from './execution-failure-cause'
import { REQUEST_BASE_REF_FEATURE } from '../../../../../shared/request-base-ref'
import type { FailureCause } from '../../../../../shared/request-execution-types'
import type { OrcaRequest } from '../../../../../shared/request-types'

const request = (over: Partial<OrcaRequest> = {}): OrcaRequest =>
  ({
    id: 'r1',
    projectId: 'p1',
    title: 'T',
    type: 'change_request',
    status: 'request_backlog',
    baseRef: 'gone/branch',
    baseRefLocked: false,
    version: 3,
    ...over
  }) as OrcaRequest

const progress = {
  phasesTotal: 1,
  phasesDone: 0,
  tasksTotal: 4,
  tasksDone: 0,
  tasksFailed: 1,
  tasksRunning: 0
}

function card(cause: FailureCause, req?: OrcaRequest) {
  return render(
    <TooltipProvider>
      <ExecutionFailureCard
        failed={{ taskId: 't1', title: 'TG-1', attempts: 1, maxAttempts: 1, lastCause: cause }}
        progress={progress}
        busy={false}
        onReopen={vi.fn()}
        onViewLog={vi.fn()}
        onCancel={vi.fn()}
        request={req}
      />
    </TooltipProvider>
  )
}

const CHANGE = { name: 'Change starting branch' }

beforeEach(() => {
  rpc
    .mockReset()
    .mockImplementation((method) =>
      method === 'repo.list'
        ? { repos: [{ id: 'repo-1', url: '/work/ledger', displayName: 'ledger', devServerId: '' }] }
        : {}
    )
  useAppStore.setState({
    requestFlowFeatures: new Set([REQUEST_BASE_REF_FEATURE]),
    requestsById: {},
    requestPage: { section: 'requests', requestId: null, backlogView: 'requests', listFilters: {} }
  })
  vi.spyOn(console, 'error').mockImplementation(() => {})
})
afterEach(() => {
  cleanup()
  vi.restoreAllMocks()
})

describe('base_ref_not_found cause', () => {
  it('has its own label and advice', () => {
    expect(failureCauseLabel('base_ref_not_found')).toBe('Starting branch not found')
    expect(failureCauseAdvice('base_ref_not_found')).toMatch(/Choose another starting branch/)
    card('base_ref_not_found', request())
    expect(screen.getByTestId('execution-failure-card')).toHaveTextContent(
      'Cause: Starting branch not found'
    )
  })

  it('offers Change starting branch when the branch is not locked', async () => {
    card('base_ref_not_found', request())
    expect(await screen.findByRole('button', CHANGE)).toBeEnabled()
  })

  it('does not offer it when the branch is locked', async () => {
    card('base_ref_not_found', request({ baseRefLocked: true, baseRefLockReason: 'task' }))
    await new Promise((resolve) => setTimeout(resolve, 20))
    expect(screen.queryByRole('button', CHANGE)).toBeNull()
  })

  it.each<FailureCause>(['timed_out', 'verify_failed', 'unknown'])(
    'does not offer it for %s',
    async (cause) => {
      card(cause, request())
      await new Promise((resolve) => setTimeout(resolve, 20))
      expect(screen.queryByRole('button', CHANGE)).toBeNull()
    }
  )

  it('does not offer it when the gateway cannot store the ref', async () => {
    useAppStore.setState({ requestFlowFeatures: new Set() })
    card('base_ref_not_found', request())
    await new Promise((resolve) => setTimeout(resolve, 20))
    expect(screen.queryByRole('button', CHANGE)).toBeNull()
  })

  it('uses the open Request of the page when the card is not given one', async () => {
    useAppStore.setState({
      requestsById: { r1: request() },
      requestPage: {
        section: 'requests',
        requestId: 'r1',
        backlogView: 'requests',
        listFilters: {}
      }
    })
    card('base_ref_not_found')
    expect(await screen.findByRole('button', CHANGE)).toBeInTheDocument()
  })

  it('has no action without any Request to change', async () => {
    card('base_ref_not_found')
    await new Promise((resolve) => setTimeout(resolve, 20))
    expect(screen.queryByRole('button', CHANGE)).toBeNull()
  })

  it("choosing another branch saves it with the Request's version", async () => {
    rpc.mockImplementation((method) => {
      if (method === 'repo.list') {
        return { repos: [{ id: 'repo-1', url: '/w', displayName: 'ledger', devServerId: '' }] }
      }
      return method === 'request.setBaseRef'
        ? {
            request: {
              id: 'r1',
              projectId: 'p1',
              baseRef: 'release/1.2',
              baseRefLocked: false,
              version: 4
            }
          }
        : {}
    })
    card('base_ref_not_found', request())
    fireEvent.click(await screen.findByRole('button', CHANGE))
    fireEvent.change(await screen.findByRole('combobox', { name: 'Search branches' }), {
      target: { value: 'rel' }
    })
    fireEvent.click(await screen.findByRole('option', { name: 'release/1.2' }))
    await waitFor(() =>
      expect(rpc).toHaveBeenCalledWith('request.setBaseRef', {
        id: 'r1',
        baseRef: 'release/1.2',
        expectedVersion: 3
      })
    )
    await waitFor(() => expect(useAppStore.getState().requestsById.r1?.baseRef).toBe('release/1.2'))
  })

  it('shows a refusal under the actions and keeps the action available', async () => {
    rpc.mockImplementation((method) => {
      if (method === 'repo.list') {
        return { repos: [{ id: 'repo-1', url: '/w', displayName: 'ledger', devServerId: '' }] }
      }
      throw Object.assign(new Error('REQUEST_BASE_REF_NOT_FOUND: nope {"suggestions":["main"]}'), {
        code: 'runtime_error'
      })
    })
    card('base_ref_not_found', request())
    fireEvent.click(await screen.findByRole('button', CHANGE))
    fireEvent.change(await screen.findByRole('combobox', { name: 'Search branches' }), {
      target: { value: 'rel' }
    })
    fireEvent.click(await screen.findByRole('option', { name: 'release/1.2' }))
    expect(
      await screen.findByText(
        'This branch or tag was not found in the repository. Pick another one.'
      )
    ).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'main' })).toBeInTheDocument()
    expect(screen.getByRole('button', CHANGE)).toBeEnabled()
  })
})
