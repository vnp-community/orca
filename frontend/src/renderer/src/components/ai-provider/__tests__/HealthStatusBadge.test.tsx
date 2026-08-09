// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { render, screen, cleanup } from '@testing-library/react'
import { afterEach, describe, expect, it } from 'vitest'
import { HealthStatusBadge } from '../HealthStatusBadge'

afterEach(() => cleanup())

describe('HealthStatusBadge', () => {
  it('active → green text with CheckCircle', () => {
    render(<HealthStatusBadge status="active" />)
    const badge = screen.getByTestId('status-badge-active')
    expect(badge).toBeInTheDocument()
    expect(badge).toHaveClass('text-green-600')
    expect(badge.textContent).toContain('Active')
  })

  it('invalid → red text with XCircle', () => {
    render(<HealthStatusBadge status="invalid" />)
    const badge = screen.getByTestId('status-badge-invalid')
    expect(badge).toHaveClass('text-red-600')
    expect(badge.textContent).toContain('Invalid Key')
  })

  it('quota_exceeded → orange text with AlertTriangle', () => {
    render(<HealthStatusBadge status="quota_exceeded" />)
    const badge = screen.getByTestId('status-badge-quota_exceeded')
    expect(badge).toHaveClass('text-orange-600')
    expect(badge.textContent).toContain('Quota Exceeded')
  })

  it('unreachable → gray text with WifiOff', () => {
    render(<HealthStatusBadge status="unreachable" />)
    const badge = screen.getByTestId('status-badge-unreachable')
    expect(badge).toHaveClass('text-gray-500')
    expect(badge.textContent).toContain('Unreachable')
  })

  it('pending → yellow text with Clock', () => {
    render(<HealthStatusBadge status="pending" />)
    const badge = screen.getByTestId('status-badge-pending')
    expect(badge).toHaveClass('text-yellow-600')
    expect(badge.textContent).toContain('Pending')
  })
})
