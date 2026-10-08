// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { act, cleanup, fireEvent, render, screen } from '@testing-library/react'
import type { RunnableProfile } from '../../../../../shared/code-intel-quality-types'
import type {
  ActiveQualityRun,
  QualityRunError
} from '../../../store/slices/code-intel-quality-state'
import { QualityRunControl } from './QualityRunControl'
import type { QualityRunControlProps } from './QualityRunControl'
import { QualityRunErrorNotice, QualityRunFinishedNotice } from './QualityRunNotice'

beforeEach(() => vi.useFakeTimers())
afterEach(() => {
  cleanup()
  vi.useRealTimers()
})

const profile = (over: Partial<RunnableProfile> = {}): RunnableProfile => ({
  id: 'full',
  title: 'Full',
  kind: 'suite',
  ready: true,
  heavy: false,
  scopes: [],
  missing: [],
  ...over
})
const run = (over: Partial<ActiveQualityRun> = {}): ActiveQualityRun => ({
  runId: 'r1',
  profile: 'full',
  scope: 'changed',
  phase: 'running',
  stage: '',
  percent: null,
  message: '',
  startedAt: 0,
  ...over
})
const props = (over: Partial<QualityRunControlProps> = {}): QualityRunControlProps => ({
  runnable: [profile()],
  selected: 'full',
  selectedProfile: profile(),
  scope: 'changed',
  run: null,
  locked: false,
  spinnerDelayMs: 100,
  onSelectProfile: vi.fn(),
  onSelectScope: vi.fn(),
  onStart: vi.fn(),
  onCancel: vi.fn(),
  ...over
})

describe('QualityRunControl', () => {
  it('starts with a fixed-label button and no command input', () => {
    const p = props()
    render(<QualityRunControl {...p} />)
    fireEvent.click(screen.getByRole('button', { name: /Run checks/ }))
    expect(p.onStart).toHaveBeenCalledTimes(1)
    expect(screen.queryByRole('textbox')).toBeNull()
  })

  it('locks the moment the run is starting and only spins after the delay', () => {
    const { rerender } = render(
      <QualityRunControl {...props({ run: run({ phase: 'starting', runId: null }) })} />
    )
    const button = screen.getByRole('button', { name: /Starting/ })
    expect(button.hasAttribute('disabled')).toBe(true)
    expect(button.querySelector('.animate-spin')).toBeNull()
    act(() => {
      vi.advanceTimersByTime(99)
    })
    expect(
      screen.getByRole('button', { name: /Starting/ }).querySelector('.animate-spin')
    ).toBeNull()
    act(() => {
      vi.advanceTimersByTime(2)
    })
    expect(
      screen.getByRole('button', { name: /Starting/ }).querySelector('.animate-spin')
    ).toBeTruthy()
    rerender(
      <QualityRunControl
        {...props({ run: run({ phase: 'starting', runId: null }), spinnerDelayMs: 200 })}
      />
    )
  })

  it('the remote delay is longer', () => {
    render(
      <QualityRunControl
        {...props({ run: run({ phase: 'starting', runId: null }), spinnerDelayMs: 200 })}
      />
    )
    act(() => {
      vi.advanceTimersByTime(150)
    })
    expect(
      screen.getByRole('button', { name: /Starting/ }).querySelector('.animate-spin')
    ).toBeNull()
    act(() => {
      vi.advanceTimersByTime(60)
    })
    expect(
      screen.getByRole('button', { name: /Starting/ }).querySelector('.animate-spin')
    ).toBeTruthy()
  })

  it('shows a bar for a known percent and a plain spinner with no number for null', () => {
    const { rerender } = render(
      <QualityRunControl
        {...props({
          run: run({ percent: 40, stage: 'lint', stepIndex: 1, stepCount: 4, message: 'working' })
        })}
      />
    )
    expect(screen.getByText('40%')).toBeTruthy()
    expect(screen.getByText('Stage: lint')).toBeTruthy()
    expect(screen.getByText('Step 1 of 4')).toBeTruthy()
    expect(screen.getByText('working')).toBeTruthy()
    rerender(<QualityRunControl {...props({ run: run({ percent: null }) })} />)
    act(() => {
      vi.advanceTimersByTime(300)
    })
    expect(screen.queryByText(/%/)).toBeNull()
    expect(
      screen
        .getByTestId('quality-run-progress')
        .querySelector('.animate-spin.motion-reduce\\:animate-none')
    ).toBeTruthy()
    expect(screen.getByText('Progress is not known')).toBeTruthy()
  })

  it('cancel is a quiet ghost action and cancelling does not claim success', () => {
    const p = props({ run: run() })
    const { rerender } = render(<QualityRunControl {...p} />)
    const cancel = screen.getByRole('button', { name: 'Cancel' })
    expect(cancel.getAttribute('data-variant')).toBe('ghost')
    fireEvent.click(cancel)
    expect(p.onCancel).toHaveBeenCalled()
    rerender(<QualityRunControl {...props({ run: run({ phase: 'cancelling' }) })} />)
    expect(screen.getByText('Cancelling, waiting for confirmation')).toBeTruthy()
    expect(screen.queryByText(/was cancelled/)).toBeNull()
    expect(screen.getByRole('button', { name: 'Cancel' }).hasAttribute('disabled')).toBe(true)
  })

  it('a profile that is not ready lists what is missing and blocks running', () => {
    const missing = [{ check: 'go', reason: 'not installed', hint: 'install go' }]
    render(
      <QualityRunControl {...props({ selectedProfile: profile({ ready: false, missing }) })} />
    )
    expect(screen.getByText(/Missing: go \(not installed\)/)).toBeTruthy()
    expect(screen.getByText(/Hint: install go/)).toBeTruthy()
    expect(screen.getByRole('button', { name: /Run checks/ }).hasAttribute('disabled')).toBe(true)
  })

  it('locks when offline or forbidden and when nothing is selected', () => {
    const { rerender } = render(<QualityRunControl {...props({ locked: true })} />)
    expect(screen.getByRole('button', { name: /Run checks/ }).hasAttribute('disabled')).toBe(true)
    rerender(
      <QualityRunControl {...props({ selected: null, selectedProfile: null, runnable: [] })} />
    )
    expect(screen.getByRole('button', { name: /Run checks/ }).hasAttribute('disabled')).toBe(true)
  })
})

