/**
 * use-insert-review-report.ts — FE-CV-TASK-090-07
 *
 * Builds the "Insert review report" handler for the PR/MR composer. Undefined while
 * the quality flag is off, so the button is simply not rendered. Never inserts on
 * its own: only when the user clicks.
 *
 * @module components/review-map/report/use-insert-review-report
 */

import { useCallback, useRef } from 'react'
import { toast } from 'sonner'
import { translate } from '@/i18n/i18n'
import {
  localizedHostedReviewCopy,
  resolveSupportedHostedReviewCopyProvider
} from '@/i18n/hosted-review-localized-copy'
import type { HostedReviewProvider } from '../../../../../shared/hosted-review'
import { trackReviewReportExported } from '@/lib/review-telemetry'
import { mergeReviewReportIntoBody } from './merge-review-report-into-body'
import { buildReviewReportMarkdown } from './review-report-markdown'
import { useReviewReport } from './use-review-report'

export function useInsertReviewReport(opts: {
  worktreeId: string | null
  projectId: string | null | undefined
  base?: string | null
  provider: HostedReviewProvider | null | undefined
  body: string
  setBody: (value: string) => void
}): (() => Promise<void>) | undefined {
  const { available, fetchReport } = useReviewReport(opts)
  // Why: the fetch can take seconds; merge into the description as it is when the report arrives.
  const bodyRef = useRef(opts.body)
  bodyRef.current = opts.body
  const { provider, setBody } = opts

  const insert = useCallback(async (): Promise<void> => {
    const fetched = await fetchReport()
    if (!fetched.ok) {
      if (fetched.reason === 'error') {
        toast.error(translate('auto.components.reviewMap.report.menu.fetchError', 'Could not load the review report.'))
      }
      return
    }
    const copy = localizedHostedReviewCopy(resolveSupportedHostedReviewCopyProvider(provider))
    const { markdown, truncated } = buildReviewReportMarkdown(fetched.model, { t: translate, reviewLabel: copy.reviewLabel })
    setBody(mergeReviewReportIntoBody(bodyRef.current, markdown))
    trackReviewReportExported({
      format: 'markdown_pr_insert',
      provider: provider === 'github' || provider === 'gitlab' || provider === 'azure-devops' || provider === 'gitea' ? provider : 'none',
      truncated
    })
  }, [fetchReport, provider, setBody])

  return available ? insert : undefined
}
