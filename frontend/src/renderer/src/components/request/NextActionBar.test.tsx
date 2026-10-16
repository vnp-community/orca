// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { cleanup, render, screen, fireEvent } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { NextActionBar } from './NextActionBar'

afterEach(cleanup)

describe('NextActionBar', () => {
  it('renders nothing visible for none, but keeps a polite live region', () => {
    render(<NextActionBar action={{ kind: 'none', title: '' }} onCommand={vi.fn()} />)
    expect(screen.queryByTestId('next-action-bar')).toBeNull()
    expect(screen.getByTestId('next-action-empty').getAttribute('aria-live')).toBe('polite')
  })

  it('shows the title, the detail and a button that runs the command', () => {
    const onCommand = vi.fn()
    const action = {
      kind: 'failed' as const,
      title: 'TG-1 failed 2/2 times',
      detail: 'kept',
      primary: { label: 'Reopen and rerun', command: 'reopen' as const }
    }
    render(<NextActionBar action={action} onCommand={onCommand} />)
    expect(screen.getByTestId('next-action-bar').getAttribute('data-kind')).toBe('failed')
    expect(screen.getByText('kept')).toBeTruthy()
    fireEvent.click(screen.getByRole('button', { name: 'Reopen and rerun' }))
    expect(onCommand).toHaveBeenCalledWith('reopen', action)
  })

  it('has no button when the action has no primary command', () => {
    render(
      <NextActionBar
        action={{ kind: 'waiting', title: 'Orca is writing the plan' }}
        onCommand={vi.fn()}
      />
    )
    expect(screen.queryByTestId('next-action-primary')).toBeNull()
  })
})
