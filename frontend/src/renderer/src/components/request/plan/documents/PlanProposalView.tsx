/**
 * PlanProposalView — CR-REQ-045
 *
 * The "Proposal" view of a native Plan: everything the AI proposed (summary, notes, every phase and task with its description,
 * type, estimate, labels, dependencies and the prompt it will run with), which the tree only shows in part. Read only.
 *
 * @module components/request/plan/documents/PlanProposalView
 */

import React from 'react'
import { AlertTriangle, ChevronDown, TriangleAlert } from 'lucide-react'
import { translate } from '@/i18n/i18n'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from '@/components/ui/collapsible'
import { Skeleton } from '@/components/ui/skeleton'
import { usePlanProposalOfRun } from '../../../../hooks/usePlanProposalOfRun'
import { requestErrorMessage } from '../../request-error-message'
import { RequestMarkdownContent } from '../../solution/RequestMarkdownContent'
import {
  formatDependencies,
  formatEstimatedHours,
  toProposalOutline,
  type OutlineTask
} from './plan-proposal-model'
import type { PlanRevision } from '../../../../../../shared/request-plan-revision'
import type { OrcaRequest } from '../../../../../../shared/request-types'

type Props = {
  request: Pick<OrcaRequest, 'id' | 'status' | 'version' | 'planTaskId'>
  /** The version the reader chose; null shows the current one. */
  shown: PlanRevision | null
  current: PlanRevision | null
}

function TaskItem({ task }: { task: OutlineTask }): React.JSX.Element {
  const hours = formatEstimatedHours(task.estimatedHours)
  return (
    <li
      className="flex flex-col gap-1.5 rounded-md border border-border px-3 py-2"
      data-testid={`proposal-task-${task.number}`}
    >
      <div className="flex flex-wrap items-center gap-1.5">
        <span className="text-sm font-medium break-words">
          {task.number}. {task.title}
        </span>
        {task.taskType !== '' && <Badge variant="outline">{task.taskType}</Badge>}
        {hours !== null && <Badge variant="outline">{hours}</Badge>}
        {task.irreversible && (
          <Badge
            variant="outline"
            data-testid="proposal-irreversible"
            className="border-destructive/30 bg-destructive/5 text-destructive"
          >
            <TriangleAlert aria-hidden />
            {translate(
              'auto.components.request.plan.documents.proposal.irreversible',
              'Irreversible'
            )}
          </Badge>
        )}
        {task.labels.map((label) => (
          <Badge key={label} variant="secondary">
            {label}
          </Badge>
        ))}
      </div>
      {task.dependsOn.length > 0 && (
        <p className="text-xs text-muted-foreground" data-testid="proposal-depends">
          {translate(
            'auto.components.request.plan.documents.proposal.dependsOn',
            'Depends on {{tasks}}',
            { tasks: formatDependencies(task.dependsOn) }
          )}
        </p>
      )}
      {task.description.trim() !== '' && (
        <RequestMarkdownContent gfm content={task.description} title={task.title} />
      )}
      {task.promptTemplate.trim() !== '' && (
        <details>
          <summary className="cursor-pointer text-xs text-muted-foreground">
            {translate('auto.components.request.plan.documents.proposal.prompt', 'Agent prompt')}
          </summary>
          <pre className="mt-1 max-h-60 overflow-auto scrollbar-sleek whitespace-pre-wrap break-words rounded-md border border-border bg-muted/40 p-2 font-mono text-xs">
            {task.promptTemplate}
          </pre>
        </details>
      )}
    </li>
  )
}

