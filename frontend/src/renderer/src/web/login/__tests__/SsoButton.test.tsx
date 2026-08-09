// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { render, screen, cleanup } from '@testing-library/react'
import { afterEach, describe, expect, it } from 'vitest'
import { SsoButton } from '../SsoButton'

describe('SsoButton', () => {
  afterEach(cleanup)

  it('renders a link with the correct label for github', () => {
    render(<SsoButton provider="github" />)
    expect(screen.getByRole('link', { name: /continue with github/i })).toBeInTheDocument()
  })

  it('href points to /auth/sso/:provider', () => {
    render(<SsoButton provider="github" />)
    expect(screen.getByRole('link', { name: /github/i })).toHaveAttribute(
      'href',
      '/auth/sso/github'
    )
  })

  it('renders google SSO button with correct href', () => {
    render(<SsoButton provider="google" />)
    expect(screen.getByRole('link', { name: /continue with google/i })).toHaveAttribute(
      'href',
      '/auth/sso/google'
    )
  })

  it('applies provider-specific CSS class', () => {
    render(<SsoButton provider="keycloak" />)
    expect(screen.getByRole('link', { name: /keycloak/i })).toHaveClass('sso-button--keycloak')
  })
})
