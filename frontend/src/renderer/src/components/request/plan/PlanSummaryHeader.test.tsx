// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { PlanSummaryHeader } from './PlanSummaryHeader'
import { twoPhaseTree } from './plan-test-fixtures'

afterEach(cleanup)

describe('PlanSummaryHeader view switch', () => {
  it('has Tree and Graph only, unless a documents entry is asked for', () => {
    render(
      <PlanSummaryHeader tree={twoPhaseTree()} approval={null} view="tree" onViewChange={vi.fn()} />
    )
    expect(screen.getAllByRole('radio').map((r) => r.textContent)).toEqual(['Tree', 'Graph'])
    expect(screen.queryByTestId('plan-view-documents')).toBeNull()
  })

  it('names the third entry Documents for an OpenSpec change and Proposal for a native plan', () => {
    const { rerender } = render(
      <PlanSummaryHeader
        tree={twoPhaseTree()}
        approval={null}
        view="tree"
        onViewChange={vi.fn()}
        documentsView="documents"
      />
    )
    expect(screen.getAllByRole('radio').map((r) => r.textContent)).toEqual([
      'Tree',
      'Graph',
      'Documents'
    ])
    rerender(
      <PlanSummaryHeader
        tree={twoPhaseTree()}
        approval={null}
        view="tree"
        onViewChange={vi.fn()}
        documentsView="proposal"
      />
    )
    expect(screen.getAllByRole('radio').map((r) => r.textContent)).toEqual([
      'Tree',
      'Graph',
      'Proposal'
    ])
  })

  it('reports each of the three choices and ignores a deselect', () => {
    const onViewChange = vi.fn()
    render(
      <PlanSummaryHeader
        tree={twoPhaseTree()}
        approval={null}
        view="documents"
        onViewChange={onViewChange}
        documentsView="documents"
      />
    )
    fireEvent.click(screen.getByRole('radio', { name: 'Graph' }))
    fireEvent.click(screen.getByRole('radio', { name: 'Tree' }))
    expect(onViewChange.mock.calls).toEqual([['graph'], ['tree']])
    onViewChange.mockClear()
    fireEvent.click(screen.getByRole('radio', { name: 'Documents' }))
    expect(onViewChange).not.toHaveBeenCalled()
  })

  it('has no switch at all without a handler, as in the graph entry point', () => {
    render(<PlanSummaryHeader tree={twoPhaseTree()} approval={null} documentsView="documents" />)
    expect(screen.queryAllByRole('radio')).toHaveLength(0)
  })
})
