/**
 * TestScriptOutputSheet — what the Test Script Agent wrote for a Request, read from its worktree on the dev server
 * (docs/integrate/TDD-REQ-TESTSCRIPT-001): the generated files, the coverage of the spec's expected results, the agent's report.
 *
 * Files and the manifest come from `request.testScriptFile`, which only serves the paths the job reported; the report from
 * `request.testScriptReport`. A location in the coverage table opens the file at the assertion's line.
 *
 * @module components/request/execution/TestScriptOutputSheet
 */

import React, { useMemo, useState } from 'react'
import { translate } from '@/i18n/i18n'
import { Badge } from '@/components/ui/badge'
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle
} from '@/components/ui/sheet'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { RequestMarkdownContent } from '../solution/RequestMarkdownContent'
import { useTestScriptText } from '../../../hooks/useTestScriptText'
import { parseTestScriptManifest } from '../../../../../shared/request-testscript-parsers'
import type { TestScriptRun } from '../../../../../shared/request-testscript-types'
import { TestScriptCodeView, TestScriptCoverageTable, TextStateNote } from './TestScriptOutputViews'

const T = 'auto.components.request.execution.TestScriptOutput.'

type Tab = 'files' | 'coverage' | 'report'

function fileStatusVariant(status: string): 'secondary' | 'destructive' | 'outline' {
  if (status === 'generated') {
    return 'secondary'
  }
  return status === 'failed' ? 'destructive' : 'outline'
}

function FilesTab({
  requestId,
  run,
  selected,
  focusLine,
  onSelect
}: {
  requestId: string
  run: TestScriptRun
  selected: string | null
  focusLine?: number
  onSelect: (path: string) => void
}): React.JSX.Element {
  const files = run.summary.files
  const state = useTestScriptText(requestId, selected ? { kind: 'file', path: selected } : null)
  if (files.length === 0) {
    return (
      <p className="text-xs text-muted-foreground">
        {translate(`${T}noFiles`, 'The job reported no files.')}
      </p>
    )
  }
  return (
    <div className="flex flex-col gap-3">
      <ul className="flex flex-col gap-1" data-testid="testscript-files">
        {files.map((f) => (
          <li key={f.path}>
            <button
              type="button"
              onClick={() => onSelect(f.path)}
              aria-pressed={selected === f.path}
              className={[
                'flex w-full items-center justify-between gap-2 rounded-md border px-2 py-1 text-left text-xs',
                selected === f.path ? 'border-primary bg-primary/5' : 'hover:bg-muted/50'
              ].join(' ')}
            >
              <code className="truncate">{f.path}</code>
              <span className="flex shrink-0 items-center gap-2 text-muted-foreground">
                {translate(`${T}cases`, '{{n}} cases', { n: f.cases.length })}
                <Badge variant={fileStatusVariant(f.status)}>{f.status}</Badge>
              </span>
            </button>
          </li>
        ))}
      </ul>
      {selected ? (
        <div className="flex flex-col gap-2">
          <TextStateNote state={state} />
          {state.text && <TestScriptCodeView text={state.text} focusLine={focusLine} />}
        </div>
      ) : (
        <p className="text-xs text-muted-foreground">
          {translate(`${T}pickFile`, 'Pick a file to read it.')}
        </p>
      )}
    </div>
  )
}

function CoverageTab({
  requestId,
  run,
  onOpen
}: {
  requestId: string
  run: TestScriptRun
  onOpen: (file: string, line?: number) => void
}): React.JSX.Element {
  const manifestPath = run.summary.manifest
  const state = useTestScriptText(
    requestId,
    manifestPath ? { kind: 'file', path: manifestPath } : null
  )
  const manifest = useMemo(() => parseTestScriptManifest(state.text), [state.text])
  if (!manifestPath) {
    return (
      <p className="text-xs text-muted-foreground">
        {translate(`${T}noManifest`, 'The job wrote no manifest.')}
      </p>
    )
  }
  return (
    <div className="flex flex-col gap-2">
      <TextStateNote state={state} />
      {state.text && <TestScriptCoverageTable rows={manifest.rows} onOpen={onOpen} />}
    </div>
  )
}

function ReportTab({ requestId }: { requestId: string }): React.JSX.Element {
  const state = useTestScriptText(requestId, { kind: 'report' })
  return (
    <div className="flex flex-col gap-2" data-testid="testscript-report">
      <TextStateNote state={state} />
      {state.text && <RequestMarkdownContent content={state.text} gfm />}
    </div>
  )
}

export function TestScriptOutputSheet({
  requestId,
  run,
  initialFile,
  open,
  onOpenChange
}: {
  requestId: string
  run: TestScriptRun
  /** The file to show first; the first file of the job when unset. */
  initialFile?: string
  open: boolean
  onOpenChange: (open: boolean) => void
}): React.JSX.Element {
  const [tab, setTab] = useState<Tab>('files')
  const [selected, setSelected] = useState<string | null>(
    initialFile ?? run.summary.files[0]?.path ?? null
  )
  const [focusLine, setFocusLine] = useState<number | undefined>(undefined)

  const openAt = (file: string, line?: number): void => {
    setSelected(file)
    setFocusLine(line)
    setTab('files')
  }

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent
        className="w-[900px] max-w-full overflow-auto sm:max-w-[900px]"
        data-testid="testscript-output"
      >
        <SheetHeader>
          <SheetTitle>{translate(`${T}title`, 'Test scripts')}</SheetTitle>
          <SheetDescription>
            <code>{run.branch}</code>
            {run.commitSha && ` · ${run.commitSha.slice(0, 8)}`}
            {run.worktreePath && (
              <span className="block break-all text-xs">
                {translate(`${T}worktree`, 'Worktree on the dev server:')}{' '}
                <code>{run.worktreePath}</code>
              </span>
            )}
          </SheetDescription>
        </SheetHeader>
        <Tabs value={tab} onValueChange={(v) => setTab(v as Tab)} className="px-4 pb-4">
          <TabsList>
            <TabsTrigger value="files">{translate(`${T}tab.files`, 'Files')}</TabsTrigger>
            <TabsTrigger value="coverage">{translate(`${T}tab.coverage`, 'Coverage')}</TabsTrigger>
            <TabsTrigger value="report">{translate(`${T}tab.report`, 'Report')}</TabsTrigger>
          </TabsList>
          <TabsContent value="files">
            <FilesTab
              requestId={requestId}
              run={run}
              selected={selected}
              focusLine={focusLine}
              onSelect={(p) => {
                setSelected(p)
                setFocusLine(undefined)
              }}
            />
          </TabsContent>
          <TabsContent value="coverage">
            <CoverageTab requestId={requestId} run={run} onOpen={openAt} />
          </TabsContent>
          <TabsContent value="report">
            <ReportTab requestId={requestId} />
          </TabsContent>
        </Tabs>
      </SheetContent>
    </Sheet>
  )
}
