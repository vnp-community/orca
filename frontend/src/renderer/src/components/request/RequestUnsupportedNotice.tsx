/**
 * RequestUnsupportedNotice — CR-REQ-018-05
 *
 * Shown instead of RequestPage content when requestFlowSupport === 'unsupported'.
 *
 * @module components/request/RequestUnsupportedNotice
 */

import React from 'react'
import { Inbox } from 'lucide-react'
import { translate } from '@/i18n/i18n'

export function RequestUnsupportedNotice(): React.JSX.Element {
  return (
    <div className="flex flex-col items-center justify-center h-full gap-4 py-16 text-center text-muted-foreground px-8">
      <Inbox className="size-10 opacity-40" aria-hidden />
      <h2 className="text-base font-semibold text-foreground">
        {translate(
          'auto.components.request.RequestUnsupportedNotice.title',
          'Request management not supported'
        )}
      </h2>
      <p className="text-sm max-w-md">
        {translate(
          'auto.components.request.RequestUnsupportedNotice.body',
          'The connected runtime does not support the request-service. Update the backend or connect to a runtime that includes it.'
        )}
      </p>
    </div>
  )
}
