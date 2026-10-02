import { describe, expect, it } from 'vitest'
import { parseMcpDeepLink } from './mcp-deep-link'

describe('parseMcpDeepLink', () => {
  it('parses approvals deep link', () => {
    expect(
      parseMcpDeepLink({
        pathname: '/',
        search: '?section=mcp&tab=approvals&approval=abc'
      })
    ).toEqual({ tab: 'approvals', focusId: 'abc' })
  })
  it('accepts legacy /settings and falls back to connect for unknown tabs', () => {
    expect(
      parseMcpDeepLink({
        pathname: '/settings',
        search: '?section=mcp&tab=zzz'
      })
    ).toEqual({
      tab: 'connect'
    })
  })
  it('ignores other sections', () => {
    expect(parseMcpDeepLink({ pathname: '/', search: '?section=git' })).toBeNull()
    expect(parseMcpDeepLink({ pathname: '/', search: '' })).toBeNull()
  })
})
