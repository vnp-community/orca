// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { render, screen, cleanup, fireEvent, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const useRequestMock = vi.fn()
const actions = {
  classify: vi.fn(),
  confirmType: vi.fn(),
  changeType: vi.fn(),
  returnToBacklog: vi.fn(),
  reopen: vi.fn(),
  cancel: vi.fn(),
  spawnChild: vi.fn(),
  generatePlan: vi.fn(),
  startPhase: vi.fn()
}
vi.mock('../../hooks/useRequest', () => ({ useRequest: (...a: unknown[]) => useRequestMock(...a) }))
vi.mock('../../hooks/useRequestActions', () => ({ useRequestActions: () => actions }))
vi.mock('sonner', () => ({ toast: Object.assign(vi.fn(), { error: vi.fn(), success: vi.fn() }) }))
vi.mock('../../runtime/request-rpc-client', () => ({
  callRequestRpc: vi.fn(async () => ({ ok: false, error: { kind: 'not_found' } }))
}))

import { useAppStore } from '@/store'
import { callRequestRpc } from '../../runtime/request-rpc-client'
import { RequestDetailPane } from './RequestDetailPane'

const request = (over: Record<string, unknown> = {}) => ({
  id: 'r1',
  number: 7,
  title: 'Fix login',
  body: 'Body <b>text</b>',
  type: 'change_request',
  status: 'analyzing',
  projectId: 'p',
  createdAt: '2026-01-01T00:00:00Z',
  updatedAt: '2026-01-01T00:00:00Z',
  ...over
})
const state = (over: Record<string, unknown> = {}) => ({
  request: request(),
  history: [],
  links: [],
  linksSupported: true,
  isLoading: false,
  error: null,
  refetch: vi.fn(),
  ...over
})

beforeEach(() => {
  useRequestMock.mockReset()
  Object.values(actions).forEach((f) => f.mockReset().mockResolvedValue({ ok: true, value: {} }))
})
afterEach(cleanup)

