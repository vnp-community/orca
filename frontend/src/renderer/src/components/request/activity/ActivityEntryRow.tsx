/**
 * ActivityEntryRow — FE-REQ-TASK-037-07
 *
 * One line of the History feed: icon, headline, who and when, attempt, and one line of context.
 * State is always carried by an icon and words, never by colour alone.
 *
 * @module components/request/activity/ActivityEntryRow
 */

import React from 'react'
import { Bot, Cog, Cpu, OctagonAlert, TriangleAlert, User, type LucideIcon } from 'lucide-react'
import { cn } from '@/lib/utils'
import { i18n } from '@/i18n/i18n'
import { formatAbsoluteTime, formatRequestRelativeTime } from '@/lib/request-relative-time'
import type {
  ActivityActorKind,
  RequestActivityEntry
} from '../../../../../shared/request-execution-types'
import { Button } from '@/components/ui/button'
import { translate } from '@/i18n/i18n'
import {
  activityIcon,
  activityTargetTab,
  activityTone,
  type ActivityTargetTab,
  type ActivityTone
} from './activity-entry-presentation'
import {
  activityActorLabel,
  activityAttemptLabel,
  activityDetailText,
  activitySeverityLabel,
  activityTitle
} from './activity-entry-text'

const ACTOR_ICON: Record<ActivityActorKind, LucideIcon> = {
  user: User,
  ai: Bot,
  agent: Cpu,
  system: Cog
}

const TONE_ICON_CLASS: Record<ActivityTone, string> = {
  neutral: 'text-muted-foreground',
  success: 'text-status-success',
  attention: 'text-primary',
  error: 'text-destructive'
}

const TAB_LABEL: Record<ActivityTargetTab, () => string> = {
  overview: () => translate('auto.components.request.RequestDetailPane.tab.overview', 'Overview'),
  analysis: () => translate('auto.components.request.RequestDetailPane.tab.analysis', 'Analysis'),
  plan: () => translate('auto.components.request.RequestDetailPane.tab.plan', 'Plan'),
  execution: () => translate('auto.components.request.RequestDetailPane.tab.execution', 'Execution')
}

type RowProps = {
  entry: RequestActivityEntry
  /** Goes to the tab that holds what the line is about; omitted hides the link. */
  onOpenTab?: (tab: ActivityTargetTab) => void
}

export function ActivityEntryRow({ entry, onOpenTab }: RowProps): React.JSX.Element {
  const target = onOpenTab ? activityTargetTab(entry) : null
  const tone = activityTone(entry)
  const Icon = activityIcon(entry)
  const ActorIcon = ACTOR_ICON[entry.actorKind] ?? Cog
  const severityLabel = activitySeverityLabel(tone)
  const attempt = activityAttemptLabel(entry)
  const detail = activityDetailText(entry)
  const SeverityIcon = tone === 'error' ? OctagonAlert : TriangleAlert

  return (
    <li
      className="flex items-start gap-3 px-4 py-2"
      data-testid="activity-entry"
      data-kind={entry.kind}
      data-severity={entry.severity}
    >
      <Icon className={cn('mt-0.5 size-4 shrink-0', TONE_ICON_CLASS[tone])} aria-hidden />
      <div className="flex min-w-0 flex-1 flex-col gap-0.5">
        <div className="flex flex-wrap items-center gap-x-2 gap-y-0.5">
          <span className="text-sm font-medium text-foreground">{activityTitle(entry)}</span>
          {severityLabel && (
            <span
              className={cn(
                'inline-flex items-center gap-1 rounded-full border px-1.5 py-0.5 text-[10px] font-medium',
                tone === 'error'
                  ? 'border-destructive text-destructive'
                  : 'border-primary text-primary'
              )}
              data-testid="activity-entry-severity"
            >
              <SeverityIcon className="size-3" aria-hidden />
              {severityLabel}
            </span>
          )}
          {attempt && <span className="text-xs text-muted-foreground">{attempt}</span>}
        </div>
        {detail && (
          <p className="line-clamp-2 whitespace-pre-wrap break-words text-xs text-muted-foreground">
            {detail}
          </p>
        )}
      </div>
      {target && onOpenTab && (
        <Button
          size="xs"
          variant="ghost"
          onClick={() => onOpenTab(target)}
          data-testid="activity-entry-open"
        >
          {translate('auto.components.request.activity.openTab', 'Open {{tab}}', {
            tab: TAB_LABEL[target]()
          })}
        </Button>
      )}
      <div className="flex shrink-0 items-center gap-1 text-xs text-muted-foreground">
        <ActorIcon className="size-3" aria-hidden />
        <span>{activityActorLabel(entry.actorKind)}</span>
        <time
          dateTime={entry.occurredAt}
          title={formatAbsoluteTime(entry.occurredAt, i18n.language || 'en')}
        >
          {formatRequestRelativeTime(entry.occurredAt)}
        </time>
      </div>
    </li>
  )
}
