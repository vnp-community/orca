/**
 * QualityCoveragePanel.tsx — FE-CV-TASK-087-16
 *
 * Diff coverage gauge, per-file coverage treemap and the uncovered line list. Mounted by the
 * quality lens only while its block is open. Missing data is an empty state with its reason,
 * never 0%; `estimated` is told apart from `measured` in words, not only by style.
 *
 * @module components/review-map/quality/QualityCoveragePanel
 */

import { useAppStore } from '@/store'
import { useQualityCoverage } from '@/hooks/useQualityCoverage'
import { useQualityProfiles } from '@/hooks/useQualityProfiles'
import { Button } from '../../ui/button'
import { DiffCoverageGauge } from '../../quality-charts/DiffCoverageGauge'
import { MetricTreemap } from '../../quality-charts/MetricTreemap'
import { chartCopy } from '../../quality-charts/quality-chart-copy'
import type { CoverageReport } from '../../../../../shared/code-intel-quality-visualization-types'
import { toPercent } from './coverage-percent-normalization'
import { describeQualityBlockError } from './quality-block-error-message'
import { QualityBlockStateFrame } from './QualityBlockStateFrame'
import { buildCoverageTreemapItems } from './quality-coverage-treemap-items'
import { QualityCoverageUncoveredList } from './QualityCoverageUncoveredList'
import type { QualityBlockProps } from './quality-lens-block-types'
import { qv } from './quality-visualization-copy'

function SourceNotice({ report }: { report: CoverageReport }): React.JSX.Element {
  const estimated = report.source === 'estimated'
  const pct = toPercent(report.totals.pct)
  return (
    <div
      className="flex flex-col gap-1 text-xs text-foreground"
      data-coverage-source={estimated ? 'estimated' : 'measured'}
    >
      <p className={estimated ? 'font-medium' : undefined}>
        {estimated ? qv('coverage.estimatedBanner') : qv('coverage.measuredLine')}
        {estimated && report.estimatedNote ? ` ${report.estimatedNote}` : ''}
      </p>
      {pct !== null && typeof report.totals.stmts === 'number' ? (
        <p className="text-muted-foreground">
          {qv('coverage.totals', { percent: pct, stmts: report.totals.stmts })}
        </p>
      ) : null}
      {report.dirty ? <p className="text-muted-foreground">{qv('coverage.dirty')}</p> : null}
    </div>
  )
}

function DiffGaugeBlock({
  report,
  warn,
  fail
}: {
  report: CoverageReport
  warn: number | null
  fail: number | null
}): React.JSX.Element {
  const diff = report.diff
  const source = report.source === 'estimated' ? 'estimated' : 'measured'
  if (!diff || diff.changedExecutable <= 0) {
    // Why: no data is an explained empty state; a gauge here would read as 0%.
    const reason = !diff
      ? qv('coverage.noDiff')
      : diff.reason
        ? qv('coverage.noDiffReason', { reason: diff.reason })
        : qv('coverage.noChangedLines')
    return (
      <QualityBlockStateFrame
        id="quality-coverage-diff"
        title={qv('coverage.diffTitle')}
        status="empty"
        emptyReason={reason}
      />
    )
  }
  return (
    <div className="flex flex-col gap-2">
      <DiffCoverageGauge
        frame={{ id: 'quality-coverage-diff', title: qv('coverage.diffTitle') }}
        covered={diff.covered}
        total={diff.changedExecutable}
        thresholds={{ warnBelow: warn, failBelow: fail }}
        source={source}
        partial={diff.partial}
      />
      {diff.partial ? (
        <p className="text-xs text-foreground" data-coverage-partial>
          {qv('coverage.partial')}
        </p>
      ) : null}
      {diff.excludedFiles.length > 0 ? (
        <details className="text-xs" data-coverage-excluded>
          <summary className="cursor-pointer text-foreground">
            {qv('coverage.excluded', { count: diff.excludedFiles.length })}
          </summary>
          <ul className="mt-1 flex flex-col gap-0.5 text-muted-foreground">
            {diff.excludedFiles.map((f) => (
              <li key={f.path}>
                {qv('coverage.excludedItem', { path: f.path, reason: f.reason })}
              </li>
            ))}
          </ul>
        </details>
      ) : null}
    </div>
  )
}

