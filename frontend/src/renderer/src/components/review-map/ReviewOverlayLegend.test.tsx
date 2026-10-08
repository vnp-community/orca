// @vitest-environment happy-dom
import { cleanup, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it } from 'vitest'
import { TooltipProvider } from '@/components/ui/tooltip'
import { ReviewOverlayLegend } from './ReviewOverlayLegend'

afterEach(cleanup)

describe('ReviewOverlayLegend', () => {
  it('lists only the given flags with text labels', () => {
    render(
      <TooltipProvider>
        <ReviewOverlayLegend flags={['changed', 'untested']} />
      </TooltipProvider>
    )
    expect(screen.getAllByRole('listitem')).toHaveLength(2)
    expect(screen.getByText('Changed')).toBeTruthy()
    expect(screen.getByText('No test found')).toBeTruthy()
    expect(screen.queryByText('Affected')).toBeNull()
  })
  it('renders nothing when no flag has data', () => {
    const { container } = render(<ReviewOverlayLegend flags={[]} />)
    expect(container.firstChild).toBeNull()
  })
})
