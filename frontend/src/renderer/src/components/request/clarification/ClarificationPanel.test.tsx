// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { act, cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const callRequestRpc = vi.fn()
vi.mock('../../../runtime/request-rpc-client', () => ({ callRequestRpc: (...a: unknown[]) => callRequestRpc(...a) }))

import { ClarificationPanel } from './ClarificationPanel'
import type { OrcaRequest } from '../../../../../shared/request-types'

const request = { id: 'r1', status: 'awaiting_information' } as unknown as OrcaRequest
const ok = (value: unknown) => ({ ok: true, value })
const clar = (over: Record<string, unknown> = {}) => ({
  id: 'c1', display_id: 'CL-1', request_id: 'r1', status: 'open', version: 5, round: 1, source: 'readiness', resume_status: 'planning',
  questions: [
    { id: 'q1', seq: 1, kind: 'text', prompt: 'What is the target?', required: true },
    { id: 'q2', seq: 2, kind: 'single_choice', prompt: 'Which env?', options: ['dev', 'prod'], suggested_default_json: '"dev"', required: true }
  ],
  ...over
})

function serve(list: unknown, answer: unknown = ok({})) {
  callRequestRpc.mockImplementation(async (method: string) => (method === 'clarification.answer' ? answer : ok({ clarifications: [list] })))
}

beforeEach(() => {
  callRequestRpc.mockReset()
})
afterEach(cleanup)

async function openPanel(props: Partial<Parameters<typeof ClarificationPanel>[0]> = {}) {
  render(<ClarificationPanel request={request} currentUserId="u1" isAdmin={false} {...props} />)
  return screen.findByTestId('clarification-panel')
}

