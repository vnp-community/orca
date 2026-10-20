/**
 * PlanRewriteStrip — CR-REQ-044
 *
 * A thin strip above the Plan tree while the server rewrites a Plan that already exists, so a reloaded page still tells the user
 * a new version is on its way.
 *
 * @module components/request/plan/PlanRewriteStrip
 */

import React from 'react'
import { Loader2 } from 'lucide-react'
import { translate } from '@/i18n/i18n'

export function PlanRewriteStrip(): React.JSX.Element {
  return (
    <div
      role="status"
      className="flex items-center gap-2 border-b border-border px-4 py-2 text-xs text-muted-foreground"
      data-testid="plan-rewrite-strip"
    >
      <Loader2 className="size-3.5 animate-spin" aria-hidden />
      {translate(
        'auto.components.request.plan.PlanRewriteStrip.text',
        'Orca is rewriting the plan...'
      )}
    </div>
  )
}
