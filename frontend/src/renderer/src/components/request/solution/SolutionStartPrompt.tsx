/**
 * SolutionStartPrompt — TASK-FE-E2E-009
 *
 * Shown when a Request is `analyzing` but no Solution and no AI run exist: names the action
 * instead of leaving the tab on "AI is analyzing" forever.
 *
 * @module components/request/solution/SolutionStartPrompt
 */

import React from 'react'
import { Loader2, Sparkles } from 'lucide-react'
import { translate } from '@/i18n/i18n'
import { Button } from '@/components/ui/button'

type Props = {
  onStart: () => void
  /** True while the call is in flight or the AI rate-limit class is cooling down. */
  disabled: boolean
  submitting: boolean
}

export function SolutionStartPrompt({ onStart, disabled, submitting }: Props): React.JSX.Element {
  return (
    <div data-testid="solution-start-prompt" className="flex flex-col items-start gap-3">
      <p className="text-sm text-muted-foreground">
        {translate(
          'auto.components.request.SolutionStartPrompt.description',
          'No analysis has started for this request yet. Start it to get solution options to review.'
        )}
      </p>
      <Button size="sm" disabled={disabled} onClick={onStart} data-testid="solution-generate">
        {submitting ? (
          <Loader2 className="size-3.5 animate-spin" aria-hidden />
        ) : (
          <Sparkles className="size-3.5" aria-hidden />
        )}
        {translate('auto.components.request.SolutionStartPrompt.generate', 'Generate solution')}
      </Button>
    </div>
  )
}
