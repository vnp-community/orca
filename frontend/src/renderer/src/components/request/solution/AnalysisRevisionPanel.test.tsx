// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { useState } from 'react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { AnalysisRevisionPanel } from './AnalysisRevisionPanel'
import type { AnalysisRevision } from './useAnalysisRevision'

afterEach(() => {
  cleanup()
  vi.restoreAllMocks()
})

const userAgent = (value: string): void => {
  vi.spyOn(window.navigator, 'userAgent', 'get').mockReturnValue(value)
}
const MAC = 'Mozilla/5.0 (Macintosh; Intel Mac OS X 14_0) AppleWebKit/605.1.15'
const WINDOWS = 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36'
const LINUX = 'Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36'

const revisionOf = (over: Partial<AnalysisRevision> = {}): AnalysisRevision => ({
  canShow: true,
  busy: false,
  nextVersion: undefined,
  draft: '',
  setDraft: vi.fn(),
  error: null,
  clearError: vi.fn(),
  submit: vi.fn().mockResolvedValue({ ok: true }),
  ...over
})

// A panel whose text box really holds what is typed, like the hook does.
function Harness({
  base,
  kind = 'solution'
}: {
  base: AnalysisRevision
  kind?: 'solution' | 'diagnosis' | 'findings' | 'answer'
}) {
  const [draft, setDraft] = useState(base.draft)
  return <AnalysisRevisionPanel kind={kind} revision={{ ...base, draft, setDraft }} />
}

