/**
 * PlanAutoWriting — CR-REQ-044
 *
 * Shown while the server writes the Plan on its own. No button: a second Generate would only compete with the run in flight.
 *
 * @module components/request/plan/PlanAutoWriting
 */

import React from 'react'
import { translate } from '@/i18n/i18n'
import { Skeleton } from '@/components/ui/skeleton'

const P = 'auto.components.request.plan.PlanAutoWriting.'

type Props = {
  attempt: number
  maxAttempts: number
  /** A failed attempt is waiting for its turn to run again. */
  waiting: boolean
  /** Nothing has started yet; the server is expected to start in a moment. */
  starting: boolean
}

export function PlanAutoWriting({
  attempt,
  maxAttempts,
  waiting,
  starting
}: Props): React.JSX.Element {
  return (
    <div
      className="flex w-full max-w-md flex-col items-center gap-3"
      aria-busy="true"
      data-testid="plan-auto-writing"
    >
      <div className="flex w-full flex-col gap-2">
        <Skeleton className="h-5 w-1/2" />
        <Skeleton className="h-14 w-full" />
      </div>
      <div role="status" className="flex flex-col items-center gap-1">
        <p className="text-sm text-muted-foreground">
          {starting
            ? translate(`${P}starting`, 'Orca is starting the plan...')
            : translate(`${P}title`, 'Orca is writing the plan...')}
        </p>
        {attempt > 1 && (
          <p className="text-xs text-muted-foreground" data-testid="plan-auto-attempt">
            {translate(`${P}attempt`, 'Attempt {{attempt}} of {{max}}', {
              attempt,
              max: Math.max(maxAttempts, attempt)
            })}
          </p>
        )}
        {waiting && (
          <p className="text-xs text-muted-foreground" data-testid="plan-auto-waiting">
            {translate(`${P}retrying`, 'Trying again shortly')}
          </p>
        )}
      </div>
    </div>
  )
}
