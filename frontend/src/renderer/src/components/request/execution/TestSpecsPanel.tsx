/**
 * TestSpecsPanel — the test-spec lane of a Request, shown next to the code run in the Execution tab.
 *
 * tdd-agent writes test specs from the Solution (or the Plan bundle) while the code is built. The lane is advisory: it never
 * blocks the Request, so a failed or blocked run only says why and offers a Re-run. Hidden when the deployment has the lane
 * off or never ran it for this Request.
 *
 * @module components/request/execution/TestSpecsPanel
 */

import React, { useCallback, useState } from 'react'
import { Check, CircleDashed, Loader2, OctagonX, RotateCw, SkipForward } from 'lucide-react'
import { translate } from '@/i18n/i18n'
import { Button } from '@/components/ui/button'
import { callRequestRpc } from '../../../runtime/request-rpc-client'
import { REQUEST_RPC_METHODS } from '../../../../../shared/request-rpc-methods'
import { parseTestSpecDocument } from '../../../../../shared/request-testspec-parsers'
import { requestErrorMessage } from '../request-error-message'
import type { UseTestSpecsResult } from '../../../hooks/useTestSpecs'
import type {
  TestSpecDocumentName,
  TestSpecRun,
  TestSpecStatus
} from '../../../../../shared/request-testspec-types'

const T = 'auto.components.request.execution.TestSpecsPanel.'

const DOCUMENTS: { name: TestSpecDocumentName; label: string }[] = [
  { name: 'requirements', label: 'Requirements' },
  { name: 'design', label: 'Design' },
  { name: 'cases', label: 'Cases' }
]

