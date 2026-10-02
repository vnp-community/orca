// @vitest-environment happy-dom
import { afterEach, describe, it, vi } from 'vitest'
import { cleanup, render, screen } from '@testing-library/react'
import { OAuthConsentRoute } from './OAuthConsentRoute'

vi.mock('../components/mcp/consent/McpConsentPage', () => ({
  McpConsentPage: () => <div>consent page</div>
}))

afterEach(cleanup)

describe('OAuthConsentRoute', () => {
  it('renders only the consent page (no App shell)', async () => {
    render(<OAuthConsentRoute />)
    await screen.findByText('consent page')
  })
})
