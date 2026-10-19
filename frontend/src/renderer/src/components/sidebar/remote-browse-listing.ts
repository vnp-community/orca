import type { DirEntry } from './remote-file-browser-helpers'

// Why a cap: the dev-server agent relay drops frames above ~32 KiB, and a directory like macOS
// $TMPDIR (~1000 entries) alone exceeds that. The gateway caps at the same number (limit default 500).
export const REMOTE_BROWSE_ENTRY_LIMIT = 500

export type BrowseListing = {
  resolvedPath: string
  entries: DirEntry[]
  // Backends that predate the cap omit both; normalizeBrowseListing fills them in.
  truncated: boolean
  totalEntries: number
}

type RawBrowseListing = {
  resolvedPath: string
  entries: DirEntry[]
  truncated?: boolean
  totalEntries?: number
}

/** Backward compatible: a backend without `truncated` is trimmed client-side so the UI never stalls. */
export function normalizeBrowseListing(
  raw: RawBrowseListing,
  limit = REMOTE_BROWSE_ENTRY_LIMIT
): BrowseListing {
  const entries = raw.entries ?? []
  if (typeof raw.truncated === 'boolean') {
    return {
      resolvedPath: raw.resolvedPath,
      entries,
      truncated: raw.truncated,
      totalEntries: raw.totalEntries ?? entries.length
    }
  }
  const overLimit = entries.length > limit
  return {
    resolvedPath: raw.resolvedPath,
    entries: overLimit ? entries.slice(0, limit) : entries,
    truncated: overLimit,
    totalEntries: raw.totalEntries ?? entries.length
  }
}

const HOST_RECONNECTING_ERROR_ANCHORS = [
  'infra_dev_server_not_connected',
  'infra_agent_exec_failed',
  'dev server disconnected',
  'failed to relay to dev server agent'
]

/** The agent dropped or is not connected: worth waiting for, not a path problem. */
export function isHostReconnectingBrowseError(message: string): boolean {
  const lower = message.toLowerCase()
  return HOST_RECONNECTING_ERROR_ANCHORS.some((anchor) => lower.includes(anchor))
}

/**
 * Segments of an absolute path that can be listed in one call. Anything with `.`/`..`/empty parts
 * keeps the step-by-step resolution, which handles them (and gives per-segment errors).
 */
export function directListingTarget(
  basePath: string,
  committedSegments: readonly string[]
): string | null {
  if (committedSegments.length === 0) {
    return null
  }
  if (committedSegments.some((segment) => segment === '' || segment === '.' || segment === '..')) {
    return null
  }
  return committedSegments.reduce(
    (current, segment) => (current === '/' ? `/${segment}` : `${current}/${segment}`),
    basePath
  )
}
