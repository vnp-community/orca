/**
 * SolutionRevisionFeedback — CR-REQ-041
 *
 * Collapsible "What was asked for this version": the reviewer's full request, shown as plain text. It is typed by a
 * person and written to every reader of the Request, so it is never parsed as Markdown or HTML.
 *
 * @module components/request/solution/SolutionRevisionFeedback
 */

import React from 'react'
import { ChevronRight } from 'lucide-react'
import { translate } from '@/i18n/i18n'
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from '@/components/ui/collapsible'

export function SolutionRevisionFeedback({
  feedback
}: {
  feedback: string | undefined
}): React.JSX.Element | null {
  if (!feedback || feedback.trim() === '') {
    return null
  }
  return (
    <Collapsible
      data-testid="solution-revision-feedback"
      className="rounded-md border border-border"
    >
      <CollapsibleTrigger className="group flex w-full items-center gap-1 px-3 py-2 text-left text-xs font-medium outline-none focus-visible:ring-[3px] focus-visible:ring-ring/50">
        <ChevronRight
          className="size-3.5 transition-transform group-data-[state=open]:rotate-90"
          aria-hidden
        />
        {translate(
          'auto.components.request.SolutionVersionSwitcher.askedFor',
          'What was asked for this version'
        )}
      </CollapsibleTrigger>
      <CollapsibleContent>
        <p
          data-testid="solution-revision-feedback-text"
          className="whitespace-pre-wrap break-words px-3 pb-3 text-sm text-foreground"
        >
          {feedback}
        </p>
      </CollapsibleContent>
    </Collapsible>
  )
}
