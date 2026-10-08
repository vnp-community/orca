/** ErdSchemaGroupNode.tsx — FE-CV-TASK-057-05. Background band that labels one schema. */

import type { NodeProps } from '@xyflow/react'

export type ErdSchemaGroupData = { schema: string; width: number; height: number }

export function ErdSchemaGroupNode({ data }: NodeProps): React.JSX.Element {
  const { schema, width, height } = data as ErdSchemaGroupData
  return (
    <div
      className="rounded-lg border border-dashed border-border bg-muted/20"
      style={{ width, height }}
      aria-hidden="true"
    >
      <span className="p-1 text-[10px] uppercase tracking-wide text-muted-foreground">
        {schema}
      </span>
    </div>
  )
}
