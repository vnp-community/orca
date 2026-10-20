// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { useState } from 'react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { PlanRevisionPanel } from './PlanRevisionPanel'
import type { PlanRevisionApi } from './usePlanRevision'

afterEach(() => {
  cleanup()
  vi.restoreAllMocks()
})

const userAgent = (value: string): void => {
  vi.spyOn(window.navigator, 'userAgent', 'get').mockReturnValue(value)
}
const MAC = 'Mozilla/5.0 (Macintosh; Intel Mac OS X 14_0) AppleWebKit/605.1.15'
const LINUX = 'Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36'

const revisionOf = (over: Partial<PlanRevisionApi> = {}): PlanRevisionApi => ({
  canShow: true,
  visible: true,
  busy: false,
  nextVersion: undefined,
  slow: false,
  draft: '',
  setDraft: vi.fn(),
  error: null,
  submit: vi.fn().mockResolvedValue({ ok: true }),
  retry: vi.fn().mockResolvedValue(undefined),
  retrying: false,
  retryError: null,
  ...over
})

function Harness({ base, engine }: { base: PlanRevisionApi; engine?: string }) {
  const [draft, setDraft] = useState(base.draft)
  return <PlanRevisionPanel engine={engine} revision={{ ...base, draft, setDraft }} />
}

