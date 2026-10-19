// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { TaskLoadStates } from '../TaskLoadStates'

afterEach(cleanup)

const base = {
  loaded: true,
  isLoading: false,
  loadError: null,
  visibleCount: 0,
  totalCount: 0,
  filtersActive: false,
  planningHidden: false,
  onRetry: vi.fn()
}

describe('TaskLoadStates', () => {
  it('shows nothing before the first load ends and while tasks are visible', () => {
    const { container, rerender } = render(<TaskLoadStates {...base} loaded={false} />)
    expect(container).toBeEmptyDOMElement()
    rerender(<TaskLoadStates {...base} visibleCount={2} totalCount={2} />)
    expect(container).toBeEmptyDOMElement()
  })

  it('says "loading" only while there is nothing to show yet', () => {
    const { rerender } = render(<TaskLoadStates {...base} loaded={false} isLoading />)
    expect(screen.getByTestId('task-loading')).toBeInTheDocument()
    rerender(<TaskLoadStates {...base} isLoading visibleCount={1} totalCount={1} />)
    expect(screen.queryByTestId('task-loading')).toBeNull()
  })

  it('a project without tasks is empty, not an error', () => {
    render(<TaskLoadStates {...base} />)
    expect(screen.getByTestId('task-empty').textContent).toBe('No tasks yet.')
    expect(screen.queryByRole('alert')).toBeNull()
  })

  it('explains why a non-empty project shows nothing: filters or hidden Plan/Phase', () => {
    const { rerender } = render(<TaskLoadStates {...base} totalCount={4} filtersActive />)
    expect(screen.getByTestId('task-empty-filtered')).toBeInTheDocument()
    rerender(<TaskLoadStates {...base} totalCount={4} planningHidden />)
    expect(screen.getByTestId('task-empty-planning-hidden').textContent).toContain(
      'Show Plan/Phase'
    )
  })

  it('a failed load is an alert with the cause, the code and a retry, and never says "No tasks"', () => {
    const onRetry = vi.fn()
    render(
      <TaskLoadStates
        {...base}
        loadError={{ kind: 'network', code: 'NETWORK_ERROR' }}
        onRetry={onRetry}
      />
    )
    const alert = screen.getByTestId('task-load-error')
    expect(alert).toHaveAttribute('role', 'alert')
    expect(alert.textContent).toContain('Could not load the tasks.')
    expect(alert.textContent).toContain('Network error')
    expect(screen.getByTestId('task-load-error-code').textContent).toContain('NETWORK_ERROR')
    expect(screen.queryByTestId('task-empty')).toBeNull()
    fireEvent.click(screen.getByRole('button', { name: 'Retry' }))
    expect(onRetry).toHaveBeenCalledTimes(1)
  })

  it('names missing access separately from a broken connection', () => {
    render(<TaskLoadStates {...base} loadError={{ kind: 'forbidden', code: 'TASK_FORBIDDEN' }} />)
    expect(screen.getByTestId('task-load-error').textContent).toContain('do not have access')
  })

  it('keeps the last list visible and says the refresh failed when tasks are already there', () => {
    render(
      <TaskLoadStates
        {...base}
        totalCount={3}
        visibleCount={3}
        loadError={{ kind: 'unavailable', code: 'SERVICE_UNAVAILABLE' }}
      />
    )
    expect(screen.getByTestId('task-load-error').textContent).toContain(
      'Showing the last loaded list'
    )
  })
})
