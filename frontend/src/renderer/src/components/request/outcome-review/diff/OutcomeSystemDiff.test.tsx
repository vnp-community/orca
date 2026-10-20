// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { render, screen, cleanup, fireEvent, within, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

vi.mock('sonner', () => ({ toast: Object.assign(vi.fn(), { error: vi.fn(), success: vi.fn() }) }))
const rpc = vi.fn()
vi.mock('../../../../runtime/request-rpc-client', () => ({
  callRequestRpc: (...a: unknown[]) => rpc(...a)
}))

import { resetReviewArtifactCache } from '../../../../hooks/useReviewArtifact'
import { OutcomeSystemDiff } from './OutcomeSystemDiff'
import { makeItem, makeModel, makeReview, makeVerdict } from '../outcome-review.fixture'
import type { OutcomeReview, ReviewSection } from '../../../../../../shared/outcome-review-types'
import type { RequestStatus } from '../../../../../../shared/request-types'

// The runtime's own localStorage is not reliable under this test environment; the code under test only needs the Storage API.
function memoryStorage(): Storage {
  const data = new Map<string, string>()
  return {
    get length() {
      return data.size
    },
    clear: () => data.clear(),
    getItem: (k: string) => data.get(k) ?? null,
    key: (i: number) => [...data.keys()][i] ?? null,
    removeItem: (k: string) => void data.delete(k),
    setItem: (k: string, v: string) => void data.set(k, v)
  }
}

beforeEach(() => {
  rpc.mockReset()
  resetReviewArtifactCache()
  Object.defineProperty(window, 'localStorage', { value: memoryStorage(), configurable: true })
  rpc.mockImplementation(async (_m: string, p: { artifactId: string }) => ({
    ok: true,
    value: {
      artifact: {
        artifactId: p.artifactId,
        section: 'architecture',
        variant: 'delta',
        blobDigest: `d-${p.artifactId}`,
        viewBox: '0 0 800 400',
        revision: 2,
        origin: 'auto',
        diagramType: 'architecture',
        skillVersion: 'diagram-design@abc',
        createdAt: '2026-10-20T10:00:00Z',
        title: `drawing ${p.artifactId}`
      },
      mediaType: 'text/html',
      content: '<svg><rect/></svg>'
    }
  }))
})
afterEach(cleanup)

const items = [
  makeItem({
    key: 'e1',
    section: 'architecture',
    change: 'added',
    title: 'Gateway calls bug management',
    origin: 'agent',
    expected: false,
    source: 'code',
    severity: 'deviation',
    evidence: [
      { kind: 'kg', ref: 'CT-02', snapshot: 'snap-4' },
      { kind: 'file', ref: 'backend-go/a.go' }
    ],
    containers: ['ms-gateway']
  }),
  makeItem({ key: 'e2', section: 'architecture', change: 'unchanged', title: 'Gateway exists' }),
  makeItem({ key: 'f1', section: 'flow', change: 'removed', title: 'Old call', origin: 'agent' }),
  makeItem({
    key: 'r1',
    section: 'requirements',
    change: 'unchanged',
    origin: 'fact',
    title: 'AC-1: one approval',
    before: 'planned',
    after: 'delivered',
    evidence: [{ kind: 'task', ref: 't1' }],
    detail:
      'All 1 task(s) tied to this criterion are done. Orca does not know which check proved it.'
  }),
  makeItem({
    key: 'c1',
    section: 'contracts',
    change: 'added',
    title: 'RPC GetThing',
    origin: 'fact',
    evidence: [{ kind: 'rpc', ref: 'GetThing' }]
  })
]
const diagrams = [
  {
    section: 'architecture',
    variant: 'design',
    type: 'architecture',
    inputDigest: 'd1',
    status: 'cached',
    artifactId: 'a-design'
  },
  {
    section: 'architecture',
    variant: 'delta',
    type: 'architecture',
    inputDigest: 'd2',
    status: 'cached',
    artifactId: 'a-delta'
  },
  { section: 'flow', variant: 'delta', type: 'sequence', inputDigest: 'd3', status: 'missing' },
  { section: 'data', variant: 'delta', type: 'database', inputDigest: 'd4', status: 'failed' }
]

