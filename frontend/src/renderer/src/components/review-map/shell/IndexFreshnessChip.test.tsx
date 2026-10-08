// @vitest-environment happy-dom
import { afterEach, describe, expect, it, vi } from 'vitest'
import { act, cleanup, fireEvent, render, screen } from '@testing-library/react'
import { makeStatus } from '../review-test-data'
import type { IndexOverall } from '../review-wire-types'
import type { ReviewDataApi } from '../review-shell-data'
import { describeIndexChip, IndexFreshnessChip } from './IndexFreshnessChip'
import { ReindexButton } from './ReindexButton'

afterEach(cleanup)

const NOW = Date.parse('2026-10-07T12:10:00Z')
const tool = {
  tool: 'gitnexus',
  available: true,
  state: 'ready',
  indexedCommit: 'abcdef1234',
  indexedAt: '2026-10-07T12:08:00Z'
}

describe('describeIndexChip', () => {
  const overalls: IndexOverall[] = [
    'OFFLINE',
    'UNKNOWN',
    'NOT_INSTALLED',
    'BUILDING',
    'MISSING',
    'DEGRADED',
    'OVERLAY',
    'STALE',
    'READY'
  ]
  it('has a distinct, non-empty label for each of the 9 overall states', () => {
    const labels = overalls.map(
      (o) => describeIndexChip(makeStatus({ overall: o, tools: [tool] }), NOW).label
    )
    expect(labels.every((l) => l.length > 0)).toBe(true)
    expect(new Set(labels).size).toBe(9)
  })
  it('READY shows short sha and relative time', () => {
    expect(describeIndexChip(makeStatus({ tools: [tool] }), NOW).label).toContain('abcdef1')
  })
  it('OVERLAY carries the explanation note', () => {
    expect(describeIndexChip(makeStatus({ overall: 'OVERLAY' }), NOW).note).toMatch(
      /line numbers may be off/
    )
  })
  it('BUILDING with unknown percent never shows a number', () => {
    const label = describeIndexChip(
      makeStatus({ overall: 'BUILDING', activeJob: { id: 'j', stage: 's', percent: null } }),
      NOW
    ).label
    expect(label).not.toMatch(/\d/)
    const known = describeIndexChip(
      makeStatus({ overall: 'BUILDING', activeJob: { id: 'j', stage: 's', percent: 42 } }),
      NOW
    ).label
    expect(known).toContain('42')
  })
  it('null status is UNKNOWN, never ready', () => {
    expect(describeIndexChip(null, NOW).overall).toBe('UNKNOWN')
  })
})

function fakeApi(over: Partial<ReviewDataApi> = {}): ReviewDataApi {
  return {
    reindex: vi.fn(async () => ({ ok: true as const, value: { jobId: 'j', status: 'queued' } })),
    ...over
  } as unknown as ReviewDataApi
}

describe('IndexFreshnessChip', () => {
  it('renders the chip with its data-overall and opens details on click', () => {
    render(
      <IndexFreshnessChip
        status={makeStatus({ overall: 'OVERLAY', tools: [tool] })}
        reindex={{ worktreeId: 'w', api: fakeApi(), onStarted: vi.fn() }}
      />
    )
    const chip = screen.getByRole('button', { name: /Index status/ })
    expect(chip.getAttribute('data-overall')).toBe('OVERLAY')
    fireEvent.click(chip)
    expect(screen.getByText(/Based on the main checkout index/)).toBeTruthy()
    expect(screen.getByRole('button', { name: 'Refresh index' })).toBeTruthy()
  })
})

