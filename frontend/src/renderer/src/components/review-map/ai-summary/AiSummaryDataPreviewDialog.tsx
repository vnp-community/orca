/**
 * AiSummaryDataPreviewDialog.tsx — FE-CV-TASK-093-03
 *
 * Dialog that shows what data will be sent to the LLM provider
 * before the user confirms AI summary generation.
 *
 * Design rules:
 * - No overclaiming: "will send", not "safely sends"
 * - Default focus on Cancel button (safe default)
 * - "Send and generate" locks immediately on submit
 * - No dangerouslySetInnerHTML
 *
 * @module components/review-map/ai-summary/AiSummaryDataPreviewDialog
 */

import React, { useRef, useEffect } from 'react'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { AlertTriangle } from 'lucide-react'

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

export type AiSummaryDataManifest = {
  fileCount: number
  totalBytes: number
  withheldCount: number
  estimatedTokens: number
  suspectedInjection: boolean
}

export type AiSummaryDataPreviewDialogProps = {
  open: boolean
  manifest: AiSummaryDataManifest | null
  onConfirm: () => void
  onCancel: () => void
  translate: (key: string, params?: Record<string, unknown>) => string
  isSubmitting?: boolean
}

// ---------------------------------------------------------------------------
// Component
// ---------------------------------------------------------------------------

export function AiSummaryDataPreviewDialog({
  open,
  manifest,
  onConfirm,
  onCancel,
  translate,
  isSubmitting = false,
}: AiSummaryDataPreviewDialogProps): React.ReactElement {
  const cancelRef = useRef<HTMLButtonElement>(null)

  // Focus cancel button when dialog opens (safe default)
  useEffect(() => {
    if (open) {
      setTimeout(() => cancelRef.current?.focus(), 50)
    }
  }, [open])

  return (
    <Dialog open={open} onOpenChange={(o) => { if (!o) onCancel() }}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>
            {translate('auto.components.reviewMap.aiSummary.previewDialog.title')}
          </DialogTitle>
          <DialogDescription>
            {translate('auto.components.reviewMap.aiSummary.previewDialog.description')}
          </DialogDescription>
        </DialogHeader>

        {manifest && (
          <div className="text-sm space-y-2 py-2">
            <div className="grid grid-cols-2 gap-x-4 gap-y-1">
              <span className="text-muted-foreground">
                {translate('auto.components.reviewMap.aiSummary.previewDialog.fileCount')}
              </span>
              <span>{manifest.fileCount}</span>

              <span className="text-muted-foreground">
                {translate('auto.components.reviewMap.aiSummary.previewDialog.totalBytes')}
              </span>
              <span>{manifest.totalBytes.toLocaleString()}</span>

              <span className="text-muted-foreground">
                {translate('auto.components.reviewMap.aiSummary.previewDialog.withheld')}
              </span>
              <span>{manifest.withheldCount}</span>

              <span className="text-muted-foreground">
                {translate('auto.components.reviewMap.aiSummary.previewDialog.estimatedTokens')}
              </span>
              <span>~{manifest.estimatedTokens.toLocaleString()}</span>
            </div>

            {manifest.suspectedInjection && (
              <div
                role="alert"
                className="flex items-start gap-2 rounded border border-destructive/40 bg-destructive/10 p-2 mt-2"
              >
                <AlertTriangle className="size-4 text-destructive shrink-0 mt-0.5" aria-hidden />
                <p className="text-destructive text-xs">
                  {translate('auto.components.reviewMap.aiSummary.previewDialog.injectionWarning')}
                </p>
              </div>
            )}
          </div>
        )}

        <DialogFooter>
          <Button
            ref={cancelRef}
            variant="ghost"
            onClick={onCancel}
            disabled={isSubmitting}
          >
            {translate('auto.components.reviewMap.aiSummary.previewDialog.cancel')}
          </Button>
          <Button
            onClick={onConfirm}
            disabled={isSubmitting}
          >
            {translate('auto.components.reviewMap.aiSummary.previewDialog.confirm')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
