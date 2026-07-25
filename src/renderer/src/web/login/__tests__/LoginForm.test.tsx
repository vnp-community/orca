// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { render, screen, fireEvent, cleanup } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { LoginForm } from '../LoginForm'

describe('LoginForm', () => {
  afterEach(cleanup)

  it('calls onSubmit with email and password', () => {
    const onSubmit = vi.fn()
    render(<LoginForm onSubmit={onSubmit} isLoading={false} error={null} />)
    fireEvent.change(screen.getByLabelText(/email/i), {
      target: { value: 'alice@co.com' }
    })
    fireEvent.change(screen.getByLabelText(/password/i), {
      target: { value: 'pass123' }
    })
    fireEvent.submit(screen.getByRole('form'))
    expect(onSubmit).toHaveBeenCalledWith('alice@co.com', 'pass123')
  })

  it('does not call onSubmit for invalid email format', () => {
    const onSubmit = vi.fn()
    render(<LoginForm onSubmit={onSubmit} isLoading={false} error={null} />)
    fireEvent.change(screen.getByLabelText(/email/i), {
      target: { value: 'not-an-email' }
    })
    fireEvent.submit(screen.getByRole('form'))
    expect(onSubmit).not.toHaveBeenCalled()
  })

  it('displays a server-side error passed via props', () => {
    render(
      <LoginForm onSubmit={vi.fn()} isLoading={false} error="Invalid credentials" />
    )
    expect(screen.getByRole('alert')).toHaveTextContent('Invalid credentials')
  })

  it('disables the submit button while isLoading', () => {
    render(<LoginForm onSubmit={vi.fn()} isLoading={true} error={null} />)
    expect(screen.getByRole('button', { name: /signing in/i })).toBeDisabled()
  })
})