describe('PlanRevisionPanel', () => {
  it('starts empty with its title, hint, counter and button', () => {
    render(<PlanRevisionPanel engine="native" revision={revisionOf()} />)
    expect(
      screen.getByRole('heading', { name: 'Request changes to this plan' })
    ).toBeInTheDocument()
    expect(
      screen.getByText('The AI revises the documents. You approve the new version afterwards.')
    ).toBeInTheDocument()
    expect(screen.getByRole('textbox', { name: 'What should change?' })).toHaveValue('')
    expect(screen.getByTestId('plan-revision-count')).toHaveTextContent('0 / 2000')
    expect(screen.getByRole('button', { name: 'Revise plan' })).toBeEnabled()
    expect(screen.queryByTestId('plan-revision-error')).toBeNull()
    expect(screen.queryByTestId('plan-revision-slow')).toBeNull()
  })

  it('counts characters, not UTF-16 units, and flags the limit', () => {
    render(<PlanRevisionPanel engine="native" revision={revisionOf({ draft: '😀'.repeat(3) })} />)
    expect(screen.getByTestId('plan-revision-count')).toHaveTextContent('3 / 2000')
    cleanup()
    render(<PlanRevisionPanel engine="native" revision={revisionOf({ draft: 'a'.repeat(2001) })} />)
    expect(screen.getByTestId('plan-revision-count')).toHaveTextContent('2001 / 2000')
    expect(screen.getByRole('textbox')).toHaveAttribute('aria-invalid', 'true')
  })

  it('offers chips by engine and a chip fills the box without sending', () => {
    const submit = vi.fn()
    const { unmount } = render(<Harness base={revisionOf({ submit })} engine="openspec" />)
    fireEvent.click(screen.getByTestId('plan-revision-chip-tests'))
    expect(screen.getByRole('textbox')).toHaveValue('Add a task for tests.')
    fireEvent.click(screen.getByTestId('plan-revision-chip-tightenDesign'))
    expect(screen.getByRole('textbox')).toHaveValue('Add a task for tests. Tighten the design.')
    expect(submit).not.toHaveBeenCalled()
    unmount()
    render(<Harness base={revisionOf({ submit })} engine="native" />)
    expect(screen.getByTestId('plan-revision-chip-rollback')).toHaveTextContent(
      'Add a rollback task.'
    )
    expect(screen.queryByTestId('plan-revision-chip-tests')).toBeNull()
  })

  it('submits with the button', () => {
    const submit = vi.fn().mockResolvedValue({ ok: true })
    render(<PlanRevisionPanel engine="native" revision={revisionOf({ submit, draft: 'x' })} />)
    fireEvent.click(screen.getByRole('button', { name: 'Revise plan' }))
    expect(submit).toHaveBeenCalledTimes(1)
  })

  it('shows the version being written, announced politely, and locks the box', () => {
    render(
      <PlanRevisionPanel engine="native" revision={revisionOf({ busy: true, nextVersion: 3 })} />
    )
    const status = screen.getByTestId('plan-revision-status')
    expect(status).toHaveAttribute('aria-live', 'polite')
    expect(status).toHaveTextContent('Writing version 3…')
    expect(screen.getByRole('button', { name: 'Revise plan' })).toBeDisabled()
    expect(screen.getByRole('textbox')).toBeDisabled()
    expect(screen.getByTestId('plan-revision-chip-rollback')).toBeDisabled()
  })

  it('has an empty live region when idle and a generic text without a version number', () => {
    const { rerender } = render(<PlanRevisionPanel engine="native" revision={revisionOf()} />)
    expect(screen.getByTestId('plan-revision-status')).toBeEmptyDOMElement()
    rerender(<PlanRevisionPanel engine="native" revision={revisionOf({ busy: true })} />)
    expect(screen.getByTestId('plan-revision-status')).toHaveTextContent(
      'Writing the next version…'
    )
  })

  it('reports a round that takes long without failing it', () => {
    render(
      <PlanRevisionPanel engine="openspec" revision={revisionOf({ busy: true, slow: true })} />
    )
    expect(screen.getByTestId('plan-revision-slow')).toHaveTextContent(
      'Taking longer than expected'
    )
    cleanup()
    render(
      <PlanRevisionPanel engine="openspec" revision={revisionOf({ busy: false, slow: true })} />
    )
    expect(screen.queryByTestId('plan-revision-slow')).toBeNull()
  })

  it('shows the error under the field and keeps the text', () => {
    render(
      <PlanRevisionPanel
        engine="native"
        revision={revisionOf({
          draft: 'keep me',
          error: { kind: 'forbidden', message: 'Only approvers. Your text is kept.' }
        })}
      />
    )
    expect(screen.getByRole('alert')).toHaveTextContent('Only approvers. Your text is kept.')
    expect(screen.getByRole('textbox')).toHaveValue('keep me')
  })

  describe('keyboard shortcut', () => {
    const press = (init: KeyboardEventInit) =>
      fireEvent.keyDown(screen.getByRole('textbox'), { key: 'Enter', ...init })

    it('is Cmd+Enter on macOS and Ctrl+Enter elsewhere', () => {
      userAgent(MAC)
      const submit = vi.fn().mockResolvedValue({ ok: true })
      const { unmount } = render(
        <PlanRevisionPanel engine="native" revision={revisionOf({ submit, draft: 'x' })} />
      )
      expect(screen.getByText('or press ⌘Enter')).toBeInTheDocument()
      press({ ctrlKey: true })
      expect(submit).not.toHaveBeenCalled()
      press({ metaKey: true })
      expect(submit).toHaveBeenCalledTimes(1)
      unmount()

      userAgent(LINUX)
      render(<PlanRevisionPanel engine="native" revision={revisionOf({ submit, draft: 'x' })} />)
      expect(screen.getByText('or press Ctrl+Enter')).toBeInTheDocument()
      press({ metaKey: true })
      expect(submit).toHaveBeenCalledTimes(1)
      press({ ctrlKey: true })
      expect(submit).toHaveBeenCalledTimes(2)
    })

    it('has no other shortcut: plain Enter, Shift+Enter and IME composition do nothing', () => {
      userAgent(LINUX)
      const submit = vi.fn().mockResolvedValue({ ok: true })
      render(<PlanRevisionPanel engine="native" revision={revisionOf({ submit, draft: 'x' })} />)
      press({})
      press({ shiftKey: true })
      fireEvent.keyDown(screen.getByRole('textbox'), {
        key: 'Enter',
        ctrlKey: true,
        isComposing: true
      })
      expect(submit).not.toHaveBeenCalled()
    })
  })
})
