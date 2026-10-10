/**
 * PlanUnconfirmedAssumptionsNotice — CR-REQ-028 §2.5
 *
 * Shown when approving a Plan was refused for `needs_confirmation` assumptions without a
 * recorded answer. The ids come from the error text only; the UI has no assumption texts.
 *
 * @module components/request/plan/PlanUnconfirmedAssumptionsNotice
 */

import React from 'react'
import { TriangleAlert } from 'lucide-react'
import { translate } from '@/i18n/i18n'
import { parseUnconfirmedAssumptionIds } from '../../../../../shared/request-errors'
import { requestErrorMessage } from '../request-error-message'

const P = 'auto.components.request.plan.PlanUnconfirmedAssumptionsNotice.'

export function PlanUnconfirmedAssumptionsNotice({
  message
}: {
  message: string
}): React.JSX.Element {
  const ids = parseUnconfirmedAssumptionIds(message)
  return (
    <div
      role="alert"
      data-testid="plan-unconfirmed-assumptions-notice"
      className="flex items-start gap-2 rounded-md border border-destructive/30 bg-destructive/5 px-3 py-2 text-sm text-destructive"
    >
      <TriangleAlert className="mt-0.5 size-4 shrink-0" aria-hidden />
      <div className="space-y-0.5">
        <p>{requestErrorMessage('unconfirmed_assumptions')}</p>
        {ids.length > 0 && (
          <p className="text-xs" data-testid="plan-unconfirmed-assumption-ids">
            {translate(`${P}ids`, 'Still unconfirmed: {{ids}}', { ids: ids.join(', ') })}
          </p>
        )}
      </div>
    </div>
  )
}
