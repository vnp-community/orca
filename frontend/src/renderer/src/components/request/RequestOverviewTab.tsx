/**
 * RequestOverviewTab — CR-REQ-019-03
 *
 * Plain-text request body and metadata. The body is user/third-party content,
 * so it is rendered as text only (never as HTML).
 *
 * @module components/request/RequestOverviewTab
 */

import React from 'react'
import { translate } from '@/i18n/i18n'
import { RequestSourceBadge } from './RequestSourceBadge'
import type { OrcaRequest } from '../../../../shared/request-types'

const T = 'auto.components.request.RequestOverviewTab.'

function Field({ label, children }: { label: string; children: React.ReactNode }): React.JSX.Element {
  return (
    <div className="flex flex-col gap-0.5">
      <dt className="text-xs text-muted-foreground">{label}</dt>
      <dd className="text-sm text-foreground">{children}</dd>
    </div>
  )
}

export function RequestOverviewTab({ request }: { request: OrcaRequest }): React.JSX.Element {
  return (
    <div className="flex flex-col gap-4 p-4" data-testid="request-overview">
      {request.body ? (
        <p className="whitespace-pre-wrap break-words text-sm text-foreground">{request.body}</p>
      ) : (
        <p className="text-sm text-muted-foreground">{translate(`${T}noBody`, 'No description.')}</p>
      )}
      <dl className="grid grid-cols-2 gap-3 sm:grid-cols-4">
        {request.source && (
          <Field label={translate(`${T}source`, 'Source')}>
            <RequestSourceBadge provider={request.source.provider} ref={request.source.ref} url={request.source.url} />
          </Field>
        )}
        {request.reporterId && <Field label={translate(`${T}reporter`, 'Reporter')}>{request.reporterId}</Field>}
        {request.size && <Field label={translate(`${T}size`, 'Size')}>{request.size}</Field>}
        {request.urgency && request.urgency !== 'unknown' && (
          <Field label={translate(`${T}urgencyLabel`, 'Urgency')}>
            {translate(`${T}urgency.${request.urgency}`, request.urgency)}
          </Field>
        )}
      </dl>
    </div>
  )
}
