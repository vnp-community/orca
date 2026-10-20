// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { render, screen, cleanup, fireEvent, waitFor, within } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const spawnChild = vi.hoisted(() => vi.fn())
vi.mock('../../../../hooks/useRequestActions', () => ({
  useRequestActions: () => ({ spawnChild })
}))
vi.mock('sonner', () => ({ toast: Object.assign(vi.fn(), { error: vi.fn(), success: vi.fn() }) }))

import { OutcomeSendBackDialog } from './OutcomeSendBackDialog'
import { makeItem, makeReview } from '../outcome-review.fixture'
import type { DecisionResult } from './use-outcome-decisions'
import type { OrcaRequest } from '../../../../../../shared/request-types'

const request = {
  id: 'r1',
  number: 7,
  title: 'Add review',
  type: 'task',
  status: 'completed',
  projectId: 'p1'
} as unknown as OrcaRequest
const items = [
  makeItem({
    key: 'a',
    title: 'Gateway calls bug management',
    needsDecision: true,
    evidence: [{ kind: 'kg', ref: 'CT-02', snapshot: 'snap-4' }],
    severity: 'regression'
  }),
  makeItem({ key: 'b', title: 'Old call removed', needsDecision: true, section: 'flow' }),
  makeItem({ key: 'c', title: 'Already sent back', needsDecision: false })
]

beforeEach(() => {
  spawnChild.mockReset()
  spawnChild.mockResolvedValue({
    ok: true,
    value: {
      request: {
        id: 'child-1',
        number: 8,
        title: 'x',
        type: 'bug',
        status: 'submitted',
        projectId: 'p1'
      }
    }
  })
})
afterEach(cleanup)

function show(over: Partial<React.ComponentProps<typeof OutcomeSendBackDialog>> = {}) {
  const props = {
    request,
    review: makeReview({ model: { items } }),
    candidates: items,
    sentBack: new Set(['c']),
    decide: vi.fn(async (): Promise<DecisionResult> => ({ ok: true })),
    linkChild: vi.fn(async (): Promise<DecisionResult> => ({ ok: true })),
    onClose: vi.fn(),
    onChanged: vi.fn(),
    ...over
  }
  render(<OutcomeSendBackDialog {...props} />)
  return props
}

describe('OutcomeSendBackDialog', () => {
  it('lists the findings, with what was already sent back ticked, and will not go on with none', () => {
    show({ sentBack: new Set() })
    const rows = screen.getAllByTestId('outcome-send-back-candidate')
    expect(rows).toHaveLength(3)
    expect(screen.getByTestId('outcome-send-back-continue')).toBeDisabled()
    expect(screen.getByText('Choose at least one finding.')).toBeInTheDocument()
    cleanup()
    show()
    expect(
      within(screen.getAllByTestId('outcome-send-back-candidate')[2]).getByRole('checkbox')
    ).toBeChecked()
    expect(screen.getByTestId('outcome-send-back-continue')).toBeEnabled()
  })

  it('records what is not yet sent back, one at a time, then opens the follow-up filled in from the findings', async () => {
    const p = show()
    fireEvent.click(
      within(screen.getAllByTestId('outcome-send-back-candidate')[0]).getByRole('checkbox')
    )
    fireEvent.change(screen.getByLabelText('Note for the follow-up (optional)'), {
      target: { value: 'Start with the gateway.' }
    })
    fireEvent.click(screen.getByTestId('outcome-send-back-continue'))
    await waitFor(() => expect(screen.getByLabelText('Title')).toBeInTheDocument())
    expect(p.decide).toHaveBeenCalledTimes(1)
    expect(p.decide).toHaveBeenCalledWith('a', '')
    expect(screen.getByLabelText('Title')).toHaveValue('Address review findings for Add review')
    const body = screen.getByLabelText('Description') as HTMLTextAreaElement
    expect(body.value).toContain('### Gateway calls bug management')
    expect(body.value).toContain('kg CT-02 (snapshot snap-4)')
    expect(body.value).toContain('Start with the gateway.')
    expect(body.value).toContain('### Already sent back')
    expect(body.value).not.toContain('### Old call removed')
  })

  it('creating the follow-up links the findings to it, leaves this Request alone, and uses the escalation link', async () => {
    const p = show()
    fireEvent.click(
      within(screen.getAllByTestId('outcome-send-back-candidate')[0]).getByRole('checkbox')
    )
    fireEvent.click(screen.getByTestId('outcome-send-back-continue'))
    const create = await screen.findByRole('button', { name: 'Create' })
    fireEvent.click(create)
    await waitFor(() => expect(spawnChild).toHaveBeenCalled())
    const sent = spawnChild.mock.calls[0][0]
    expect(sent).toMatchObject({ id: 'r1', reason: 'escalation' })
    expect(sent.title).toBe('Address review findings for Add review')
    expect(sent.type).toBe('task')
    await waitFor(() => expect(p.linkChild).toHaveBeenCalledWith(['a', 'c'], 'child-1'))
  })

  it('suggests a bug when most of what is sent back is a regression', async () => {
    show({
      candidates: [
        makeItem({ key: 'r', severity: 'regression', needsDecision: true, title: 'Regressed' })
      ],
      sentBack: new Set(['r'])
    })
    fireEvent.click(screen.getByTestId('outcome-send-back-continue'))
    await screen.findByLabelText('Title')
    fireEvent.click(screen.getByRole('button', { name: 'Create' }))
    await waitFor(() => expect(spawnChild).toHaveBeenCalled())
    expect(spawnChild.mock.calls[0][0].type).toBe('bug')
  })

  it('stops with the sentence of the code when recording a finding fails, and does not open the follow-up', async () => {
    const p = show({
      decide: vi.fn(
        async (): Promise<DecisionResult> => ({
          ok: false,
          error: { code: 'REQUEST_REVIEW_STALE_REVISION', message: 'RAW' }
        })
      )
    })
    fireEvent.click(
      within(screen.getAllByTestId('outcome-send-back-candidate')[0]).getByRole('checkbox')
    )
    fireEvent.click(screen.getByTestId('outcome-send-back-continue'))
    expect(await screen.findByTestId('outcome-send-back-failure')).toHaveTextContent(
      'Someone changed this review. It was reloaded; try again.'
    )
    expect(screen.queryByLabelText('Title')).toBeNull()
    expect(spawnChild).not.toHaveBeenCalled()
    expect(p.linkChild).not.toHaveBeenCalled()
  })

  it('closes without recording anything', () => {
    const p = show()
    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }))
    expect(p.onClose).toHaveBeenCalled()
    expect(p.decide).not.toHaveBeenCalled()
  })
})
