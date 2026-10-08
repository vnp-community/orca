/**
 * AiSummaryDataPreviewDialog.tsx — FE-CV-TASK-093-03
 *
 * Shows exactly what would leave the machine before the user agrees to generate an AI
 * summary. Initial focus is Cancel, so a stray Enter cannot send data. File paths and
 * provider names come from the backend and render as text only.
 *
 * @module components/review-map/ai-summary/AiSummaryDataPreviewDialog
 */

import { useRef } from 'react'
import { TriangleAlert } from 'lucide-react'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle
} from '@/components/ui/dialog'
import { translateCatalogKey } from '@/i18n/catalog-key-translate'
import type { AiSummaryManifest } from './ai-summary-wire-parser'

const BASE = 'auto.components.reviewMap.aiSummary.preview'
const MAX_LISTED_FILES = 12

export type AiSummaryDataPreviewDialogProps = {
  open: boolean
  manifest: AiSummaryManifest | null
  /** True while the confirmed request is being sent; locks both buttons. */
  submitting?: boolean
  onConfirm: () => void
  onCancel: () => void
  translate?: (key: string, params?: Record<string, unknown>) => string
}

export function AiSummaryDataPreviewDialog({
  open,
  manifest,
  submitting = false,
  onConfirm,
  onCancel,
  translate = translateCatalogKey
}: AiSummaryDataPreviewDialogProps): React.JSX.Element {
  const cancelRef = useRef<HTMLButtonElement>(null)
  const withheld = manifest?.files.filter((f) => f.withheld).length ?? 0
  const listed = manifest?.files.slice(0, MAX_LISTED_FILES) ?? []

  return (
    <Dialog open={open} onOpenChange={(next) => !next && !submitting && onCancel()}>
      <DialogContent
        onOpenAutoFocus={(event) => {
          event.preventDefault()
          cancelRef.current?.focus()
        }}
      >
        <DialogHeader>
          <DialogTitle>{translate(`${BASE}.title`)}</DialogTitle>
          <DialogDescription>
            {translate(`${BASE}.description`, { provider: manifest?.provider ?? translate(`${BASE}.defaultProvider`) })}
          </DialogDescription>
        </DialogHeader>

        {manifest ? (
          <div className="space-y-2 text-xs">
            <p className="text-foreground">
              {translate(`${BASE}.counts`, {
                files: manifest.files.length,
                redactions: manifest.redactions,
                tokens: manifest.estimatedTokens.toLocaleString()
              })}
            </p>
            <p className="text-muted-foreground">{translate(`${BASE}.level.${manifest.level}`)}</p>
            {withheld > 0 ? (
              <p className="text-muted-foreground">{translate(`${BASE}.withheld`, { count: withheld })}</p>
            ) : null}
            {listed.length > 0 ? (
              <ul className="max-h-40 space-y-0.5 overflow-y-auto rounded-md border border-border p-2">
                {listed.map((file) => (
                  <li key={file.path} className="flex items-center justify-between gap-2">
                    <span className="min-w-0 truncate" title={file.path}>
                      {file.path}
                    </span>
                    <span className="shrink-0 text-muted-foreground">
                      {file.withheld ? translate(`${BASE}.fileWithheld`) : `${file.bytes.toLocaleString()} B`}
                    </span>
                  </li>
                ))}
              </ul>
            ) : null}
            {manifest.files.length > MAX_LISTED_FILES ? (
              <p className="text-muted-foreground">
                {translate(`${BASE}.moreFiles`, { count: manifest.files.length - MAX_LISTED_FILES })}
              </p>
            ) : null}
            {manifest.suspectedInjection ? (
              <p role="alert" className="flex items-start gap-1.5 rounded-md border border-border p-2 text-foreground">
                <TriangleAlert className="mt-0.5 size-3.5 shrink-0 text-destructive" aria-hidden />
                {translate(`${BASE}.injectionWarning`)}
              </p>
            ) : null}
          </div>
        ) : null}

        <DialogFooter>
          <Button ref={cancelRef} type="button" variant="ghost" disabled={submitting} onClick={onCancel}>
            {translate(`${BASE}.cancel`)}
          </Button>
          <Button type="button" disabled={submitting} onClick={onConfirm}>
            {translate(`${BASE}.confirm`)}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
