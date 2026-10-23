/**
 * Views of a test-script job's output: one generated file with line numbers (the assertions tied to the spec, `# @exp`, are
 * marked), and the coverage table read from assertion-manifest.json (expected result -> assertion). Read only.
 *
 * @module components/request/execution/TestScriptOutputViews
 */

import React, { useEffect, useRef } from 'react'
import { translate } from '@/i18n/i18n'
import { Badge } from '@/components/ui/badge'
import { requestErrorMessage } from '../request-error-message'
import type { TestScriptManifestRow } from '../../../../../shared/request-testscript-types'
import type { TestScriptTextState } from '../../../hooks/useTestScriptText'

const T = 'auto.components.request.execution.TestScriptOutput.'
const EXP_RE = /#\s*@exp\s+(\S+)/

export function TextStateNote({ state }: { state: TestScriptTextState }): React.JSX.Element | null {
  if (state.error) {
    return (
      <p role="alert" className="text-xs text-destructive">
        {requestErrorMessage(state.error)}
      </p>
    )
  }
  if (state.loading) {
    return <p className="text-xs text-muted-foreground">{translate(`${T}loading`, 'Loading…')}</p>
  }
  if (state.truncated) {
    return (
      <p className="text-xs text-muted-foreground">
        {translate(
          `${T}truncated`,
          'The file is longer than the viewer shows; open it in the worktree for the rest.'
        )}
      </p>
    )
  }
  return null
}

/** A generated file with line numbers; lines carrying `# @exp` are marked, `focusLine` is scrolled to and outlined. */
export function TestScriptCodeView({
  text,
  focusLine
}: {
  text: string
  focusLine?: number
}): React.JSX.Element {
  const focusRef = useRef<HTMLDivElement | null>(null)
  useEffect(() => {
    focusRef.current?.scrollIntoView?.({ block: 'center' })
  }, [focusLine, text])
  const lines = text.split('\n')
  return (
    <div
      className="max-h-[60vh] overflow-auto rounded-md border bg-muted/30 font-mono text-xs scrollbar-sleek"
      data-testid="testscript-code"
    >
      {lines.map((line, i) => {
        const n = i + 1
        const exp = EXP_RE.exec(line)?.[1]
        const focused = n === focusLine
        return (
          <div
            key={n}
            ref={focused ? focusRef : undefined}
            data-line={n}
            data-exp={exp}
            className={[
              'flex whitespace-pre',
              exp ? 'bg-primary/10' : '',
              focused ? 'outline outline-1 outline-primary' : ''
            ].join(' ')}
          >
            <span className="w-10 shrink-0 select-none pr-2 text-right text-muted-foreground">
              {n}
            </span>
            <span className="pr-2">{line}</span>
          </div>
        )
      })}
    </div>
  )
}

// gap is neither a pass nor a failure: the spec leaves the value open, so it stays outlined like other neutral states.
function statusVariant(status: string): 'secondary' | 'destructive' | 'outline' {
  if (status === 'ok') {
    return 'secondary'
  }
  return status === 'missing' || status === 'mismatch' ? 'destructive' : 'outline'
}

/** Expected results of the spec and where each is asserted; a location opens the file at that line. */
export function TestScriptCoverageTable({
  rows,
  onOpen
}: {
  rows: TestScriptManifestRow[]
  onOpen: (file: string, line?: number) => void
}): React.JSX.Element {
  if (rows.length === 0) {
    return (
      <p className="text-xs text-muted-foreground">
        {translate(`${T}noRows`, 'The manifest lists no expected results.')}
      </p>
    )
  }
  const ok = rows.filter((r) => r.status === 'ok').length
  return (
    <div className="flex flex-col gap-2">
      <p className="text-xs" data-testid="testscript-coverage-summary">
        {translate(
          `${T}coverage`,
          '{{ok}} of {{total}} expected results have a matching assertion',
          { ok, total: rows.length }
        )}
      </p>
      <div className="max-h-[60vh] overflow-auto rounded-md border scrollbar-sleek">
        <table className="w-full text-xs" data-testid="testscript-coverage">
          <thead className="sticky top-0 bg-background text-left text-muted-foreground">
            <tr>
              <th className="p-2 font-medium">{translate(`${T}col.expected`, 'Expected')}</th>
              <th className="p-2 font-medium">{translate(`${T}col.spec`, 'Spec')}</th>
              <th className="p-2 font-medium">{translate(`${T}col.status`, 'Status')}</th>
              <th className="p-2 font-medium">{translate(`${T}col.assertion`, 'Assertion')}</th>
            </tr>
          </thead>
          <tbody>
            {rows.map((r) => (
              <tr key={r.exp} className="border-t align-top">
                <td className="p-2">
                  <code className="break-all">{r.exp}</code>
                  {r.kind && <div className="text-muted-foreground">{r.kind}</div>}
                </td>
                <td className="p-2">
                  <code>{r.op}</code> <span className="break-words">{r.value}</span>
                </td>
                <td className="p-2">
                  <Badge variant={statusVariant(r.status)}>{r.status}</Badge>
                </td>
                <td className="p-2">
                  {r.file ? (
                    <button
                      type="button"
                      className="text-left text-primary underline-offset-2 hover:underline"
                      onClick={() => onOpen(r.file as string, r.line)}
                    >
                      <code className="break-all">
                        {r.file.split('/').pop()}:{r.line}
                      </code>
                    </button>
                  ) : (
                    <span className="text-muted-foreground">—</span>
                  )}
                  {r.assertion && (
                    <div className="break-all font-mono text-muted-foreground">
                      assert {r.assertion}
                    </div>
                  )}
                  {r.status === 'gap' && r.reason && (
                    <div className="text-muted-foreground" data-testid="testscript-gap-reason">
                      {translate(`${T}gapReason`, 'Not asserted: {{reason}}', { reason: r.reason })}
                    </div>
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  )
}