export function QualityCoveragePanel({
  worktreeId,
  onOpenDiff
}: QualityBlockProps): React.JSX.Element | null {
  const coverage = useQualityCoverage(worktreeId)
  const profiles = useQualityProfiles(worktreeId)
  if (coverage.support === 'disabled' || coverage.support === 'unsupported') {
    return null
  }
  const frameBase = {
    id: 'quality-coverage',
    title: qv('coverage.title'),
    description: qv('coverage.description')
  }
  const { status, report } = coverage

  if (status === 'idle' || status === 'loading') {
    return <QualityBlockStateFrame {...frameBase} status="loading" />
  }
  if (status === 'error') {
    return (
      <QualityBlockStateFrame
        {...frameBase}
        status="error"
        errorMessage={describeQualityBlockError(coverage.error)}
        onRetry={coverage.refetch}
      />
    )
  }
  if (status === 'unavailable') {
    return (
      <QualityBlockStateFrame {...frameBase} status="empty" emptyReason={qv('block.unavailable')} />
    )
  }
  if (!report) {
    const profileCoverage = profiles.profile?.definition.coverage
    const canRun = Boolean(profileCoverage?.required && profiles.selected)
    return (
      <QualityBlockStateFrame
        {...frameBase}
        status="empty"
        emptyReason={
          <span className="flex flex-col items-start gap-2">
            <span>
              {coverage.reason
                ? qv('coverage.noReportReason', { reason: coverage.reason })
                : qv('coverage.noReport')}
            </span>
            {canRun ? (
              <Button
                type="button"
                variant="outline"
                size="sm"
                onClick={() =>
                  void useAppStore
                    .getState()
                    .startQualityRun(worktreeId, { profile: profiles.selected!, scope: 'changed' })
                }
              >
                {qv('coverage.runCheck')}
              </Button>
            ) : null}
          </span>
        }
      />
    )
  }

  const profileCoverage = profiles.profile?.definition.coverage
  const treemap = buildCoverageTreemapItems(report.files)
  const total = Math.max(report.totalCount, report.files.length)
  const cut = report.truncated || treemap.hidden > 0
  return (
    <section
      aria-label={frameBase.title}
      className="flex flex-col gap-3"
      data-quality-coverage
      data-status={status}
    >
      <h3 className="text-sm font-medium">{frameBase.title}</h3>
      {status === 'stale' ? (
        <p className="text-xs text-muted-foreground" data-chart-stale>
          {qv('block.stale')}
        </p>
      ) : null}
      <SourceNotice report={report} />
      <DiffGaugeBlock
        report={report}
        warn={toPercent(profileCoverage?.diffCoverageWarnBelow)}
        fail={toPercent(profileCoverage?.diffCoverageFailBelow)}
      />
      <MetricTreemap
        frame={{
          id: 'quality-coverage-treemap',
          title: qv('coverage.treemapTitle'),
          emptyReason: qv('coverage.noReport'),
          legend: (
            <>
              {cut ? (
                <p className="text-xs text-muted-foreground">
                  {chartCopy('frame.showing', {
                    shown: treemap.items.length,
                    total: Math.max(total, treemap.drawable)
                  })}
                </p>
              ) : null}
              {treemap.withoutStatements > 0 ? (
                <p className="text-xs text-muted-foreground">
                  {qv('coverage.noStatements', { count: treemap.withoutStatements })}
                </p>
              ) : null}
            </>
          )
        }}
        items={treemap.items}
        sizeLabel={qv('coverage.sizeLabel')}
        intensityLabel={qv('coverage.intensityLabel')}
        onSelect={(path) => onOpenDiff(path)}
      />
      <QualityCoverageUncoveredList files={report.files} onOpenDiff={onOpenDiff} />
    </section>
  )
}

export default QualityCoveragePanel