function show(
  over: Partial<{
    review: OutcomeReview
    status: RequestStatus
    section: ReviewSection
    canRegenerate: boolean
  }> = {},
  handlers: Record<string, unknown> = {}
) {
  const review = over.review ?? makeReview({ model: { items, diagrams } })
  let section: ReviewSection = over.section ?? 'architecture'
  const view = (): React.JSX.Element => (
    <OutcomeSystemDiff
      requestId="r1"
      review={review}
      requestStatus={over.status ?? 'completed'}
      section={section}
      onSectionChange={(s) => {
        section = s
        r.rerender(view())
      }}
      canRegenerate={over.canRegenerate ?? true}
      {...handlers}
    />
  )
  const r = render(view())
  return r
}

describe('OutcomeSystemDiff', () => {
  it('lists the seven sections, Architecture and Flow first, with how many things are not in the design', () => {
    const review = makeReview({
      model: {
        items,
        diagrams,
        verdict: {
          ...makeModel().verdict,
          architecture: makeVerdict({ designChecked: true, unplannedEdges: 2 }),
          flow: makeVerdict({ designChecked: false })
        }
      }
    })
    show({ review })
    const names = screen.getAllByRole('tab').map((t) => t.getAttribute('data-testid'))
    expect(names).toEqual(
      ['architecture', 'flow', 'behavior', 'data', 'contracts', 'requirements', 'config'].map(
        (s) => `outcome-subtab-${s}`
      )
    )
    expect(screen.getByTestId('outcome-subtab-badge-architecture')).toHaveTextContent(
      '2 not in design'
    )
    // Without a design to compare with there is no "not in design" count; the tab falls back to the rows the viewer has not seen.
    expect(screen.getByTestId('outcome-subtab-badge-flow')).toHaveTextContent('1 new')
  })

  it('after code (the default once delivered): the delta drawing, the changed rows and the dots', async () => {
    show()
    expect(
      screen.getByTestId('outcome-mode-switch').querySelector('[data-mode="after_code"]')
    ).toHaveAttribute('aria-pressed', 'true')
    await waitFor(() => expect(screen.getAllByTestId('review-artifact-iframe')).toHaveLength(1))
    expect(screen.getByTestId('review-artifact')).toHaveAttribute('data-artifact-id', 'a-delta')
    const rows = screen.getAllByTestId('outcome-diff-row')
    expect(rows).toHaveLength(1)
    expect(rows[0]).toHaveAttribute('data-symbol', '+')
    expect(rows[0]).toHaveTextContent('Deviates from design')
    expect(rows[0]).toHaveTextContent('Not in design')
    expect(rows[0]).toHaveTextContent('Agent reported')
    expect(
      within(rows[0]).getByTestId('outcome-diff-dots').querySelectorAll('[role="img"]')
    ).toHaveLength(3)
  })

  it('the sandbox of every drawing is empty: no scripts and no same origin', async () => {
    show()
    const frame = await screen.findByTestId('review-artifact-iframe')
    expect(frame.getAttribute('sandbox')).toBe('')
    expect(frame).toHaveAttribute('referrerpolicy', 'no-referrer')
    expect(frame).toHaveAttribute('loading', 'lazy')
    expect(frame.getAttribute('srcdoc')).toContain("default-src 'none'")
    expect(frame.getAttribute('title')).toBe('drawing a-delta')
  })

  it('the design mode lists the design itself and draws the design diagram', async () => {
    show()
    fireEvent.click(
      screen.getByTestId('outcome-mode-switch').querySelector('[data-mode="design"]')!
    )
    await waitFor(() =>
      expect(screen.getByTestId('review-artifact')).toHaveAttribute('data-artifact-id', 'a-design')
    )
    expect(screen.getAllByTestId('outcome-diff-row')).toHaveLength(1)
    expect(screen.getByTestId('outcome-diff-row')).toHaveTextContent('Gateway exists')
    expect(screen.queryByTestId('outcome-diff-dots')).toBeNull()
  })

  it('side by side shows both drawings', async () => {
    show()
    fireEvent.click(
      screen.getByTestId('outcome-mode-switch').querySelector('[data-mode="side_by_side"]')!
    )
    await waitFor(() => expect(screen.getAllByTestId('review-artifact')).toHaveLength(2))
    expect(
      screen
        .getAllByTestId('review-artifact')
        .map((f) => f.getAttribute('data-artifact-id'))
        .sort()
    ).toEqual(['a-delta', 'a-design'])
  })

  it('before the code only Design can be chosen, and the others say when', () => {
    show({
      review: makeReview({ phase: 'design', model: { items, diagrams } }),
      status: 'executing'
    })
    const sw = screen.getByTestId('outcome-mode-switch')
    expect(sw.querySelector('[data-mode="design"]')).toHaveAttribute('aria-pressed', 'true')
    expect(sw.querySelector('[data-mode="after_code"]')).toHaveAttribute('aria-disabled', 'true')
    expect(sw.querySelector('[data-mode="after_code"]')).toHaveAttribute(
      'title',
      'Available when execution completes'
    )
    fireEvent.click(sw.querySelector('[data-mode="side_by_side"]')!)
    expect(sw.querySelector('[data-mode="design"]')).toHaveAttribute('aria-pressed', 'true')
  })

  it('Execution finished but no after-code review yet: it says to generate it first', () => {
    show({
      review: makeReview({ phase: 'design', model: { items, diagrams } }),
      status: 'completed'
    })
    expect(
      screen.getByTestId('outcome-mode-switch').querySelector('[data-mode="after_code"]')
    ).toHaveAttribute('title', 'Generate the after-code review first')
  })

  it('without the knowledge graph Design is dimmed with the reason and draws nothing', () => {
    const review = makeReview({
      model: {
        items,
        diagrams: diagrams.filter((d) => d.variant !== 'design') as never,
        kg: { available: false, gaps: [], conflicts: [] }
      }
    })
    show({ review })
    const design = screen.getByTestId('outcome-mode-switch').querySelector('[data-mode="design"]')!
    expect(design).toHaveAttribute('aria-disabled', 'true')
    expect(design).toHaveAttribute('title', 'Design not available')
  })

  it('a missing or failed drawing says so, keeps the rows, and offers a retry only to those who may', () => {
    const { unmount } = show({ section: 'flow' }, { onRegenerate: vi.fn() })
    expect(screen.getByTestId('review-artifact-notice')).toHaveTextContent(
      'No diagram for this change'
    )
    expect(screen.getByRole('button', { name: 'Try again' })).toBeInTheDocument()
    expect(screen.getAllByTestId('outcome-diff-row')).toHaveLength(1)
    unmount()
    show({ section: 'flow', canRegenerate: false }, { onRegenerate: vi.fn() })
    expect(screen.queryByRole('button', { name: 'Try again' })).toBeNull()
    expect(screen.getByTestId('review-artifact-notice')).toBeInTheDocument()
  })

  it('a failed drawing says it failed and the retry asks to draw that section again for that reason', () => {
    const onRegenerate = vi.fn()
    show({ section: 'data' }, { onRegenerate })
    expect(screen.getByTestId('review-artifact-notice')).toHaveTextContent('Drawing failed')
    fireEvent.click(screen.getByRole('button', { name: 'Try again' }))
    expect(onRegenerate).toHaveBeenCalledWith({ sections: ['data'], reason: 'retry_failed' })
  })

  it('evidence: a KG id is shown and copyable but is not a link; a file opens its diff when it can', async () => {
    const onOpenFile = vi.fn()
    const writeText = vi.fn(async () => undefined)
    Object.defineProperty(navigator, 'clipboard', { value: { writeText }, configurable: true })
    show({}, { onOpenFile })
    const chips = screen.getAllByTestId('evidence-chip')
    const kg = chips.find((c) => c.getAttribute('data-kind') === 'kg')!
    expect(kg).toHaveTextContent('CT-02 · snap-4')
    expect(kg.querySelector('a')).toBeNull()
    fireEvent.click(within(kg).getByRole('button', { name: 'Copy CT-02' }))
    await waitFor(() => expect(writeText).toHaveBeenCalledWith('CT-02'))
    fireEvent.click(screen.getByRole('button', { name: 'Open the diff of backend-go/a.go' }))
    expect(onOpenFile).toHaveBeenCalledWith('backend-go/a.go')
  })

  it('a file chip is plain text when the screen cannot open it', () => {
    show()
    expect(
      screen.getAllByTestId('evidence-chip').find((c) => c.getAttribute('data-kind') === 'file')
        ?.tagName
    ).toBe('SPAN')
  })

  it('marks rows seen in this browser only, and counts what is still new', () => {
    const review = makeReview({ model: { items, diagrams } })
    const { unmount } = show({ review, section: 'contracts' })
    expect(screen.getByTestId('outcome-subtab-badge-contracts')).toHaveTextContent('1 new')
    fireEvent.click(screen.getByRole('checkbox', { name: /Seen: RPC GetThing/ }))
    unmount()
    show({ review, section: 'contracts' })
    expect(screen.queryByTestId('outcome-subtab-badge-contracts')).toBeNull()
    expect(window.localStorage.getItem(`orca.outcomeReview.seen.${review.reviewId}`)).toContain(
      'c1'
    )
  })

  it('does not break when storage is blocked', () => {
    window.localStorage.setItem = () => {
      throw new Error('blocked')
    }
    show({ section: 'contracts' })
    fireEvent.click(screen.getByRole('checkbox', { name: /Seen: RPC GetThing/ }))
    expect(screen.getByRole('checkbox', { name: /Seen: RPC GetThing/ })).toBeChecked()
  })

  it('requirements: before → after for every criterion, said as tasks done and not as checks', () => {
    show({ section: 'requirements' })
    const row = screen.getByTestId('outcome-requirement-row')
    expect(row).toHaveTextContent('Planned')
    expect(row).toHaveTextContent('Delivered')
    expect(row).toHaveTextContent('Orca does not know which check proved it.')
    expect(within(row).getByTestId('evidence-chip')).toHaveTextContent('task: t1')
  })

  it('contracts and config wait for the code', () => {
    show({
      review: makeReview({ phase: 'design', model: { items, diagrams } }),
      status: 'executing',
      section: 'contracts'
    })
    expect(screen.getByTestId('outcome-appears-later')).toHaveTextContent(
      'This appears after execution completes.'
    )
    expect(screen.queryByTestId('outcome-diff-rows')).toBeNull()
  })

  it('the containers that changed are listed outside the drawing, with a way into the code review', async () => {
    const open = vi.fn()
    show(
      {},
      { renderContainerAction: (id: string) => <button onClick={() => open(id)}>go {id}</button> }
    )
    const list = screen.getByTestId('outcome-container-drilldown')
    expect(list).toHaveTextContent('ms-gateway')
    fireEvent.click(within(list).getByRole('button', { name: 'go ms-gateway' }))
    expect(open).toHaveBeenCalledWith('ms-gateway')
    expect(
      (await screen.findByTestId('review-artifact-iframe')).closest(
        '[data-testid="outcome-container-drilldown"]'
      )
    ).toBeNull()
  })

  it('"Show unchanged" brings back the rows that did not change', () => {
    show()
    expect(screen.getAllByTestId('outcome-diff-row')).toHaveLength(1)
    fireEvent.click(screen.getByRole('button', { name: 'Show unchanged' }))
    expect(screen.getAllByTestId('outcome-diff-row')).toHaveLength(2)
  })

  it('opening the same section again asks the server only about the digest, never to draw', async () => {
    const { unmount } = show()
    await waitFor(() => expect(screen.getByTestId('review-artifact-iframe')).toBeInTheDocument())
    unmount()
    rpc.mockClear()
    rpc.mockResolvedValue({
      ok: true,
      value: { artifact: { artifactId: 'a-delta', blobDigest: 'd-a-delta' }, notModified: true }
    })
    show()
    expect(screen.getByTestId('review-artifact-iframe')).toBeInTheDocument()
    await waitFor(() => expect(rpc).toHaveBeenCalled())
    expect(rpc.mock.calls.map((c) => c[0])).toEqual(['request.review.artifact'])
    expect(rpc.mock.calls[0][1]).toMatchObject({ ifDigest: 'd-a-delta' })
  })
})
