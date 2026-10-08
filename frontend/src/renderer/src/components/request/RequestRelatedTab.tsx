/**
 * RequestRelatedTab — CR-REQ-019-05
 *
 * Parent / child / follow-up requests of the current request. Links come from
 * `request.get`; a runtime that omits them shows a neutral "not supported" note.
 *
 * @module components/request/RequestRelatedTab
 */

import React, { useEffect, useState } from 'react'
import { Lock } from 'lucide-react'
import { translate } from '@/i18n/i18n'
import { useAppStore } from '@/store'
import { callRequestRpc } from '../../runtime/request-rpc-client'
import { parseRequest } from '../../../../shared/request-wire-parsers'
import { REQUEST_RPC_METHODS } from '../../../../shared/request-rpc-methods'
import { groupRequestLinks } from './request-link-groups'
import { RequestStatusBadge } from './RequestStatusBadge'
import { RequestTypeBadge } from './RequestTypeBadge'
import type { OrcaRequest, RequestLink } from '../../../../shared/request-types'

const T = 'auto.components.request.RequestRelatedTab.'

type Props = {
  requestId: string
  links: RequestLink[]
  linksSupported: boolean
}

export function RequestRelatedTab({ requestId, links, linksSupported }: Props): React.JSX.Element {
  const groups = React.useMemo(() => groupRequestLinks(links, requestId), [links, requestId])
  const cached = useAppStore((s) => s.requestsById)
  const upsertRequests = useAppStore((s) => s.upsertRequests)
  const setRequestPageRequest = useAppStore((s) => s.setRequestPageRequest)
  // Ids that could not be read (not_found / forbidden) so the row says so instead of spinning.
  const [unreadable, setUnreadable] = useState<Set<string>>(new Set())

  const missingKey = groups
    .flatMap((g) => g.links.map((l) => l.otherRequestId))
    .filter((id) => !cached[id] && !unreadable.has(id))
    .join(',')

  useEffect(() => {
    if (!missingKey) {return}
    let cancelled = false
    for (const id of missingKey.split(',')) {
      void callRequestRpc<unknown>(REQUEST_RPC_METHODS.GET, { id }).then((result) => {
        if (cancelled) {return}
        if (!result.ok) {
          setUnreadable((prev) => new Set(prev).add(id))
          return
        }
        const raw = (result.value as { request?: unknown } | null)?.request ?? result.value
        upsertRequests([parseRequest(raw)])
      })
    }
    return () => {
      cancelled = true
    }
  }, [missingKey, upsertRequests])

  if (!linksSupported) {
    return (
      <p className="p-4 text-sm text-muted-foreground" data-testid="request-related-unsupported">
        {translate(`${T}unsupported`, 'This runtime does not support viewing request links yet.')}
      </p>
    )
  }
  if (groups.length === 0) {
    return (
      <p className="p-4 text-sm text-muted-foreground" data-testid="request-related-empty">
        {translate(`${T}empty`, 'No related requests.')}
      </p>
    )
  }

  return (
    <div className="flex flex-col gap-4 p-4" data-testid="request-related">
      {groups.map((group) => (
        <section key={group.id} aria-label={translate(`${T}group.${group.id}`, group.id)}>
          <h3 className="mb-1 text-xs font-medium uppercase text-muted-foreground">
            {translate(`${T}group.${group.id}`, group.id)}
          </h3>
          <ul className="flex flex-col divide-y divide-border rounded border border-border">
            {group.links.map(({ link, otherRequestId }) => {
              const other: OrcaRequest | undefined = cached[otherRequestId]
              return (
                <li key={link.id || otherRequestId}>
                  {other ? (
                    <button
                      type="button"
                      onClick={() => setRequestPageRequest(other.id)}
                      className="flex w-full items-center gap-2 px-3 py-2 text-left text-sm hover:bg-accent/50"
                    >
                      <span className="text-xs text-muted-foreground">#{other.number}</span>
                      <span className="min-w-0 flex-1 truncate">{other.title}</span>
                      <RequestTypeBadge type={other.type} size="xs" />
                      <RequestStatusBadge status={other.status} size="xs" />
                    </button>
                  ) : (
                    <div className="flex items-center gap-2 px-3 py-2 text-sm text-muted-foreground">
                      {unreadable.has(otherRequestId) && <Lock className="size-3.5" aria-hidden />}
                      {unreadable.has(otherRequestId)
                        ? translate(`${T}unreadable`, 'Cannot view this request')
                        : translate(`${T}loading`, 'Loading...')}
                    </div>
                  )}
                </li>
              )
            })}
          </ul>
        </section>
      ))}
    </div>
  )
}
