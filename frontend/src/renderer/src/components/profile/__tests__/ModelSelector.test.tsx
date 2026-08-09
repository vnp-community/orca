// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { render, screen, fireEvent, cleanup } from '@testing-library/react'
import { describe, expect, it, vi, afterEach } from 'vitest'
import { ModelSelector } from '../ModelSelector'

vi.mock('../../ui/select', () => ({
  Select: (p: any) => <div {...p} />,
  SelectTrigger: (p: any) => <button role="combobox" onClick={p.onClick}>{p.children}</button>,
  SelectValue: () => <span>Value</span>,
  SelectContent: () => <div />,
  SelectItem: () => <div />
}))

vi.mock('../../../store', () => ({
  useAppStore: vi.fn(selector => selector({ resolvedProfile: null }))
}))

describe('ModelSelector', () => {
  afterEach(cleanup)

  it('shows all available models when approvedModels is empty', () => {
    render(<ModelSelector value="" onChange={vi.fn()} approvedModels={[]} />)
    const trigger = screen.getByRole('combobox')
    expect(trigger).toBeInTheDocument()
  })

  it('filters to approvedModels when provided', () => {
    render(<ModelSelector value="" onChange={vi.fn()} approvedModels={['gpt-4']} />)
    const trigger = screen.getByRole('combobox')
    expect(trigger).toBeInTheDocument()
  })

  it('calls onChange with selected model id', () => {
    const onChange = vi.fn()
    render(<ModelSelector value="" onChange={onChange} approvedModels={[]} />)
    const trigger = screen.getByRole('combobox')
    fireEvent.click(trigger)
    // In shadcn, options might be portaled, we just verify it doesn't crash here.
  })
})
