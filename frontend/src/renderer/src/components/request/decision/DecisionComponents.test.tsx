// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { DecisionHistoryList } from './DecisionHistoryList'
import { DecisionRationaleField } from './DecisionRationaleField'
import { HighRiskDecisionConfirmDialog } from './HighRiskDecisionConfirmDialog'
import type { Decision } from '../../../../../shared/request-artifact-types'

afterEach(cleanup)

const decision: Decision = {
  id: 'd1', displayId: 'DEC-1', subjectKind: 'solution_option', subjectId: 's', subjectDigest: 'x', rationale: 'because',
  riskLevel: 'high', riskReasons: ['Drops a column'], status: 'chosen', version: 3, chosenOptionId: 'o1', chooserId: 'alice'
}

describe('DecisionRationaleField', () => {
  it('marks required fields invalid after blur when shorter than 10 characters', () => {
    const onChange = vi.fn()
    render(<DecisionRationaleField value="short" onChange={onChange} required />)
    const box = screen.getByRole('textbox')
    expect(box).not.toHaveAttribute('aria-invalid')
    fireEvent.blur(box)
    expect(box).toHaveAttribute('aria-invalid', 'true')
    expect(screen.getByText('5/10')).toBeInTheDocument()
  })

  it('submits on Mod+Enter only when valid', () => {
    const onSubmit = vi.fn()
    const { rerender } = render(<DecisionRationaleField value="short" onChange={vi.fn()} required onSubmit={onSubmit} />)
    fireEvent.keyDown(screen.getByRole('textbox'), { key: 'Enter', ctrlKey: true })
    fireEvent.keyDown(screen.getByRole('textbox'), { key: 'Enter', metaKey: true })
    expect(onSubmit).not.toHaveBeenCalled()
    rerender(<DecisionRationaleField value="long enough text" onChange={vi.fn()} required onSubmit={onSubmit} />)
    fireEvent.keyDown(screen.getByRole('textbox'), { key: 'Enter', ctrlKey: true })
    fireEvent.keyDown(screen.getByRole('textbox'), { key: 'Enter', metaKey: true })
    expect(onSubmit).toHaveBeenCalledTimes(1)
  })
})

describe('HighRiskDecisionConfirmDialog', () => {
  const setup = (onConfirm = vi.fn().mockResolvedValue({ ok: true, value: {} })) => {
    const onCancel = vi.fn()
    render(<HighRiskDecisionConfirmDialog open decision={decision} optionTitle="Drop Legacy" reasons={decision.riskReasons} onConfirm={onConfirm} onCancel={onCancel} />)
    return { onConfirm, onCancel }
  }

  it('lists reasons and keeps the confirm button locked until the title matches', () => {
    setup()
    expect(screen.getByText('Drops a column')).toBeInTheDocument()
    const confirm = screen.getByRole('button', { name: 'Confirm choice' })
    expect(confirm).toBeDisabled()
    fireEvent.change(screen.getByRole('textbox'), { target: { value: '  drop legacy ' } })
    expect(confirm).toBeEnabled()
  })

  it('Enter confirms with the verbatim text and the decision version', async () => {
    const { onConfirm } = setup()
    const input = screen.getByRole('textbox')
    fireEvent.change(input, { target: { value: 'DROP LEGACY' } })
    fireEvent.keyDown(input, { key: 'Enter' })
    await waitFor(() => expect(onConfirm).toHaveBeenCalledWith('DROP LEGACY'))
  })

  it('shows a server mismatch next to the field and stays open', async () => {
    setup(vi.fn().mockResolvedValue({ ok: false, error: { kind: 'validation', code: 'x', message: 'm' } }))
    fireEvent.change(screen.getByRole('textbox'), { target: { value: 'drop legacy' } })
    fireEvent.click(screen.getByRole('button', { name: 'Confirm choice' }))
    expect(await screen.findByRole('alert')).toHaveTextContent('does not match')
    expect(screen.getByTestId('high-risk-decision-dialog')).toBeInTheDocument()
  })

  it('does not style the confirm action as destructive and cancel is silent', () => {
    const { onCancel } = setup()
    const confirm = screen.getByRole('button', { name: 'Confirm choice' })
    expect(confirm.className).not.toContain('bg-destructive')
    const cancel = screen.getByRole('button', { name: 'Cancel' })
    expect(cancel.querySelector('kbd, [data-slot="kbd"]')).toBeNull()
    fireEvent.click(cancel)
    expect(onCancel).toHaveBeenCalled()
  })
})

describe('DecisionHistoryList', () => {
  it('shows status, chooser and rationale, and dims superseded entries', () => {
    render(<DecisionHistoryList decisions={[decision, { ...decision, id: 'd0', displayId: 'DEC-0', status: 'superseded' }]} optionTitle={() => 'Option A'} />)
    expect(screen.getAllByText('Option A')).toHaveLength(2)
    expect(screen.getByText('Superseded')).toBeInTheDocument()
    expect(screen.getByText('DEC-0').closest('li')).toHaveClass('opacity-60')
  })
  it('shows an empty line', () => {
    render(<DecisionHistoryList decisions={[]} />)
    expect(screen.getByText('No decisions recorded yet')).toBeInTheDocument()
  })
})
