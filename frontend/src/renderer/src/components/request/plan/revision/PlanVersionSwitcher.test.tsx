// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import type { PlanRevision } from '../../../../../../shared/request-plan-revision'
import { PlanVersionSwitcher } from './PlanVersionSwitcher'

afterEach(cleanup)

const rev = (n: number, over: Partial<PlanRevision> = {}): PlanRevision => ({
  revision: n,
  runId: `run-${n}`,
  status: 'superseded',
  engine: 'openspec',
  revisionFeedback: '',
  actorId: 'u1',
  generatedAt: '2026-10-20T10:00:00Z',
  commit: '',
  filesChanged: 0,
  fileCount: 0,
  textKept: true,
  ...over
})

function setup(
  revisions: PlanRevision[],
  over: Partial<Parameters<typeof PlanVersionSwitcher>[0]> = {}
) {
  const props = {
    revisions,
    shownRevision: revisions[0]?.revision ?? 1,
    hasMore: false,
    loadingMore: false,
    onLoadMore: vi.fn(),
    onSelect: vi.fn(),
    ...over
  }
  render(<PlanVersionSwitcher {...props} />)
  return props
}

describe('PlanVersionSwitcher', () => {
  it('is hidden for a single version', () => {
    setup([rev(1, { status: 'current' })])
    expect(screen.queryByTestId('plan-version-list')).toBeNull()
  })

  it('lists every version newest first with a text badge, the request and the files it changed', () => {
    setup([
      rev(3, {
        status: 'current',
        revisionFeedback: 'Split the migration',
        filesChanged: 2,
        fileCount: 5
      }),
      rev(2, { revisionFeedback: 'Add tests' }),
      rev(1)
    ])
    const items = screen.getAllByRole('button')
    expect(items).toHaveLength(3)
    expect(items[0]).toHaveTextContent('Version 3')
    expect(items[0]).toHaveTextContent('Current')
    expect(items[0]).toHaveTextContent('Split the migration')
    expect(items[0]).toHaveTextContent('2 files changed')
    expect(items[1]).toHaveTextContent('Superseded')
    expect(items[2]).toHaveTextContent('First proposal')
  })

  it('marks the version on screen and reports a choice', () => {
    const props = setup([rev(2, { status: 'current' }), rev(1)], { shownRevision: 2 })
    expect(screen.getByRole('button', { name: /Version 2/ })).toHaveAttribute(
      'aria-pressed',
      'true'
    )
    expect(screen.getByRole('button', { name: /Version 1/ })).toHaveAttribute(
      'aria-pressed',
      'false'
    )
    fireEvent.click(screen.getByRole('button', { name: /Version 1/ }))
    expect(props.onSelect).toHaveBeenCalledWith(1)
  })

  it('gives the time in full in the title', () => {
    setup([rev(2, { status: 'current' }), rev(1)])
    expect(document.querySelector('time')?.getAttribute('title')).toBeTruthy()
  })

  it('shows what a run cost when it reported it', () => {
    setup([rev(2, { status: 'current', stats: { durationMs: 60_000, costUsd: 2 } }), rev(1)])
    expect(screen.getByRole('button', { name: /Version 2/ })).toHaveTextContent('1m 0s, $2.00')
  })

  it('says a version whose text was pruned has none, and still lists it', () => {
    setup([rev(2, { status: 'current' }), rev(1, { textKept: false })])
    expect(screen.getByTestId('plan-version-pruned')).toHaveTextContent(
      'The text of this version is no longer kept.'
    )
  })

  it('reads older versions page by page, and not after the last page', () => {
    const props = setup([rev(2, { status: 'current' }), rev(1)], { hasMore: true })
    fireEvent.click(screen.getByTestId('plan-version-older'))
    expect(props.onLoadMore).toHaveBeenCalledTimes(1)
    cleanup()
    setup([rev(2, { status: 'current' }), rev(1)])
    expect(screen.queryByTestId('plan-version-older')).toBeNull()
  })

  it('keeps a hundred versions in one bounded, scrolling list', () => {
    setup(Array.from({ length: 100 }, (_, i) => rev(100 - i, i === 0 ? { status: 'current' } : {})))
    expect(screen.getAllByRole('button')).toHaveLength(100)
    expect(screen.getByRole('list')).toHaveClass('overflow-y-auto')
    expect(screen.getByRole('list')).toHaveClass('max-h-60')
  })
})
