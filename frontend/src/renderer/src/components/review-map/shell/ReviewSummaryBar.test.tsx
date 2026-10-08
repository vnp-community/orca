// @vitest-environment happy-dom
import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { makeOverlay } from '../review-test-data'
import { ReviewRiskChip } from './ReviewRiskChip'
import { ReviewSummaryBar } from './ReviewSummaryBar'

afterEach(cleanup)

describe('ReviewSummaryBar', () => {
  it('always renders seven chips; zero-count chips are disabled', () => {
    render(<ReviewSummaryBar overlay={makeOverlay()} active={null} onChipClick={vi.fn()} />)
    const chips = screen.getAllByRole('button')
    expect(chips).toHaveLength(7)
    expect(chips.filter((c) => (c as HTMLButtonElement).disabled)).toHaveLength(6)
  })
  it('aria-pressed marks the active chip and clicks report the chip id', () => {
    const onChipClick = vi.fn()
    render(<ReviewSummaryBar overlay={makeOverlay()} active="files" onChipClick={onChipClick} />)
    const files = screen.getByRole('button', { name: /3 files/ })
    expect(files.getAttribute('aria-pressed')).toBe('true')
    fireEvent.click(files)
    expect(onChipClick).toHaveBeenCalledWith('files')
  })
  it('truncated arrays show the full backend count', () => {
    render(
      <ReviewSummaryBar
        overlay={makeOverlay({
          limits: { truncated: { files: true }, totalCounts: { changedFiles: 400 } }
        })}
        active={null}
        onChipClick={vi.fn()}
      />
    )
    expect(screen.getByRole('button', { name: /400 files/ })).toBeTruthy()
  })
})

describe('ReviewRiskChip', () => {
  it('renders nothing without risk', () => {
    const { container } = render(<ReviewRiskChip risk={null} />)
    expect(container.firstChild).toBeNull()
  })
  it('incomplete never claims a level', () => {
    render(<ReviewRiskChip risk={{ level: 'LOW', incomplete: true, reasons: [] }} />)
    const chip = screen.getByRole('button')
    expect(chip.getAttribute('data-risk')).toBe('INCOMPLETE')
    expect(chip.textContent).toBe('Not enough data')
  })
  it('lists reasons through translate and shows unknown keys verbatim', () => {
    render(
      <ReviewRiskChip
        risk={{
          level: 'HIGH',
          incomplete: false,
          reasons: [{ code: 'x', messageKey: 'risk.unknown.key' }]
        }}
      />
    )
    fireEvent.click(screen.getByRole('button'))
    expect(screen.getByText('risk.unknown.key')).toBeTruthy()
  })
})
