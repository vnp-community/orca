// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const { reopen, cancel, openMock, toastFn } = vi.hoisted(() => ({
  reopen: vi.fn(),
  cancel: vi.fn(),
  openMock: vi.fn(),
  toastFn: Object.assign(vi.fn(), { success: vi.fn(), error: vi.fn() })
}))
vi.mock('../../../hooks/useRequestActions', () => ({ useRequestActions: () => ({ reopen, cancel }) }))
vi.mock('../request-page-navigation', () => ({ openRequestPage: (...a: unknown[]) => openMock(...a) }))
vi.mock('sonner', () => ({ toast: toastFn }))

import { RequestBacklogTable } from './RequestBacklogTable'
import { backlogRow } from './backlog-fixtures'

const NOW = Date.parse('2026-10-07T12:00:00Z')
function setup() {
  const onChanged = vi.fn()
  const rows = [backlogRow({ requestId: 'r1', number: 1, title: 'One' }), backlogRow({ requestId: 'r2', number: 2, title: 'Two' })]
  render(
    <RequestBacklogTable rows={rows} now={NOW} locale="en" hasMore={false} isLoadingMore={false} onLoadMore={vi.fn()} onChanged={onChanged} />
  )
  return { onChanged }
}

beforeEach(() => { for (const m of [reopen, cancel, openMock, toastFn, toastFn.error]) {m.mockReset()} })
afterEach(cleanup)

describe('RequestBacklogTable', () => {
  it('reopen: dialog, confirm calls request.reopen, row disappears, toast offers View request', async () => {
    reopen.mockResolvedValue({ ok: true, value: {} })
    const { onChanged } = setup()
    fireEvent.click(screen.getAllByRole('button', { name: 'Reopen' })[0])
    fireEvent.click(within(await screen.findByTestId('reopen-request-dialog')).getByRole('button', { name: 'Reopen' }))
    await waitFor(() => expect(reopen).toHaveBeenCalledWith('r1'))
    await waitFor(() => expect(screen.queryByText('#1 One')).toBeNull())
    expect(onChanged).toHaveBeenCalled()
    const opts = toastFn.mock.calls[0][1] as { action: { label: string; onClick: () => void } }
    expect(opts.action.label).toBe('View request')
    opts.action.onClick()
    expect(openMock).toHaveBeenCalledWith({ section: 'requests', requestId: 'r1' })
  })

  it('cancel needs confirmation and sends the optional reason', async () => {
    cancel.mockResolvedValue({ ok: true, value: {} })
    setup()
    fireEvent.click(screen.getAllByRole('button', { name: 'Cancel' })[1])
    expect(cancel).not.toHaveBeenCalled()
    fireEvent.click(await screen.findByRole('button', { name: 'Cancel request' }))
    await waitFor(() => expect(cancel).toHaveBeenCalledWith({ id: 'r2', reason: undefined }))
    await waitFor(() => expect(screen.queryByText('#2 Two')).toBeNull())
  })

  it('after REQUEST_REASON_REQUIRED the reason becomes mandatory and the dialog stays open', async () => {
    cancel.mockResolvedValue({ ok: false, error: { kind: 'validation', code: 'x', message: 'REQUEST_REASON_REQUIRED: x' } })
    setup()
    fireEvent.click(screen.getAllByRole('button', { name: 'Cancel' })[0])
    fireEvent.click(await screen.findByRole('button', { name: 'Cancel request' }))
    await waitFor(() => expect(cancel).toHaveBeenCalledTimes(1))
    expect(await screen.findByText('Reason (required)')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Cancel request' })).toBeDisabled()
  })

  it('invalid_state on cancel drops the row with a neutral toast', async () => {
    cancel.mockResolvedValue({ ok: false, error: { kind: 'invalid_state', code: 'x', message: 'x' } })
    setup()
    fireEvent.click(screen.getAllByRole('button', { name: 'Cancel' })[0])
    fireEvent.click(await screen.findByRole('button', { name: 'Cancel request' }))
    await waitFor(() => expect(screen.queryByText('#1 One')).toBeNull())
    expect(toastFn.error).not.toHaveBeenCalled()
  })

  it('j/k/Enter open the selected request, but Enter on a row button does not navigate', () => {
    setup()
    const grid = screen.getByTestId('request-backlog-table')
    fireEvent.keyDown(grid, { key: 'j' })
    fireEvent.keyDown(grid, { key: 'j' })
    fireEvent.keyDown(grid, { key: 'Enter' })
    expect(openMock).toHaveBeenCalledWith({ section: 'requests', requestId: 'r2' })
    openMock.mockClear()
    fireEvent.keyDown(screen.getAllByRole('button', { name: 'Cancel' })[0], { key: 'Enter' })
    expect(openMock).not.toHaveBeenCalled()
  })
})
