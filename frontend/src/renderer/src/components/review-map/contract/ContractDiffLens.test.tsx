// @vitest-environment happy-dom
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { CONTRACT_DIFF_MIXED } from '../../../test-support/contract-findings-fixtures'

const hook = vi.hoisted(() => ({ value: null as unknown }))
vi.mock('../../../hooks/useCodeIntelContractDiff', () => ({
  useCodeIntelContractDiff: () => hook.value
}))
const actions = vi.hoisted(() => ({
  setReviewLens: vi.fn(),
  setErdService: vi.fn(),
  selectErdTable: vi.fn()
}))
vi.mock('@/store', () => ({
  useAppStore: (selector: (s: typeof actions) => unknown) => selector(actions)
}))

// Why: other agents' lens modules may not exist yet, and the registry statically imports them.
vi.mock('../review-lens-registry', () => ({ REVIEW_LENS_DEFINITIONS: [] as { id: string; load?: unknown }[] }))

vi.mock('../notes/ReviewNoteButton', () => ({ ReviewNoteButton: () => <button type="button">note</button> }))

import ContractDiffLens from './ContractDiffLens'
import { REVIEW_LENS_DEFINITIONS } from '../review-lens-registry'

const props = {
  worktreeId: 'wt',
  environmentId: null,
  scope: {} as never,
  overlay: { changedFiles: [{ path: 'gateway/ws.go', status: 'modified' }] } as never,
  selectedSymbolKey: null,
  chipFilter: null,
  onSelectSymbol: vi.fn(),
  onOpenDiff: vi.fn(),
  requestSymbolChoice: vi.fn()
}

function setHook(over: Record<string, unknown>): void {
  hook.value = { status: 'success', error: null, diff: CONTRACT_DIFF_MIXED, stale: false, reload: vi.fn(), ...over }
}

beforeEach(() => setHook({}))
afterEach(cleanup)

describe('ContractDiffLens', () => {
  it('renders summary chips from the backend summary and groups by service', () => {
    render(<ContractDiffLens {...props} />)
    expect(screen.getByRole('button', { name: /2 Breaking/ })).toBeTruthy()
    expect(screen.getByRole('button', { name: /1 Not classified/ })).toBeTruthy()
    expect(screen.getAllByText('infra-fleet-service').length).toBeGreaterThan(1)
    expect(screen.getAllByText('api-gateway').length).toBeGreaterThan(1)
  })

  it('breaking-only filter hides non-breaking rows', () => {
    render(<ContractDiffLens {...props} />)
    expect(screen.getByText('Relay.Start')).toBeTruthy()
    fireEvent.click(screen.getByLabelText('Breaking only'))
    expect(screen.queryByText('Relay.Start')).toBeNull()
    expect(screen.getByText('RelayByDevServer.timeout_ms')).toBeTruthy()
  })

  it('"only from this change" keeps changes touching changed files', () => {
    render(<ContractDiffLens {...props} />)
    fireEvent.click(screen.getByLabelText('Only from this change'))
    expect(screen.getByText('codeIntel.findings')).toBeTruthy()
    expect(screen.queryByText('Relay.Start')).toBeNull()
  })

  it('shows the empty state with the scanned kinds, not a "safe" claim', () => {
    setHook({ diff: { ...CONTRACT_DIFF_MIXED, changes: [], migrations: [], totalCount: 0, summary: { breaking: 0, risky: 0, compatible: 0, unknown: 0 } } })
    render(<ContractDiffLens {...props} />)
    expect(screen.getByText(/No contract changes were detected in this scope/)).toBeTruthy()
    expect(screen.queryByText(/safe/i)).toBeNull()
  })

  it('shows truncated hint, error with retry, and loading', () => {
    setHook({ diff: { ...CONTRACT_DIFF_MIXED, truncated: true, totalCount: 900 } })
    const { rerender } = render(<ContractDiffLens {...props} />)
    expect(screen.getByText(/Showing 5 of 900 changes/)).toBeTruthy()
    const reload = vi.fn()
    setHook({ status: 'error', diff: null, error: { kind: 'tool-failed' }, reload })
    rerender(<ContractDiffLens {...props} />)
    fireEvent.click(screen.getByText('Retry'))
    expect(reload).toHaveBeenCalled()
    setHook({ status: 'loading', diff: null })
    rerender(<ContractDiffLens {...props} />)
    expect(screen.getByRole('status')).toBeTruthy()
  })

  it('opens the ERD lens for a sql change only when the ERD lens is loadable', () => {
    ;(REVIEW_LENS_DEFINITIONS as { id: string; load?: unknown }[]).push({ id: 'erd', load: () => null })
    render(<ContractDiffLens {...props} />)
    fireEvent.click(screen.getByText('orders.tenant_id'))
    fireEvent.click(screen.getByText('Open in ERD'))
    expect(actions.setReviewLens).toHaveBeenCalledWith('wt', 'erd')
    expect(actions.selectErdTable).toHaveBeenCalledWith('wt', 'orders')
    REVIEW_LENS_DEFINITIONS.length = 0
  })
})
