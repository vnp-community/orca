// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const { reopen, toastFn } = vi.hoisted(() => ({
  reopen: vi.fn(),
  toastFn: Object.assign(vi.fn(), { success: vi.fn(), error: vi.fn() })
}))
vi.mock('../../../hooks/useRequestActions', () => ({ useRequestActions: () => ({ reopen }) }))
vi.mock('sonner', () => ({ toast: toastFn }))

import { ReopenRequestDialog } from './ReopenRequestDialog'

const request = { id: 'r1', number: 5, title: 'Broken login', returnedFromStage: 'plan' as const }

function setup() {
  const onOpenChange = vi.fn()
  const onReopened = vi.fn()
  render(<ReopenRequestDialog open onOpenChange={onOpenChange} request={request} onReopened={onReopened} />)
  return { onOpenChange, onReopened }
}

beforeEach(() => { reopen.mockReset(); toastFn.mockReset(); toastFn.error.mockReset() })
afterEach(cleanup)

describe('ReopenRequestDialog', () => {
  it('names the stage the request came back from and says it will be classified again', () => {
    setup()
    expect(screen.getByText(/Request #5 was returned from Plan and will be classified again/)).toBeInTheDocument()
  })

  it('reopens by id and reports success', async () => {
    reopen.mockResolvedValue({ ok: true, value: {} })
    const { onOpenChange, onReopened } = setup()
    fireEvent.click(screen.getByRole('button', { name: 'Reopen' }))
    await waitFor(() => expect(onReopened).toHaveBeenCalledWith('r1'))
    expect(reopen).toHaveBeenCalledWith('r1')
    expect(onOpenChange).toHaveBeenCalledWith(false)
  })

  it('treats invalid_state as already handled: neutral toast, closes, drops the row', async () => {
    reopen.mockResolvedValue({ ok: false, error: { kind: 'invalid_state', code: 'x', message: 'x' } })
    const { onOpenChange, onReopened } = setup()
    fireEvent.click(screen.getByRole('button', { name: 'Reopen' }))
    await waitFor(() => expect(onReopened).toHaveBeenCalledWith('r1'))
    expect(toastFn).toHaveBeenCalledTimes(1)
    expect(toastFn.error).not.toHaveBeenCalled()
    expect(onOpenChange).toHaveBeenCalledWith(false)
  })

  it('keeps the dialog open on forbidden (toast) and on network errors (inline)', async () => {
    reopen.mockResolvedValueOnce({ ok: false, error: { kind: 'forbidden', code: 'x', message: 'x' } })
    const { onOpenChange, onReopened } = setup()
    fireEvent.click(screen.getByRole('button', { name: 'Reopen' }))
    await waitFor(() => expect(toastFn.error).toHaveBeenCalled())
    expect(onOpenChange).not.toHaveBeenCalled()
    reopen.mockResolvedValueOnce({ ok: false, error: { kind: 'network', code: 'x', message: 'x' } })
    fireEvent.click(screen.getByRole('button', { name: 'Reopen' }))
    expect(await screen.findByRole('alert')).toBeInTheDocument()
    expect(onReopened).not.toHaveBeenCalled()
  })

  it('ignores a second click while the first call is in flight', async () => {
    let resolve: (v: unknown) => void = () => {}
    reopen.mockReturnValue(new Promise((r) => { resolve = r }))
    setup()
    const button = screen.getByRole('button', { name: 'Reopen' })
    fireEvent.click(button)
    fireEvent.click(button)
    expect(reopen).toHaveBeenCalledTimes(1)
    resolve({ ok: true, value: {} })
    await waitFor(() => expect(button).not.toBeDisabled())
  })
})
