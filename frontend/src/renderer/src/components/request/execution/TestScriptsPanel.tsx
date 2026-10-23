/**
 * TestScriptsPanel — the test-script lane of a Request, shown under the test specs in the Execution tab.
 *
 * The Test Script Agent writes pytest scripts from the TCIR of the test specs into a git worktree of its own once execution may
 * start. The lane is advisory: it never blocks the Request and the scripts are not run here, so the panel says where they are
 * (branch, commit), how many cases passed the spec check, and opens the output: the generated files, the coverage of the spec's
 * expected results and the agent's report (TestScriptOutputSheet). Hidden when the deployment has the lane off.
 *
 * @module components/request/execution/TestScriptsPanel
 */

import React, { useState } from 'react'
import {
  Check,
  FileCode,
  CircleDashed,
  Loader2,
  OctagonX,
  RotateCw,
  SkipForward,
  TriangleAlert
} from 'lucide-react'
import { translate } from '@/i18n/i18n'
import { Button } from '@/components/ui/button'
import { TestScriptOutputSheet } from './TestScriptOutputSheet'
import { requestErrorMessage } from '../request-error-message'
import type { UseTestScriptsResult } from '../../../hooks/useTestScripts'
import type {
  TestScriptRun,
  TestScriptStatus
} from '../../../../../shared/request-testscript-types'

const T = 'auto.components.request.execution.TestScriptsPanel.'

function statusLabel(status: TestScriptStatus): string {
  switch (status) {
    case 'pending':
      return translate(`${T}pending`, 'Waiting to start')
    case 'running':
      return translate(`${T}running`, 'Generating')
    case 'completed':
      return translate(`${T}completed`, 'Generated')
    case 'needs_review':
      return translate(`${T}needsReview`, 'Generated, needs review')
    case 'blocked':
      return translate(`${T}blocked`, 'Blocked')
    case 'failed':
      return translate(`${T}failed`, 'Failed')
    case 'skipped':
      return translate(`${T}skipped`, 'Skipped')
    case 'cancelled':
      return translate(`${T}cancelled`, 'Cancelled')
    default:
      return translate(`${T}unknown`, 'Unknown')
  }
}

function StatusIcon({ status }: { status: TestScriptStatus }): React.JSX.Element {
  switch (status) {
    case 'completed':
      return <Check className="size-4 text-[color:var(--status-success)]" aria-hidden />
    case 'needs_review':
      return <TriangleAlert className="size-4 text-quality-warning" aria-hidden />
    case 'pending':
    case 'running':
      return <Loader2 className="size-4 animate-spin text-primary" aria-hidden />
    case 'failed':
    case 'blocked':
      return <OctagonX className="size-4 text-destructive" aria-hidden />
    case 'skipped':
    case 'cancelled':
      return <SkipForward className="size-4 text-muted-foreground" aria-hidden />
    default:
      return <CircleDashed className="size-4 text-muted-foreground" aria-hidden />
  }
}

function Count({ label, value }: { label: string; value: number }): React.JSX.Element {
  return (
    <div>
      <dt className="inline text-muted-foreground">{label} </dt>
      <dd className="inline font-medium">{value}</dd>
    </div>
  )
}

