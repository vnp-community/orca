/**
 * StorageSecretNode.tsx — FE-CV-TASK-058-04
 *
 * A secret reference: key name and Vault path only. The component reads nothing else from
 * the node on purpose, so a value smuggled into other fields can never reach the DOM.
 */

import { Handle, Position, type NodeProps } from '@xyflow/react'
import { KeyRound, ShieldAlert } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { translate } from '@/i18n/i18n'
import { cn } from '@/lib/utils'
import { ERD_CHANGE_TEXT_CLASS } from '../erd/erd-change-style'
import { STORAGE_MARK_SYMBOL, type StorageMark } from './storage-change-marks'
import { STORAGE_NODE_HEIGHT, STORAGE_NODE_WIDTH } from './storage-layout'
import type { StorageNode } from './storage-view-model'

export type StorageNodeData = { node: StorageNode; mark?: StorageMark; dimmed: boolean }

export async function copyKeyName(name: string): Promise<void> {
  try {
    await navigator.clipboard.writeText(name)
  } catch {
    // Clipboard may be blocked (permissions, insecure context); copying a key name is optional.
  }
}

export function StorageSecretNode({ data, selected }: NodeProps): React.JSX.Element {
  const { node, mark, dimmed } = data as StorageNodeData
  return (
    <div
      role="group"
      tabIndex={0}
      aria-label={translate(
        'auto.components.reviewMap.StorageSecretNode.ariaLabel',
        'Secret reference {{name}}',
        { name: node.name }
      )}
      data-testid={`storage-node-${node.id}`}
      className={cn(
        'rounded-md border bg-card px-2 py-1.5 text-xs outline-none focus-visible:ring-2 focus-visible:ring-ring',
        dimmed && 'opacity-40',
        selected && 'ring-2 ring-ring'
      )}
      style={{ width: STORAGE_NODE_WIDTH, height: STORAGE_NODE_HEIGHT }}
    >
      <Handle type="target" position={Position.Left} isConnectable={false} className="opacity-0" />
      <div className="flex items-center gap-1 font-medium">
        <KeyRound className="size-3 shrink-0" aria-hidden="true" />
        <span className="truncate">{node.name}</span>
        {mark && mark !== 'related' ? (
          <span className={cn('ml-auto font-mono', ERD_CHANGE_TEXT_CLASS[mark])} aria-hidden="true">
            {STORAGE_MARK_SYMBOL[mark]}
          </span>
        ) : null}
        {node.masked ? (
          <ShieldAlert
            className="ml-auto size-3 shrink-0"
            aria-label={translate(
              'auto.components.reviewMap.StorageSecretNode.masked',
              'Sensitive text was masked in the interface'
            )}
          />
        ) : null}
      </div>
      {node.vaultPath ? (
        <div className="truncate text-[11px] text-muted-foreground">{node.vaultPath}</div>
      ) : null}
      <div className="flex items-center text-[10px] text-muted-foreground">
        <span className="truncate">
          {translate(
            'auto.components.reviewMap.StorageSecretNode.neverShown',
            'value is never shown'
          )}
        </span>
        <Button
          type="button"
          variant="ghost"
          size="xs"
          className="ml-auto h-4 px-1 text-[10px]"
          onClick={() => void copyKeyName(node.name)}
        >
          {translate('auto.components.reviewMap.StorageSecretNode.copyKey', 'Copy key name')}
        </Button>
      </div>
    </div>
  )
}
