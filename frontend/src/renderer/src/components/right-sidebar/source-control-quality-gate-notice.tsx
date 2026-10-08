/**
 * source-control-quality-gate-notice.tsx — FE-CV-TASK-085-04
 *
 * Quality gate notice component for the Source Control right-sidebar panel.
 *
 * Design rules:
 * - STYLEGUIDE tokens only (no hex)
 * - role="status" aria-live="polite" (not alert)
 * - Icon per severity state (not only color)
 * - Loader2 spinner after ~200ms
 * - Strings via translate()
 * - Reason display: plain text only (no HTML)
 * - pass → renders null
 * - No "safe" / "threshold met" language
 *
 * @module components/right-sidebar/source-control-quality-gate-notice
 */

import { useEffect, useState } from 'react'
import {
  ShieldAlert,
  ShieldX,
  HelpCircle,
  Loader2,
  ChevronRight,
  Play
} from 'lucide-react'
import { Button } from '@/components/ui/button'
import { translateCatalogKey } from '@/i18n/catalog-key-translate'
import type { QualityGateNoticeViewModel } from './source-control-quality-gate-view-model'

// ---------------------------------------------------------------------------
// Props
// ---------------------------------------------------------------------------

export type SourceControlQualityGateNoticeProps = {
  viewModel: QualityGateNoticeViewModel
  isLoading: boolean
  timedOut: boolean
  canRunChecks: boolean
  onRunChecks: () => void
  onOpenReason: (checkName: string) => void
  /** Override for tests; defaults to the app i18n catalog. */
  translate?: (key: string, params?: Record<string, unknown>) => string
  /** Git provider — affects copy ('pull request' vs 'merge request') */
  provider?: 'github' | 'gitlab' | 'other' | null
}

// ---------------------------------------------------------------------------
// Spinner with entrance delay
// ---------------------------------------------------------------------------

function DelayedSpinner({ delayMs = 200 }: { delayMs?: number }) {
  const [visible, setVisible] = useState(false)
  useEffect(() => {
    const t = setTimeout(() => setVisible(true), delayMs)
    return () => clearTimeout(t)
  }, [delayMs])
  if (!visible) {return null}
  return <Loader2 className="h-3.5 w-3.5 animate-spin text-muted-foreground" aria-hidden />
}

// ---------------------------------------------------------------------------
// Severity icon
// ---------------------------------------------------------------------------

function SeverityIcon({ severity }: { severity: 'warn' | 'fail' | 'unknown' }) {
  if (severity === 'fail') {
    return <ShieldX className="h-3.5 w-3.5 shrink-0 text-[var(--color-review-violation)]" aria-hidden />
  }
  if (severity === 'warn') {
    return <ShieldAlert className="h-3.5 w-3.5 shrink-0 text-[var(--color-review-untested)]" aria-hidden />
  }
  return <HelpCircle className="h-3.5 w-3.5 shrink-0 text-muted-foreground" aria-hidden />
}

// ---------------------------------------------------------------------------
// Component
// ---------------------------------------------------------------------------

export function SourceControlQualityGateNotice({
  viewModel,
  isLoading,
  timedOut,
  canRunChecks,
  onRunChecks,
  onOpenReason,
  translate = translateCatalogKey,
  provider
}: SourceControlQualityGateNoticeProps) {
  // pass → hidden
  if (!viewModel.visible) {return null}

  const { severity, stale, unavailable, reasons, reasonCount } = viewModel

  const labelKey = `auto.components.right.sidebar.qualityGateNotice.title.${severity}`
  const titleText = translate(labelKey, { provider: provider === 'gitlab' ? 'merge_request' : 'pull_request' })

  return (
    <div
      role="status"
      aria-live="polite"
      aria-label={titleText}
      className="rounded-md border border-border bg-card px-3 py-2 text-xs space-y-1.5 break-words"
    >
      {/* Header row */}
      <div className="flex items-center gap-1.5">
        {isLoading ? (
          <DelayedSpinner />
        ) : (
          <SeverityIcon severity={severity} />
        )}
        <span className="font-medium text-foreground">{titleText}</span>
        {stale && (
          <span className="ml-auto text-[10px] text-muted-foreground">
            {translate('auto.components.right.sidebar.qualityGateNotice.stale')}
          </span>
        )}
        {timedOut && !isLoading && (
          <span className="ml-auto text-[10px] text-muted-foreground">
            {translate('auto.components.right.sidebar.qualityGateNotice.timedOut')}
          </span>
        )}
      </div>

      {/* Unavailable notice */}
      {unavailable && !isLoading && (
        <p className="text-muted-foreground">
          {translate('auto.components.right.sidebar.qualityGateNotice.unavailable')}
        </p>
      )}

      {/* Reasons list */}
      {reasons.length > 0 && (
        <ul className="space-y-0.5" aria-label={translate('auto.components.right.sidebar.qualityGateNotice.reasons')}>
          {reasons.map((r, i) => (
            <li key={i} className="flex items-center gap-1">
              <span className="h-1 w-1 rounded-full shrink-0 bg-muted-foreground" aria-hidden />
              <span className="text-foreground/80">
                {/* Reason label — plain text only */}
                {translate(r.labelKey, { check: r.check, ...r.params })}
              </span>
              <button
                type="button"
                aria-label={translate('auto.components.right.sidebar.qualityGateNotice.openReason', { check: r.check })}
                className="ml-auto shrink-0 text-muted-foreground hover:text-foreground transition-colors"
                onClick={() => onOpenReason(r.check)}
              >
                <ChevronRight className="h-3 w-3" aria-hidden />
              </button>
            </li>
          ))}
        </ul>
      )}

      {/* Extra count */}
      {reasonCount > reasons.length && (
        <p className="text-[10px] text-muted-foreground">
          {translate('auto.components.right.sidebar.qualityGateNotice.moreReasons', {
            count: reasonCount - reasons.length
          })}
        </p>
      )}

      {/* Action buttons */}
      <div className="flex gap-1.5 pt-0.5">
        {canRunChecks && (
          <Button
            variant="outline"
            size="xs"
            onClick={onRunChecks}
            disabled={isLoading}
            className="h-5 gap-1 text-[10px]"
          >
            <Play className="h-2.5 w-2.5" aria-hidden />
            {translate('auto.components.right.sidebar.qualityGateNotice.runChecks')}
          </Button>
        )}
      </div>
    </div>
  )
}
