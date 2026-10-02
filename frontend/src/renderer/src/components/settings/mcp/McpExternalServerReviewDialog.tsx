import { useCallback, useEffect, useMemo, useState } from 'react'
import { Loader2Icon } from 'lucide-react'
import type { McpExternalServer } from '../../../../../shared/mcp-types'
import { translate } from '@/i18n/i18n'
import { parseMcpError } from '@/runtime/runtime-mcp-error'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle
} from '@/components/ui/dialog'
import { Label } from '@/components/ui/label'
import { Skeleton } from '@/components/ui/skeleton'
import { McpCopyButton } from './McpCopyButton'
import { McpExternalServerToolDiff } from './McpExternalServerToolDiff'
import { UntrustedToolText } from './UntrustedToolText'
import { diffTools } from './mcp-tool-diff'
import { externalServerErrorText } from './mcp-external-server-validation'
import type { McpExternalServersApi, McpProbeResult } from './use-external-servers'

type Props = {
  server: McpExternalServer
  api: Pick<McpExternalServersApi, 'probe' | 'review'>
  paused: boolean
  onClose: () => void
}

type Probe =
  | { status: 'loading' }
  | { status: 'ready'; result: McpProbeResult }
  | { status: 'error'; message: string }

export function McpExternalServerReviewDialog({
  server,
  api,
  paused,
  onClose
}: Props): React.JSX.Element {
  const [probe, setProbe] = useState<Probe>({ status: 'loading' })
  const [reviewed, setReviewed] = useState(false)
  const [busy, setBusy] = useState(false)
  const [note, setNote] = useState<string | null>(null)
  const [actionError, setActionError] = useState<string | null>(null)
  const needsDecision = server.status === 'pending_review' || server.toolsChanged

  const runProbe = useCallback(
    async (isCurrent: () => boolean = () => true): Promise<void> => {
      setProbe({ status: 'loading' })
      setReviewed(false)
      try {
        const result = await api.probe(server.id)
        if (isCurrent()) {
          setProbe({ status: 'ready', result })
        }
      } catch (e) {
        const err = parseMcpError(e)
        if (isCurrent()) {
          setProbe({ status: 'error', message: externalServerErrorText(err.code, err.detail) })
        }
      }
    },
    [api, server.id]
  )

  useEffect(() => {
    let live = true
    void runProbe(() => live)
    return () => {
      live = false
    }
    // Probe once per opened server; `api` identity changes must not re-probe.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [server.id])

  const result = probe.status === 'ready' ? probe.result : null
  const diff = useMemo(
    () => (result?.approvedTools ? diffTools(result.tools, result.approvedTools) : null),
    [result]
  )
  const isStdio = (result?.transport ?? server.transport) === 'stdio'
  const target =
    server.transport === 'http'
      ? (server.url ?? '')
      : [server.command, ...(server.args ?? [])].join(' ')

  const decide = async (decision: 'approve' | 'reject'): Promise<void> => {
    if (!result) {
      return
    }
    setBusy(true)
    setActionError(null)
    try {
      await api.review(server.id, decision, result.digest)
      onClose()
    } catch (e) {
      const err = parseMcpError(e)
      if (err.code === 'MCP_SERVER_DIGEST_MISMATCH') {
        setNote(
          translate(
            'auto.mcp.external.digestChanged',
            'The server changed while you were reviewing — probing again.'
          )
        )
        setBusy(false)
        await runProbe()
        return
      }
      setActionError(externalServerErrorText(err.code, err.detail))
      setBusy(false)
    }
  }

  return (
    <Dialog open onOpenChange={(open) => !open && !busy && onClose()}>
      <DialogContent className="max-h-[90vh] overflow-y-auto sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle>
            {translate('auto.mcp.external.reviewTitle', 'Review server {{name}}', {
              name: server.name
            })}
          </DialogTitle>
          <DialogDescription>
            {translate(
              'auto.mcp.external.reviewDescription',
              'Check what this server exposes before agents are allowed to use it.'
            )}
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-3" aria-live="polite">
          <div className="space-y-1 text-sm">
            <UntrustedToolText text={target} className="font-mono" />
            {result ? (
              <div className="flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
                <span className="uppercase">{result.transport}</span>
                <span>{translate('auto.mcp.external.digest', 'Digest')}</span>
                <code title={result.digest}>{result.digest.slice(0, 12)}</code>
                <McpCopyButton
                  text={result.digest}
                  ariaLabel={translate('auto.mcp.external.copyDigest', 'Copy digest')}
                />
              </div>
            ) : null}
          </div>
          {note ? (
            <p role="status" className="rounded-md border border-border px-3 py-2 text-sm">
              {note}
            </p>
          ) : null}
          {probe.status === 'loading' ? (
            <div aria-busy="true" className="space-y-2">
              <Skeleton className="h-9 w-full" />
              <Skeleton className="h-9 w-full" />
            </div>
          ) : null}
          {probe.status === 'error' ? (
            <div
              role="alert"
              className="flex items-center justify-between gap-2 rounded-md border border-destructive/40 px-3 py-2 text-sm text-destructive"
            >
              <span>{probe.message}</span>
              <Button type="button" variant="outline" size="sm" onClick={() => void runProbe()}>
                {translate('auto.mcp.pane.retry', 'Retry')}
              </Button>
            </div>
          ) : null}
          {result && isStdio ? (
            <div className="space-y-2 rounded-md border border-border p-3 text-sm">
              <p>
                {translate(
                  'auto.mcp.external.stdioNoProbe',
                  "Orca doesn't run stdio servers to list their tools. Review the command and arguments carefully."
                )}
              </p>
              <UntrustedToolText text={target} className="font-mono" />
              {server.envRefs.length ? (
                <UntrustedToolText
                  text={`env: ${server.envRefs.map((r) => r.name).join(', ')}`}
                  className="font-mono"
                />
              ) : null}
            </div>
          ) : null}
          {result && !isStdio ? (
            <>
              <p className="rounded-md border border-border px-3 py-2 text-xs text-muted-foreground">
                {translate(
                  'auto.mcp.external.untrustedNote',
                  'Text below comes from the external server and is untrusted. Read it for hidden instructions.'
                )}
              </p>
              {diff ? (
                <McpExternalServerToolDiff diff={diff} />
              ) : server.toolsChanged ? (
                <p role="status" className="text-sm">
                  {translate(
                    'auto.mcp.external.noPrevious',
                    'This server was approved before; compare with the previous version manually.'
                  )}
                </p>
              ) : null}
              <section className="space-y-1">
                <h4 className="text-xs font-medium text-muted-foreground uppercase">
                  {translate('auto.mcp.external.toolsNow', 'Tools ({{n}})', {
                    n: result.tools.length
                  })}
                </h4>
                {result.tools.length === 0 ? (
                  <p className="text-sm text-muted-foreground">
                    {translate('auto.mcp.external.noTools', 'This server exposes no tools.')}
                  </p>
                ) : (
                  <ul className="space-y-2">
                    {result.tools.map((t) => (
                      <li key={t.name} className="rounded-md border border-border p-2">
                        <UntrustedToolText text={t.name} className="font-mono" />
                        <UntrustedToolText text={t.description} />
                      </li>
                    ))}
                  </ul>
                )}
              </section>
            </>
          ) : null}
          {actionError ? (
            <p role="alert" className="text-sm text-destructive">
              {actionError}
            </p>
          ) : null}
          {needsDecision && result ? (
            <div className="flex items-center gap-2">
              <Checkbox
                id="mcp-ext-reviewed"
                checked={reviewed}
                onCheckedChange={(c) => setReviewed(c === true)}
              />
              <Label htmlFor="mcp-ext-reviewed">
                {isStdio
                  ? translate('auto.mcp.external.reviewedCommand', 'I reviewed the command')
                  : translate('auto.mcp.external.reviewedTools', 'I reviewed the tools')}
              </Label>
            </div>
          ) : null}
        </div>
        <DialogFooter>
          <Button type="button" variant="ghost" disabled={busy} onClick={onClose}>
            {translate('auto.mcp.external.close', 'Close')}
          </Button>
          {result ? (
            <Button
              type="button"
              variant="outline"
              disabled={busy || paused}
              title={translate('auto.mcp.external.rejectHint', 'Rejecting disables the server')}
              onClick={() => void decide('reject')}
            >
              {translate('auto.mcp.external.reject', 'Reject')}
            </Button>
          ) : null}
          {needsDecision ? (
            <Button
              type="button"
              disabled={busy || paused || !result || !reviewed}
              onClick={() => void decide('approve')}
            >
              {busy ? <Loader2Icon className="animate-spin" aria-hidden /> : null}
              {translate('auto.mcp.external.approve', 'Approve')}
            </Button>
          ) : null}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
