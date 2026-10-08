// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { TooltipProvider } from '@/components/ui/tooltip'
import type { QualityCall } from '../../../store/slices/code-intel-quality-slice-context'

const call = vi.fn<QualityCall>()
vi.mock('@/store', async () => {
  const { createCodeIntelQualityTestStore } =
    await import('../../../test-support/code-intel-quality-test-store')
  return { useAppStore: createCodeIntelQualityTestStore((...a) => call(...a)) }
})

import { useAppStore } from '@/store'
import { ENABLED_QUALITY_SUPPORT } from '../../../test-support/code-intel-quality-test-store'
import { QualityGateChip } from './QualityGateChip'

const flush = async (): Promise<void> => {
  for (let i = 0; i < 6; i++) {
    await Promise.resolve()
  }
}

beforeEach(() => {
  call.mockReset()
  call.mockImplementation(() =>
    Promise.resolve({
      ok: true,
      result: {
        gate: {
          verdict: 'warn',
          profile: 'full@repo/v1',
          reasons: [{ check: 'coverage', result: 'warn' }],
          basedOn: { runIds: [] }
        }
      }
    })
  )
  useAppStore.setState({
    codeIntelSupportState: ENABLED_QUALITY_SUPPORT,
    codeIntelQualityByWorktree: {}
  })
})
afterEach(() => cleanup())

const chip = (onOpen?: () => void) =>
  render(
    <TooltipProvider>
      <QualityGateChip worktreeId="wt" onOpen={onOpen} />
    </TooltipProvider>
  )

describe('QualityGateChip', () => {
  it('renders the verdict and opens the lens on click', async () => {
    const onOpen = vi.fn()
    chip(onOpen)
    await vi.waitFor(() => expect(screen.getByTestId('quality-gate-chip')).toBeTruthy())
    expect(screen.getByText('Has warnings')).toBeTruthy()
    fireEvent.click(screen.getByTestId('quality-gate-chip'))
    expect(onOpen).toHaveBeenCalled()
  })

  it('renders nothing and makes no call when quality is disabled', async () => {
    useAppStore.setState({
      codeIntelSupportState: {
        state: 'enabled',
        effective: { codeIntelEnabled: true, qualityGateEnabled: false }
      }
    })
    const { container } = chip()
    await flush()
    expect(container.innerHTML).toBe('')
    expect(call).not.toHaveBeenCalled()
  })

  it('renders nothing until a gate is known', async () => {
    call.mockImplementation(() => new Promise(() => undefined))
    const { container } = chip()
    await flush()
    expect(container.innerHTML).toBe('')
  })
})
