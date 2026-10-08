import { createElement } from 'react'
import { renderToStaticMarkup } from 'react-dom/server'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { PressRegistry } from '../test-support/react-native-static-render-mock'

const presses = vi.hoisted(() => new Map<string, () => void>() as PressRegistry)

vi.mock('react-native', async () =>
  (await import('../test-support/react-native-static-render-mock')).createReactNativeStaticMock(
    presses,
    { width: 390, height: 844 }
  )
)
vi.mock('lucide-react-native', () => {
  const icon = () => null
  return {
    AlertTriangle: icon,
    Check: icon,
    ChevronRight: icon,
    CircleDot: icon,
    GitBranch: icon,
    GitPullRequest: icon,
    MessageSquare: icon,
    X: icon
  }
})

import { MobileSourceControlBranchCard } from './MobileSourceControlBranchCard'

function card(over: Record<string, unknown> = {}): string {
  return renderToStaticMarkup(
    createElement(MobileSourceControlBranchCard, {
      branchLabel: 'feature/x',
      syncLabel: null,
      unstagedCount: 1,
      stagedCount: 0,
      branchCount: 2,
      conflictOperation: null,
      conflictBusy: false,
      conflictAborting: false,
      onAbortConflict: vi.fn(),
      prChip: null,
      onOpenPr: vi.fn(),
      ...over
    })
  )
}

beforeEach(() => presses.clear())

describe('MobileSourceControlBranchCard review summary chip', () => {
  it('renders no chip until availability is known', () => {
    expect(card()).not.toContain('Review summary')
    expect(card({ reviewSummaryChip: null, onOpenReviewSummary: vi.fn() })).not.toContain(
      'Review summary'
    )
  })

  it('renders the chip with a text detail and opens the summary on press', () => {
    const onOpenReviewSummary = vi.fn()
    const html = card({
      reviewSummaryChip: {
        label: 'Review summary',
        detail: 'Risk High · 2 open findings',
        tone: 'danger'
      },
      onOpenReviewSummary
    })
    expect(html).toContain('Review summary')
    expect(html).toContain('Risk High · 2 open findings')
    presses.get('Review summary: Risk High · 2 open findings')?.()
    expect(onOpenReviewSummary).toHaveBeenCalledTimes(1)
  })
})
