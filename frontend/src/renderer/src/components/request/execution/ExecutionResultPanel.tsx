/**
 * ExecutionResultPanel — FE-REQ-TASK-036-08
 *
 * Structured result of a task run (CR-REQ-029). Everything is plain text. Older
 * tasks without a record fall back to the legacy output; unsupported runtimes
 * never reach this panel (TaskDetail hides the tab).
 *
 * @module components/request/execution/ExecutionResultPanel
 */

import React, { useEffect, useState } from 'react'
import { FileWarning } from 'lucide-react'
import { translate } from '@/i18n/i18n'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from '@/components/ui/sheet'
import { Skeleton } from '@/components/ui/skeleton'
import { useExecutionResult, type ExecutionResultStatus } from '../../../hooks/useExecutionResult'
import { openRequestPage } from '../request-page-navigation'
import { ExecutionChecksTable } from './ExecutionChecksTable'
import { classifyFiles, compareChecks, describeFailureRoute, secretScanState } from './execution-result-comparison'
import type { ExecutionResult } from '../../../../../shared/request-artifact-types'

const T = 'auto.components.request.execution.'
export const OUTPUT_PREVIEW_MAX = 16 * 1024
const SKELETON_DELAY_MS = 200

type Execution = { result: ExecutionResult | null; status: ExecutionResultStatus }
type Props = { taskId: string; requestId?: string | null; execution?: Execution; legacyOutput?: string }

const STATUS_TONE: Record<string, string> = {
  done: 'text-status-success border-status-success-border',
  failed: 'text-destructive border-destructive/40',
  blocked: 'text-muted-foreground border-border',
  needs_info: 'text-muted-foreground border-border'
}

