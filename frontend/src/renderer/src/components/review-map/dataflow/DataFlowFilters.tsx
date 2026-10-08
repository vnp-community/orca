/**
 * DataFlowFilters.tsx — FE-CV-TASK-056-05
 *
 * Server-side `triggerKind` / `service` filters of the flow list. Service options come from the
 * flows loaded so far (the contract has no service listing); the chosen one stays selectable.
 *
 * @module components/review-map/dataflow/DataFlowFilters
 */

import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue
} from '@/components/ui/select'
import { translate } from '@/i18n/i18n'

export const DATA_FLOW_TRIGGER_KINDS = ['http', 'grpc', 'event', 'cron', 'ws-channel'] as const

// Radix Select forbids an empty item value; this sentinel means "no filter".
const ALL = '__all__'

function triggerLabel(kind: string): string {
  switch (kind) {
    case 'http':
      return translate('auto.components.reviewMap.dataflow.trigger.http', 'HTTP')
    case 'grpc':
      return translate('auto.components.reviewMap.dataflow.trigger.grpc', 'gRPC')
    case 'event':
      return translate('auto.components.reviewMap.dataflow.trigger.event', 'Event')
    case 'cron':
      return translate('auto.components.reviewMap.dataflow.trigger.cron', 'Schedule')
    default:
      return translate('auto.components.reviewMap.dataflow.trigger.ws', 'WebSocket')
  }
}

export function DataFlowFilters({
  triggerKind,
  onTriggerKindChange,
  service,
  services,
  onServiceChange
}: {
  triggerKind: string | null
  onTriggerKindChange: (kind: string | null) => void
  service: string | null
  services: readonly string[]
  onServiceChange: (service: string | null) => void
}): React.JSX.Element {
  const serviceOptions = [...new Set([...(service ? [service] : []), ...services])].sort()
  return (
    <div className="grid grid-cols-2 gap-1.5">
      <Select
        value={triggerKind ?? ALL}
        onValueChange={(v) => onTriggerKindChange(v === ALL ? null : v)}
      >
        <SelectTrigger
          size="sm"
          className="w-full text-xs"
          aria-label={translate('auto.components.reviewMap.dataflow.triggerFilter', 'Trigger')}
        >
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value={ALL}>
            {translate('auto.components.reviewMap.dataflow.allTriggers', 'All triggers')}
          </SelectItem>
          {DATA_FLOW_TRIGGER_KINDS.map((kind) => (
            <SelectItem key={kind} value={kind}>
              {triggerLabel(kind)}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
      <Select value={service ?? ALL} onValueChange={(v) => onServiceChange(v === ALL ? null : v)}>
        <SelectTrigger
          size="sm"
          className="w-full text-xs"
          aria-label={translate('auto.components.reviewMap.dataflow.serviceFilter', 'Service')}
        >
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value={ALL}>
            {translate('auto.components.reviewMap.dataflow.allServices', 'All services')}
          </SelectItem>
          {serviceOptions.map((name) => (
            <SelectItem key={name} value={name}>
              {name}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </div>
  )
}