describe('ReindexButton', () => {
  it('two quick clicks make exactly one call and the button locks immediately', async () => {
    let resolve!: (v: unknown) => void
    const reindex = vi.fn(() => new Promise((r) => (resolve = r)))
    const onStarted = vi.fn()
    render(
      <ReindexButton
        worktreeId="w"
        status={makeStatus()}
        api={fakeApi({ reindex: reindex as never })}
        onStarted={onStarted}
      />
    )
    const btn = screen.getByRole('button', { name: 'Refresh index' })
    fireEvent.click(btn)
    fireEvent.click(btn)
    expect(reindex).toHaveBeenCalledTimes(1)
    expect((btn as HTMLButtonElement).disabled).toBe(true)
    await act(async () => resolve({ ok: true, value: { jobId: 'j', status: 'queued' } }))
    expect(onStarted).toHaveBeenCalledTimes(1)
  })

  it('full rebuild asks for an inline confirmation first', async () => {
    const api = fakeApi()
    render(<ReindexButton worktreeId="w" status={makeStatus()} api={api} onStarted={vi.fn()} />)
    fireEvent.click(screen.getByRole('button', { name: 'Rebuild all' }))
    expect(api.reindex).not.toHaveBeenCalled()
    await act(async () =>
      fireEvent.click(screen.getByRole('button', { name: 'Rebuild everything' }))
    )
    expect(api.reindex).toHaveBeenCalledWith('w', 'full')
  })

  it('rate-limited shows a cooldown countdown and locks the button', async () => {
    vi.useFakeTimers()
    const api = fakeApi({
      reindex: vi.fn(async () => ({
        ok: false as const,
        error: { kind: 'rate-limited' as const, message: 'cd', retryAfterSeconds: 3 }
      })) as never
    })
    render(<ReindexButton worktreeId="w" status={makeStatus()} api={api} onStarted={vi.fn()} />)
    await act(async () => fireEvent.click(screen.getByRole('button', { name: 'Refresh index' })))
    expect(screen.getByRole('status').textContent).toContain('3s')
    expect(
      (screen.getByRole('button', { name: 'Refresh index' }) as HTMLButtonElement).disabled
    ).toBe(true)
    await act(async () => void vi.advanceTimersByTime(1000))
    expect(screen.getByRole('status').textContent).toContain('2s')
    vi.useRealTimers()
  })

  it('reindex-in-progress attaches to the job instead of showing an error', async () => {
    const onStarted = vi.fn()
    const api = fakeApi({
      reindex: vi.fn(async () => ({
        ok: false as const,
        error: { kind: 'reindex-in-progress' as const, message: 'x' }
      })) as never
    })
    render(<ReindexButton worktreeId="w" status={makeStatus()} api={api} onStarted={onStarted} />)
    await act(async () => fireEvent.click(screen.getByRole('button', { name: 'Refresh index' })))
    expect(onStarted).toHaveBeenCalled()
    expect(screen.queryByRole('alert')).toBeNull()
  })

  it('unknown percent renders an indeterminate bar, known percent renders Progress', () => {
    const { rerender } = render(
      <ReindexButton
        worktreeId="w"
        status={makeStatus({
          overall: 'BUILDING',
          activeJob: { id: 'j', stage: 'parse', percent: null }
        })}
        api={fakeApi()}
        onStarted={vi.fn()}
      />
    )
    expect(screen.getByRole('progressbar').getAttribute('data-slot')).toBeNull()
    rerender(
      <ReindexButton
        worktreeId="w"
        status={makeStatus({
          overall: 'BUILDING',
          activeJob: { id: 'j', stage: 'parse', percent: 40 }
        })}
        api={fakeApi()}
        onStarted={vi.fn()}
      />
    )
    const bar = screen.getByRole('progressbar')
    expect(bar.getAttribute('data-slot')).toBe('progress')
    expect(bar.querySelector('[data-slot=progress-indicator]')?.getAttribute('style')).toContain(
      'translateX(-60%)'
    )
  })

  it('other errors are inline, never a toast', async () => {
    const api = fakeApi({
      reindex: vi.fn(async () => ({
        ok: false as const,
        error: { kind: 'tool-failed' as const, message: 'boom' }
      })) as never
    })
    render(<ReindexButton worktreeId="w" status={makeStatus()} api={api} onStarted={vi.fn()} />)
    await act(async () => fireEvent.click(screen.getByRole('button', { name: 'Refresh index' })))
    expect(screen.getByRole('alert').textContent).toBe('boom')
  })
})
