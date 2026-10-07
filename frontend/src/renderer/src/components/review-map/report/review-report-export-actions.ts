/**
 * review-report-export-actions.ts — FE-CV-TASK-090-05
 *
 * Standalone export action functions for review reports.
 * No React imports — pure functions for clipboard and file download.
 *
 * @module components/review-map/report/review-report-export-actions
 */

import { buildReviewReportMarkdown } from './review-report-markdown'
import { buildReviewReportHtml } from './review-report-html'
import type { ReviewReportModel } from './review-report-model-parser'

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

export type ExportResult =
  | { ok: true }
  | { ok: false; reason: 'clipboard_unavailable' | 'download_failed' | 'unknown' }

// ---------------------------------------------------------------------------
// Copy helpers
// ---------------------------------------------------------------------------

async function writeToClipboard(text: string): Promise<ExportResult> {
  // Prefer native API (Electron exposes window.api.ui.writeClipboardText)
  const api = (window as Record<string, unknown>).api as
    | { ui?: { writeClipboardText?: (text: string) => void } }
    | undefined

  if (api?.ui?.writeClipboardText) {
    try {
      api.ui.writeClipboardText(text)
      return { ok: true }
    } catch {
      // fall through to web fallback
    }
  }

  // Web fallback: navigator.clipboard
  if (navigator?.clipboard?.writeText) {
    try {
      await navigator.clipboard.writeText(text)
      return { ok: true }
    } catch {
      return { ok: false, reason: 'clipboard_unavailable' }
    }
  }

  return { ok: false, reason: 'clipboard_unavailable' }
}

// ---------------------------------------------------------------------------
// Export actions
// ---------------------------------------------------------------------------

export type ReviewReportExportActions = {
  /** Copy report as Markdown (for pasting into any editor) */
  copyMarkdown: () => Promise<ExportResult>
  /** Copy report formatted for PR/MR description insertion */
  copyForReview: () => Promise<ExportResult>
  /** Download report as an offline HTML file */
  downloadHtml: (filenameHint?: string) => ExportResult
}

/**
 * Build the set of export actions for a given report model.
 * Filename must not contain absolute paths.
 */
export function buildReviewReportExportActions(
  model: ReviewReportModel
): ReviewReportExportActions {
  return {
    copyMarkdown: async () => {
      const markdown = buildReviewReportMarkdown(model)
      return writeToClipboard(markdown)
    },

    copyForReview: async () => {
      // copyForReview includes only the section bodies without sentinel markers
      // so it can be pasted as plain PR description text
      const sections = model.sections
        .map((s) => `**${s.title}**\n\n${s.body.trim()}`)
        .join('\n\n---\n\n')
      const text = `**${model.title}**\n\n${model.description}\n\n${sections}`.trim()
      return writeToClipboard(text)
    },

    downloadHtml: (filenameHint?: string) => {
      try {
        const html = buildReviewReportHtml(model)
        const blob = new Blob([html], { type: 'text/html;charset=utf-8' })
        const url = URL.createObjectURL(blob)

        // Sanitize filename — basename only, no path traversal
        const rawName = (filenameHint ?? 'review-report').replace(/[/\\:*?"<>|]/g, '_')
        const filename = `${rawName}.html`

        const a = document.createElement('a')
        a.href = url
        a.download = filename
        a.rel = 'noopener'
        document.body.appendChild(a)
        a.click()
        document.body.removeChild(a)
        URL.revokeObjectURL(url)
        return { ok: true }
      } catch {
        return { ok: false, reason: 'download_failed' }
      }
    },
  }
}
