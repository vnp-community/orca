/**
 * ErdGhostTableNode.tsx — FE-CV-TASK-057-05
 *
 * Placeholder for a table owned by another service. Click hops the ERD to that service
 * (a back-stack entry is kept by the store action).
 */

import { Handle, Position, type NodeProps } from '@xyflow/react'
import { translate } from '@/i18n/i18n'

export type ErdGhostNodeData = {
  service: string
  table: string
  onOpenService: (service: string) => void
}

export function ErdGhostTableNode({ data }: NodeProps): React.JSX.Element {
  const { service, table, onOpenService } = data as ErdGhostNodeData
  return (
    <button
      type="button"
      data-testid={`erd-ghost-${service}.${table}`}
      onClick={() => onOpenService(service)}
      className="w-[264px] rounded-md border border-dashed bg-muted/40 px-2 py-1.5 text-left text-xs hover:bg-muted"
    >
      <Handle type="target" position={Position.Left} isConnectable={false} className="opacity-0" />
      <Handle type="source" position={Position.Right} isConnectable={false} className="opacity-0" />
      <span className="block truncate font-medium">{table}</span>
      <span className="block truncate text-[11px] text-muted-foreground">
        {translate(
          'auto.components.reviewMap.ErdGhostTableNode.owner',
          'Owned by {{service}} — open its ERD',
          { service }
        )}
      </span>
    </button>
  )
}
