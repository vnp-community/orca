/**
 * ReviewArtifactView — FE-OR-T211, T214
 *
 * The drawing itself, or the reason there is none. The drawing is HTML or SVG written by an agent and is not trusted: it goes in
 * an iframe whose `sandbox` is empty (no scripts, no same origin, no forms, no popups, no navigation) with a srcdoc that has been
 * cleaned and carries its own content policy. Nothing of it is ever put into the page. A drawing that is missing or failed is not
 * a blank space: the view says so, and the rows of the section are still on the screen.
 *
 * @module components/request/outcome-review/artifact/ReviewArtifactView
 */

import React, { useMemo } from 'react'
import { Loader2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { useDocumentThemeVersion } from '../../../../hooks/useDocumentThemeVersion'
import { actionText, notDrawnText } from './review-artifact-copy'
import { aspectRatioOf, buildArtifactSrcdoc } from './review-artifact-srcdoc'
import { currentTheme, readThemeTokens } from './review-artifact-theme-tokens'
import type { RegenerationReason } from '../../../../hooks/reviewArtifactCalls'
import type { UseReviewArtifactResult } from '../../../../hooks/useReviewArtifact'
import type { DiagramRef } from '../../../../../../shared/outcome-review-types'

export type RegeneratePreset = { sections: string[]; reason?: RegenerationReason }

type Props = {
  artifact: UseReviewArtifactResult
  diagram: DiagramRef
  /** The accessible name when the drawing has no title of its own. */
  label: string
  /** The viewer may start a regeneration (the server decides again). */
  canRegenerate: boolean
  onRegenerate?: (preset: RegeneratePreset) => void
  /** A drawing the server no longer has: the review should be read again. */
  onChanged?: () => void
  /** Leave the height to the content (the enlarged view) instead of capping it at 70% of the window. */
  uncapped?: boolean
}

function Notice({ text, action }: { text: string; action?: React.ReactNode }): React.JSX.Element {
  return (
    <div
      className="flex items-center gap-3 rounded-md border border-dashed border-border p-3 text-sm text-muted-foreground"
      data-testid="review-artifact-notice"
    >
      <span className="min-w-0 flex-1">{text}</span>
      {action}
    </div>
  )
}

export function ReviewArtifactView({
  artifact,
  diagram,
  label,
  canRegenerate,
  onRegenerate,
  onChanged,
  uncapped = false
}: Props): React.JSX.Element {
  const drawn = diagram.status === 'cached' && !!diagram.artifactId
  const themeVersion = useDocumentThemeVersion()
  // The theme version is part of the value only to say that these tokens were read after the theme changed.
  const theme = useMemo(
    () => ({ tokens: readThemeTokens(), mode: currentTheme(), version: themeVersion }),
    [themeVersion]
  )
  const srcdoc = useMemo(
    () =>
      artifact.html === null
        ? null
        : buildArtifactSrcdoc({ html: artifact.html, tokens: theme.tokens, theme: theme.mode }),
    [artifact.html, theme]
  )

  const retry =
    canRegenerate && onRegenerate ? (
      <Button
        size="xs"
        variant="outline"
        onClick={() => onRegenerate({ sections: [diagram.section], reason: 'retry_failed' })}
      >
        {actionText('retry')}
      </Button>
    ) : undefined

  if (diagram.status === 'drawing') {
    return (
      <div
        className="flex items-center gap-2 rounded-md border border-border p-3 text-sm text-muted-foreground"
        aria-busy="true"
        data-testid="review-artifact-drawing"
      >
        <Loader2 className="size-4 animate-spin" aria-hidden />
        {notDrawnText('drawing')}
      </div>
    )
  }
  if (diagram.status === 'disabled') {
    return <Notice text={notDrawnText('disabled')} />
  }
  if (!drawn) {
    return (
      <Notice
        text={notDrawnText(diagram.status === 'failed' ? 'failed' : 'missing')}
        action={retry}
      />
    )
  }
  if (artifact.status === 'loading' || artifact.status === 'idle') {
    return <Skeleton className="h-48 w-full" data-testid="review-artifact-loading" />
  }
  if (artifact.status === 'not_found') {
    return (
      <Notice
        text={notDrawnText('gone')}
        action={
          onChanged && (
            <Button size="xs" variant="outline" onClick={onChanged}>
              {actionText('retry')}
            </Button>
          )
        }
      />
    )
  }
  if (artifact.status === 'error' || srcdoc === null) {
    return (
      <Notice
        text={notDrawnText('unreadable')}
        action={
          <Button size="xs" variant="outline" onClick={artifact.reload}>
            {actionText('retry')}
          </Button>
        }
      />
    )
  }
  const ratio = aspectRatioOf(artifact.meta?.viewBox)
  return (
    // sandbox="" is the whole point: no allow-scripts, no allow-same-origin, and nothing else is granted.
    <iframe
      title={artifact.meta?.title || label}
      sandbox=""
      referrerPolicy="no-referrer"
      loading="lazy"
      srcDoc={srcdoc}
      className="w-full rounded-md border border-border bg-background"
      style={{
        aspectRatio: ratio ? String(ratio) : '16 / 9',
        ...(uncapped ? {} : { maxHeight: '70vh' })
      }}
      data-testid="review-artifact-iframe"
    />
  )
}
