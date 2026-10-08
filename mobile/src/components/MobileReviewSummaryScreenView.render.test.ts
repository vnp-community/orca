import { createElement } from 'react'
import { renderToStaticMarkup } from 'react-dom/server'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { PressRegistry } from '../test-support/react-native-static-render-mock'

const presses = vi.hoisted(() => new Map<string, () => void>() as PressRegistry)
const windowSize = vi.hoisted(() => ({ width: 390, height: 844 }))

vi.mock('react-native', async () =>
  (await import('../test-support/react-native-static-render-mock')).createReactNativeStaticMock(
    presses,
    windowSize
  )
)
vi.mock('react-native-safe-area-context', async () => ({
  SafeAreaView: (
    await import('../test-support/react-native-static-render-mock')
  ).createReactNativeStaticMock(presses, windowSize).SafeAreaView
}))
vi.mock('lucide-react-native', () => {
  const icon = () => null
  return { ChevronLeft: icon, RefreshCw: icon, AlertCircle: icon, AlertTriangle: icon, Info: icon }
})

import { MobileReviewSummaryScreenView } from './MobileReviewSummaryScreenView'
import type { MobileReviewSummaryState } from '../session/mobile-review-summary-loaders'

const FINDINGS = [
  {
    key: 'w1',
    kind: 'structure',
    severity: 'warning' as const,
    title: 'Layer violation',
    summary: 'UI imports transport',
    origin: 'touched' as const,
    filePath: 'src/ui.tsx',
    startLine: 7,
    inChangedFiles: true
  },
  {
    key: 'e1',
    kind: 'quality',
    severity: 'error' as const,
    title: 'Unhandled rejection',
    summary: 'connect() can reject',
    origin: 'introduced' as const,
    filePath: 'src/rpc.ts',
    startLine: 42,
    inChangedFiles: true
  }
]

function controller(screenState: MobileReviewSummaryState, over: Record<string, unknown> = {}) {
  return {
    screenState,
    connState: 'connected',
    filter: 'all',
    setFilter: vi.fn(),
    expandedKey: null,
    toggleExpanded: vi.fn(),
    refreshing: false,
    refresh: vi.fn(async () => {}),
    reconnect: vi.fn(),
    openFileDiff: vi.fn(),
    worktreeLabel: 'feature/x',
    ...over
  } as never as Parameters<typeof MobileReviewSummaryScreenView>[0]['controller']
}

const readyState: MobileReviewSummaryState = {
  kind: 'ready',
  summary: {
    available: true,
    index: { state: 'stale', indexedCommit: 'abcdef123456' },
    counts: { files: 6, symbols: 31, flows: 2, tables: 1, contracts: 1, uncovered: 3 },
    risk: { level: 'HIGH', reasons: ['Touches reconnect flow'] },
    findings: {
      totalOpen: 120,
      bySeverity: { error: 1, warning: 1, info: 0 },
      items: FINDINGS,
      truncated: true
    },
    stale: true,
    truncated: true
  }
}

function render(c: ReturnType<typeof controller>, onBack = vi.fn()): string {
  return renderToStaticMarkup(
    createElement(MobileReviewSummaryScreenView, { controller: c, onBack })
  )
}

beforeEach(() => {
  presses.clear()
  windowSize.width = 390
  windowSize.height = 844
})

describe('MobileReviewSummaryScreenView', () => {
  it('shows a spinner while loading', () => {
    const html = render(controller({ kind: 'loading' }))
    expect(html).toContain('<progress')
    expect(html).toContain('feature/x')
  })

  it('shows the unavailable message without a retry button', () => {
    const html = render(
      controller({ kind: 'unavailable', message: 'Code intelligence is turned off' })
    )
    expect(html).toContain('Code intelligence is turned off')
    expect(presses.has('Retry')).toBe(false)
  })

  it('error state offers Retry wired to refresh', () => {
    const c = controller({ kind: 'error', message: 'Boom' })
    const html = render(c)
    expect(html).toContain('Boom')
    presses.get('Retry')?.()
    expect(c.refresh).toHaveBeenCalledTimes(1)
  })

  it('ready state renders the stale banner, risk, metrics, truncation and sorted findings', () => {
    const html = render(controller(readyState))
    expect(html).toContain('The index is behind the latest commit.')
    expect(html).toContain('Index abcdef1')
    expect(html).toContain('Risk: High')
    expect(html).toContain('Touches reconnect flow')
    expect(html).toContain('Findings (120)')
    expect(html).toContain('data-refreshing="false"')
    // Errors sort ahead of warnings regardless of host order.
    expect(html.indexOf('Unhandled rejection')).toBeLessThan(html.indexOf('Layer violation'))
    expect(html).not.toContain('View file diff')
  })

  it('expanded finding shows its summary and opens the file diff', () => {
    const c = controller(readyState, { expandedKey: 'e1' })
    const html = render(c)
    expect(html).toContain('connect() can reject')
    presses.get('View file diff')?.()
    expect(c.openFileDiff).toHaveBeenCalledWith(FINDINGS[1])
  })

  it('filter chips call setFilter and narrow the list', () => {
    const c = controller(readyState, { filter: 'error' })
    const html = render(c)
    expect(html).not.toContain('Layer violation')
    expect(html).toContain('Unhandled rejection')
    expect(presses.size).toBeGreaterThan(3)
  })

  it('header back and refresh are wired', () => {
    const onBack = vi.fn()
    const c = controller({ kind: 'loading' })
    render(c, onBack)
    const labels = [...presses.keys()]
    const back = labels.find((l) => /back/i.test(l))
    const refresh = labels.find((l) => /refresh/i.test(l))
    expect(back && refresh).toBeTruthy()
    presses.get(back!)?.()
    presses.get(refresh!)?.()
    expect(onBack).toHaveBeenCalledTimes(1)
    expect(c.refresh).toHaveBeenCalledTimes(1)
  })

  it('renders both columns side by side on a wide tablet layout', () => {
    const rows = (html: string) => html.split('data-row').length
    const narrow = rows(render(controller(readyState)))
    windowSize.width = 1194
    windowSize.height = 834
    const html = render(controller(readyState))
    expect(html).toContain('Risk: High')
    expect(rows(html)).toBe(narrow + 1)
  })
})
