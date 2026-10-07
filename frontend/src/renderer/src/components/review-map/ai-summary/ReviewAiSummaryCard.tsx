/**
 * ReviewAiSummaryCard.tsx — FE-CV-TASK-093-04
 *
 * Card component displaying the AI review summary result.
 * Collapsed by default; supports regenerate and feedback actions.
 *
 * Rules:
 * - No dangerouslySetInnerHTML — plain text only
 * - No "AI reviewed" — use "AI Summary" label only
 * - Spinner after ~200ms delay
 * - No import of quality gate hook
 *
 * @module components/review-map/ai-summary/ReviewAiSummaryCard
 */

import React, { useState, useEffect } from 'react'
import { ChevronDown, ChevronRight, Loader2, RefreshCw, ThumbsUp, ThumbsDown } from 'lucide-react'
import { Button } from '@/components/ui/button'
import type { AiSummaryModel } from './ai-summary-wire-parser'

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

export type ReviewAiSummaryCardProps = {
  model: AiSummaryModel | null
  /** Whether the summary is currently being generated */
  isGenerating: boolean
  errorCode: string | null
  onRegenerate?: () => void
  onFeedback?: (useful: boolean) => void
  translate: (key: string, params?: Record<string, unknown>) => string
}

// ---------------------------------------------------------------------------
// Spinner delay hook — show spinner only after 200ms
// ---------------------------------------------------------------------------

function useDelayedVisible(active: boolean, delayMs = 200): boolean {
  const [visible, setVisible] = useState(false)
  useEffect(() => {
    if (!active) { setVisible(false); return }
    const id = setTimeout(() => setVisible(true), delayMs)
    return () => clearTimeout(id)
  }, [active, delayMs])
  return visible
}

// ---------------------------------------------------------------------------
// Component
// ---------------------------------------------------------------------------

export function ReviewAiSummaryCard({
  model,
  isGenerating,
  errorCode,
  onRegenerate,
  onFeedback,
  translate,
}: ReviewAiSummaryCardProps): React.ReactElement | null {
  const [expanded, setExpanded] = useState(false)
  const [feedbackGiven, setFeedbackGiven] = useState<boolean | null>(null)

  const showSpinner = useDelayedVisible(isGenerating)

  const handleFeedback = (useful: boolean) => {
    setFeedbackGiven(useful)
    onFeedback?.(useful)
  }

  // Generating state
  if (isGenerating) {
    return (
      <div
        role="status"
        aria-label={translate('auto.components.reviewMap.aiSummary.card.generating')}
        className="flex items-center gap-2 p-3 text-sm text-muted-foreground"
      >
        {showSpinner && <Loader2 className="size-3.5 animate-spin" aria-hidden />}
        {translate('auto.components.reviewMap.aiSummary.card.generating')}
      </div>
    )
  }

  // Error state
  if (errorCode) {
    return (
      <div role="alert" className="p-3">
        <p className="text-sm text-destructive">
          {translate('auto.components.reviewMap.aiSummary.card.error')}
        </p>
        {onRegenerate && (
          <Button variant="outline" size="xs" onClick={onRegenerate} className="mt-2">
            <RefreshCw className="size-3.5 mr-1" aria-hidden />
            {translate('auto.components.reviewMap.aiSummary.card.retry')}
          </Button>
        )}
      </div>
    )
  }

  // No data yet
  if (!model) return null

  return (
    <section
      aria-label={translate('auto.components.reviewMap.aiSummary.card.label')}
      className="border border-border rounded-md overflow-hidden"
    >
      {/* Header — always visible */}
      <button
        type="button"
        onClick={() => setExpanded((e) => !e)}
        className="w-full flex items-center justify-between px-3 py-2 text-sm font-medium hover:bg-muted/50 transition-colors"
        aria-expanded={expanded}
      >
        <span>{translate('auto.components.reviewMap.aiSummary.card.label')}</span>
        {expanded
          ? <ChevronDown className="size-3.5" aria-hidden />
          : <ChevronRight className="size-3.5" aria-hidden />}
      </button>

      {/* Body — collapsed by default */}
      {expanded && (
        <div className="px-3 pb-3 space-y-3">
          {model.title && (
            <p className="text-xs text-muted-foreground italic">{model.title}</p>
          )}

          {model.summary && (
            <p className="text-sm">{model.summary}</p>
          )}

          {model.sections.map((section) => (
            <div key={section.id}>
              <h4 className="text-xs font-medium uppercase tracking-wide text-muted-foreground mb-1">
                {section.title}
              </h4>
              <p className="text-sm whitespace-pre-wrap break-words">{section.body}</p>
            </div>
          ))}

          {/* Footer actions */}
          <div className="flex items-center gap-2 pt-2 border-t border-border">
            {onRegenerate && (
              <Button variant="ghost" size="xs" onClick={onRegenerate}>
                <RefreshCw className="size-3.5 mr-1" aria-hidden />
                {translate('auto.components.reviewMap.aiSummary.card.regenerate')}
              </Button>
            )}

            {feedbackGiven === null && onFeedback && (
              <>
                <Button
                  variant="ghost"
                  size="xs"
                  onClick={() => handleFeedback(true)}
                  aria-label={translate('auto.components.reviewMap.aiSummary.card.useful')}
                >
                  <ThumbsUp className="size-3.5" aria-hidden />
                </Button>
                <Button
                  variant="ghost"
                  size="xs"
                  onClick={() => handleFeedback(false)}
                  aria-label={translate('auto.components.reviewMap.aiSummary.card.notUseful')}
                >
                  <ThumbsDown className="size-3.5" aria-hidden />
                </Button>
              </>
            )}
          </div>
        </div>
      )}
    </section>
  )
}
