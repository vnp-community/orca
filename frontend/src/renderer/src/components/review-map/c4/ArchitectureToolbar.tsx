/** ArchitectureToolbar.tsx — FE-CV-TASK-055-03 */

import { Network, Table2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { translate } from '@/i18n/i18n'
import type { ContainerRef } from '../../../../../shared/code-intel-architecture-types'
import { C4ContainerPicker } from './C4ContainerPicker'

export type C4ViewMode = 'graph' | 'list'

export function ArchitectureToolbar({
  containers,
  container,
  onContainerChange,
  mode,
  onModeChange,
  includeHidden,
  onIncludeHiddenChange,
  onEdit,
  readOnly
}: {
  containers: readonly ContainerRef[]
  container: string | null
  onContainerChange: (id: string) => void
  mode: C4ViewMode
  onModeChange: (mode: C4ViewMode) => void
  includeHidden: boolean
  onIncludeHiddenChange: (value: boolean) => void
  onEdit: () => void
  readOnly?: boolean
}): React.JSX.Element {
  return (
    <div role="toolbar" aria-label={translate('auto.components.reviewMap.c4.toolbar', 'Architecture options')} className="flex flex-wrap items-center gap-2 border-b px-3 py-1.5">
      <C4ContainerPicker containers={containers} value={container} onChange={onContainerChange} />
      <div className="flex items-center gap-1">
        <Button size="xs" variant={mode === 'graph' ? 'secondary' : 'ghost'} aria-pressed={mode === 'graph'} onClick={() => onModeChange('graph')}>
          <Network className="size-3.5" aria-hidden="true" />
          {translate('auto.components.reviewMap.c4.mode.graph', 'Graph')}
        </Button>
        <Button size="xs" variant={mode === 'list' ? 'secondary' : 'ghost'} aria-pressed={mode === 'list'} onClick={() => onModeChange('list')}>
          <Table2 className="size-3.5" aria-hidden="true" />
          {translate('auto.components.reviewMap.c4.mode.list', 'List')}
        </Button>
      </div>
      <label className="flex items-center gap-1.5 text-xs">
        <input type="checkbox" checked={includeHidden} onChange={(e) => onIncludeHiddenChange(e.target.checked)} />
        {translate('auto.components.reviewMap.c4.includeHidden', 'Show hidden components')}
      </label>
      <Button size="xs" variant="outline" className="ml-auto" disabled={!container} onClick={onEdit}>
        {readOnly
          ? translate('auto.components.reviewMap.c4.viewYaml', 'View c4.yaml')
          : translate('auto.components.reviewMap.c4.editYaml', 'Edit c4.yaml')}
      </Button>
    </div>
  )
}
