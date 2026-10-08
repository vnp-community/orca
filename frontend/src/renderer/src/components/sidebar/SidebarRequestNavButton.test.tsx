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
    expect(screen.getByLabelText('7 pending approvals')).toBeInTheDocument()
    useAppStore.setState({ pendingApprovalCount: 250 })
    rerender(<SidebarRequestNavButton />)
    expect(screen.getByText('99+')).toBeInTheDocument()
  })

  it('opens the requests view on click', () => {
    render(<SidebarRequestNavButton />)
    fireEvent.click(screen.getByRole('button'))
    expect(useAppStore.getState().activeView).toBe('requests')
  })
})
