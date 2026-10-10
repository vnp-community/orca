/**
 * SolutionOpenQuestions — CR-REQ-027 §2.5, CR-REQ-028 §2.5
 *
 * Lists the AI's open questions before the decision bar. A `blocking` question
 * keeps approval refused until it has a recorded answer, so say that up front.
 * `renderAction` is the per-question slot; the panel plugs the "Ask back" button in it.
 *
 * @module components/request/solution/SolutionOpenQuestions
 */

import React from 'react'
import { CircleHelp, TriangleAlert } from 'lucide-react'
import { cn } from '@/lib/utils'
import { translate } from '@/i18n/i18n'
import { Badge } from '@/components/ui/badge'
import type { SolutionOpenQuestion } from '../../../../../shared/request-types'

const NO_IDS: string[] = []

export type SolutionOpenQuestionsProps = {
  questions: SolutionOpenQuestion[]
  /** Ids the server said are still unanswered (from a refused approval). */
  refusedIds?: string[]
  /** False once the decision is made: the list stays, the approval notice goes. */
  actionable: boolean
  renderAction?: (question: SolutionOpenQuestion) => React.ReactNode
}

export function SolutionOpenQuestions({
  questions,
  refusedIds = NO_IDS,
  actionable,
  renderAction
}: SolutionOpenQuestionsProps): React.JSX.Element | null {
  if (questions.length === 0 && refusedIds.length === 0) {
    return null
  }
  const refused = new Set(refusedIds)
  const hasBlocking = questions.some((q) => q.blocking)
  const showNotice = actionable && (hasBlocking || refused.size > 0)
  const wasRefused = refused.size > 0

  return (
    <section data-testid="solution-open-questions" className="space-y-2">
      <h3 className="text-sm font-medium">
        {translate('auto.components.request.SolutionOpenQuestions.title', 'Open questions')}
      </h3>
      {showNotice && (
        <div
          role={wasRefused ? 'alert' : 'status'}
          data-testid="solution-blocking-notice"
          data-refused={wasRefused ? 'true' : 'false'}
          className={cn(
            'flex items-start gap-2 rounded-md border px-3 py-2 text-sm',
            wasRefused
              ? 'border-destructive/30 bg-destructive/5 text-destructive'
              : 'border-border bg-muted text-foreground'
          )}
        >
          <TriangleAlert className="mt-0.5 size-4 shrink-0" aria-hidden />
          <p>
            {wasRefused
              ? translate(
                  'auto.components.request.SolutionOpenQuestions.refused',
                  'Approval was refused. These blocking questions are still unanswered: {{ids}}',
                  { ids: refusedIds.join(', ') }
                )
              : translate(
                  'auto.components.request.SolutionOpenQuestions.notice',
                  'Approval stays blocked until every blocking question below has a recorded answer.'
                )}
          </p>
        </div>
      )}
      {questions.length > 0 && (
        <ul className="space-y-2">
          {questions.map((q) => (
            <li
              key={q.id}
              data-testid={q.blocking ? 'solution-blocking-question' : 'solution-open-question'}
              data-question-id={q.id}
              data-refused={refused.has(q.id) ? 'true' : undefined}
              className="flex flex-col gap-1.5 rounded-md border border-border px-3 py-2 text-sm"
            >
              <div className="flex flex-wrap items-center gap-2">
                <CircleHelp className="size-4 shrink-0 text-muted-foreground" aria-hidden />
                <span className="text-xs text-muted-foreground">{q.id}</span>
                {q.blocking ? (
                  <Badge variant="destructive">
                    {translate(
                      'auto.components.request.SolutionOpenQuestions.blocking',
                      'Blocking'
                    )}
                  </Badge>
                ) : (
                  <Badge variant="outline">
                    {translate(
                      'auto.components.request.SolutionOpenQuestions.nonBlocking',
                      'Not blocking'
                    )}
                  </Badge>
                )}
              </div>
              <p className="whitespace-pre-wrap">{q.text}</p>
              {renderAction?.(q)}
            </li>
          ))}
        </ul>
      )}
    </section>
  )
}