export function ExecutionResultPanel({ taskId, requestId = null, execution, legacyOutput }: Props): React.JSX.Element | null {
  const own = useExecutionResult(execution ? null : taskId, requestId)
  const { result, status } = execution ?? own
  const [showSkeleton, setShowSkeleton] = useState(false)
  const [outputKey, setOutputKey] = useState<string | null>(null)

  useEffect(() => {
    if (status !== 'loading') {
      setShowSkeleton(false)
      return
    }
    const t = setTimeout(() => setShowSkeleton(true), SKELETON_DELAY_MS)
    return () => clearTimeout(t)
  }, [status])

  if (status === 'unsupported') {return null}
  if (status === 'idle' || status === 'loading') {
    return showSkeleton ? <Skeleton className="h-24 w-full" /> : null
  }
  if (status === 'error') {
    return <p role="alert" className="p-2 text-xs text-destructive">{translate(`${T}error`, 'Could not load the result')}</p>
  }
  if (status === 'legacy' || !result) {
    return (
      <div className="flex flex-col gap-1 p-2" data-testid="execution-legacy">
        <p className="text-xs text-muted-foreground">{translate(`${T}legacy`, 'This result has no structure')}</p>
        <pre className="max-h-64 overflow-auto whitespace-pre-wrap rounded-md border border-border bg-muted p-2 text-xs scrollbar-sleek">
          {legacyOutput ?? translate(`${T}noOutput`, 'No output')}
        </pre>
      </div>
    )
  }
  if (result.parseStatus !== 'ok') {
    return (
      <div className="flex flex-col gap-1 p-2" data-testid="execution-malformed">
        <p role="alert" className="text-xs text-destructive">{translate(`${T}malformed`, 'The result is not in the expected format')}</p>
        {result.stdoutTail ? <pre className="max-h-64 overflow-auto whitespace-pre-wrap rounded-md border border-border bg-muted p-2 text-xs scrollbar-sleek">{result.stdoutTail}</pre> : null}
      </div>
    )
  }

  const files = classifyFiles(result.filesChanged, result.verdict)
  const checks = compareChecks(result.checksRun, result.verdict)
  const scan = secretScanState(result.verdict)
  const route = result.failureClass && result.failureClass !== 'unknown' ? describeFailureRoute(result.failureClass) : null
  const outputs = Object.entries(result.outputs ?? {})
  const preview = outputKey ? JSON.stringify(result.outputs?.[outputKey], null, 2) ?? '' : ''

  return (
    <div className="flex flex-col gap-3 p-2" data-testid="execution-result-panel">
      <div className="flex flex-wrap items-center gap-2">
        {result.status ? <Badge variant="outline" className={STATUS_TONE[result.status]}>{translate(`${T}status.${result.status}`, result.status)}</Badge> : null}
        <span className="text-xs text-muted-foreground">{translate(`${T}attempt`, 'Attempt {{n}}', { n: result.attempt })}</span>
      </div>
      {result.summary ? <p className="whitespace-pre-wrap text-sm">{result.summary}</p> : null}

      <section aria-label={translate(`${T}files.title`, 'Changed files')}>
        <h4 className="text-xs font-medium">{translate(`${T}files.title`, 'Changed files')}</h4>
        {files.length === 0 ? <p className="text-xs text-muted-foreground">{translate(`${T}files.none`, 'No files changed')}</p> : (
          <ul className="mt-1 flex flex-col gap-0.5">
            {files.map((f) => (
              <li key={f.path} className="flex items-center gap-1 text-xs" data-in-scope={f.inScope}>
                {!f.inScope ? <FileWarning className="size-3 text-risk-high" aria-hidden /> : null}
                <span className="break-all font-mono">{f.path}</span>
                <span className={f.inScope ? 'text-muted-foreground' : 'font-medium text-risk-high'}>
                  {f.inScope ? translate(`${T}files.inScope`, 'In scope') : translate(`${T}files.outOfScope`, 'Out of scope')}
                </span>
              </li>
            ))}
          </ul>
        )}
      </section>

      <ExecutionChecksTable rows={checks} />

      <p className="text-xs" data-testid="execution-secret-scan" data-state={scan}>
        {translate(`${T}secrets.label`, 'Secret scan')}: {translate(`${T}secrets.${scan}`, scan)}
      </p>

      {outputs.length > 0 ? (
        <section aria-label={translate(`${T}outputs.title`, 'Outputs')}>
          <h4 className="text-xs font-medium">{translate(`${T}outputs.title`, 'Outputs')}</h4>
          <ul className="mt-1 flex flex-col gap-0.5">
            {outputs.map(([name, value]) => (
              <li key={name} className="flex items-center gap-2 text-xs">
                <span className="font-mono">{name}</span>
                <span className="text-muted-foreground">{Array.isArray(value) ? 'array' : typeof value}</span>
                <Button size="xs" variant="ghost" onClick={() => setOutputKey(name)}>{translate(`${T}outputs.view`, 'View')}</Button>
              </li>
            ))}
          </ul>
        </section>
      ) : null}

      {route ? <p className="text-xs text-muted-foreground" data-testid="execution-failure-route">{translate(route.messageKey, route.fallback, route.params)}</p> : null}
      {result.status === 'needs_info' && requestId ? (
        <Button size="sm" variant="outline" className="self-start" onClick={() => openRequestPage({ section: 'requests', requestId })}>
          {translate(`${T}toQuestion`, 'Answer the question')}
        </Button>
      ) : null}

      <Sheet open={outputKey !== null} onOpenChange={(o) => { if (!o) {setOutputKey(null)} }}>
        <SheetContent side="right" className="gap-3 p-4 sm:max-w-xl">
          <SheetHeader className="p-0">
            <SheetTitle>{outputKey}</SheetTitle>
            <SheetDescription>{translate(`${T}outputs.preview`, 'Shown as plain text, cut at 16 KB')}</SheetDescription>
          </SheetHeader>
          <pre className="max-h-[70vh] overflow-auto whitespace-pre-wrap rounded-md border border-border bg-muted p-2 text-xs scrollbar-sleek">{preview.slice(0, OUTPUT_PREVIEW_MAX)}</pre>
        </SheetContent>
      </Sheet>
    </div>
  )
}
