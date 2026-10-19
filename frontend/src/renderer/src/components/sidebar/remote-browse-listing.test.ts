import { describe, expect, it } from 'vitest'
import {
  directListingTarget,
  isHostReconnectingBrowseError,
  normalizeBrowseListing,
  REMOTE_BROWSE_ENTRY_LIMIT
} from './remote-browse-listing'

const entries = (count: number): { name: string; isDirectory: boolean }[] =>
  Array.from({ length: count }, (_, index) => ({ name: `d${index}`, isDirectory: true }))

describe('normalizeBrowseListing', () => {
  it('passes a backend-capped listing through untouched', () => {
    const listing = normalizeBrowseListing({
      resolvedPath: '/tmp',
      entries: entries(500),
      truncated: true,
      totalEntries: 1000
    })
    expect(listing).toMatchObject({ truncated: true, totalEntries: 1000 })
    expect(listing.entries).toHaveLength(500)
  })

  it('treats a missing `truncated` as false for a small listing (older backend)', () => {
    expect(normalizeBrowseListing({ resolvedPath: '/', entries: entries(3) })).toMatchObject({
      truncated: false,
      totalEntries: 3
    })
  })

  it('trims an older backend listing past the limit so the UI cannot stall', () => {
    const listing = normalizeBrowseListing({
      resolvedPath: '/private/var/folders',
      entries: entries(REMOTE_BROWSE_ENTRY_LIMIT + 20)
    })
    expect(listing.entries).toHaveLength(REMOTE_BROWSE_ENTRY_LIMIT)
    expect(listing).toMatchObject({ truncated: true, totalEntries: REMOTE_BROWSE_ENTRY_LIMIT + 20 })
  })
})

describe('isHostReconnectingBrowseError', () => {
  it.each([
    'INFRA_AGENT_EXEC_FAILED: failed to relay to dev server agent',
    'rpc error: INFRA_DEV_SERVER_NOT_CONNECTED',
    'dev server disconnected'
  ])('recognises %s', (message) => {
    expect(isHostReconnectingBrowseError(message)).toBe(true)
  })

  it('does not mistake a bad path for a reconnect', () => {
    expect(isHostReconnectingBrowseError('ENOENT: no such file or directory')).toBe(false)
  })
})

describe('directListingTarget', () => {
  it('joins an absolute path so it can be listed in one call', () => {
    expect(directListingTarget('/', ['private', 'var', 'folders'])).toBe('/private/var/folders')
    expect(directListingTarget('/home/alice', ['src', 'app'])).toBe('/home/alice/src/app')
  })

  it('falls back to step-by-step resolution for . and .. and for no segments', () => {
    expect(directListingTarget('/a', ['b', '..', 'c'])).toBeNull()
    expect(directListingTarget('/a', ['.'])).toBeNull()
    expect(directListingTarget('/a', [])).toBeNull()
  })
})
