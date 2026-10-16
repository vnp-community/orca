/**
 * HandoverRow — FE-REQ-TASK-037-05
 *
 * The end of an OpenSpec run: the change is archived and handed over after the last task. A native run has no such stage and
 * says it is done when its last task is.
 *
 * @module components/request/execution/HandoverRow
 */

import React from 'react'
import { Archive, Check, Loader2, OctagonX } from 'lucide-react'
import { translate } from '@/i18n/i18n'
import type { ExecutionHandover } from '../../../../../shared/request-execution-types'

const T = 'auto.components.request.execution.HandoverRow.'

type Props = { engine: 'native' | 'openspec'; handover: ExecutionHandover }

export function HandoverRow({ engine, handover }: Props): React.JSX.Element {
  if (engine !== 'openspec') {
    return (
      <p className="px-1 text-xs text-muted-foreground" data-testid="execution-footer-native">
        {translate(`${T}nativeDone`, 'Done when the last task is done.')}
      </p>
    )
  }
  const view = (() => {
    switch (handover.state) {
      case 'archiving':
        return {
          Icon: Loader2,
          spin: true,
          tone: 'text-primary',
          text: translate(`${T}archiving`, 'Archiving the change and preparing the handover')
        }
      case 'ready':
        return {
          Icon: Check,
          spin: false,
          tone: 'text-[color:var(--status-success)]',
          text: translate(`${T}ready`, 'Change archived and ready for handover')
        }
      case 'failed':
        return {
          Icon: OctagonX,
          spin: false,
          tone: 'text-destructive',
          text: translate(`${T}failed`, 'Archiving failed after {{count}} attempts', {
            count: handover.archiveAttempts
          })
        }
      default:
        return {
          Icon: Archive,
          spin: false,
          tone: 'text-muted-foreground',
          text: translate(
            `${T}pending`,
            'After the last task the change is archived and handed over'
          )
        }
    }
  })()
  return (
    <div
      className="flex items-center gap-2 rounded-md border border-border px-3 py-2 text-sm"
      data-testid="execution-handover"
      data-handover-state={handover.state}
    >
      <view.Icon className={`size-4 ${view.tone} ${view.spin ? 'animate-spin' : ''}`} aria-hidden />
      <span className="min-w-0 flex-1">{view.text}</span>
      {handover.summaryRef && (
        <span className="truncate font-mono text-xs text-muted-foreground">
          {handover.summaryRef}
        </span>
      )}
    </div>
  )
}
