import React from 'react'
import { CircleCheck } from 'lucide-react'
import { translate } from '@/i18n/i18n'
import type { OrcaTask } from '../../../../shared/task-types'
import { summarizeTaskDependencies } from './task-dependency-progress'

const T = 'auto.components.task.TaskDetail.'

type Props = { dependencies: readonly OrcaTask[]; loadFailed: boolean }

// "Blocks" cannot be derived from one task's call (see TaskDetail), so only the "this task depends on" side shows.
export function TaskDependenciesSection({ dependencies, loadFailed }: Props): React.JSX.Element {
  const { total, done, pending } = summarizeTaskDependencies(dependencies)
  return (
    <div className="pt-2 text-xs">
      <p className="font-semibold mb-1">{translate(`${T}dependencies`, 'Dependencies')}</p>
      {pending.length > 0 ? (
        <div className="text-muted-foreground flex gap-1">
          <span>← {translate(`${T}blockedBy`, 'Blocked by:')}</span>
          <span>{pending.map((d) => d.title).join(', ')}</span>
        </div>
      ) : null}
      {total > 0 ? (
        <div className="text-muted-foreground flex flex-wrap items-center gap-1 mt-1">
          <span>{translate(`${T}dependsOn`, 'Depends on')}</span>
          <span data-testid="task-dependency-progress">
            {translate(`${T}dependsOnProgress`, '{{done}} of {{total}} done', {
              done: done.length,
              total
            })}
          </span>
          {done.map((d) => (
            <span key={d.id} className="inline-flex items-center gap-0.5">
              <CircleCheck className="size-3 text-status-success" aria-hidden />
              {d.title}
            </span>
          ))}
        </div>
      ) : null}
      <div className="text-muted-foreground flex gap-1 mt-1">
        <span>→ Blocks:</span>
        <span>Not supported on the Details tab — see the DAG tab</span>
      </div>
      {total === 0 && (
        <div className="text-muted-foreground">
          {translate(`${T}noDependencies`, 'No dependencies')}
        </div>
      )}
      {loadFailed && (
        <div className="text-destructive mt-1">
          {translate(`${T}dependenciesFailed`, 'Failed to load dependencies')}
        </div>
      )}
    </div>
  )
}
