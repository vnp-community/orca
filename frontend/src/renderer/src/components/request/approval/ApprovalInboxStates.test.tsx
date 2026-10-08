// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import {
  ApprovalInboxEmptyFiltered, ApprovalInboxEmptyState, ApprovalInboxErrorState, ApprovalInboxSkeleton
} from './ApprovalInboxStates'

afterEach(cleanup)

describe('ApprovalInboxStates', () => {
  it('renders six skeleton rows', () => {
    render(<ApprovalInboxSkeleton />)
    expect(screen.getByTestId('approval-skeleton').children).toHaveLength(6)
  })
  it('renders the empty state', () => {
    render(<ApprovalInboxEmptyState />)
    expect(screen.getByText('Nothing is waiting for your approval.')).toBeInTheDocument()
  })
  it('renders the filtered empty state with a working clear button', () => {
    const onClear = vi.fn()
    render(<ApprovalInboxEmptyFiltered onClear={onClear} />)
    fireEvent.click(screen.getByRole('button', { name: 'Clear filters' }))
    expect(onClear).toHaveBeenCalled()
  })
  it('network error offers retry; forbidden does not', () => {
    const onRetry = vi.fn()
    const { unmount } = render(<ApprovalInboxErrorState kind="network" onRetry={onRetry} />)
    fireEvent.click(screen.getByRole('button', { name: 'Retry' }))
    expect(onRetry).toHaveBeenCalled()
    unmount()
    render(<ApprovalInboxErrorState kind="forbidden" onRetry={onRetry} />)
    expect(screen.queryByRole('button', { name: 'Retry' })).toBeNull()
    expect(screen.getByRole('alert')).toBeInTheDocument()
  })
})
