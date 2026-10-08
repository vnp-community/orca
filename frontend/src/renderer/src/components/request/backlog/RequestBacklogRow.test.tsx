// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { Table, TableBody } from '@/components/ui/table'
import { RequestBacklogRow } from './RequestBacklogRow'
import { backlogRow } from './backlog-fixtures'
import type { RequestBacklogRowData } from '../../../../../shared/request-backlog-types'

afterEach(cleanup)
const NOW = Date.parse('2026-10-07T12:00:00Z')

function renderRow(row: RequestBacklogRowData) {
  const h = { onOpen: vi.fn(), onReopen: vi.fn(), onCancel: vi.fn() }
  render(
    <Table><TableBody>
      <RequestBacklogRow row={row} now={NOW} locale="en" active={false} rowProps={{}} {...h} />
    </TableBody></Table>
  )
  return h
}

describe('RequestBacklogRow', () => {
  it('shows the translated stage, category, reason and relative time', () => {
    renderRow(backlogRow())
    expect(screen.getByText('#9 Crash on save')).toBeInTheDocument()
    expect(screen.getByText('Plan')).toBeInTheDocument()
    expect(screen.getByText('Not feasible')).toBeInTheDocument()
    expect(screen.getByText('Needs a vendor change').className).toContain('line-clamp-2')
    expect(screen.getByText('Needs a vendor change')).toHaveAttribute('title', 'Needs a vendor change')
    expect(screen.getByText(/yesterday/)).toBeInTheDocument()
  })

  it('shows dashes for unknown stage/category/type and "AI / System" for system actors', () => {
    renderRow(backlogRow({ returnedFromStage: 'unknown', returnedCategory: 'unknown', type: null, returnedBy: 'system', returnReason: undefined }))
    expect(screen.getAllByText('-').length).toBeGreaterThanOrEqual(3)
    expect(screen.getByText('AI / System')).toBeInTheDocument()
  })

  it('emits open, reopen and cancel', () => {
    const h = renderRow(backlogRow())
    fireEvent.click(screen.getByText('#9 Crash on save'))
    fireEvent.click(screen.getByRole('button', { name: 'Reopen' }))
    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }))
    expect(h.onOpen).toHaveBeenCalled()
    expect(h.onReopen).toHaveBeenCalled()
    expect(h.onCancel).toHaveBeenCalled()
  })
})
