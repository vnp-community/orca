// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'

vi.mock('@/store', async () => {
  const { createCodeIntelQualityTestStore } =
    await import('../../../test-support/code-intel-quality-test-store')
  return {
    useAppStore: createCodeIntelQualityTestStore(() =>
      Promise.resolve({ ok: true as const, result: {} })
    )
  }
})

import { useAppStore } from '@/store'
import { QualityAnnotationStrip } from './QualityAnnotationStrip'

beforeEach(() => useAppStore.setState({ codeIntelQualityByWorktree: {} }))
afterEach(() => cleanup())

describe('QualityAnnotationStrip', () => {
  it('renders nothing without a notice or toggle', () => {
    const { container } = render(
      <QualityAnnotationStrip worktreeId="wt" notice={null} toggleVisible={false} />
    )
    expect(container.innerHTML).toBe('')
  })

  it('shows the notice text and flips annotationsOn', () => {
    render(<QualityAnnotationStrip worktreeId="wt" notice="content-changed" toggleVisible />)
    expect(screen.getByRole('status').textContent).toContain('hidden')
    const button = screen.getByRole('button')
    expect(button.getAttribute('aria-pressed')).toBe('true')
    fireEvent.click(button)
    expect(useAppStore.getState().codeIntelQualityByWorktree.wt.ui.annotationsOn).toBe(false)
  })

  it('renders nothing without a worktree', () => {
    const { container } = render(
      <QualityAnnotationStrip worktreeId={undefined} notice="dirty" toggleVisible />
    )
    expect(container.innerHTML).toBe('')
  })
})