function statusLabel(status: TestSpecStatus): string {
  switch (status) {
    case 'pending':
      return translate(`${T}pending`, 'Waiting to start')
    case 'starting':
      return translate(`${T}starting`, 'Starting on the dev server')
    case 'running':
      return translate(`${T}running`, 'Generating')
    case 'completed':
      return translate(`${T}completed`, 'Generated')
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

function StatusIcon({ status }: { status: TestSpecStatus }): React.JSX.Element {
  switch (status) {
    case 'completed':
      return <Check className="size-4 text-[color:var(--status-success)]" aria-hidden />
    case 'pending':
    case 'starting':
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

function modeLabel(mode: string | undefined): string | null {
  if (mode === 'solution') {
    return translate(`${T}fromSolution`, 'from the Solution')
  }
  if (mode === 'plan') {
    return translate(`${T}fromPlan`, 'from the Plan bundle')
  }
  return null
}

function DocumentViewer({
  requestId,
  name
}: {
  requestId: string
  name: TestSpecDocumentName
}): React.JSX.Element {
  const [text, setText] = useState('')
  const [next, setNext] = useState<number | null>(0)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const loadMore = useCallback(() => {
    if (next === null) {
      return
    }
    setLoading(true)
    void callRequestRpc<unknown>(REQUEST_RPC_METHODS.TEST_SPEC_DOCUMENT, {
      id: requestId,
      document: name,
      offset: next
    }).then((res) => {
      setLoading(false)
      if (!res.ok) {
        setError(res.error.kind)
        return
      }
      const page = parseTestSpecDocument(res.value)
      setError(null)
      setText((t) => t + page.text)
      setNext(page.hasNext ? page.next : null)
    })
  }, [requestId, name, next])

  React.useEffect(() => {
    loadMore()
    // Loads the first page once; later pages come from the button.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  return (
    <div className="flex flex-col gap-2" data-testid={`testspec-doc-${name}`}>
      {error && (
        <p role="alert" className="text-xs text-destructive">
          {requestErrorMessage(error)}
        </p>
      )}
      <pre className="max-h-80 overflow-auto whitespace-pre-wrap rounded-md border bg-muted/30 p-2 text-xs">
        {text || (loading ? '…' : '')}
      </pre>
      {next !== null && text !== '' && (
        <Button size="xs" variant="outline" disabled={loading} onClick={loadMore}>
          {translate(`${T}loadMore`, 'Load more')}
        </Button>
      )}
    </div>
  )
}

function RunSummary({
  run,
  requestId
}: {
  run: TestSpecRun
  requestId: string
}): React.JSX.Element {
  const [doc, setDoc] = useState<TestSpecDocumentName | null>(null)
  const { counts, gaps, notes } = run.summary
  return (
    <div className="flex flex-col gap-2">
      <dl className="flex flex-wrap gap-x-4 gap-y-1 text-xs" data-testid="testspec-counts">
        <div>
          <dt className="inline text-muted-foreground">
            {translate(`${T}requirements`, 'Requirements')}{' '}
          </dt>
          <dd className="inline font-medium">{counts.requirements}</dd>
        </div>
        <div>
          <dt className="inline text-muted-foreground">{translate(`${T}cases`, 'Cases')} </dt>
          <dd className="inline font-medium">{counts.cases}</dd>
        </div>
        <div>
          <dt className="inline text-muted-foreground">
            {translate(`${T}gaps`, 'Open questions')}{' '}
          </dt>
          <dd className="inline font-medium">{counts.gaps}</dd>
        </div>
      </dl>
      {gaps.length > 0 && (
        <ul className="flex list-disc flex-col gap-1 pl-5 text-xs" data-testid="testspec-gaps">
          {gaps.map((g, i) => (
            <li key={`${g.code}-${i}`}>
              {g.question || g.missing}
              {g.owner && <span className="text-muted-foreground"> · {g.owner}</span>}
            </li>
          ))}
        </ul>
      )}
      {notes.length > 0 && (
        <ul className="list-disc pl-5 text-xs text-muted-foreground">
          {notes.map((n, i) => (
            <li key={i}>{n}</li>
          ))}
        </ul>
      )}
      <div className="flex flex-wrap gap-2">
        {DOCUMENTS.map((d) => (
          <Button
            key={d.name}
            size="xs"
            variant={doc === d.name ? 'secondary' : 'outline'}
            onClick={() => setDoc(doc === d.name ? null : d.name)}
          >
            {translate(`${T}doc.${d.name}`, d.label)}
          </Button>
        ))}
      </div>
      {doc && <DocumentViewer key={`${run.id}-${doc}`} requestId={requestId} name={doc} />}
    </div>
  )
}

export type TestSpecsPanelProps = {
  requestId: string
  testSpecs: UseTestSpecsResult
}

export function TestSpecsPanel({
  requestId,
  testSpecs
}: TestSpecsPanelProps): React.JSX.Element | null {
  const { testSpecs: data, error, restarting, restartError, restart, unsupported } = testSpecs
  if (unsupported || !data?.enabled) {
    return null
  }
  const run = data.current
  const active = run ? ['pending', 'starting', 'running'].includes(run.status) : false
  const mode = modeLabel(run?.contextMode)

  return (
    <section
      className="flex flex-col gap-2 rounded-md border p-3"
      aria-label={translate(`${T}title`, 'Test specs')}
      data-testid="testspec-panel"
    >
      <header className="flex items-center justify-between gap-2">
        <div className="flex items-center gap-2 text-sm font-medium">
          {run && <StatusIcon status={run.status} />}
          <span>{translate(`${T}title`, 'Test specs')}</span>
          {run && (
            <span
              className="text-xs font-normal text-muted-foreground"
              data-testid="testspec-status"
            >
              {statusLabel(run.status)}
              {run.attempt > 1 &&
                ` · ${translate(`${T}attempt`, 'attempt {{n}}', { n: run.attempt })}`}
              {mode && ` · ${mode}`}
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
          'Generated next to the code by tdd-agent. Advisory: it never blocks this Request.'
        )}
      </p>
      {!run && (
        <p className="text-xs text-muted-foreground" data-testid="testspec-none">
          {translate(`${T}none`, 'Starts when the Request begins executing.')}
        </p>
      )}
      {(error || restartError) && (
        <p role="alert" className="text-xs text-destructive">
          {requestErrorMessage((restartError ?? error) as string)}
        </p>
      )}
      {run && ['blocked', 'failed', 'skipped', 'cancelled'].includes(run.status) && (
        <p className="text-xs" data-testid="testspec-reason">
          {run.reasonMessage || run.reasonCode}
          {run.reasonMessage && run.reasonCode && (
            <span className="text-muted-foreground"> ({run.reasonCode})</span>
          )}
        </p>
      )}
      {run?.status === 'completed' && <RunSummary run={run} requestId={requestId} />}
    </section>
  )
}
