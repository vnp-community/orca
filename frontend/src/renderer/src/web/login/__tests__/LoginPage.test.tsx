// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { render, screen, fireEvent, waitFor, cleanup } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { LoginPage } from '../LoginPage'
import * as authApiClient from '../../../auth/auth-api-client'

vi.mock('../../../auth/auth-api-client')

const mockUser = {
  id: 'u1',
  email: 'alice@co.com',
  name: 'Alice',
  role: 'developer' as const,
  provider: 'none' as const
}

describe('LoginPage', () => {
  afterEach(cleanup)

  describe('local login form', () => {
    it('renders email, password fields and Sign In button', () => {
      render(<LoginPage availableProviders={[]} onLoginSuccess={vi.fn()} />)
      expect(screen.getByLabelText(/email/i)).toBeInTheDocument()
      expect(screen.getByLabelText(/password/i)).toBeInTheDocument()
      expect(screen.getByRole('button', { name: /sign in/i })).toBeInTheDocument()
    })

    it('calls loginLocal with email and password on submit', async () => {
      const onSuccess = vi.fn()
      vi.mocked(authApiClient.loginLocal).mockResolvedValueOnce(mockUser)
      render(<LoginPage availableProviders={[]} onLoginSuccess={onSuccess} />)
      fireEvent.change(screen.getByLabelText(/email/i), {
        target: { value: 'alice@co.com' }
      })
      fireEvent.change(screen.getByLabelText(/password/i), {
        target: { value: 'password123' }
      })
      fireEvent.click(screen.getByRole('button', { name: /sign in/i }))
      await waitFor(() =>
        expect(onSuccess).toHaveBeenCalledWith(
          expect.objectContaining({ email: 'alice@co.com' })
        )
      )
    })

    it('shows error message when loginLocal throws', async () => {
      vi.mocked(authApiClient.loginLocal).mockRejectedValueOnce(
        Object.assign(new Error('Invalid credentials'), { code: 'invalid_credentials' })
      )
      render(<LoginPage availableProviders={[]} onLoginSuccess={vi.fn()} />)
      fireEvent.change(screen.getByLabelText(/email/i), {
        target: { value: 'bad@co.com' }
      })
      fireEvent.change(screen.getByLabelText(/password/i), {
        target: { value: 'wrong' }
      })
      fireEvent.click(screen.getByRole('button', { name: /sign in/i }))
      await waitFor(() =>
        expect(screen.getByRole('alert')).toHaveTextContent(/invalid credentials/i)
      )
    })

    it('disables button during loading', async () => {
      vi.mocked(authApiClient.loginLocal).mockImplementationOnce(
        () => new Promise(() => {}) // never resolves
      )
      render(<LoginPage availableProviders={[]} onLoginSuccess={vi.fn()} />)
      fireEvent.change(screen.getByLabelText(/email/i), {
        target: { value: 'a@b.com' }
      })
      fireEvent.change(screen.getByLabelText(/password/i), {
        target: { value: 'p' }
      })
      fireEvent.click(screen.getByRole('button', { name: /sign in/i }))
      expect(screen.getByRole('button', { name: /signing in/i })).toBeDisabled()
    })
  })

  describe('SSO buttons', () => {
    it('renders SSO buttons for available providers', () => {
      render(
        <LoginPage availableProviders={['github', 'google']} onLoginSuccess={vi.fn()} />
      )
      expect(
        screen.getByRole('link', { name: /continue with github/i })
      ).toBeInTheDocument()
      expect(
        screen.getByRole('link', { name: /continue with google/i })
      ).toBeInTheDocument()
    })

    it('SSO button href points to /auth/sso/:provider', () => {
      render(<LoginPage availableProviders={['github']} onLoginSuccess={vi.fn()} />)
      expect(screen.getByRole('link', { name: /github/i })).toHaveAttribute(
        'href',
        '/auth/sso/github'
      )
    })

    it('does not render SSO section when no providers', () => {
      render(<LoginPage availableProviders={[]} onLoginSuccess={vi.fn()} />)
      expect(screen.queryByText(/continue with/i)).not.toBeInTheDocument()
    })
  })

  // FIX CR-FE2E-002: PairCodeFallback removed from LoginPage — this page only
  // renders while sessionUser === null (multi-user backend path), where local/
  // SSO login is always available. See specs/frontend/crs/frontend-e2ee/.
  it('does not render the pairing code fallback', () => {
    render(<LoginPage availableProviders={[]} onLoginSuccess={vi.fn()} />)
    expect(screen.queryByPlaceholderText(/pairing url or code/i)).not.toBeInTheDocument()
    expect(screen.queryByText(/pairing url or code/i)).not.toBeInTheDocument()
  })
})
