/**
 * ReadinessReportSheet — FE-REQ-TASK-036-07
 *
 * Findings grouped by tier plus one primary action chosen by the outcome.
 * Run itself stays the existing TaskDetail button; no new run path here.
 *
 * @module components/request/readiness/ReadinessReportSheet
 */

import React, { useState } from 'react'
import { translate } from '@/i18n/i18n'
import { Button } from '@/components/ui/button'
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from '@/components/ui/sheet'
import { ReadinessBadge } from './ReadinessBadge'
import { getReadinessAction, groupFindingsByTier } from './readiness-action-rules'
import { openRequestPage } from '../request-page-navigation'
import type { Result } from '../../../runtime/request-rpc-client'
import type { TaskReadinessReport } from '../../../../../shared/request-artifact-types'

const T = 'auto.components.request.readiness.'
const TIER_FALLBACK: Record<string, string> = { structure: 'Structure', semantic: 'Semantics', environment: 'Environment' }

type Props = {
  open: boolean
  onOpenChange: (open: boolean) => void
  report: TaskReadinessReport | null
  requestId?: string
  canWrite: boolean
  hasDevServer: boolean
  onCheck: () => Promise<Result<unknown>>
  /** Navigate to the dev server connection screen (host app decides where). */
  onConnectDevServer?: () => void
  onNotifyOperator?: () => void
}

export function ReadinessReportSheet({ open, onOpenChange, report, requestId, canWrite, hasDevServer, onCheck, onConnectDevServer, onNotifyOperator }: Props): React.JSX.Element {
  const [checking, setChecking] = useState(false)
  const action = getReadinessAction(report, { canWrite, hasDevServer })

  const runAction = (): void => {
    if (action.kind === 'answer' && requestId) {
      openRequestPage({ section: 'requests', requestId })
      onOpenChange(false)
    } else if (action.kind === 'regenerate' && requestId) {
      // Why: backend returns the request to the Plan step; the UI only navigates there.
      openRequestPage({ section: 'requests', requestId, focus: 'plan' })
      onOpenChange(false)
    } else if (action.kind === 'connect') {
      onConnectDevServer?.()
    } else if (action.kind === 'notify') {
      onNotifyOperator?.()
    }
  }
  const actionable = action.kind === 'answer' || action.kind === 'regenerate' || (action.kind === 'connect' && onConnectDevServer) || (action.kind === 'notify' && onNotifyOperator)

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent side="right" className="gap-3 p-4 sm:max-w-lg" data-testid="readiness-report-sheet">
        <SheetHeader className="p-0">
          <SheetTitle>{translate(`${T}title`, 'Task readiness')}</SheetTitle>
          <SheetDescription>{report ? [report.checkedAt, report.specDigest?.slice(0, 8)].filter(Boolean).join(' · ') : translate(`${T}notChecked`, 'Not checked yet')}</SheetDescription>
        </SheetHeader>
        <ReadinessBadge report={report} />
        {report ? groupFindingsByTier(report.findings).map((g) => (
          <section key={g.tier} aria-label={translate(`${T}tier.${g.tier}`, TIER_FALLBACK[g.tier] ?? g.tier)}>
            <h4 className="text-xs font-medium">{translate(`${T}tier.${g.tier}`, TIER_FALLBACK[g.tier] ?? g.tier)}</h4>
            <ul className="mt-1 flex flex-col gap-1.5">
              {g.findings.map((f, i) => (
                <li key={`${f.code}-${i}`} className="text-xs">
                  <code className="font-mono">{f.code}</code> <span>{f.message}</span>
                  {f.path ? <span className="block break-all text-muted-foreground">{f.path}</span> : null}
                </li>
              ))}
            </ul>
            {g.tier === 'environment' ? <p className="mt-1 text-[11px] text-muted-foreground">{translate(`${T}envVarNames`, 'Only variable names are shown, never values.')}</p> : null}
          </section>
        )) : null}
        {action.noteKey ? <p className="text-xs text-muted-foreground" data-testid="readiness-note">{translate(action.noteKey, action.noteFallback ?? '')}</p> : null}
        <div className="mt-auto flex flex-wrap items-center gap-2">
          {actionable ? <Button size="sm" onClick={runAction}>{translate(action.labelKey, action.fallback)}</Button> : null}
          {action.kind === 'run' ? <span className="text-xs text-muted-foreground">{translate(`${T}runHint`, 'Use Run on the task when you are ready.')}</span> : null}
          <Button
            size="sm"
            variant="outline"
            disabled={checking}
            onClick={async () => {
              setChecking(true)
              try { await onCheck() } finally { setChecking(false) }
            }}
          >
            {checking ? translate(`${T}checking`, 'Checking...') : translate(`${T}check`, 'Check readiness')}
          </Button>
        </div>
      </SheetContent>
    </Sheet>
  )
}