export function PlanProposalView({ request, shown, current }: Props): React.JSX.Element {
  const onScreen = shown ?? current
  const proposal = usePlanProposalOfRun(request.id, {
    runId: onScreen?.runId,
    revision: onScreen?.revision,
    refreshKey: `${request.version ?? ''}|${request.planTaskId ?? ''}`
  })
  const view = proposal.view

  if (view === null && proposal.error === null) {
    return (
      <div className="flex flex-col gap-2 p-4" aria-busy="true" data-testid="plan-proposal-loading">
        <Skeleton className="h-5 w-1/2" />
        <Skeleton className="h-16 w-full" />
        <Skeleton className="h-16 w-full" />
      </div>
    )
  }
  if (view === null) {
    return (
      <div
        role="alert"
        data-testid="plan-proposal-error"
        className="flex flex-wrap items-center gap-2 p-4 text-sm text-destructive"
      >
        <AlertTriangle className="size-4 shrink-0" aria-hidden />
        <span className="min-w-0 flex-1">{requestErrorMessage(proposal.error)}</span>
        <Button size="sm" variant="outline" onClick={proposal.refetch}>
          {translate('auto.components.request.plan.documents.tryAgain', 'Try again')}
        </Button>
      </div>
    )
  }
  if (view.proposal === null) {
    const text =
      view.status === 'running'
        ? translate(
            'auto.components.request.plan.documents.proposal.writing',
            'A new version is being written...'
          )
        : view.status === 'failed'
          ? translate(
              'auto.components.request.plan.documents.proposal.runFailed',
              'The latest run failed ({{code}}), so there is no proposal to show.',
              { code: view.errorCode === '' ? '-' : view.errorCode }
            )
          : translate(
              'auto.components.request.plan.documents.proposal.none',
              'There is no proposal to show yet.'
            )
    return (
      <p className="p-4 text-sm text-muted-foreground" data-testid="plan-proposal-empty">
        {text}
      </p>
    )
  }

  const outline = toProposalOutline(view.proposal)
  // Why: without a version list the newest run is shown, and while a rewrite runs that is not the saved plan yet.
  const maybeUnsaved = onScreen === null && request.status === 'planning'
  return (
    <div className="flex flex-col gap-3 px-4 py-3" data-testid="plan-proposal-view">
      {maybeUnsaved && (
        <p className="text-xs text-muted-foreground" data-testid="plan-proposal-unsaved">
          {translate(
            'auto.components.request.plan.documents.proposal.notSaved',
            'This is the latest proposal. It is not saved as the plan yet.'
          )}
        </p>
      )}
      {outline.title.trim() !== '' && <h3 className="text-sm font-semibold">{outline.title}</h3>}
      {outline.summary.trim() !== '' && (
        <RequestMarkdownContent gfm content={outline.summary} title={outline.title} />
      )}
      {outline.notes.trim() !== '' && (
        <section className="flex flex-col gap-1" data-testid="proposal-notes">
          <h4 className="text-xs font-medium">
            {translate('auto.components.request.plan.documents.proposal.notes', 'Notes')}
          </h4>
          <RequestMarkdownContent gfm content={outline.notes} />
        </section>
      )}
      {outline.taskCount === 0 && (
        <p className="text-sm text-muted-foreground" data-testid="proposal-no-tasks">
          {translate(
            'auto.components.request.plan.documents.proposal.noTasks',
            'The proposal has no tasks.'
          )}
        </p>
      )}
      {outline.phases.map((phase) => (
        <Collapsible
          key={phase.number}
          defaultOpen
          className="rounded-md border border-border"
          data-testid={`proposal-phase-${phase.number}`}
        >
          <CollapsibleTrigger className="group flex w-full items-center gap-2 px-3 py-2 text-left text-sm font-medium outline-none focus-visible:ring-[3px] focus-visible:ring-ring/50">
            <ChevronDown
              className="size-3.5 shrink-0 transition-transform group-data-[state=closed]:-rotate-90"
              aria-hidden
            />
            <span className="min-w-0 flex-1 break-words">
              {translate(
                'auto.components.request.plan.documents.proposal.phase',
                'Phase {{n}}: {{title}}',
                {
                  n: phase.number,
                  title: phase.title
                }
              )}
            </span>
          </CollapsibleTrigger>
          <CollapsibleContent className="flex flex-col gap-2 border-t border-border px-3 py-2">
            {phase.dependsOn.length > 0 && (
              <p className="text-xs text-muted-foreground">
                {translate(
                  'auto.components.request.plan.documents.proposal.phaseDependsOn',
                  'Depends on phase {{phases}}',
                  { phases: formatDependencies(phase.dependsOn) }
                )}
              </p>
            )}
            {phase.description.trim() !== '' && (
              <RequestMarkdownContent gfm content={phase.description} title={phase.title} />
            )}
            <ol className="flex flex-col gap-2">
              {phase.tasks.map((task) => (
                <TaskItem key={task.number} task={task} />
              ))}
            </ol>
          </CollapsibleContent>
        </Collapsible>
      ))}
      {outline.tasks.length > 0 && (
        <ol className="flex flex-col gap-2">
          {outline.tasks.map((task) => (
            <TaskItem key={task.number} task={task} />
          ))}
        </ol>
      )}
    </div>
  )
}
