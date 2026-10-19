// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { cleanup, fireEvent, render, screen, within } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import type { Solution } from '../../../../../shared/request-types'
import { SolutionVersionSwitcher } from './SolutionVersionSwitcher'

afterEach(cleanup)

const sol = (over: Partial<Solution> & { id: string }): Solution => ({
  requestId: 'r1',
  kind: 'solution',
  status: 'ready',
  ...over
})

const history = (): Solution[] => [
  sol({
    id: 's3',
    seq: 3,
    revisionFeedback: 'Add a rollback plan',
    generatedAt: '2026-10-19T10:00:00Z'
  }),
  sol({ id: 's2', seq: 2, status: 'superseded', revisionFeedback: 'Compare on cost' }),
  sol({ id: 's1', seq: 1, status: 'superseded' })
]

describe('SolutionVersionSwitcher', () => {
  it('renders nothing for fewer than two versions', () => {
    const { container } = render(
      <SolutionVersionSwitcher solutions={[sol({ id: 'a' })]} currentId="a" onSelect={vi.fn()} />
    )
    expect(container).toBeEmptyDOMElement()
  })

  it('shows version numbers from seq with text badges, notes and the first proposal', () => {
    render(<SolutionVersionSwitcher solutions={history()} currentId="s3" onSelect={vi.fn()} />)
    const rows = screen.getAllByRole('button')
    expect(rows).toHaveLength(3)
    expect(rows[0]).toHaveTextContent('Version 3')
    expect(rows[0]).toHaveTextContent('Current')
    expect(rows[0]).toHaveTextContent('Add a rollback plan')
    expect(rows[1]).toHaveTextContent('Version 2')
    expect(rows[1]).toHaveTextContent('Superseded')
    expect(rows[2]).toHaveTextContent('Version 1')
    expect(rows[2]).toHaveTextContent('First proposal')
  })

  it('labels rejected and approved versions with text', () => {
    render(
      <SolutionVersionSwitcher
        solutions={[
          sol({ id: 'b', seq: 2, status: 'chosen' }),
          sol({ id: 'a', seq: 1, status: 'rejected' })
        ]}
        currentId="b"
        onSelect={vi.fn()}
      />
    )
    expect(screen.getByText('Approved')).toBeInTheDocument()
    expect(screen.getByText('Rejected')).toBeInTheDocument()
  })

  it('numbers versions by order when the gateway sent no seq', () => {
    render(
      <SolutionVersionSwitcher
        solutions={[
          sol({ id: 'c', generatedAt: '2026-10-19T12:00:00Z' }),
          sol({ id: 'b', status: 'superseded', generatedAt: '2026-10-19T11:00:00Z' }),
          sol({ id: 'a', status: 'superseded', generatedAt: '2026-10-19T10:00:00Z' })
        ]}
        currentId="c"
        onSelect={vi.fn()}
      />
    )
    const rows = screen.getAllByRole('button')
    expect(rows.map((r) => /Version (\d+)/.exec(r.textContent ?? '')?.[1])).toEqual(['3', '2', '1'])
  })

  it('marks the version on screen and reports a selection', () => {
    const onSelect = vi.fn()
    render(<SolutionVersionSwitcher solutions={history()} currentId="s3" onSelect={onSelect} />)
    expect(screen.getAllByRole('button')[0]).toHaveAttribute('aria-pressed', 'true')
    fireEvent.click(screen.getAllByRole('button')[1])
    expect(onSelect).toHaveBeenCalledWith('s2')
  })

  it('cuts a long note to 80 characters and shows feedback as plain text', () => {
    const solutions = [
      sol({ id: 'b', seq: 2, revisionFeedback: `<script>alert(1)</script>${'x'.repeat(200)}` }),
      sol({ id: 'a', seq: 1, status: 'superseded' })
    ]
    const { container } = render(
      <SolutionVersionSwitcher solutions={solutions} currentId="b" onSelect={vi.fn()} />
    )
    const note = within(screen.getAllByRole('button')[0]).getByTestId('solution-version-note')
    expect([...(note.textContent ?? '')]).toHaveLength(80)
    expect(note.textContent?.startsWith('<script>')).toBe(true)
    expect(container.querySelector('script')).toBeNull()
  })

  describe('with 100 versions', () => {
    const many = Array.from({ length: 100 }, (_, i) =>
      sol({ id: `s${100 - i}`, seq: 100 - i, status: i === 0 ? 'ready' : 'superseded' })
    )

    it('shows 20 at a time and pages through all of them', () => {
      render(<SolutionVersionSwitcher solutions={many} currentId="s100" onSelect={vi.fn()} />)
      expect(
        screen.getAllByRole('button').filter((b) => b.hasAttribute('data-version-id'))
      ).toHaveLength(20)
      expect(screen.getByText('1–20 of 100')).toBeInTheDocument()
      expect(screen.getByTestId('solution-version-newer')).toBeDisabled()
      fireEvent.click(screen.getByTestId('solution-version-older'))
      expect(screen.getByText('21–40 of 100')).toBeInTheDocument()
      expect(screen.getAllByRole('button')[0]).toHaveTextContent('Version 80')
      for (let i = 0; i < 3; i += 1) {
        fireEvent.click(screen.getByTestId('solution-version-older'))
      }
      expect(screen.getByText('81–100 of 100')).toBeInTheDocument()
      expect(screen.getByTestId('solution-version-older')).toBeDisabled()
    })

    it('has no pager for 20 versions or fewer', () => {
      render(
        <SolutionVersionSwitcher
          solutions={many.slice(0, 20)}
          currentId="s100"
          onSelect={vi.fn()}
        />
      )
      expect(screen.queryByTestId('solution-version-older')).toBeNull()
    })

    it('follows the selection to its page, but keeps the page when only the list grows', () => {
      const { rerender } = render(
        <SolutionVersionSwitcher solutions={many} currentId="s100" onSelect={vi.fn()} />
      )
      fireEvent.click(screen.getByTestId('solution-version-older'))
      rerender(
        <SolutionVersionSwitcher solutions={[...many]} currentId="s100" onSelect={vi.fn()} />
      )
      expect(screen.getByText('21–40 of 100')).toBeInTheDocument()
      rerender(<SolutionVersionSwitcher solutions={many} currentId="s50" onSelect={vi.fn()} />)
      expect(screen.getByText('41–60 of 100')).toBeInTheDocument()
      rerender(<SolutionVersionSwitcher solutions={many} currentId="s100" onSelect={vi.fn()} />)
      expect(screen.getByText('1–20 of 100')).toBeInTheDocument()
    })
  })
})
