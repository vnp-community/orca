// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { TooltipProvider } from '@/components/ui/tooltip'
import { BacklogSegmentControl, backlogViewForKeyEvent } from './BacklogSegmentControl'

afterEach(cleanup)

const counts = { requests: { count: 5, plus: true }, tasks: null, execute: { count: 0, plus: false } }

function setup() {
  const onChange = vi.fn()
  render(
    <TooltipProvider>
      <BacklogSegmentControl value="requests" onChange={onChange} counts={counts} />
    </TooltipProvider>
  )
  return onChange
}

describe('BacklogSegmentControl', () => {
  it('changes view by click, never reports an empty selection, and shows counts with + for more pages', () => {
    const onChange = setup()
    expect(screen.getByTestId('backlog-count-requests')).toHaveTextContent('5+')
    expect(screen.queryByTestId('backlog-count-tasks')).toBeNull()
    fireEvent.click(screen.getByRole('radio', { name: /Execute/ }))
    expect(onChange).toHaveBeenCalledWith('execute')
    onChange.mockClear()
    fireEvent.click(screen.getByRole('radio', { name: /Requests/ }))
    expect(onChange).not.toHaveBeenCalled()
  })
})

describe('backlogViewForKeyEvent', () => {
  const target = document.createElement('div')
  it('maps 1/2/3 to views', () => {
    expect(backlogViewForKeyEvent({ key: '1', target })).toBe('requests')
    expect(backlogViewForKeyEvent({ key: '2', target })).toBe('tasks')
    expect(backlogViewForKeyEvent({ key: '3', target })).toBe('execute')
    expect(backlogViewForKeyEvent({ key: '4', target })).toBeNull()
  })
  it('ignores typing targets, modifiers (either platform) and IME', () => {
    const input = document.createElement('input')
    expect(backlogViewForKeyEvent({ key: '1', target: input })).toBeNull()
    expect(backlogViewForKeyEvent({ key: '1', target, ctrlKey: true })).toBeNull()
    expect(backlogViewForKeyEvent({ key: '1', target, metaKey: true })).toBeNull()
    expect(backlogViewForKeyEvent({ key: '1', target, shiftKey: true })).toBeNull()
    expect(backlogViewForKeyEvent({ key: '1', target, altKey: true })).toBeNull()
    expect(backlogViewForKeyEvent({ key: '1', target, isComposing: true })).toBeNull()
  })
})
