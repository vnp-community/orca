/**
 * ReviewAiSummaryCard.tsx — FE-CV-TASK-093-04
 *
 * Collapsed-by-default card for the AI-inferred summary. Always labelled as inferred
 * (model + data level shown), rendered as plain text, kept apart from the quality gate.
 * File references are only clickable when the file really is in the changed set.
 *
 * @module components/review-map/ai-summary/ReviewAiSummaryCard
 */

import { useState } from 'react'
import { ChevronDown, Loader2, Sparkles } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from '@/components/ui/collapsible'
import { translateCatalogKey } from '@/i18n/catalog-key-translate'
import { AiSummaryDataPreviewDialog } from './AiSummaryDataPreviewDialog'
import type { UseReviewAiSummaryResult } from './use-review-ai-summary'
import type { AiSummaryLevel } from './ai-summary-wire-parser'

const BASE = 'auto.components.reviewMap.aiSummary.card'

export type ReviewAiSummaryCardProps = {
  ai: UseReviewAiSummaryResult
  /** Paths of the changed files; references outside this set are shown as plain text. */
  changedFiles: ReadonlySet<string>
  onOpenFile: (path: string) => void
  /** Optional enum-only feedback hook (no content); buttons render only when provided. */
  onFeedback?: (value: 'helpful' | 'incorrect') => void
  translate?: (key: string, params?: Record<string, unknown>) => string
}

export function ReviewAiSummaryCard({
  ai,
  changedFiles,
  onOpenFile,
  onFeedback,
  translate = translateCatalogKey
}: ReviewAiSummaryCardProps): React.JSX.Element | null {
  const [open, setOpen] = useState(false)
  const [chosenLevel, setChosenLevel] = useState<AiSummaryLevel>('metadata')
  if (ai.state === 'hidden') {
    return null
  }

  const busy = ai.state === 'previewing' || ai.state === 'generating'
  const summary = ai.view?.summary ?? null

  const fileLink = (path: string): React.JSX.Element =>
    changedFiles.has(path) ? (
      <button type="button" className="underline-offset-2 hover:underline" onClick={() => onOpenFile(path)}>
        {path}
      </button>
    ) : (
      <span>{path}</span>
    )

  return (
    <Collapsible open={open} onOpenChange={setOpen} className="rounded-md border border-border bg-card text-xs">
      <div className="flex items-center gap-2 px-3 py-2">
        <CollapsibleTrigger asChild>
          <button type="button" className="flex min-w-0 flex-1 items-center gap-1.5 text-left font-medium text-foreground">
            <Sparkles className="size-3.5 shrink-0" aria-hidden />
            <span className="truncate">{translate(`${BASE}.title`)}</span>
            <ChevronDown className={`size-3.5 shrink-0 transition-transform ${open ? 'rotate-180' : ''}`} aria-hidden />
          </button>
        </CollapsibleTrigger>
        {busy ? (
          <>
            <Loader2 className="size-3.5 animate-spin text-muted-foreground" aria-hidden />
            <Button type="button" variant="ghost" size="xs" onClick={ai.cancel}>
              {translate(`${BASE}.cancel`)}
            </Button>
          </>
        ) : (
          <Button
            type="button"
            variant="outline"
            size="xs"
            onClick={() => void ai.preview(chosenLevel)}
          >
            {translate(summary ? `${BASE}.regenerate` : `${BASE}.generate`)}
          </Button>
        )}
      </div>

      <CollapsibleContent className="space-y-2 border-t border-border px-3 py-2">
        {ai.allowedLevels.length > 1 ? (
          <div className="flex gap-1" role="group" aria-label={translate(`${BASE}.levelLabel`)}>
            {ai.allowedLevels.map((level) => (
              <Button
                key={level}
                type="button"
                size="xs"
                variant={chosenLevel === level ? 'secondary' : 'outline'}
                aria-pressed={chosenLevel === level}
                onClick={() => setChosenLevel(level)}
              >
                {translate(`${BASE}.level.${level}`)}
              </Button>
            ))}
          </div>
        ) : null}

        {busy ? (
          <p role="status" className="text-muted-foreground">
            {translate(ai.state === 'previewing' ? `${BASE}.preparing` : `${BASE}.generating`)}
          </p>
        ) : null}

        {ai.state === 'error' && ai.error ? (
          <div role="status" className="space-y-1">
            <p className="text-muted-foreground">
              {translate(`${BASE}.error.${ai.error}`, { seconds: ai.retryAfterSeconds ?? 0 })}
            </p>
            {ai.error !== 'forbidden' ? (
              <Button type="button" variant="outline" size="xs" onClick={() => void ai.preview(chosenLevel)}>
                {translate(`${BASE}.retry`)}
              </Button>
            ) : null}
          </div>
        ) : null}

        {summary ? (
          <div className="space-y-2">
            <p className="text-muted-foreground">
              {translate(`${BASE}.disclaimer`)}{' '}
              {translate(`${BASE}.meta`, { model: summary.model || '—', level: translate(`${BASE}.level.${summary.level === 'diff' ? 'diff' : 'metadata'}`) })}
            </p>
            {/* Model output: plain text nodes only (no HTML, no Markdown, no links). */}
            <p className="whitespace-pre-wrap break-words text-foreground">{summary.summary}</p>
            {summary.risks.length > 0 ? (
              <div>
                <p className="font-medium text-foreground">{translate(`${BASE}.risks`)}</p>
                <ul className="list-disc space-y-0.5 pl-4">
                  {summary.risks.map((risk, i) => (
                    <li key={i} className="break-words">
                      {risk.text}
                      {risk.refs.length > 0 ? (
                        <span className="ml-1 text-muted-foreground">
                          {risk.refs.map((ref, j) => (
                            <span key={ref}>
                              {j > 0 ? ', ' : ''}
                              {fileLink(ref)}
                            </span>
                          ))}
                        </span>
                      ) : null}
                    </li>
                  ))}
                </ul>
              </div>
            ) : null}
            {summary.readFirst.length > 0 ? (
              <div>
                <p className="font-medium text-foreground">{translate(`${BASE}.readFirst`)}</p>
                <ul className="space-y-0.5">
                  {summary.readFirst.map((item) => (
                    <li key={item.file} className="break-words">
                      {fileLink(item.file)}
                      <span className="text-muted-foreground"> — {item.why}</span>
                    </li>
                  ))}
                </ul>
              </div>
            ) : null}
            {summary.refsDropped > 0 ? (
              <p className="text-muted-foreground">{translate(`${BASE}.refsDropped`, { count: summary.refsDropped })}</p>
            ) : null}
            {onFeedback ? (
              <div className="flex gap-1">
                <Button type="button" variant="outline" size="xs" onClick={() => onFeedback('helpful')}>
                  {translate(`${BASE}.helpful`)}
                </Button>
                <Button type="button" variant="outline" size="xs" onClick={() => onFeedback('incorrect')}>
                  {translate(`${BASE}.incorrect`)}
                </Button>
              </div>
            ) : null}
          </div>
        ) : ai.state === 'idle' ? (
          <p className="text-muted-foreground">{translate(`${BASE}.idleHint`)}</p>
        ) : null}
      </CollapsibleContent>

      <AiSummaryDataPreviewDialog
        open={ai.state === 'awaiting-consent'}
        manifest={ai.manifest}
        onConfirm={() => void ai.confirmAndGenerate()}
        onCancel={ai.cancel}
        translate={translate}
      />
    </Collapsible>
  )
}
