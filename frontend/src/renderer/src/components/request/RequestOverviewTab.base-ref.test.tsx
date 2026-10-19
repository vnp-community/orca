// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { cleanup, render, screen } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

vi.mock('../../runtime/runtime-rpc-client', () => ({
  callRuntimeRpc: vi.fn(async () => ({})),
  getActiveRuntimeTarget: () => ({ kind: 'local' }),
  subscribeRuntimeStreamChannel: vi.fn()
}))

import { TooltipProvider } from '@/components/ui/tooltip'
import { useAppStore } from '@/store'
import { resetRequestFlowCache } from '../../hooks/useRequestFlow'
import { RequestOverviewTab } from './RequestOverviewTab'
import { REQUEST_BASE_REF_FEATURE } from '../../../../shared/request-base-ref'
import type { OrcaRequest } from '../../../../shared/request-types'

const request = (over: Partial<OrcaRequest> = {}): OrcaRequest =>
  ({
    id: 'r1',
    projectId: 'p1',
    title: 'T',
    body: 'Fix it',
    type: 'bug',
    status: 'analyzing',
    urgency: 'normal',
    baseRef: 'release/1.0',
    baseRefLocked: true,
    baseRefLockReason: 'task',
    ...over
  }) as OrcaRequest

beforeEach(() => {
  resetRequestFlowCache()
  useAppStore.setState({
    requestFlowFeatures: new Set([REQUEST_BASE_REF_FEATURE]),
    requestsById: {}
  })
})
afterEach(cleanup)

const mount = (req: OrcaRequest) =>
  render(
    <TooltipProvider>
      <RequestOverviewTab request={req} />
    </TooltipProvider>
  )

describe('RequestOverviewTab starting branch row', () => {
  it('shows the row below the details for a request that runs worktrees', () => {
    mount(request())
    expect(screen.getByText('Fix it')).toBeInTheDocument()
    expect(screen.getByTestId('request-base-ref-row')).toHaveTextContent(
      'Worktrees start from release/1.0'
    )
  })

  it('leaves the rest of the tab as it was for a question, and for an old gateway', () => {
    const { unmount } = mount(request({ type: 'question' }))
    expect(screen.queryByTestId('request-base-ref-row')).toBeNull()
    expect(screen.getByText('Fix it')).toBeInTheDocument()
    unmount()
    useAppStore.setState({ requestFlowFeatures: new Set() })
    mount(request())
    expect(screen.queryByTestId('request-base-ref-row')).toBeNull()
    expect(screen.getByTestId('request-overview')).toBeInTheDocument()
  })
})
