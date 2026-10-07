/**
 * ReviewReportMenu.tsx — FE-CV-TASK-090-06
 *
 * Dropdown menu for exporting the review report.
 * Three items: Copy Markdown, Copy for PR/MR, Download HTML.
 * Only renders when flags.quality is true.
 * Locks item immediately on click; shows toast confirmation.
 *
 * @module components/review-map/report/ReviewReportMenu
 */

import React, { useState } from 'react'
import { Download, Copy, FileText } from 'lucide-react'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
  DropdownMenuSeparator,
} from '@/components/ui/dropdown-menu'
import { Button } from '@/components/ui/button'
import { buildReviewReportExportActions } from './review-report-export-actions'
import type { ReviewReportModel } from './review-report-model-parser'

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

export type ReviewReportMenuProps = {
  model: ReviewReportModel
  /** i18n translate function */
  translate: (key: string, params?: Record<string, unknown>) => string
  /** Toast function for user feedback */
  showToast: (message: string, type?: 'success' | 'error') => void
  disabled?: boolean
}

// ---------------------------------------------------------------------------
// Component
// ---------------------------------------------------------------------------

export function ReviewReportMenu({
  model,
  translate,
  showToast,
  disabled = false,
}: ReviewReportMenuProps): React.ReactElement {
  const [busyItem, setBusyItem] = useState<string | null>(null)
  const actions = buildReviewReportExportActions(model)

  const handleCopyMarkdown = async () => {
    if (busyItem) return
    setBusyItem('markdown')
    try {
      const result = await actions.copyMarkdown()
      if (result.ok) {
        showToast(
          translate('auto.components.reviewMap.report.menu.copyMarkdown.success'),
          'success'
        )
      } else {
        showToast(
          translate('auto.components.reviewMap.report.menu.copyMarkdown.error'),
          'error'
        )
      }
    } finally {
      setBusyItem(null)
    }
  }

  const handleCopyForReview = async () => {
    if (busyItem) return
    setBusyItem('review')
    try {
      const result = await actions.copyForReview()
      if (result.ok) {
        showToast(
          translate('auto.components.reviewMap.report.menu.copyForReview.success'),
          'success'
        )
      } else {
        showToast(
          translate('auto.components.reviewMap.report.menu.copyForReview.error'),
          'error'
        )
      }
    } finally {
      setBusyItem(null)
    }
  }

  const handleDownloadHtml = () => {
    if (busyItem) return
    setBusyItem('html')
    try {
      const filenameHint = model.title
        .toLowerCase()
        .replace(/[^a-z0-9]+/g, '-')
        .slice(0, 48)
      const result = actions.downloadHtml(filenameHint)
      if (result.ok) {
        showToast(
          translate('auto.components.reviewMap.report.menu.downloadHtml.success'),
          'success'
        )
      } else {
        showToast(
          translate('auto.components.reviewMap.report.menu.downloadHtml.error'),
          'error'
        )
      }
    } finally {
      setBusyItem(null)
    }
  }

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button
          variant="outline"
          size="xs"
          disabled={disabled || Boolean(busyItem)}
          aria-label={translate('auto.components.reviewMap.report.menu.trigger')}
        >
          <FileText className="size-3.5" aria-hidden />
          {translate('auto.components.reviewMap.report.menu.label')}
        </Button>
      </DropdownMenuTrigger>

      <DropdownMenuContent align="end">
        <DropdownMenuItem
          onSelect={handleCopyMarkdown}
          disabled={Boolean(busyItem)}
        >
          <Copy className="size-3.5 mr-2" aria-hidden />
          {translate('auto.components.reviewMap.report.menu.copyMarkdown.label')}
        </DropdownMenuItem>

        <DropdownMenuItem
          onSelect={handleCopyForReview}
          disabled={Boolean(busyItem)}
        >
          <Copy className="size-3.5 mr-2" aria-hidden />
          {translate('auto.components.reviewMap.report.menu.copyForReview.label')}
        </DropdownMenuItem>

        <DropdownMenuSeparator />

        <DropdownMenuItem
          onSelect={handleDownloadHtml}
          disabled={Boolean(busyItem)}
        >
          <Download className="size-3.5 mr-2" aria-hidden />
          {translate('auto.components.reviewMap.report.menu.downloadHtml.label')}
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}