describe('AnalysisRevisionPanel', () => {
  it('starts empty with a zero counter and an enabled button', () => {
    render(<AnalysisRevisionPanel kind="solution" revision={revisionOf()} />)
    expect(screen.getByRole('textbox', { name: 'What should change?' })).toHaveValue('')
    expect(screen.getByTestId('analysis-revision-count')).toHaveTextContent('0 / 2000')
    expect(screen.getByRole('button', { name: 'Revise analysis' })).toBeEnabled()
    expect(screen.queryByTestId('analysis-revision-error')).toBeNull()
  })

  it('counts characters, not UTF-16 units, and flags the limit', () => {
    render(
      <AnalysisRevisionPanel kind="solution" revision={revisionOf({ draft: '😀'.repeat(3) })} />
    )
    expect(screen.getByTestId('analysis-revision-count')).toHaveTextContent('3 / 2000')
    cleanup()
    render(
      <AnalysisRevisionPanel kind="solution" revision={revisionOf({ draft: 'a'.repeat(2001) })} />
    )
    expect(screen.getByTestId('analysis-revision-count')).toHaveTextContent('2001 / 2000')
    expect(screen.getByTestId('analysis-revision-count')).toHaveClass('text-destructive')
    expect(screen.getByRole('textbox')).toHaveAttribute('aria-invalid', 'true')
  })

  it('reports typed text to the hook', () => {
    const setDraft = vi.fn()
    render(<AnalysisRevisionPanel kind="solution" revision={revisionOf({ setDraft })} />)
    fireEvent.change(screen.getByRole('textbox'), { target: { value: 'Add a rollback plan' } })
    expect(setDraft).toHaveBeenCalledWith('Add a rollback plan')
  })

  it('offers chips for the kind of document and a chip fills the box without sending', () => {
    const submit = vi.fn()
    render(<Harness base={revisionOf({ submit })} kind="diagnosis" />)
    const chips = screen.getByRole('group', { name: 'Suggestions' }).querySelectorAll('button')
    expect(chips).toHaveLength(4)
    fireEvent.click(screen.getByTestId('analysis-revision-chip-evidence'))
    expect(screen.getByRole('textbox')).toHaveValue('Show the evidence for the root cause.')
    fireEvent.click(screen.getByTestId('analysis-revision-chip-prevent'))
    expect(screen.getByRole('textbox')).toHaveValue(
      'Show the evidence for the root cause. Suggest how to prevent it.'
    )
    expect(submit).not.toHaveBeenCalled()
  })

  it('differs by kind: no chips for a kind it does not know', () => {
    render(<AnalysisRevisionPanel kind={'mystery' as never} revision={revisionOf()} />)
    expect(screen.queryByRole('group', { name: 'Suggestions' })).toBeNull()
  })

  it('submits with the button', () => {
    const submit = vi.fn().mockResolvedValue({ ok: true })
    render(<AnalysisRevisionPanel kind="solution" revision={revisionOf({ submit, draft: 'x' })} />)
    fireEvent.click(screen.getByRole('button', { name: 'Revise analysis' }))
    expect(submit).toHaveBeenCalledTimes(1)
  })

  it('shows the busy state with the version being written, announced politely', () => {
    render(
      <AnalysisRevisionPanel
        kind="solution"
        revision={revisionOf({ busy: true, nextVersion: 3 })}
      />
    )
    const status = screen.getByTestId('analysis-revision-status')
    expect(status).toHaveAttribute('aria-live', 'polite')
    expect(status).toHaveTextContent('Analyzing version 3…')
    expect(screen.getByRole('button', { name: 'Revise analysis' })).toBeDisabled()
    expect(screen.getByRole('textbox')).toBeDisabled()
    expect(screen.getByTestId('analysis-revision-chip-lowerRisk')).toBeDisabled()
  })

  it('has an empty live region when idle and a generic busy text without a version number', () => {
    const { rerender } = render(<AnalysisRevisionPanel kind="solution" revision={revisionOf()} />)
    expect(screen.getByTestId('analysis-revision-status')).toBeEmptyDOMElement()
    rerender(<AnalysisRevisionPanel kind="solution" revision={revisionOf({ busy: true })} />)
    expect(screen.getByTestId('analysis-revision-status')).toHaveTextContent(
      'Analyzing the next version…'
    )
  })

  it('shows the error under the field and ties it to the text box', () => {
    render(
      <AnalysisRevisionPanel
        kind="solution"
        revision={revisionOf({
          draft: 'keep me',
          error: { kind: 'forbidden', message: 'Only approvers. Your text is kept.' }
        })}
      />
    )
    const alert = screen.getByRole('alert')
    expect(alert).toHaveTextContent('Only approvers. Your text is kept.')
    expect(screen.getByRole('textbox')).toHaveValue('keep me')
    expect(screen.getByRole('textbox').getAttribute('aria-describedby')).toContain(alert.id)
    expect(screen.getByRole('textbox')).toHaveAttribute('aria-invalid', 'true')
  })

  describe('keyboard shortcut', () => {
    const press = (init: KeyboardEventInit) =>
      fireEvent.keyDown(screen.getByRole('textbox'), { key: 'Enter', ...init })

    it('is Cmd+Enter on macOS and ignores Ctrl+Enter there', () => {
      userAgent(MAC)
      const submit = vi.fn().mockResolvedValue({ ok: true })
      render(
        <AnalysisRevisionPanel kind="solution" revision={revisionOf({ submit, draft: 'x' })} />
      )
      expect(screen.getByText('or press ⌘Enter')).toBeInTheDocument()
      expect(screen.getByRole('textbox')).toHaveAttribute('aria-keyshortcuts', 'Meta+Enter')
      press({ ctrlKey: true })
      expect(submit).not.toHaveBeenCalled()
      press({ metaKey: true })
      expect(submit).toHaveBeenCalledTimes(1)
    })

    it.each([
      ['Windows', WINDOWS],
      ['Linux', LINUX]
    ])('is Ctrl+Enter on %s and ignores Cmd+Enter there', (_name, ua) => {
      userAgent(ua)
      const submit = vi.fn().mockResolvedValue({ ok: true })
      render(
        <AnalysisRevisionPanel kind="solution" revision={revisionOf({ submit, draft: 'x' })} />
      )
      expect(screen.getByText('or press Ctrl+Enter')).toBeInTheDocument()
      expect(screen.getByRole('textbox')).toHaveAttribute('aria-keyshortcuts', 'Control+Enter')
      press({ metaKey: true })
      expect(submit).not.toHaveBeenCalled()
      press({ ctrlKey: true })
      expect(submit).toHaveBeenCalledTimes(1)
    })

    it('has no other shortcut: plain Enter, Shift+Enter, other keys and IME composition do nothing', () => {
      userAgent(LINUX)
      const submit = vi.fn().mockResolvedValue({ ok: true })
      render(
        <AnalysisRevisionPanel kind="solution" revision={revisionOf({ submit, draft: 'x' })} />
      )
      press({})
      press({ shiftKey: true })
      press({ ctrlKey: true, shiftKey: true })
      press({ ctrlKey: true, altKey: true })
      fireEvent.keyDown(screen.getByRole('textbox'), { key: 'k', ctrlKey: true })
      fireEvent.keyDown(screen.getByRole('textbox'), {
        key: 'Enter',
        ctrlKey: true,
        isComposing: true
      })
      expect(submit).not.toHaveBeenCalled()
    })
  })
})
