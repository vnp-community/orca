// @vitest-environment happy-dom
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, beforeAll, describe, expect, it, vi } from 'vitest'
import { taskPickerLabel, WorktreeTaskLinkPickerView } from './WorktreeTaskLinkPicker'

vi.mock('../../../hooks/useTasks', () => ({ useTasks: () => ({ filteredTasks: [] }) }))

beforeAll(() => {
  // cmdk measures with these in the browser.
  globalThis.ResizeObserver ??= class {
    observe() {}
    unobserve() {}
    disconnect() {}
  } as unknown as typeof ResizeObserver
  Element.prototype.scrollIntoView ??= () => {}
})
afterEach(cleanup)

const translate = (key: string) => key
const tasks = [
  { id: 't1', title: 'Add weekly filter', taskNumber: 42 },
  { id: 't2', title: 'Fix login' }
]

describe('taskPickerLabel', () => {
  it('formats the per-project number', () => {
    expect(taskPickerLabel(tasks[0])).toBe('#TG-42 Add weekly filter')
    expect(taskPickerLabel(tasks[1])).toBe('Fix login')
  })
})

describe('WorktreeTaskLinkPickerView', () => {
  it('lists tasks and links the chosen id', () => {
    const onLink = vi.fn()
    render(<WorktreeTaskLinkPickerView tasks={tasks} linkedTaskId={null} onLink={onLink} onUnlink={vi.fn()} translate={translate} />)
    fireEvent.click(screen.getByText(/taskPicker\.link/))
    fireEvent.click(screen.getByText('#TG-42 Add weekly filter'))
    expect(onLink).toHaveBeenCalledWith('t1')
  })

  it('offers unlink only when linked', () => {
    const onUnlink = vi.fn()
    const { rerender } = render(
      <WorktreeTaskLinkPickerView tasks={tasks} linkedTaskId={null} onLink={vi.fn()} onUnlink={onUnlink} translate={translate} />
    )
    expect(screen.queryByText(/taskPicker\.unlink/)).toBeNull()
    rerender(<WorktreeTaskLinkPickerView tasks={tasks} linkedTaskId="t1" onLink={vi.fn()} onUnlink={onUnlink} translate={translate} />)
    fireEvent.click(screen.getByText(/taskPicker\.unlink/))
    expect(onUnlink).toHaveBeenCalledTimes(1)
  })

  it('disables controls when disabled', () => {
    render(<WorktreeTaskLinkPickerView tasks={tasks} linkedTaskId="t1" disabled onLink={vi.fn()} onUnlink={vi.fn()} translate={translate} />)
    for (const button of screen.getAllByRole('button')) {expect((button as HTMLButtonElement).disabled).toBe(true)}
  })
})
