// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { afterEach, describe, it, expect, vi } from 'vitest'
import { render, screen, cleanup } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import React from 'react'
import { ConnectionStatusBanner } from '../ConnectionStatusBanner'

// Why: happy-dom env doesn't auto-cleanup between tests — call explicitly
afterEach(() => cleanup())

vi.mock('sonner', () => ({
  toast: {
    success: vi.fn(),
    warning: vi.fn(),
    dismiss: vi.fn()
  }
}))

describe('ConnectionStatusBanner', () => {
  it('is invisible when connected', () => {
    render(<ConnectionStatusBanner status="connected" onRetry={vi.fn()} />)
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  it('shows banner when disconnected', () => {
    render(<ConnectionStatusBanner status="disconnected" onRetry={vi.fn()} />)
    expect(screen.getByRole('alert')).toBeInTheDocument()
    expect(screen.getByText(/connection lost/i)).toBeInTheDocument()
  })

  it('shows banner when connecting', () => {
    render(<ConnectionStatusBanner status="connecting" onRetry={vi.fn()} />)
    expect(screen.getByRole('alert')).toBeInTheDocument()
    expect(screen.getByText(/connecting/i)).toBeInTheDocument()
  })

  it('retry button calls onRetry', async () => {
    const onRetry = vi.fn()
    const user = userEvent.setup()
    render(<ConnectionStatusBanner status="disconnected" onRetry={onRetry} />)
    await user.click(screen.getByRole('button', { name: /retry/i }))
    expect(onRetry).toHaveBeenCalledOnce()
  })

  it('shows loading indicator when connecting', () => {
    render(<ConnectionStatusBanner status="connecting" onRetry={vi.fn()} />)
    // Why: queryAllByRole used since multiple elements may share role across renders
    const statuses = screen.queryAllByRole('status')
    expect(statuses.length).toBeGreaterThan(0)
  })

  it('transitions: connected→disconnected shows banner', () => {
    const { rerender } = render(
      <ConnectionStatusBanner status="connected" onRetry={vi.fn()} />
    )
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()

    rerender(<ConnectionStatusBanner status="disconnected" onRetry={vi.fn()} />)
    expect(screen.getByRole('alert')).toBeInTheDocument()
  })
})
