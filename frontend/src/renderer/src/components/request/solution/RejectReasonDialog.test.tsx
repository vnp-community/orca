// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { RejectReasonDialog } from './RejectReasonDialog'

afterEach(() => {
  cleanup()
  vi.restoreAllMocks()
})

function setup(onSubmit = vi.fn().mockResolvedValue({ ok: true }), extra = {}) {
  const onOpenChange = vi.fn()
  render(<RejectReasonDialog open onOpenChange={onOpenChange} onSubmit={onSubmit} {...extra} />)
  const box = screen.getByRole('textbox')
  return { onSubmit, onOpenChange, box }
}

describe('RejectReasonDialog', () => {
  it('blocks empty and short reasons', () => {
    const { onSubmit, box } = setup()
    const submit = screen.getByRole('button', { name: 'Reject' })
    expect(submit).toBeDisabled()
    fireEvent.change(box, { target: { value: '   short  ' } })
    expect(submit).toBeDisabled()
    fireEvent.keyDown(box, { key: 'Enter', ctrlKey: true })
    expect(onSubmit).not.toHaveBeenCalled()
    fireEvent.blur(box)
    expect(box).toHaveAttribute('aria-invalid', 'true')
  })

  it('submits exactly 10 characters after trim and closes on ok', async () => {
    const { onSubmit, onOpenChange, box } = setup()
    fireEvent.change(box, { target: { value: '  1234567890  ' } })
    fireEvent.click(screen.getByRole('button', { name: 'Reject' }))
    await waitFor(() => expect(onSubmit).toHaveBeenCalledWith('1234567890', { regenerate: false }))
    await waitFor(() => expect(onOpenChange).toHaveBeenCalledWith(false))
  })

  it('Ctrl+Enter submits on Linux; Meta+Enter does not', async () => {
    vi.spyOn(navigator, 'userAgent', 'get').mockReturnValue('X11; Linux')
    const { onSubmit, box } = setup()
    fireEvent.change(box, { target: { value: 'a long enough reason' } })
    fireEvent.keyDown(box, { key: 'Enter', metaKey: true })
    expect(onSubmit).not.toHaveBeenCalled()
    fireEvent.keyDown(box, { key: 'Enter', ctrlKey: true })
    await waitFor(() => expect(onSubmit).toHaveBeenCalledTimes(1))
  })

  it('Meta+Enter submits on Mac; Ctrl+Enter does not', async () => {
    vi.spyOn(navigator, 'userAgent', 'get').mockReturnValue('Macintosh; Mac OS X')
    const { onSubmit, box } = setup()
    fireEvent.change(box, { target: { value: 'a long enough reason' } })
    fireEvent.keyDown(box, { key: 'Enter', ctrlKey: true })
    expect(onSubmit).not.toHaveBeenCalled()
    fireEvent.keyDown(box, { key: 'Enter', metaKey: true })
    await waitFor(() => expect(onSubmit).toHaveBeenCalledTimes(1))
  })

  it('keeps open and shows an error on failure; sends regenerate flag', async () => {
    const onSubmit = vi.fn().mockResolvedValue({ ok: false, error: { kind: 'network' } })
    const { onOpenChange, box } = setup(onSubmit, { offerRegenerate: true })
    fireEvent.change(box, { target: { value: 'a long enough reason' } })
    fireEvent.click(screen.getByRole('button', { name: 'Reject' }))
    expect(await screen.findByRole('alert')).toBeInTheDocument()
    expect(onSubmit).toHaveBeenCalledWith('a long enough reason', { regenerate: true })
    expect(onOpenChange).not.toHaveBeenCalledWith(false)
  })

  it('locks the button while submitting', () => {
    setup(undefined, { submitting: true })
    expect(screen.getByRole('button', { name: 'Reject' })).toBeDisabled()
  })
})