describe('QualityRunErrorNotice', () => {
  const show = (error: QualityRunError, onRecheck = vi.fn(), onDismiss = vi.fn()) => {
    render(<QualityRunErrorNotice error={error} onRecheck={onRecheck} onDismiss={onDismiss} />)
    return { onRecheck, onDismiss }
  }

  it('env not ready lists missing checks inline and offers a recheck', () => {
    const { onRecheck } = show({
      kind: 'env-not-ready',
      message: '',
      missing: [{ check: 'node_modules', reason: 'absent' }]
    })
    expect(screen.getByText('The environment is not ready to run these checks')).toBeTruthy()
    expect(screen.getByText(/Missing: node_modules \(absent\)/)).toBeTruthy()
    fireEvent.click(screen.getByRole('button', { name: 'Check again' }))
    expect(onRecheck).toHaveBeenCalled()
  })

  it('profile unknown shows the available names', () => {
    show({ kind: 'profile-unknown', message: '', available: ['a', 'b'] })
    expect(screen.getByText('Available: a, b')).toBeTruthy()
  })

  it('rate limited shows the wait; forbidden and offline keep reading possible', () => {
    show({ kind: 'rate-limited', message: '', retryAfterSeconds: 12 })
    expect(screen.getByText('Try again in 12 s')).toBeTruthy()
    cleanup()
    show({ kind: 'forbidden', message: '' })
    expect(screen.getByText(/results stay readable/)).toBeTruthy()
    cleanup()
    show({ kind: 'offline', message: '' })
    expect(screen.getByText(/cached results are shown/)).toBeTruthy()
  })

  it('an unknown error shows the backend message as text', () => {
    show({ kind: 'unknown', message: '<i>boom</i>' })
    expect(screen.getByText('<i>boom</i>')).toBeTruthy()
  })

  it('is a persistent alert, not a toast', () => {
    show({ kind: 'unknown', message: '' })
    expect(screen.getByRole('alert')).toBeTruthy()
  })
})

describe('QualityRunFinishedNotice', () => {
  it('failed and interrupted runs are reported as not completed, with copyable details', () => {
    const writeText = vi.fn()
    Object.defineProperty(navigator, 'clipboard', { value: { writeText }, configurable: true })
    render(
      <QualityRunFinishedNotice
        run={run({ phase: 'finished', status: 'interrupted', message: 'lost agent' })}
        onDismiss={vi.fn()}
      />
    )
    expect(screen.getByText('The run was interrupted and did not complete')).toBeTruthy()
    fireEvent.click(screen.getByRole('button', { name: 'Copy details' }))
    expect(writeText).toHaveBeenCalledWith(expect.stringContaining('r1'))
  })

  it('cancelled is neutral and a success renders nothing', () => {
    const { container, rerender } = render(
      <QualityRunFinishedNotice
        run={run({ phase: 'finished', status: 'cancelled' })}
        onDismiss={vi.fn()}
      />
    )
    expect(screen.getByText('The run was cancelled')).toBeTruthy()
    rerender(
      <QualityRunFinishedNotice
        run={run({ phase: 'finished', status: 'succeeded' })}
        onDismiss={vi.fn()}
      />
    )
    expect(container.innerHTML).toBe('')
  })
})
