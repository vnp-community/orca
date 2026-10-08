// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { SolutionOptionCompare } from './SolutionOptionCompare'
import type { Solution } from '../../../../../shared/request-types'

afterEach(cleanup)

const opt = (id: string, extra = {}) => ({ id, title: `Option ${id}`, summary: `sum ${id}`, pros: ['p'], cons: ['c'], ...extra })
const solution = (n: number, extra: Partial<Solution> = {}): Solution => ({
  id: 's1', requestId: 'r1', kind: 'solution', status: 'ready',
  options: Array.from({ length: n }, (_, i) => opt(`o${i}`)), ...extra
})

function setup(s: Solution, props: Partial<React.ComponentProps<typeof SolutionOptionCompare>> = {}) {
  const onSelect = vi.fn()
  const view = render(
    <SolutionOptionCompare solution={s} requestType="change_request" selectedId={null} onSelect={onSelect} readOnly={false} {...props} />
  )
  return { onSelect, ...view }
}

describe('SolutionOptionCompare', () => {
  it.each([1, 2, 3])('renders %i option card(s)', (n) => {
    setup(solution(n))
    expect(screen.getAllByRole('radio')).toHaveLength(n)
  })

  it('warns when a change_request has fewer than 2 options and offers regenerate', () => {
    const onRegenerate = vi.fn()
    setup(solution(1), { onRegenerate })
    expect(screen.getByRole('alert')).toHaveTextContent('At least 2 options')
    fireEvent.click(screen.getByRole('button', { name: 'Regenerate' }))
    expect(onRegenerate).toHaveBeenCalled()
  })

  it('selects, reflects aria-checked and toggles to the comparison table', () => {
    const { onSelect, rerender } = setup(solution(2))
    fireEvent.click(screen.getByTestId('solution-option-o1'))
    expect(onSelect).toHaveBeenCalledWith('o1')
    rerender(<SolutionOptionCompare solution={solution(2)} requestType="change_request" selectedId="o1" onSelect={onSelect} readOnly={false} />)
    expect(screen.getByTestId('solution-option-o1')).toHaveAttribute('aria-checked', 'true')
    expect(screen.getByTestId('solution-option-o0')).toHaveAttribute('aria-checked', 'false')
    fireEvent.click(screen.getByRole('button', { name: /Compare/ }))
    expect(screen.getByTestId('solution-comparison-table')).toBeInTheDocument()
    expect(screen.getAllByLabelText('Differs between options').length).toBeGreaterThan(0)
    fireEvent.click(screen.getByRole('button', { name: /Cards/ }))
    expect(screen.getAllByRole('radio')).toHaveLength(2)
  })

  it('does not select when readOnly and marks the chosen option', () => {
    const s = solution(2, { status: 'chosen', chosenOptionId: 'o1' })
    const { onSelect } = setup(s, { readOnly: true })
    fireEvent.click(screen.getByTestId('solution-option-o0'))
    expect(onSelect).not.toHaveBeenCalled()
    expect(screen.getByText('Chosen')).toBeInTheDocument()
  })

  it('moves selection with arrow keys', () => {
    const { onSelect } = setup(solution(3), { selectedId: 'o0' })
    fireEvent.keyDown(screen.getByTestId('solution-option-o0'), { key: 'ArrowRight' })
    expect(onSelect).toHaveBeenLastCalledWith('o1')
    fireEvent.keyDown(screen.getByTestId('solution-option-o0'), { key: 'ArrowLeft' })
    expect(onSelect).toHaveBeenLastCalledWith('o2')
  })

  it('tolerates options with missing fields', () => {
    setup({ ...solution(0), options: [{ id: 'x', title: '' }, { id: 'y', title: '' }] })
    expect(screen.getAllByText('No data').length).toBeGreaterThan(0)
  })
})
