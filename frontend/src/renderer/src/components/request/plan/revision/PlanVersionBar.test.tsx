// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import type { PlanRevision } from '../../../../../../shared/request-plan-revision'
import { PlanVersionBar } from './PlanVersionBar'
import type { PlanVersionsApi } from './usePlanVersions'

afterEach(cleanup)

const rev = (n: number, over: Partial<PlanRevision> = {}): PlanRevision => ({
  revision: n,
  runId: `run-${n}`,
  status: n === 3 ? 'current' : 'superseded',
  engine: 'openspec',
  revisionFeedback: '',
  actorId: '',
  generatedAt: '2026-10-20T10:00:00Z',
  commit: '',
  filesChanged: 0,
  fileCount: 0,
  textKept: true,
  ...over
})

function versions(over: Partial<PlanVersionsApi> = {}, revisions = [rev(3), rev(2), rev(1)]) {
  const api: PlanVersionsApi = {
    list: {
      revisions,
      current: revisions.find((r) => r.status === 'current') ?? null,
      hasMore: false,
      loadMore: vi.fn(),
      isLoading: false,
      isLoadingMore: false,
      error: null,
      supported: true,
      refetch: vi.fn()
    },
    shown: null,
    viewingOlder: false,
    newerAvailable: false,
    select: vi.fn(),
    backToCurrent: vi.fn(),
    ...over
  }
  return api
}

describe('PlanVersionBar', () => {
  it('renders nothing for one version, nothing on a gateway without versions', () => {
    const { container, rerender } = render(
      <PlanVersionBar versions={versions({}, [rev(3)])} view="tree" onSelect={vi.fn()} />
    )
    expect(container).toBeEmptyDOMElement()
    const off = versions()
    off.list.supported = false
    rerender(<PlanVersionBar versions={off} view="tree" onSelect={vi.fn()} />)
    expect(container).toBeEmptyDOMElement()
  })

  it('lists the versions in a disclosure and reports the choice', () => {
    const onSelect = vi.fn()
    render(<PlanVersionBar versions={versions()} view="documents" onSelect={onSelect} />)
    expect(screen.getByRole('button', { name: /Versions \(3\)/ })).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: /Versions \(3\)/ }))
    fireEvent.click(screen.getByRole('button', { name: /Version 2/ }))
    expect(onSelect).toHaveBeenCalledWith(2)
    expect(screen.queryByTestId('plan-older-version-banner')).toBeNull()
  })

  it('says an older version is read only and the way back is one click', () => {
    const v = versions({ viewingOlder: true, shown: rev(1) })
    render(<PlanVersionBar versions={v} view="documents" onSelect={vi.fn()} />)
    expect(screen.getByTestId('plan-older-version-banner')).toHaveTextContent(
      "You're viewing an older version."
    )
    expect(screen.getByTestId('plan-tree-current-note')).toHaveTextContent(
      'The tree shows the current version.'
    )
    fireEvent.click(screen.getByTestId('plan-back-to-current'))
    expect(v.backToCurrent).toHaveBeenCalledTimes(1)
  })

  it('leaves the tree reminder out when the tree itself is on screen', () => {
    render(
      <PlanVersionBar
        versions={versions({ viewingOlder: true, shown: rev(1) })}
        view="tree"
        onSelect={vi.fn()}
      />
    )
    expect(screen.getByTestId('plan-older-version-banner')).toBeInTheDocument()
    expect(screen.queryByTestId('plan-tree-current-note')).toBeNull()
  })

  it('announces a newer version with a way to show it', () => {
    const v = versions({ viewingOlder: true, shown: rev(1), newerAvailable: true })
    render(<PlanVersionBar versions={v} view="documents" onSelect={vi.fn()} />)
    expect(screen.getByTestId('plan-newer-version-notice')).toHaveTextContent(
      'A newer version is available.'
    )
    fireEvent.click(screen.getByTestId('plan-show-newer'))
    expect(v.backToCurrent).toHaveBeenCalled()
  })

  it('shows the whole request that produced the version on screen, only when there is one', () => {
    const long = `Please split the migration.\n${'More detail. '.repeat(30)}`
    const { rerender } = render(
      <PlanVersionBar
        versions={versions({}, [rev(3, { revisionFeedback: long }), rev(2), rev(1)])}
        view="tree"
        onSelect={vi.fn()}
      />
    )
    fireEvent.click(screen.getByRole('button', { name: 'What was asked for this version' }))
    const text = screen.getByText(/Please split the migration/)
    expect(text).toHaveClass('whitespace-pre-wrap')
    expect(text.textContent).toContain('More detail.')
    rerender(<PlanVersionBar versions={versions()} view="tree" onSelect={vi.fn()} />)
    expect(screen.queryByTestId('plan-version-asked')).toBeNull()
  })

  it('shows the request of the older version when that one is on screen', () => {
    const older = rev(2, { revisionFeedback: 'Add tests' })
    render(
      <PlanVersionBar
        versions={versions({ viewingOlder: true, shown: older }, [rev(3), older, rev(1)])}
        view="documents"
        onSelect={vi.fn()}
      />
    )
    fireEvent.click(screen.getByRole('button', { name: 'What was asked for this version' }))
    expect(screen.getByText('Add tests')).toBeInTheDocument()
  })
})
