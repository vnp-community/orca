// TASK-FE-004: LoginForm — controlled email/password form for the Login page.
// Parent supplies onSubmit, isLoading, error to keep this component pure.
import type { FormEvent } from 'react';
import { useState } from 'react'

type Props = {
  onSubmit: (email: string, password: string) => void
  isLoading: boolean
  error: string | null
}

const EMAIL_RE = /^[^\s@]+@[^\s@]+\.[^\s@]+$/

export function LoginForm({ onSubmit, isLoading, error }: Props) {
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [localError, setLocalError] = useState<string | null>(null)

  function handleSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    setLocalError(null)
    if (!EMAIL_RE.test(email)) {
      setLocalError('Please enter a valid email address')
      return
    }
    onSubmit(email, password)
  }

  const displayError = error ?? localError

  return (
    <form onSubmit={handleSubmit} aria-label="Login form" role="form" noValidate>
      {displayError && (
        <div role="alert" className="login-form__error">
          {displayError}
        </div>
      )}

      <div className="login-form__field">
        <label htmlFor="login-email">Email</label>
        <input
          id="login-email"
          type="email"
          value={email}
          onChange={(e) => setEmail(e.target.value)}
          disabled={isLoading}
          autoComplete="email"
          autoFocus
        />
      </div>

      <div className="login-form__field">
        <label htmlFor="login-password">Password</label>
        <input
          id="login-password"
          type="password"
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          disabled={isLoading}
          autoComplete="current-password"
        />
      </div>

      <button
        type="submit"
        disabled={isLoading}
        className="login-form__submit"
      >
        {isLoading ? 'Signing in…' : 'Sign In'}
      </button>
    </form>
  )
}
