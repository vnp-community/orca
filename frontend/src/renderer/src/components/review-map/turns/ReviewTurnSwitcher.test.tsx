// @vitest-environment happy-dom
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import type { ReviewTurnMarker } from '../../../../../shared/code-intel-types'
import { ReviewTurnSwitcher } from './ReviewTurnSwitcher'
import { selectTurnPair } from './review-turn-selection'
import { buildAgentTurnVerificationViewModel } from './agent-turn-verification-view-model'

afterEach(cleanup)

const marker = (n: number, files: [string, string][] = []): ReviewTurnMarker => ({
  turnId: `p:${n}`,
  worktreeId: 'wt',
  paneKey: 'p',
  agentType: 'claude',
  startedAt: null,
  endedAt: n * 1000,
  baseOid: null,
  headOid: null,
  files: files.map(([p, h]) => ({ p, h })),
  overlayAvailable: false
})

const baseProps = {
  mode: 'all' as const,
  onModeChange: vi.fn(),
  selectedTurnId: null,
  onSelectTurn: vi.fn()
}

describe('selectTurnPair', () => {
  it('defaults to the newest marker and its predecessor; null without a predecessor', () => {
    const ms = [marker(1), marker(2), marker(3)]
    expect(selectTurnPair(ms, null)?.current.turnId).toBe('p:3')
    expect(selectTurnPair(ms, null)?.previous.turnId).toBe('p:2')
    expect(selectTurnPair(ms, 'p:2')?.previous.turnId).toBe('p:1')
    expect(selectTurnPair(ms, 'p:1')).toBeNull()
    expect(selectTurnPair([marker(1)], null)).toBeNull()
    expect(selectTurnPair([], null)).toBeNull()
  })
})

describe('ReviewTurnSwitcher', () => {
  it('disables the comparison modes and explains why when there is no previous turn', () => {
    render(<ReviewTurnSwitcher {...baseProps} markers={[marker(1)]} />)
    expect((screen.getByText('Since previous turn') as HTMLButtonElement).disabled).toBe(true)
    expect((screen.getByText('View previous turn') as HTMLButtonElement).disabled).toBe(true)
    expect(screen.getByText('No previous turn to compare yet.')).toBeTruthy()
  })

  it('enables the modes with two markers and reports mode changes', () => {
    const onModeChange = vi.fn()
    render(<ReviewTurnSwitcher {...baseProps} onModeChange={onModeChange} markers={[marker(1), marker(2)]} />)
    const since = screen.getByText('Since previous turn') as HTMLButtonElement
    expect(since.disabled).toBe(false)
    fireEvent.click(since)
    expect(onModeChange).toHaveBeenCalledWith('since-previous')
  })

  it('since-previous: shows label counts, the estimate caveat, and reports the comparison', () => {
    const onCompare = vi.fn()
    render(
      <ReviewTurnSwitcher
        {...baseProps}
        mode="since-previous"
        onCompare={onCompare}
        markers={[marker(1, [['a', '1'], ['b', '1']]), marker(2, [['a', '1'], ['b', '2'], ['c', '1']])]}
      />
    )
    expect(screen.getByText('New in this turn: 1')).toBeTruthy()
    expect(screen.getByText('Changed in this turn: 1')).toBeTruthy()
    expect(screen.getByText('Unchanged since previous turn: 1')).toBeTruthy()
    expect(screen.getByText(/Estimate:/)).toBeTruthy()
    expect(onCompare).toHaveBeenLastCalledWith(expect.objectContaining({ estimated: true }))
  })

  it('previous mode is read-only: states it and renders no diff buttons', () => {
    render(<ReviewTurnSwitcher {...baseProps} mode="previous" markers={[marker(1), marker(2)]} />)
    expect(screen.getByText(/Read-only view of the previous turn/)).toBeTruthy()
    expect(screen.queryByText(/diff/i, { selector: 'button' })).toBeNull()
  })

  it('falls back to "all" when the mode needs a previous turn that is gone', () => {
    const onModeChange = vi.fn()
    render(<ReviewTurnSwitcher {...baseProps} mode="since-previous" onModeChange={onModeChange} markers={[marker(1)]} />)
    expect(onModeChange).toHaveBeenCalledWith('all')
  })

  it('lists turns with sent-note counts and lets the user pick one', () => {
    const onSelectTurn = vi.fn()
    render(
      <ReviewTurnSwitcher
        {...baseProps}
        onSelectTurn={onSelectTurn}
        sentNoteCountByTurn={{ 'p:2': 3 }}
        markers={[marker(1), marker(2)]}
      />
    )
    fireEvent.click(screen.getByText('Turns (2)'))
    expect(screen.getByText('3 sent notes')).toBeTruthy()
    fireEvent.click(screen.getAllByText(/claude ·/)[1])
    expect(onSelectTurn).toHaveBeenCalledWith('p:1')
  })

  it('shows an inline save failure with retry', () => {
    const onRetrySave = vi.fn()
    render(<ReviewTurnSwitcher {...baseProps} markers={[marker(1)]} saveFailed onRetrySave={onRetrySave} />)
    fireEvent.click(screen.getByText('Retry'))
    expect(onRetrySave).toHaveBeenCalled()
    expect(screen.getByRole('alert').textContent).toContain('Could not save the turn marker.')
  })

  it('shows the agent verification line of the shown turn, never of another one', () => {
    const verificationByTurn = {
      'p:2': buildAgentTurnVerificationViewModel({
        claims: { items: [{ kind: 'tests_pass', basis: 'ran_command', agreement: 'contradicted' }] }
      })
    }
    const { rerender } = render(
      <ReviewTurnSwitcher {...baseProps} markers={[marker(1), marker(2)]} verificationByTurn={verificationByTurn} />
    )
    expect(screen.getByText(/differs from the agent/)).toBeTruthy()
    // "previous turn" shows turn 1, which has no recorded verification.
    rerender(
      <ReviewTurnSwitcher
        {...baseProps}
        mode="previous"
        markers={[marker(1), marker(2)]}
        verificationByTurn={verificationByTurn}
      />
    )
    expect(screen.queryByText(/differs from the agent/)).toBeNull()
  })
})
