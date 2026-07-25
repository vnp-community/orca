import type { DevServer } from '../../../../shared/dev-server-types'

// ─── Platform Label Map ───────────────────────────────────────────────────────

const PLATFORM_LABEL: Record<string, string> = {
  darwin: 'macOS',
  win32: 'Windows',
  linux: 'Linux',
  freebsd: 'FreeBSD',
  openbsd: 'OpenBSD',
}

// ─── Types ────────────────────────────────────────────────────────────────────

type Props = {
  status: DevServer['status']
  platform?: NodeJS.Platform | null
  /** Set to false to render just the dot without a text label. Default: true */
  showLabel?: boolean
  className?: string
}

// ─── Component ────────────────────────────────────────────────────────────────

/**
 * Compact badge that shows dev server connection status and (when connected)
 * the human-readable OS name derived from the server's reported platform.
 *
 * Uses BEM-style CSS classes (`dev-server-badge--<status>`) so styling can be
 * applied independently of component internals.
 */
export function DevServerStatusBadge({ status, platform, showLabel = true, className }: Props) {
  const label =
    status === 'connected' && platform
      ? (PLATFORM_LABEL[platform] ?? String(platform))
      : status

  return (
    <span
      className={[
        'dev-server-badge',
        `dev-server-badge--${status}`,
        className,
      ]
        .filter(Boolean)
        .join(' ')}
      aria-label={`Dev server status: ${label}`}
    >
      <span className="dev-server-badge__dot" aria-hidden="true" />
      {showLabel && <span className="dev-server-badge__label">{label}</span>}
    </span>
  )
}
