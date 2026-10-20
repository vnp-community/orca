/**
 * PlanChecksPanel — CR-REQ-045
 *
 * What was checked about the OpenSpec change and what its author said about the project's design knowledge. The checks are
 * Orca's; the knowledge report is the agent's own word and is labelled that way. Both exist for the current version only.
 *
 * @module components/request/plan/documents/PlanChecksPanel
 */

import React from 'react'
import { ChevronDown, Info, TriangleAlert } from 'lucide-react'
import { translate } from '@/i18n/i18n'
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from '@/components/ui/collapsible'
import type {
  PlanCheck,
  PlanCheckSeverity,
  PlanKnowledge
} from '../../../../../../shared/request-plan-revision'

type Props = {
  checks: readonly PlanCheck[]
  knowledge: PlanKnowledge | null
  /** The version on screen is the current one; the backend keeps checks and knowledge for that one only. */
  isCurrent: boolean
}

function severityLabel(severity: PlanCheckSeverity): string {
  switch (severity) {
    case 'warning':
      return translate('auto.components.request.plan.documents.checks.severityWarning', 'Warning')
    case 'error':
      return translate('auto.components.request.plan.documents.checks.severityError', 'Error')
    case 'info':
      return translate('auto.components.request.plan.documents.checks.severityInfo', 'Info')
  }
}

function TextList({
  items,
  testId
}: {
  items: readonly string[]
  testId: string
}): React.JSX.Element {
  return (
    <ul className="list-disc space-y-0.5 pl-4" data-testid={testId}>
      {items.map((item, index) => (
        // The agent may repeat a line; the position is the only stable identity.
        <li key={`${index}:${item}`} className="break-words">
          {item}
        </li>
      ))}
    </ul>
  )
}

function KnowledgeReport({ knowledge }: { knowledge: PlanKnowledge }): React.JSX.Element {
  return (
    <div className="flex flex-col gap-1" data-testid="plan-knowledge-report">
      <h4 className="text-xs font-medium">
        {translate(
          'auto.components.request.plan.documents.knowledge.title',
          'Reported by the authoring agent'
        )}
      </h4>
      <p className="text-xs text-muted-foreground">
        {knowledge.available
          ? translate(
              'auto.components.request.plan.documents.knowledge.available',
              'Design knowledge was available.'
            )
          : translate(
              'auto.components.request.plan.documents.knowledge.unavailable',
              'Design knowledge was not available.'
            )}
        {knowledge.snapshot !== '' &&
          ` ${translate(
            'auto.components.request.plan.documents.knowledge.snapshot',
            'Snapshot {{snapshot}}.',
            { snapshot: knowledge.snapshot }
          )}`}
      </p>
      {knowledge.read.length > 0 && (
        <details>
          <summary className="cursor-pointer text-xs">
            {translate(
              'auto.components.request.plan.documents.knowledge.read',
              'Elements read: {{count}}',
              { count: knowledge.read.length }
            )}
          </summary>
          <TextList items={knowledge.read} testId="plan-knowledge-read" />
        </details>
      )}
      {knowledge.gaps.length > 0 && (
        <div>
          <p className="text-xs font-medium">
            {translate('auto.components.request.plan.documents.knowledge.gaps', 'Gaps')}
          </p>
          <TextList items={knowledge.gaps} testId="plan-knowledge-gaps" />
        </div>
      )}
      {knowledge.conflicts.length > 0 && (
        <div>
          <p className="text-xs font-medium">
            {translate('auto.components.request.plan.documents.knowledge.conflicts', 'Conflicts')}
          </p>
          <TextList items={knowledge.conflicts} testId="plan-knowledge-conflicts" />
        </div>
      )}
      <p className="text-xs text-muted-foreground" data-testid="plan-knowledge-disclaimer">
        {translate(
          'auto.components.request.plan.documents.knowledge.disclaimer',
          "This is the agent's own report; Orca did not verify it."
        )}
      </p>
    </div>
  )
}

export function PlanChecksPanel({ checks, knowledge, isCurrent }: Props): React.JSX.Element | null {
  if (!isCurrent) {
    return (
      <p className="text-xs text-muted-foreground" data-testid="plan-checks-current-only">
        {translate(
          'auto.components.request.plan.documents.checks.currentOnly',
          'Checks are shown for the current version only.'
        )}
      </p>
    )
  }
  if (checks.length === 0 && knowledge === null) {
    return null
  }
  const warnings = checks.filter((c) => c.severity !== 'info').length
  return (
    <Collapsible
      defaultOpen={warnings > 0}
      className="rounded-md border border-border"
      data-testid="plan-checks-panel"
    >
      <CollapsibleTrigger className="group flex w-full items-center gap-2 px-3 py-2 text-left text-xs font-medium outline-none focus-visible:ring-[3px] focus-visible:ring-ring/50">
        <ChevronDown
          className="size-3.5 transition-transform group-data-[state=closed]:-rotate-90"
          aria-hidden
        />
        {translate('auto.components.request.plan.documents.checks.title', 'Checks')}
        {warnings > 0 && (
          <span
            className="font-normal text-quality-warning"
            data-testid="plan-checks-warning-count"
          >
            {translate(
              'auto.components.request.plan.documents.checks.warningCount',
              '{{count}} to review',
              { count: warnings }
            )}
          </span>
        )}
      </CollapsibleTrigger>
      <CollapsibleContent className="flex flex-col gap-3 border-t border-border px-3 py-2">
        {checks.length > 0 && (
          <ul className="flex flex-col gap-1.5" data-testid="plan-checks-list">
            {checks.map((check, index) => (
              <li
                key={`${index}:${check.code}`}
                className="flex items-start gap-2 text-xs"
                data-severity={check.severity}
              >
                {check.severity === 'info' ? (
                  <Info className="mt-0.5 size-3.5 shrink-0 text-muted-foreground" aria-hidden />
                ) : (
                  <TriangleAlert
                    className="mt-0.5 size-3.5 shrink-0 text-quality-warning"
                    aria-hidden
                  />
                )}
                <span className="min-w-0 break-words">
                  <span className="font-medium">{severityLabel(check.severity)}</span>{' '}
                  <code className="font-mono">{check.code}</code>
                  {check.where !== '' && (
                    <span className="text-muted-foreground"> ({check.where})</span>
                  )}
                  {check.message !== '' && <span>: {check.message}</span>}
                </span>
              </li>
            ))}
          </ul>
        )}
        {knowledge !== null && <KnowledgeReport knowledge={knowledge} />}
      </CollapsibleContent>
    </Collapsible>
  )
}
