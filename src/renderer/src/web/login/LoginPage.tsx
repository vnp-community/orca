// TASK-FE-006: LoginPage — root component shown when no session is detected.
// Composes LoginForm + SsoButton list + PairCodeFallback.
import { useState } from 'react'
import { LoginForm } from './LoginForm'
import { SsoButton } from './SsoButton'
import { PairCodeFallback } from './PairCodeFallback'
import { loginLocal } from '../../auth/auth-api-client'
import type { AuthUser, AuthError, SsoProvider } from '../../auth/auth-types'
import '../../assets/login.css'

type Props = {
  availableProviders: SsoProvider[]
  onLoginSuccess: (user: AuthUser) => void
}

export function LoginPage({ availableProviders, onLoginSuccess }: Props) {
  const [isLoading, setIsLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  async function handleLocalLogin(email: string, password: string) {
    setIsLoading(true)
    setError(null)
    try {
      const user = await loginLocal(email, password)
      onLoginSuccess(user)
    } catch (err) {
      // AuthError carries a user-visible message already
      setError((err as AuthError).message ?? 'An unexpected error occurred')
    } finally {
      setIsLoading(false)
    }
  }

  return (
    <div className="login-page" data-testid="login-page">
      <header className="login-header">
        <h1 className="login-header__logo">Orca</h1>
        <p className="login-header__tagline">Collaborative Dev Environment</p>
      </header>

      <main className="login-content">
        {/* Local email/password form */}
        <LoginForm onSubmit={handleLocalLogin} isLoading={isLoading} error={error} />

        {/* SSO buttons — only rendered when providers are configured */}
        {availableProviders.length > 0 && (
          <>
            <div className="login-divider" aria-hidden="true">
              or
            </div>
            <div className="login-sso-buttons">
              {availableProviders.map((provider) => (
                <SsoButton key={provider} provider={provider} />
              ))}
            </div>
          </>
        )}

        {/* PairCode backward-compat section */}
        <div className="login-divider" aria-hidden="true">
          or
        </div>
        <PairCodeFallback />
      </main>
    </div>
  )
}
