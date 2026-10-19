// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const rpc = vi.hoisted(() => ({ call: vi.fn() }))
vi.mock('../../../runtime/request-rpc-client', () => ({ callRequestRpc: rpc.call }))

import { resetRequestCooldowns, startRequestCooldown } from '../../../runtime/request-rate-limit'
import { SolutionAskBackDialog } from './SolutionAskBackDialog'

const questions = [
  { id: 'Q-1', text: 'Which database holds the audit trail?', blocking: true },
  { id: 'Q-2', text: 'Keep the old endpoint?', blocking: true },
  { id: 'Q-3', text: 'Rename the module?', blocking: false }
]

beforeEach(() => {
  rpc.call.mockReset()
  rpc.call.mockResolvedValue({ ok: true, value: { clarification: { id: 'c1' } } })
  resetRequestCooldowns()
})
afterEach(cleanup)

function mount(over: Partial<React.ComponentProps<typeof SolutionAskBackDialog>> = {}) {
  const props = {
    open: true,
    onOpenChange: vi.fn(),
    requestId: 'r1',
    solutionId: 's1',
    questions,
    refusedIds: [] as string[],
    onAsked: vi.fn(),
    ...over
  }
  render(<SolutionAskBackDialog {...props} />)
  return props
}

describe('SolutionAskBackDialog', () => {
  it('sends one clarification.request for every blocking question with the contract payload', async () => {
    const props = mount()
    expect(screen.getAllByTestId('solution-ask-back-reason')).toHaveLength(2)
    const reason1 = screen.getAllByTestId('solution-ask-back-reason')[0] as HTMLTextAreaElement
    expect(reason1.value).toContain('Which database holds the audit trail?')
    fireEvent.change(reason1, { target: { value: 'We need the store before choosing' } })
    fireEvent.click(screen.getByTestId('solution-ask-back-submit'))
    await waitFor(() => expect(props.onAsked).toHaveBeenCalledTimes(1))
    expect(rpc.call).toHaveBeenCalledTimes(1)
    const [method, params] = rpc.call.mock.calls[0]
    expect(method).toBe('clarification.request')
    expect(params).toMatchObject({
      requestId: 'r1',
      source: 'solution_open_question',
      sourceRef: 's1'
    })
    expect(params.questions).toHaveLength(2)
    expect(params.questions[0]).toEqual({
      questionKey: 'open_question:Q-1',
      kind: 'text',
      prompt: 'Which database holds the audit trail?',
      reason: 'We need the store before choosing',
      required: true
    })
    expect(params.questions[1].questionKey).toBe('open_question:Q-2')
    expect(props.onOpenChange).toHaveBeenCalledWith(false)
  })

  it('starts from the questions the server refused on', () => {
    mount({ refusedIds: ['Q-2'] })
    const reasons = screen.getAllByTestId('solution-ask-back-reason') as HTMLTextAreaElement[]
    expect(reasons.find((r) => r.dataset.questionId === 'Q-1')).toBeDisabled()
    expect(reasons.find((r) => r.dataset.questionId === 'Q-2')).toBeEnabled()
  })

  it('needs a reason and at least one question before it can send', () => {
    mount()
    const submit = screen.getByTestId('solution-ask-back-submit')
    const [reason1] = screen.getAllByTestId('solution-ask-back-reason')
    fireEvent.change(reason1, { target: { value: '   ' } })
    expect(submit).toBeDisabled()
    fireEvent.change(reason1, { target: { value: 'because' } })
    expect(submit).toBeEnabled()
    fireEvent.click(screen.getByRole('checkbox', { name: 'Q-1' }))
    fireEvent.click(screen.getByRole('checkbox', { name: 'Q-2' }))
    expect(submit).toBeDisabled()
    expect(rpc.call).not.toHaveBeenCalled()
  })

  it('does not send twice on a double click', async () => {
    let release: (v: unknown) => void = () => undefined
    rpc.call.mockReturnValue(
      new Promise((res) => {
        release = res
      })
    )
    const props = mount()
    const submit = screen.getByTestId('solution-ask-back-submit')
    fireEvent.click(submit)
    fireEvent.click(submit)
    expect(rpc.call).toHaveBeenCalledTimes(1)
    release({ ok: true, value: {} })
    await waitFor(() => expect(props.onAsked).toHaveBeenCalledTimes(1))
  })

  it('stays open and explains a refusal for a state that cannot ask', async () => {
    rpc.call.mockResolvedValue({
      ok: false,
      error: {
        kind: 'invalid_state',
        code: 'runtime_error',
        message: 'REQUEST_CLARIFICATION_STATE_NOT_ALLOWED: open clarification exists'
      }
    })
    const props = mount()
    fireEvent.click(screen.getByTestId('solution-ask-back-submit'))
    expect(await screen.findByTestId('solution-ask-back-error')).toHaveTextContent(
      'another question round may already be open'
    )
    expect(props.onAsked).not.toHaveBeenCalled()
    expect(props.onOpenChange).not.toHaveBeenCalledWith(false)
  })

  it('locks sending while the write limiter says wait', () => {
    startRequestCooldown('write', 5000)
    mount()
    expect(screen.getByTestId('solution-ask-back-submit')).toBeDisabled()
  })
})
