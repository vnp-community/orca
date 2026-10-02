import { describe, expect, it } from 'vitest'
import { formatMcpRelativeTime } from './mcp-relative-time'

const now = Date.parse('2026-10-02T12:00:00Z')
describe('formatMcpRelativeTime', () => {
  it('formats past times', () => {
    expect(formatMcpRelativeTime('2026-10-02T11:58:00Z', now)).toBe('2 minutes ago')
    expect(formatMcpRelativeTime('2026-10-01T12:00:00Z', now)).toBe('yesterday')
    expect(formatMcpRelativeTime('2026-10-02T09:00:00Z', now)).toBe('3 hours ago')
  })
  it('formats now and invalid input', () => {
    expect(formatMcpRelativeTime('2026-10-02T12:00:00Z', now)).toBe('now')
    expect(formatMcpRelativeTime('garbage', now)).toBe('—')
    expect(formatMcpRelativeTime(undefined, now)).toBe('—')
  })
})
