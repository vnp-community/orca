// @vitest-environment happy-dom
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
vi.mock('../notes/ReviewNoteButton', () => ({
  ReviewNoteButton: (p: { anchor: unknown }) => (
    <button type="button" data-testid="note-btn" data-anchor={JSON.stringify(p.anchor)}>
      note
    </button>
  )
}))

import { ContractCompatibilityBadge } from './ContractCompatibilityBadge'
import { ContractChangeTable } from './ContractChangeTable'
import { ContractChangeDetail } from './ContractChangeDetail'
import { ContractMigrationGroup } from './ContractMigrationGroup'
import { CONTRACT_CHANGES_MIXED, makeContractChange } from '../../../test-support/contract-findings-fixtures'

afterEach(cleanup)

describe('ContractCompatibilityBadge', () => {
  it('shows distinct text for breaking, risky, unknown and compatible', () => {
    const { rerender } = render(<ContractCompatibilityBadge compatibility="breaking" />)
    expect(screen.getByText('Breaking')).toBeTruthy()
    rerender(<ContractCompatibilityBadge compatibility="risky" />)
    expect(screen.getByText('Risky')).toBeTruthy()
    rerender(<ContractCompatibilityBadge compatibility="weird-new-value" />)
    expect(screen.getByText('Not classified')).toBeTruthy()
    expect(document.querySelector('[data-compat="unknown"]')).not.toBeNull()
    rerender(<ContractCompatibilityBadge compatibility="compatible" />)
    expect(screen.getByText('Compatible')).toBeTruthy()
  })
})

describe('ContractChangeTable', () => {
  it('renders before/after tokens, details fallback and the compatibility text', () => {
    const onSelect = vi.fn()
    render(<ContractChangeTable changes={CONTRACT_CHANGES_MIXED} selectedId={null} onSelect={onSelect} />)
    expect(screen.getByRole('table')).toBeTruthy()
    expect(screen.getAllByText('Breaking').length).toBe(2)
    expect(screen.getByText('Risky')).toBeTruthy()
    expect(screen.getByText('Not classified')).toBeTruthy()
    // c4 has no before/after => details key/value, with the DSN masked.
    expect(screen.queryByText(/hunter2/)).toBeNull()
    fireEvent.click(screen.getByText('Relay.Start'))
    expect(onSelect).toHaveBeenCalledWith('c2')
  })

  it('shows HTML in names as text, not markup', () => {
    const evil = makeContractChange({ id: 'x', name: '<img src=x onerror=alert(1)>' })
    const { container } = render(<ContractChangeTable changes={[evil]} selectedId={null} onSelect={() => {}} />)
    expect(container.querySelector('img')).toBeNull()
    expect(screen.getByText('<img src=x onerror=alert(1)>')).toBeTruthy()
  })
})

describe('ContractChangeDetail', () => {
  const base = { changedFiles: new Set(['proto/relay.proto']), onOpenDiff: vi.fn(), onSelectSymbol: vi.fn() }

  it('says consumers were not found (not "unused") for a breaking change', () => {
    render(<ContractChangeDetail {...base} change={makeContractChange()} />)
    expect(screen.getByText(/No consumers found \(they may be outside this repo/)).toBeTruthy()
    expect(screen.queryByText(/unused|nobody/i)).toBeNull()
  })

  it('opens the diff at the evidence line only for changed files', () => {
    const onOpenDiff = vi.fn()
    render(
      <ContractChangeDetail
        {...base}
        onOpenDiff={onOpenDiff}
        change={makeContractChange({ files: ['proto/relay.proto', 'other.proto'] })}
      />
    )
    fireEvent.click(screen.getByText('View diff'))
    expect(onOpenDiff).toHaveBeenCalledWith('proto/relay.proto', 12)
    expect(screen.getByText('not in this change')).toBeTruthy()
  })

  it('offers Open in ERD only for sql changes when an ERD handler exists', () => {
    const onOpenErd = vi.fn()
    const sql = makeContractChange({ id: 's', kind: 'sql-column', name: 'orders.tenant_id', service: 'order-service' })
    const { rerender } = render(<ContractChangeDetail {...base} change={sql} onOpenErd={onOpenErd} />)
    fireEvent.click(screen.getByText('Open in ERD'))
    expect(onOpenErd).toHaveBeenCalledWith('orders', 'order-service')
    rerender(<ContractChangeDetail {...base} change={makeContractChange()} onOpenErd={onOpenErd} />)
    expect(screen.queryByText('Open in ERD')).toBeNull()
    rerender(<ContractChangeDetail {...base} change={sql} />)
    expect(screen.queryByText('Open in ERD')).toBeNull()
  })

  it('offers a Note anchored to the contract change only when a worktree is given', () => {
    const { rerender } = render(<ContractChangeDetail {...base} change={makeContractChange()} />)
    expect(screen.queryByTestId('note-btn')).toBeNull()
    rerender(<ContractChangeDetail {...base} worktreeId="wt" change={makeContractChange()} />)
    expect(JSON.parse(screen.getByTestId('note-btn').getAttribute('data-anchor')!)).toMatchObject({
      kind: 'graph-node',
      lens: 'contract',
      nodeKey: 'c1',
      filePath: 'proto/relay.proto',
      startLine: 12,
      label: 'RelayByDevServer.timeout_ms'
    })
  })

  it('lists consumers and selects a symbol', () => {
    const onSelectSymbol = vi.fn()
    const change = makeContractChange({
      consumers: [{ kind: 'rpc-client', symbol: { key: 'k1', kind: 'function', name: 'callRelay', filePath: 'a.ts' } }]
    })
    render(<ContractChangeDetail {...base} onSelectSymbol={onSelectSymbol} change={change} />)
    fireEvent.click(screen.getByText('callRelay'))
    expect(onSelectSymbol).toHaveBeenCalledWith('k1')
  })
})

describe('ContractMigrationGroup', () => {
  const migration = {
    service: 'order-service',
    dialects: ['postgres'],
    files: ['db/0042.sql'],
    statements: [{ table: 'orders', op: 'DROP COLUMN', column: 'tenant_id', compatibility: 'breaking', ruleId: 'sql.column.dropped' }],
    tables: [{ table: 'orders', service: 'order-service', change: 'modified', accessors: [], columnsReferenced: [], evidence: [] }],
    findings: []
  }

  it('lists statements and opens the ERD for a table', () => {
    const onOpenErd = vi.fn()
    render(<ContractMigrationGroup migration={migration} onOpenErd={onOpenErd} />)
    expect(screen.getByText('DROP COLUMN orders.tenant_id')).toBeTruthy()
    fireEvent.click(screen.getByText('Open in ERD'))
    expect(onOpenErd).toHaveBeenCalledWith('orders', 'order-service')
  })
})
