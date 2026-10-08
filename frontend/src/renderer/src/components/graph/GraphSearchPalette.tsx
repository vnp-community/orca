/**
 * GraphSearchPalette — FE-REQ-TASK-032-06 (cmdk, own filtering for stable results)
 *
 * @module components/graph/GraphSearchPalette
 */

import React, { useMemo, useState } from 'react'
import { translate } from '@/i18n/i18n'
import { CommandDialog, CommandEmpty, CommandGroup, CommandInput, CommandItem, CommandList } from '@/components/ui/command'
import { filterGraphNodes } from './graph-panel-state'
import type { GraphNode } from '../../../../shared/graph-types'

type Props = {
  open: boolean
  nodes: readonly GraphNode[]
  onOpenChange: (open: boolean) => void
  onPick: (node: GraphNode) => void
}

export function GraphSearchPalette({ open, nodes, onOpenChange, onPick }: Props): React.JSX.Element {
  const [query, setQuery] = useState('')
  const results = useMemo(() => filterGraphNodes(nodes, query), [nodes, query])
  const byKind = useMemo(() => {
    const m = new Map<string, GraphNode[]>()
    for (const n of results) {m.set(n.kind, [...(m.get(n.kind) ?? []), n])}
    return [...m.entries()]
  }, [results])

  return (
    <CommandDialog
      open={open}
      onOpenChange={(next) => {
        if (!next) {setQuery('')}
        onOpenChange(next)
      }}
      title={translate('auto.components.graph.Search.title', 'Search graph')}
      description={translate('auto.components.graph.Search.description', 'Find a node by label, kind or group')}
      shouldFilter={false}
    >
      <CommandInput
        value={query}
        onValueChange={setQuery}
        placeholder={translate('auto.components.graph.Search.placeholder', 'Search nodes...')}
      />
      <CommandList>
        <CommandEmpty>{translate('auto.components.graph.Search.empty', 'No matching nodes')}</CommandEmpty>
        {byKind.map(([kind, list]) => (
          <CommandGroup key={kind} heading={kind}>
            {list.map((n) => (
              <CommandItem key={n.id} value={n.id} onSelect={() => { setQuery(''); onPick(n) }}>
                <span className="truncate">{n.label}</span>
                {n.group ? <span className="ml-auto truncate text-xs text-muted-foreground">{n.group}</span> : null}
              </CommandItem>
            ))}
          </CommandGroup>
        ))}
      </CommandList>
    </CommandDialog>
  )
}