describe('RequestDetailPane', () => {
  it('shows a skeleton while loading, a not_found message and a network retry', () => {
    useRequestMock.mockReturnValue(state({ request: null, isLoading: true }))
    const { unmount } = render(<RequestDetailPane requestId="r1" />)
    expect(screen.getByTestId('request-detail-skeleton')).toBeInTheDocument()
    unmount()

    useRequestMock.mockReturnValue(state({ request: null, error: 'not_found' }))
    useAppStore.setState({
      requestPage: {
        section: 'requests',
        requestId: 'r1',
        backlogView: 'requests',
        listFilters: {}
      }
    })
    const second = render(<RequestDetailPane requestId="r1" />)
    fireEvent.click(screen.getByText('Back to list'))
    expect(useAppStore.getState().requestPage.requestId).toBeNull()
    second.unmount()

    const refetch = vi.fn()
    useRequestMock.mockReturnValue(state({ request: null, error: 'network', refetch }))
    render(<RequestDetailPane requestId="r1" />)
    fireEvent.click(screen.getByText('Retry'))
    expect(refetch).toHaveBeenCalled()
  })

  it.each([
    ['analyzing', ['Return to backlog', 'Change type', 'Cancel request'], ['Reopen']],
    ['request_backlog', ['Reopen', 'Change type', 'Cancel request'], ['Return to backlog']],
    ['completed', [], ['Cancel request', 'Reopen', 'Return to backlog', 'Change type']],
    ['awaiting_type_confirmation', ['Cancel request'], ['Change type', 'Return to backlog']]
  ])('shows the right header buttons for %s', (status, present, absent) => {
    useRequestMock.mockReturnValue(state({ request: request({ status }) }))
    render(<RequestDetailPane requestId="r1" />)
    const header = screen.getByTestId('request-detail-header')
    for (const label of present) {
      expect(header).toHaveTextContent(label)
    }
    for (const label of absent) {
      expect(header).not.toHaveTextContent(label)
    }
  })

  it('renders the body as plain text', () => {
    useRequestMock.mockReturnValue(state())
    render(<RequestDetailPane requestId="r1" />)
    expect(screen.getByTestId('request-overview')).toHaveTextContent('Body <b>text</b>')
    expect(screen.getByTestId('request-overview').querySelector('b')).toBeNull()
  })

  it('hides the Analysis tab for task and the Plan tab for question', () => {
    useRequestMock.mockReturnValue(state({ request: request({ type: 'task' }) }))
    const { unmount } = render(<RequestDetailPane requestId="r1" />)
    expect(screen.queryByRole('tab', { name: 'Analysis' })).toBeNull()
    expect(screen.getByRole('tab', { name: 'Plan' })).toBeInTheDocument()
    unmount()
    useRequestMock.mockReturnValue(state({ request: request({ type: 'question' }) }))
    render(<RequestDetailPane requestId="r1" />)
    expect(screen.queryByRole('tab', { name: 'Plan' })).toBeNull()
  })

  it('shows the backlog banner and reopens', async () => {
    useRequestMock.mockReturnValue(
      state({
        request: request({
          status: 'request_backlog',
          returnedFromStage: 'plan',
          returnReason: 'needs info'
        })
      })
    )
    render(<RequestDetailPane requestId="r1" />)
    const banner = screen.getByTestId('request-backlog-banner')
    expect(banner).toHaveTextContent('needs info')
    fireEvent.click(banner.querySelector('button')!)
    await waitFor(() => expect(actions.reopen).toHaveBeenCalledWith('r1'))
  })

  it('return to backlog blocks an empty reason and sends {id,stage,reason}', async () => {
    useRequestMock.mockReturnValue(state())
    render(<RequestDetailPane requestId="r1" />)
    fireEvent.click(screen.getByText('Return to backlog'))
    const submit = screen.getAllByRole('button', { name: 'Return to backlog' }).at(-1)!
    expect(submit).toBeDisabled()
    fireEvent.change(screen.getByLabelText('Why is it being returned?'), {
      target: { value: '  waiting on vendor ' }
    })
    fireEvent.click(submit)
    await waitFor(() =>
      expect(actions.returnToBacklog).toHaveBeenCalledWith({
        id: 'r1',
        stage: 'analysis',
        reason: 'waiting on vendor'
      })
    )
  })

  it('cancel sends the request id and optional reason', async () => {
    useRequestMock.mockReturnValue(state())
    render(<RequestDetailPane requestId="r1" />)
    fireEvent.click(screen.getByText('Cancel request'))
    fireEvent.click(screen.getAllByRole('button', { name: 'Cancel request' }).at(-1)!)
    await waitFor(() =>
      expect(actions.cancel).toHaveBeenCalledWith({ id: 'r1', reason: undefined })
    )
  })

  it('awaiting_information: header "Answer" for an assignee scrolls to the panel; timeline marks the resume step', async () => {
    const clarification = {
      id: 'c1',
      display_id: 'CL-1',
      request_id: 'r1',
      status: 'open',
      version: 1,
      round: 1,
      resume_status: 'planning',
      assignee_ids: ['u1'],
      questions: [{ id: 'q1', seq: 1, kind: 'text', prompt: 'Target?', required: true }]
    }
    vi.mocked(callRequestRpc).mockImplementation((async (method: string) =>
      method === 'clarification.list'
        ? { ok: true, value: { clarifications: [clarification] } }
        : { ok: false, error: { kind: 'not_found' } }) as never)
    useAppStore.setState({ currentUser: { id: 'u1', role: 'developer' } as never })
    useRequestMock.mockReturnValue(state({ request: request({ status: 'awaiting_information' }) }))
    render(<RequestDetailPane requestId="r1" />)
    const answer = await screen.findByRole('button', { name: 'Answer' })
    const panel = screen.getByTestId('clarification-panel')
    const scrolled = vi.fn()
    panel.scrollIntoView = scrolled
    fireEvent.click(answer)
    expect(scrolled).toHaveBeenCalled()
    expect(document.activeElement).toBe(panel)
    const current = screen
      .getByTestId('request-stage-timeline')
      .querySelector('[aria-current="step"]')
    expect(current).toHaveTextContent('Plan')
  })

  it('awaiting_information: no "Answer" button for a non-assignee', async () => {
    vi.mocked(callRequestRpc).mockImplementation((async (method: string) =>
      method === 'clarification.list'
        ? {
            ok: true,
            value: {
              clarifications: [
                {
                  id: 'c1',
                  request_id: 'r1',
                  status: 'open',
                  version: 1,
                  round: 1,
                  assignee_ids: ['someone'],
                  questions: []
                }
              ]
            }
          }
        : { ok: false, error: { kind: 'not_found' } }) as never)
    useAppStore.setState({ currentUser: { id: 'u1', role: 'developer' } as never })
    useRequestMock.mockReturnValue(state({ request: request({ status: 'awaiting_information' }) }))
    render(<RequestDetailPane requestId="r1" />)
    await screen.findByTestId('clarification-readonly')
    expect(screen.queryByRole('button', { name: 'Answer' })).toBeNull()
  })

  it('shows the type confirmation card while awaiting confirmation', () => {
    useRequestMock.mockReturnValue(
      state({ request: request({ status: 'awaiting_type_confirmation', confidence: 0.4 }) })
    )
    render(<RequestDetailPane requestId="r1" />)
    expect(screen.getByTestId('type-confirmation-card')).toBeInTheDocument()
    expect(screen.getByText('40%')).toBeInTheDocument()
    expect(screen.getByRole('alert')).toHaveTextContent('not confident')
  })
})