function RunResult({
  run,
  requestId
}: {
  run: TestScriptRun
  requestId: string
}): React.JSX.Element {
  // undefined: sheet closed; null: open on the first file; a path: open on that file.
  const [openOn, setOpenOn] = useState<string | null | undefined>(undefined)
  const { counts, files } = run.summary
  const flagged = files.filter((f) => f.status !== 'generated')
  return (
    <div className="flex flex-col gap-2">
      <dl className="flex flex-wrap gap-x-4 gap-y-1 text-xs" data-testid="testscript-counts">
        <Count label={translate(`${T}generated`, 'Generated')} value={counts.generated} />
        <Count
          label={translate(`${T}needsReviewCount`, 'Needs review')}
          value={counts.needsReview + counts.failed}
        />
        <Count label={translate(`${T}skippedCount`, 'Skipped')} value={counts.skipped} />
        <Count label={translate(`${T}files`, 'Files')} value={counts.files} />
      </dl>
      <p className="text-xs" data-testid="testscript-where">
        <span className="text-muted-foreground">{translate(`${T}branch`, 'Branch')} </span>
        <code>{run.branch}</code>
        {run.commitSha && (
          <>
            <span className="text-muted-foreground"> · {translate(`${T}commit`, 'commit')} </span>
            <code>{run.commitSha.slice(0, 8)}</code>
          </>
        )}
      </p>
      {flagged.length > 0 && (
        <ul className="flex list-disc flex-col gap-1 pl-5 text-xs" data-testid="testscript-flagged">
          {flagged.map((f) => (
            <li key={f.path}>
              <button
                type="button"
                className="text-left text-primary underline-offset-2 hover:underline"
                onClick={() => setOpenOn(f.path)}
              >
                <code>{f.path}</code>
              </button>
              <span className="text-muted-foreground">
                {' '}
                · {f.status} · {translate(`${T}errors`, '{{n}} problems', { n: f.errors })}
              </span>
            </li>
          ))}
        </ul>
      )}
      <div className="flex flex-wrap gap-2">
        <Button size="xs" variant="outline" onClick={() => setOpenOn(null)}>
          <FileCode className="mr-1 size-3" aria-hidden />
          {translate(`${T}openOutput`, 'Open output')}
        </Button>
      </div>
      {openOn !== undefined && (
        <TestScriptOutputSheet
          key={openOn ?? ''}
          requestId={requestId}
          run={run}
          initialFile={openOn ?? undefined}
          open
          onOpenChange={(next) => {
            if (!next) {
              setOpenOn(undefined)
            }
          }}
        />
      )}
    </div>
  )
}

export type TestScriptsPanelProps = {
  requestId: string
  testScripts: UseTestScriptsResult
}

export function TestScriptsPanel({
  requestId,
  testScripts
}: TestScriptsPanelProps): React.JSX.Element | null {
  const { testScripts: data, error, restarting, restartError, restart, unsupported } = testScripts
  if (unsupported || !data?.enabled) {
    return null
  }
  const run = data.current
  const active = run ? run.status === 'pending' || run.status === 'running' : false

  return (
    <section
      className="flex flex-col gap-2 rounded-md border p-3"
      aria-label={translate(`${T}title`, 'Test scripts')}
      data-testid="testscript-panel"
    >
      <header className="flex items-center justify-between gap-2">
        <div className="flex items-center gap-2 text-sm font-medium">
          {run && <StatusIcon status={run.status} />}
          <span>{translate(`${T}title`, 'Test scripts')}</span>
          {run && (
            <span
              className="text-xs font-normal text-muted-foreground"
              data-testid="testscript-status"
            >
              {statusLabel(run.status)}
              {run.feature && ` · ${run.feature}${run.specVersion ? ` v${run.specVersion}` : ''}`}
              {run.attempt > 1 &&
                ` · ${translate(`${T}attempt`, 'attempt {{n}}', { n: run.attempt })}`}
            </span>
          )}
        </div>
        {run && !active && (
          <Button size="xs" variant="outline" disabled={restarting} onClick={restart}>
            <RotateCw className="mr-1 size-3" aria-hidden />
            {translate(`${T}rerun`, 'Re-run')}
          </Button>
        )}
      </header>
      <p className="text-xs text-muted-foreground">
        {translate(
          `${T}advisory`,
          'Written from the test specs by the Test Script Agent in a worktree of its own, beside the code. Not run here; it never changes the Request status.'
        )}
      </p>
      {!run && (
        <p className="text-xs text-muted-foreground" data-testid="testscript-none">
          {translate(`${T}none`, 'Starts when execution starts and the test specs are ready.')}
        </p>
      )}
      {(error || restartError) && (
        <p role="alert" className="text-xs text-destructive">
          {requestErrorMessage((restartError ?? error) as string)}
        </p>
      )}
      {run &&
        ['blocked', 'failed', 'skipped', 'cancelled', 'needs_review'].includes(run.status) &&
        (run.reasonMessage || run.reasonCode) && (
          <p className="text-xs" data-testid="testscript-reason">
            {run.reasonMessage || run.reasonCode}
            {run.reasonMessage && run.reasonCode && (
              <span className="text-muted-foreground"> ({run.reasonCode})</span>
            )}
          </p>
        )}
      {run && (run.status === 'completed' || run.status === 'needs_review') && (
        <RunResult run={run} requestId={requestId} />
      )}
    </section>
  )
}
