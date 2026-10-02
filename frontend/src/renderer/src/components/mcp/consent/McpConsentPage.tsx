import { useEffect, useRef, useState } from 'react'
import { ShieldAlertIcon } from 'lucide-react'
import type { McpConsentRequest, McpScopeId } from '../../../../../shared/mcp-types'
import { translate } from '@/i18n/i18n'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardFooter, CardHeader, CardTitle } from '@/components/ui/card'
import { Checkbox } from '@/components/ui/checkbox'
import { Skeleton } from '@/components/ui/skeleton'
import { isHighRisk } from '@/lib/mcp-risk'
import { mcpScopeDescription, mcpScopeLabel } from '@/lib/mcp-labels'
import { readConsentRequestId, useMcpConsent, type McpConsentState } from './use-mcp-consent'

function defaultSelection(req: McpConsentRequest): Set<McpScopeId> {
  const out = new Set<McpScopeId>()
  for (const s of req.scopes) {
    // Why: high-risk scopes are never pre-ticked; the user must opt in deliberately.
    if (req.alreadyGranted.includes(s.id as McpScopeId) || !isHighRisk(s.risk)) {
      out.add(s.id as McpScopeId)
    }
  }
  return out
}

function errorText(code: string): string {
  switch (code) {
    case 'MCP_CONSENT_EXPIRED':
      return translate(
        'auto.mcp.consent.expired',
        'This request has expired. Go back to the app and start the connection again.'
      )
    case 'MCP_CONSENT_NOT_FOUND':
      return translate(
        'auto.mcp.consent.notFound',
        'This request is no longer valid or was already answered.'
      )
    case 'MCP_DISABLED':
      return translate(
        'auto.mcp.consent.disabled',
        'MCP access is turned off for this organization.'
      )
    default:
      return translate('auto.mcp.consent.genericError', 'Something went wrong.')
  }
}

function ConsentBody({
  req,
  submitting,
  notice,
  onDecide
}: {
  req: McpConsentRequest
  submitting: boolean
  notice: boolean
  onDecide: (decision: 'approve' | 'deny', scopes: McpScopeId[]) => void
}): React.JSX.Element {
  const [selected, setSelected] = useState(() => defaultSelection(req))
  const toggle = (id: McpScopeId, on: boolean): void =>
    setSelected((prev) => {
      const next = new Set(prev)
      if (on) {
        next.add(id)
      } else {
        next.delete(id)
      }
      return next
    })
  return (
    <>
      <CardContent className="space-y-4">
        <div className="space-y-1 text-sm text-muted-foreground">
          <p>{req.tenant.name}</p>
          <p className="font-mono">
            {translate('auto.mcp.consent.redirectsTo', 'Will redirect to {{host}}', {
              host: req.redirectHost
            })}
          </p>
          {req.clientUri ? <p className="font-mono">{safeHost(req.clientUri)}</p> : null}
          {req.isNewClient ? (
            <Badge variant="outline">{translate('auto.mcp.consent.newApp', 'New app')}</Badge>
          ) : null}
        </div>
        {req.registeredViaDcr ? (
          <div
            role="note"
            className="rounded-md border border-border px-3 py-2 text-sm text-muted-foreground"
          >
            {translate(
              'auto.mcp.consent.dcrNote',
              'This app registered itself automatically. Only continue if you started this connection.'
            )}
          </div>
        ) : null}
        <fieldset className="space-y-3" disabled={submitting}>
          <legend className="mb-2 text-sm font-medium">
            {translate('auto.mcp.consent.asking', 'This app is asking to:')}
          </legend>
          {req.scopes.map((s) => {
            const high = isHighRisk(s.risk)
            const id = `mcp-consent-${s.id.replace(/[^A-Za-z0-9]/g, '_')}`
            return (
              <div key={s.id} className="flex items-start gap-3 py-1">
                <Checkbox
                  id={id}
                  className="mt-1"
                  aria-describedby={`${id}-desc`}
                  checked={selected.has(s.id as McpScopeId)}
                  onCheckedChange={(v) => toggle(s.id as McpScopeId, v === true)}
                />
                <label htmlFor={id} className="flex-1 cursor-pointer space-y-0.5">
                  <span className="flex flex-wrap items-center gap-2 text-sm font-medium">
                    {mcpScopeLabel(s)}
                    {high ? (
                      <Badge variant="destructive">
                        <ShieldAlertIcon aria-hidden />
                        {translate('auto.mcp.risk.highRisk', 'High risk')}
                      </Badge>
                    ) : null}
                    {s.risk === 'write_reversible' ? (
                      <Badge variant="outline">
                        {translate('auto.mcp.consent.canChange', 'Can change data')}
                      </Badge>
                    ) : null}
                    {req.alreadyGranted.includes(s.id as McpScopeId) ? (
                      <Badge variant="secondary">
                        {translate('auto.mcp.consent.alreadyAllowed', 'Already allowed')}
                      </Badge>
                    ) : null}
                  </span>
                  <span
                    id={`${id}-desc`}
                    className={`block text-xs text-muted-foreground ${high ? 'font-semibold' : ''}`}
                  >
                    {mcpScopeDescription(s)}
                  </span>
                </label>
              </div>
            )
          })}
        </fieldset>
        {notice ? (
          <p role="alert" className="text-sm text-destructive">
            {translate(
              'auto.mcp.consent.scopeNotice',
              "Some of these permissions can't be granted. Adjust your selection and try again."
            )}
          </p>
        ) : null}
      </CardContent>
      <CardFooter className="justify-end gap-2">
        <Button variant="ghost" disabled={submitting} onClick={() => onDecide('deny', [])}>
          {translate('auto.mcp.consent.deny', 'Deny')}
        </Button>
        <Button
          disabled={submitting || selected.size === 0}
          onClick={() => onDecide('approve', [...selected])}
        >
          {submitting
            ? translate('auto.mcp.consent.working', 'Working…')
            : translate('auto.mcp.consent.allow', 'Allow access')}
        </Button>
      </CardFooter>
    </>
  )
}

