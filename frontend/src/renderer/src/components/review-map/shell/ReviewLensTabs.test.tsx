// @vitest-environment happy-dom
import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import {
  getReviewLenses,
  type ReviewLensDefinition,
  type ReviewLensProps
} from '../review-lens-registry'
import { makeOverlay } from '../review-test-data'
import { ReviewLensTabs } from './ReviewLensTabs'

afterEach(cleanup)

const props: ReviewLensProps = {
  worktreeId: 'w',
  environmentId: null,
  scope: { kind: 'branch', baseRef: 'main', includeUncommitted: true },
  overlay: makeOverlay(),
  selectedSymbolKey: null,
  chipFilter: null,
  onSelectSymbol: vi.fn(),
  onOpenDiff: vi.fn(),
  requestSymbolChoice: vi.fn()
}

describe('ReviewLensTabs', () => {
  const lenses = getReviewLenses({ quality: false })

  it('renders one tab per registered lens, in registry order', () => {
    render(
      <ReviewLensTabs
        lenses={lenses}
        activeLensId="impact"
        onSelectLens={vi.fn()}
        lensProps={props}
      />
    )
    expect(screen.getAllByRole('tab').map((t) => t.getAttribute('data-lens'))).toEqual(
      lenses.map((l) => l.id)
    )
  })

  it('lenses without a loader show the placeholder', () => {
    // Why: every canonical lens now ships a loader, so use a synthetic one without it.
    const bare: ReviewLensDefinition = {
      id: 'bare',
      order: 1,
      labelKey: 'x.bare',
      labelFallback: 'Bare'
    }
    render(
      <ReviewLensTabs
        lenses={[bare]}
        activeLensId="bare"
        onSelectLens={vi.fn()}
        lensProps={props}
      />
    )
    expect(screen.getByText('Bare is not available yet.')).toBeTruthy()
  })

  it('adding a lens definition needs no change to the tabs component', async () => {
    const extra: ReviewLensDefinition = {
      id: 'extra',
      order: 5,
      labelKey: 'x',
      labelFallback: 'Extra',
      load: async () => ({
        default: (p: ReviewLensProps) => <div data-testid="extra-body">{p.worktreeId}</div>
      })
    }
    render(
      <ReviewLensTabs
        lenses={[extra, ...lenses]}
        activeLensId="extra"
        onSelectLens={vi.fn()}
        lensProps={props}
      />
    )
    await waitFor(() => expect(screen.getByTestId('extra-body').textContent).toBe('w'))
    expect(screen.getAllByRole('tab')[0].textContent).toBe('Extra')
  })

  it('selecting a tab reports the lens id', () => {
    const onSelectLens = vi.fn()
    render(
      <ReviewLensTabs
        lenses={lenses}
        activeLensId="impact"
        onSelectLens={onSelectLens}
        lensProps={props}
      />
    )
    fireEvent.mouseDown(screen.getByRole('tab', { name: 'ERD' }))
    fireEvent.click(screen.getByRole('tab', { name: 'ERD' }))
    expect(onSelectLens).toHaveBeenCalledWith('erd')
  })
})
