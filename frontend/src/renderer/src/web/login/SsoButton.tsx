// TASK-FE-005: SsoButton — link that redirects to backend SSO OAuth flow.
// Renders as an <a> so the browser follows the redirect natively (no JS fetch).
import type { SsoProvider } from '../../../auth/auth-types'

const PROVIDER_CONFIG: Record<SsoProvider, { label: string; icon: string }> = {
  github:   { label: 'Continue with GitHub',   icon: '🐙' },
  google:   { label: 'Continue with Google',   icon: '🔵' },
  keycloak: { label: 'Continue with Keycloak', icon: '🔑' }
}

type Props = { provider: SsoProvider }

export function SsoButton({ provider }: Props) {
  const { label, icon } = PROVIDER_CONFIG[provider]
  return (
    <a
      href={`/auth/sso/${provider}`}
      className={`sso-button sso-button--${provider}`}
      aria-label={label}
    >
      <span className="sso-button__icon" aria-hidden="true">
        {icon}
      </span>
      <span className="sso-button__label">{label}</span>
    </a>
  )
}
