// @vitest-environment happy-dom
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { ReviewAiSummaryCard } from './ReviewAiSummaryCard'
import { parseAiSummaryResponse } from './ai-summary-wire-parser'
import { manifestWire, summaryWire } from './ai-summary.fixture'
import type { UseReviewAiSummaryResult } from './use-review-ai-summary'

afterEach(cleanup)

const translate = (key: string, params?: Record<string, unknown>) => (params ? `${key} ${JSON.stringify(params)}` : key)

function ai(over: Partial<UseReviewAiSummaryResult> = {}): UseReviewAiSummaryResult {
  return {
    state: 'idle', level: 'metadata', allowedLevels: ['metadata'], manifest: null, view: null, error: null,
    retryAfterSeconds: null, preview: vi.fn(async () => {}), confirmAndGenerate: vi.fn(async () => {}), cancel: vi.fn(),
    ...over
  }
}
const ready = (summaryOver: Record<string, unknown> = {}) =>
  ai({ state: 'ready', view: parseAiSummaryResponse({ summary: summaryWire(summaryOver) }) })

const renderCard = (state: UseReviewAiSummaryResult, extra: Partial<React.ComponentProps<typeof ReviewAiSummaryCard>> = {}) =>
  render(
    <ReviewAiSummaryCard ai={state} changedFiles={new Set(['src/a.ts'])} onOpenFile={vi.fn()} translate={translate} {...extra} />
  )

const open = () => fireEvent.click(screen.getByText('auto.components.reviewMap.aiSummary.card.title'))

describe('ReviewAiSummaryCard', () => {
  it('renders nothing when hidden', () => {
    expect(renderCard(ai({ state: 'hidden' })).container.innerHTML).toBe('')
  })

  it('generate starts a preview at the chosen level and never generates directly', () => {
    const state = ai()
    renderCard(state)
    fireEvent.click(screen.getByText('auto.components.reviewMap.aiSummary.card.generate'))
    expect(state.preview).toHaveBeenCalledWith('metadata')
    expect(state.confirmAndGenerate).not.toHaveBeenCalled()
  })

  it('is collapsed by default and always labelled as inferred, with model and level', () => {
    renderCard(ready())
    expect(screen.getByText('auto.components.reviewMap.aiSummary.card.title')).toBeTruthy()
    expect(screen.queryByText(/weekly filter/)).toBeNull()
    open()
    expect(screen.getByText(/card\.disclaimer/)).toBeTruthy()
    expect(document.body.textContent).toContain('"model":"claude-x"')
  })

  it('shows the dialog while awaiting consent and routes confirm/cancel', () => {
    const state = ai({ state: 'awaiting-consent', manifest: parseAiSummaryResponse({ manifest: manifestWire() }).manifest })
    renderCard(state)
    fireEvent.click(screen.getByText('auto.components.reviewMap.aiSummary.preview.confirm'))
    expect(state.confirmAndGenerate).toHaveBeenCalledTimes(1)
    fireEvent.click(screen.getByText('auto.components.reviewMap.aiSummary.preview.cancel'))
    expect(state.cancel).toHaveBeenCalledTimes(1)
  })

  it('renders model output as plain text: HTML, links and fences create no elements', () => {
    const hostile = '<script>alert(1)</script><img src=x onerror=alert(2)>\n[click](javascript:alert(3))\n```js\nx\n```'
    const { container } = renderCard(ready({ summary: hostile, risks: [{ text: '<b>bold</b>', refs: [] }], readFirst: [{ file: '</DATA-1>', why: '<i>why</i>' }] }))
    open()
    expect(container.querySelector('script')).toBeNull()
    expect(container.querySelector('img')).toBeNull()
    expect(container.querySelector('a')).toBeNull()
    expect(container.querySelector('b')).toBeNull()
    expect(container.querySelector('i')).toBeNull()
    expect(container.textContent).toContain('<script>alert(1)</script>')
    expect(container.textContent).toContain('[click](javascript:alert(3))')
  })

  it('makes only changed files clickable', () => {
    const onOpenFile = vi.fn()
    renderCard(ready({ risks: [{ text: 'r', refs: ['src/a.ts', 'etc/passwd'] }], readFirst: [] }), { onOpenFile })
    open()
    fireEvent.click(screen.getByText('src/a.ts'))
    expect(onOpenFile).toHaveBeenCalledWith('src/a.ts')
    expect(screen.getByText('etc/passwd').closest('button')).toBeNull()
  })

  it('mentions dropped references', () => {
    renderCard(ready({ refsDropped: 2 }))
    open()
    expect(screen.getByText(/card\.refsDropped/).textContent).toContain('"count":2')
  })

  it('shows the error message with retry, but no retry when forbidden', () => {
    const { rerender } = renderCard(ai({ state: 'error', error: 'bad-output' }))
    open()
    expect(screen.getByText(/error\.bad-output/)).toBeTruthy()
    expect(screen.getByText('auto.components.reviewMap.aiSummary.card.retry')).toBeTruthy()
    rerender(<ReviewAiSummaryCard ai={ai({ state: 'error', error: 'forbidden' })} changedFiles={new Set()} onOpenFile={vi.fn()} translate={translate} />)
    expect(screen.queryByText('auto.components.reviewMap.aiSummary.card.retry')).toBeNull()
  })

  it('offers cancel while generating', () => {
    const state = ai({ state: 'generating' })
    renderCard(state)
    fireEvent.click(screen.getByText('auto.components.reviewMap.aiSummary.card.cancel'))
    expect(state.cancel).toHaveBeenCalled()
  })

  it('shows level choice only when diff is allowed and passes it to preview', () => {
    const state = ai({ allowedLevels: ['metadata', 'diff'] })
    renderCard(state)
    open()
    fireEvent.click(screen.getByText('auto.components.reviewMap.aiSummary.card.level.diff'))
    fireEvent.click(screen.getByText('auto.components.reviewMap.aiSummary.card.generate'))
    expect(state.preview).toHaveBeenCalledWith('diff')
  })

  it('renders feedback buttons only when a handler is given', () => {
    const onFeedback = vi.fn()
    const { rerender } = renderCard(ready())
    open()
    expect(screen.queryByText('auto.components.reviewMap.aiSummary.card.helpful')).toBeNull()
    rerender(<ReviewAiSummaryCard ai={ready()} changedFiles={new Set()} onOpenFile={vi.fn()} onFeedback={onFeedback} translate={translate} />)
    fireEvent.click(screen.getByText('auto.components.reviewMap.aiSummary.card.incorrect'))
    expect(onFeedback).toHaveBeenCalledWith('incorrect')
  })

  it('is structurally separate from the quality gate (no gate module imports)', async () => {
    const { readFileSync } = await import('node:fs')
    for (const file of ['ReviewAiSummaryCard.tsx', 'use-review-ai-summary.ts']) {
      const src = readFileSync(`${import.meta.dirname}/${file}`, 'utf-8')
      expect(src).not.toMatch(/quality-gate|use-source-control-quality-gate|QualityGate/)
    }
  })
})