function safeHost(uri: string): string {
  try {
    return new URL(uri).host
  } catch {
    return ''
  }
}

function StateBody({
  state,
  decide,
  retry
}: {
  state: McpConsentState
  decide: (d: 'approve' | 'deny', s: McpScopeId[]) => void
  retry: () => void
}): React.JSX.Element {
  switch (state.kind) {
    case 'loading':
      return (
        <CardContent className="space-y-3" aria-busy="true">
          <Skeleton className="h-5 w-2/3" />
          {[0, 1, 2].map((i) => (
            <Skeleton key={i} className="h-10 w-full" />
          ))}
        </CardContent>
      )
    case 'ready':
    case 'submitting':
      return (
        <ConsentBody
          req={state.req}
          submitting={state.kind === 'submitting'}
          notice={state.kind === 'ready' && state.notice === 'scope'}
          onDecide={decide}
        />
      )
    case 'redirecting':
      return (
        <CardContent className="space-y-2 text-sm" role="status" aria-live="polite">
          <p>
            {translate(
              'auto.mcp.consent.returning',
              'Returning to {{client}}… You can close this tab once the app confirms.',
              { client: state.clientName }
            )}
          </p>
          <a className="text-primary underline" href={state.url} rel="noreferrer noopener">
            {translate('auto.mcp.consent.continue', 'Continue')}
          </a>
        </CardContent>
      )
    case 'error':
      return (
        <CardContent className="space-y-3">
          <div role="alert" className="text-sm text-destructive">
            {errorText(state.code)}
          </div>
          {state.code === 'UNKNOWN' ? (
            <Button variant="ghost" onClick={retry}>
              {translate('auto.mcp.pane.retry', 'Retry')}
            </Button>
          ) : null}
        </CardContent>
      )
  }
}

export function McpConsentPage(): React.JSX.Element {
  const requestId = readConsentRequestId(window.location.search)
  const { state, decide, retry } = useMcpConsent(requestId)
  const heading = useRef<HTMLHeadingElement>(null)
  // Why: focus the heading, not "Allow" — a stray Enter must not grant access on a security screen.
  useEffect(() => {
    heading.current?.focus()
  }, [])
  const clientName =
    state.kind === 'ready' || state.kind === 'submitting' ? state.req.clientName : null
  return (
    <main className="flex min-h-dvh items-center justify-center bg-background px-4 py-8">
      <Card className="w-full max-w-md">
        <CardHeader>
          <p className="text-xs text-muted-foreground">Orca</p>
          <CardTitle>
            <h1 ref={heading} tabIndex={-1} className="outline-none">
              {clientName
                ? translate('auto.mcp.consent.title', 'Authorize {{client}}', {
                    client: clientName
                  })
                : translate('auto.mcp.consent.titleGeneric', 'Authorize app')}
            </h1>
          </CardTitle>
        </CardHeader>
        <StateBody state={state} decide={decide} retry={retry} />
      </Card>
    </main>
  )
}