describe('ClarificationPanel', () => {
  it('renders nothing unless the request is awaiting information', () => {
    const { container } = render(<ClarificationPanel request={{ ...request, status: 'planning' } as OrcaRequest} currentUserId="u1" isAdmin={false} />)
    expect(container).toBeEmptyDOMElement()
    expect(callRequestRpc).not.toHaveBeenCalled()
  })

  it('locks submit until every required question is answered; a suggestion never auto-submits', async () => {
    serve(clar())
    await openPanel()
    const submit = screen.getByRole('button', { name: 'Submit answers' })
    expect(submit).toBeDisabled()
    fireEvent.change(screen.getByRole('textbox'), { target: { value: 'staging' } })
    expect(submit).toBeDisabled()
    fireEvent.click(screen.getByRole('button', { name: 'Use suggestion' }))
    expect(submit).toBeEnabled()
    expect(callRequestRpc).not.toHaveBeenCalledWith('clarification.answer', expect.anything())
  })

  it('submits all answers once with acceptDefault and expectedVersion, then shows the resume message', async () => {
    serve(clar(), ok({ still_missing: false }))
    await openPanel()
    fireEvent.change(screen.getByRole('textbox'), { target: { value: 'staging' } })
    fireEvent.click(screen.getByRole('button', { name: 'Use suggestion' }))
    fireEvent.click(screen.getByRole('button', { name: 'Submit answers' }))
    await waitFor(() => expect(screen.getByTestId('clarification-submitted')).toBeInTheDocument())
    expect(callRequestRpc).toHaveBeenCalledWith('clarification.answer', {
      clarificationId: 'c1', complete: true, expectedVersion: 5,
      answers: [{ questionId: 'q1', valueJson: '"staging"', acceptDefault: false }, { questionId: 'q2', valueJson: '', acceptDefault: true }]
    })
    expect(screen.getByTestId('clarification-submitted').textContent).toContain('Plan')
  })

  it('Mod+Enter in the text area submits when valid', async () => {
    serve(clar({ questions: [{ id: 'q1', seq: 1, kind: 'text', prompt: 'Why?', required: true }] }))
    await openPanel()
    const box = screen.getByRole('textbox')
    fireEvent.change(box, { target: { value: 'because' } })
    fireEvent.keyDown(box, { key: 'Enter', ctrlKey: true, metaKey: true })
    fireEvent.keyDown(box, { key: 'Enter', ctrlKey: true })
    fireEvent.keyDown(box, { key: 'Enter', metaKey: true })
    await waitFor(() => expect(callRequestRpc).toHaveBeenCalledWith('clarification.answer', expect.anything()))
  })

  it('is read-only for people who are not assigned (admins are exempt)', async () => {
    serve(clar({ assignee_ids: ['someone-else'] }))
    await openPanel()
    expect(screen.getByTestId('clarification-readonly')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Submit answers' })).toBeNull()
    cleanup()
    serve(clar({ assignee_ids: ['someone-else'] }))
    await openPanel({ isAdmin: true })
    expect(screen.queryByTestId('clarification-readonly')).toBeNull()
  })

  it('overdue disables submit and explains the backlog', async () => {
    serve(clar({ due_at: '2020-01-01T00:00:00Z' }))
    await openPanel()
    expect(screen.getByTestId('clarification-deadline').textContent).toContain('backlog')
    expect(screen.getByRole('button', { name: 'Submit answers' })).toBeDisabled()
  })

  it('a version conflict reloads and keeps the draft; network errors offer retry', async () => {
    serve(clar({ questions: [{ id: 'q1', seq: 1, kind: 'text', prompt: 'Why?', required: true }] }), { ok: false, error: { kind: 'conflict', code: 'x', message: 'REQUEST_CLARIFICATION_VERSION_CONFLICT' } })
    await openPanel()
    fireEvent.change(screen.getByRole('textbox'), { target: { value: 'keep me' } })
    fireEvent.click(screen.getByRole('button', { name: 'Submit answers' }))
    await waitFor(() => expect(document.querySelector('[role="alert"]')).not.toBeNull())
    expect((screen.getByRole('textbox') as HTMLTextAreaElement).value).toBe('keep me')
    cleanup()
    serve(clar({ questions: [{ id: 'q1', seq: 1, kind: 'text', prompt: 'Why?', required: true }] }), { ok: false, error: { kind: 'network', code: 'N', message: 'offline' } })
    await openPanel()
    fireEvent.change(screen.getByRole('textbox'), { target: { value: 'x' } })
    fireEvent.click(screen.getByRole('button', { name: 'Submit answers' }))
    await waitFor(() => expect(screen.getByText('Retry')).toBeInTheDocument())
  })

  it('maps INVALID_ANSWER with a question id to a field error', async () => {
    serve(clar({ questions: [{ id: 'q1', seq: 1, kind: 'text', prompt: 'Why?', required: true }] }), { ok: false, error: { kind: 'validation', code: 'x', message: 'REQUEST_CLARIFICATION_INVALID_ANSWER: question_id=q1 too vague' } })
    await openPanel()
    fireEvent.change(screen.getByRole('textbox'), { target: { value: 'meh' } })
    fireEvent.click(screen.getByRole('button', { name: 'Submit answers' }))
    await waitFor(() => expect(document.querySelector('[role="alert"]')).not.toBeNull())
    expect(document.querySelector('[role="alert"]')?.textContent).toContain('too vague')
    expect(screen.getByRole('textbox')).toHaveAttribute('aria-invalid', 'true')
  })

  it('shows a skeleton, then a waiting line when no clarification arrives', async () => {
    vi.useFakeTimers()
    callRequestRpc.mockResolvedValue(ok({ clarifications: [] }))
    render(<ClarificationPanel request={request} currentUserId="u1" isAdmin={false} />)
    expect(screen.getByTestId('clarification-skeleton')).toBeInTheDocument()
    await act(async () => { await vi.advanceTimersByTimeAsync(3100) })
    expect(screen.getByTestId('clarification-waiting')).toBeInTheDocument()
    vi.useRealTimers()
  })
})
