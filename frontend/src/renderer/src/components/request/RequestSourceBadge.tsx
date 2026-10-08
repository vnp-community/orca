/**
 * RequestSourceBadge — CR-REQ-018-05
 *
 * Renders a source provider badge with optional link.
 * URL sanitization: only http:/https: links pass through — no javascript: schemes.
 *
 * @module components/request/RequestSourceBadge
 */

import React from 'react'
import {
  ExternalLink, Github, GitBranch, Layout, Cpu, Pencil, Webhook, Circle
} from 'lucide-react'
import { cn } from '@/lib/utils'
import { translate } from '@/i18n/i18n'
import { getSourcePresentation } from './request-status-presentation'
import type { RequestSourceProvider } from '../../../../shared/request-types'

const ICONS: Record<string, React.ComponentType<{ className?: string }>> = {
  ExternalLink, Github, GitBranch, Layout, Cpu, Pencil, Webhook, Circle
}

/** Sanitise URL: only http/https links are allowed — blocks javascript: and other schemes */
function sanitizeUrl(url: string | undefined): string | undefined {
  if (!url) {return undefined}
  try {
    const parsed = new URL(url)
    if (parsed.protocol === 'http:' || parsed.protocol === 'https:') {
      return url
    }
    return undefined
  } catch {
    return undefined
  }
}

type Props = {
  provider: RequestSourceProvider
  ref?: string
  url?: string
  size?: 'sm' | 'xs'
  className?: string
}

export function RequestSourceBadge({ provider, ref: refProp, url, size = 'sm', className }: Props): React.JSX.Element {
  const pres = getSourcePresentation(provider)
  const IconComponent = ICONS[pres.iconName] ?? Circle
  const label = translate(pres.labelKey, provider)
  const safeUrl = sanitizeUrl(url)
  const displayText = refProp ?? label

  const inner = (
    <span
      className={cn(
        'inline-flex items-center gap-1 rounded border border-border/60 bg-muted/40 text-muted-foreground px-1.5 py-0.5',
        size === 'xs' ? 'text-[10px]' : 'text-xs',
        className
      )}
      aria-label={`${label}${refProp ? `: ${refProp}` : ''}`}
    >
      <IconComponent className={size === 'xs' ? 'size-2.5' : 'size-3'} aria-hidden />
      <span>{displayText}</span>
      {safeUrl && <ExternalLink className="size-2.5 opacity-60" aria-hidden />}
    </span>
  )

  if (safeUrl) {
    return (
      <a
        href={safeUrl}
        target="_blank"
        rel="noopener noreferrer"
        className="inline-flex hover:opacity-80 transition-opacity"
        aria-label={`${label}: ${refProp ?? ''} (opens in new tab)`}
      >
        {inner}
      </a>
    )
  }

  return inner
}
