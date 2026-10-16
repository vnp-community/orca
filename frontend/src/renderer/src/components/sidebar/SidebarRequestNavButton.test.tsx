// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { render, screen, cleanup, fireEvent } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it } from 'vitest'
import { useAppStore } from '@/store'
import { SidebarRequestNavButton } from './SidebarRequestNavButton'

beforeEach(() =>
  useAppStore.setState({
    requestFlowSupport: 'supported',
    pendingApprovalCount: 0,
    activeView: 'terminal'
  })
)
afterEach(cleanup)

describe('SidebarRequestNavButton', () => {
  it('is hidden unless the runtime supports the request flow', () => {
    useAppStore.setState({ requestFlowSupport: 'unsupported' })
    const { container } = render(<SidebarRequestNavButton />)
    expect(container).toBeEmptyDOMElement()
  })

  it('hides the badge at 0, shows the count, and caps at 99+', () => {
    const { rerender } = render(<SidebarRequestNavButton />)
    expect(screen.queryByText('0')).toBeNull()
    useAppStore.setState({ pendingApprovalCount: 7 })
    rerender(<SidebarRequestNavButton />)
    expect(screen.getByText('7')).toBeInTheDocument()
    expect(screen.getByRole('button')).toHaveAccessibleName('Requests, 7 pending approvals')
    useAppStore.setState({ pendingApprovalCount: 250 })
    rerender(<SidebarRequestNavButton />)
    expect(screen.getByText('99+')).toBeInTheDocument()
  })

  it('renders a labelled row with the primary-tone pill and no aria-pressed', () => {
    useAppStore.setState({ pendingApprovalCount: 3 })
    render(<SidebarRequestNavButton />)
    const button = screen.getByTestId('sidebar-requests')
    expect(button).toHaveTextContent('Requests')
    expect(button).toHaveAttribute('title', 'Requests, 3 pending approvals')
    expect(button).not.toHaveAttribute('aria-pressed')
    expect(screen.getByTestId('sidebar-requests-badge')).toHaveClass('bg-primary')
    expect(button).not.toHaveAttribute('aria-current')
    expect(button).toHaveClass('text-worktree-sidebar-foreground/60')
  })

  it('marks the row as the current page when the requests view is active', () => {
    useAppStore.setState({ activeView: 'requests' })
    render(<SidebarRequestNavButton />)
    const button = screen.getByTestId('sidebar-requests')
    expect(button).toHaveAttribute('aria-current', 'page')
    expect(button).toHaveClass('bg-worktree-sidebar-accent')
    expect(button).toHaveAccessibleName('Requests')
  })

  it('opens the requests view on click', () => {
    render(<SidebarRequestNavButton />)
    fireEvent.click(screen.getByRole('button'))
    expect(useAppStore.getState().activeView).toBe('requests')
  })
})
