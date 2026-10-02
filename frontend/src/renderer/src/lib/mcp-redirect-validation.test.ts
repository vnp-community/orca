import { describe, expect, it } from 'vitest'
import { validateConsentRedirect } from './mcp-redirect-validation'

describe('validateConsentRedirect', () => {
  it.each([
    'javascript:alert(1)',
    'data:text/html,<script>1</script>',
    'vbscript:x',
    'file:///etc/passwd',
    'http://evil.example.com/cb',
    'https://user:pw@example.com/cb',
    '//example.com/cb',
    '/relative',
    'not a url',
    '',
    'ftp://example.com'
  ])('rejects %s', (u) => {
    expect(validateConsentRedirect(u)).toBeNull()
  })
  it('rejects non-strings', () => {
    expect(validateConsentRedirect(undefined)).toBeNull()
    expect(validateConsentRedirect(42)).toBeNull()
  })
  it('accepts https and loopback http', () => {
    expect(validateConsentRedirect('https://claude.ai/api/mcp/auth_callback?code=1')).toBe(
      'https://claude.ai/api/mcp/auth_callback?code=1'
    )
    expect(validateConsentRedirect('http://localhost:33418/callback?code=a')).toBeTruthy()
    expect(validateConsentRedirect('http://127.0.0.1:1/cb')).toBeTruthy()
    expect(validateConsentRedirect('http://[::1]:1/cb')).toBeTruthy()
  })
})
