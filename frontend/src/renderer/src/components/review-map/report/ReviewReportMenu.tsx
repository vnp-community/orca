/**
 * ReviewReportMenu.tsx — FE-CV-TASK-090-06
 *
 * "Export report" dropdown: copy Markdown, copy for the PR/MR description, save HTML.
 * Renders nothing (and never calls the backend) while the quality flag is off.
 *
 * @module components/review-map/report/ReviewReportMenu
 */

import { useState } from 'react'
import { Copy, Download, FileText, Loader2 } from 'lucide-react'
import { toast } from 'sonner'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger
} from '@/components/ui/dropdown-menu'
import { Button } from '@/components/ui/button'
import { i18n, translate } from '@/i18n/i18n'
import {
  localizedHostedReviewCopy,
  resolveSupportedHostedReviewCopyProvider
} from '@/i18n/hosted-review-localized-copy'
import type { HostedReviewProvider } from '../../../../../shared/hosted-review'
import { buildReportFileName, copyTextToClipboard, downloadHtmlFile } from './review-report-export-actions'
import { trackReviewReportExported } from '@/lib/review-telemetry'
import { renderReportDiagramSvgs } from './review-report-diagram-svg'
import { buildReviewReportHtml } from './review-report-html'
import { buildReviewReportMarkdown, DEFAULT_REPORT_MAX_CHARS } from './review-report-markdown'
import { readReportThemeTokens } from './review-report-theme-tokens'
import { useReviewReport } from './use-review-report'

const K = 'auto.components.reviewMap.report.menu'

export type ReviewReportMenuProps = {
  worktreeId: string | null
  projectId: string | null | undefined
  base?: string | null
  provider: HostedReviewProvider | null | undefined
  repoSlug: string
}

type ExportKind = 'markdown' | 'description' | 'html'

export function telemetryProvider(
  provider: HostedReviewProvider | null | undefined
): 'github' | 'gitlab' | 'azure-devops' | 'gitea' | 'none' {
  return provider === 'github' || provider === 'gitlab' || provider === 'azure-devops' || provider === 'gitea'
    ? provider
    : 'none'
}

export function ReviewReportMenu({
  worktreeId,
  projectId,
  base,
  provider,
  repoSlug
}: ReviewReportMenuProps): React.JSX.Element | null {
  const { available, loading, fetchReport } = useReviewReport({ worktreeId, projectId, base })
  const [busy, setBusy] = useState<ExportKind | null>(null)
  if (!available) {
    return null
  }

  const run = async (kind: ExportKind): Promise<void> => {
    if (busy) {return}
    setBusy(kind)
    try {
      const fetched = await fetchReport()
      if (!fetched.ok) {
        if (fetched.reason === 'error') {
          toast.error(translate(`${K}.fetchError`, 'Could not load the review report.'))
        }
        return
      }
      const { model } = fetched
      const copy = localizedHostedReviewCopy(resolveSupportedHostedReviewCopyProvider(provider))
      const markdownOpts = { t: translate, reviewLabel: copy.reviewLabel }

      if (kind === 'html') {
        const isDark = document.documentElement.classList.contains('dark')
        const html = buildReviewReportHtml(model, {
          t: translate,
          locale: i18n.language || 'en',
          tokens: readReportThemeTokens(),
          svgByDiagram: await renderReportDiagramSvgs(model.diagrams, isDark)
        })
        const result = downloadHtmlFile(html, buildReportFileName(repoSlug, model.subject.headCommit))
        if (result.ok) {
          trackReviewReportExported({ format: 'html_save', provider: telemetryProvider(provider), truncated: false })
          toast.success(translate(`${K}.saved`, 'Report saved as HTML.'))
        }
        else {toast.error(translate(`${K}.saveError`, 'Could not save the report.'))}
        return
      }

      const { markdown, truncated } = buildReviewReportMarkdown(model, {
        ...markdownOpts,
        maxChars: kind === 'description' ? DEFAULT_REPORT_MAX_CHARS : Number.POSITIVE_INFINITY
      })
      const result = await copyTextToClipboard(markdown)
      if (result.ok) {
        trackReviewReportExported({
          format: kind === 'description' ? 'markdown_pr_insert' : 'markdown_copy',
          provider: telemetryProvider(provider),
          truncated
        })
        toast.success(
          truncated
            ? translate(`${K}.copiedShortened`, 'Copied a shortened report.')
            : translate(`${K}.copied`, 'Report copied.')
        )
      } else {
        toast.error(translate(`${K}.copyError`, 'Could not copy to the clipboard.'))
      }
    } finally {
      setBusy(null)
    }
  }

  const copy = localizedHostedReviewCopy(resolveSupportedHostedReviewCopyProvider(provider))
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="outline" size="xs" disabled={Boolean(busy) || loading}>
          {busy || loading ? (
            <Loader2 className="size-3.5 animate-spin" aria-hidden />
          ) : (
            <FileText className="size-3.5" aria-hidden />
          )}
          {translate(`${K}.label`, 'Export report')}
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        <DropdownMenuItem onSelect={() => void run('markdown')}>
          <Copy className="size-3.5" aria-hidden />
          {translate(`${K}.copyMarkdown`, 'Copy Markdown')}
        </DropdownMenuItem>
        <DropdownMenuItem onSelect={() => void run('description')}>
          <Copy className="size-3.5" aria-hidden />
          {translate(`${K}.copyForDescription`, 'Copy for {{review}} description', { review: copy.reviewLabel })}
        </DropdownMenuItem>
        <DropdownMenuSeparator />
        <DropdownMenuItem onSelect={() => void run('html')}>
          <Download className="size-3.5" aria-hidden />
          {translate(`${K}.saveHtml`, 'Save HTML')}
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}
