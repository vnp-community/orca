/** C4ContainerPicker.tsx — FE-CV-TASK-055-03 */

import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { translate } from '@/i18n/i18n'
import type { ContainerRef } from '../../../../../shared/code-intel-architecture-types'

export function C4ContainerPicker({
  containers,
  value,
  onChange
}: {
  containers: readonly ContainerRef[]
  value: string | null
  onChange: (id: string) => void
}): React.JSX.Element {
  return (
    <Select value={value ?? undefined} onValueChange={onChange}>
      <SelectTrigger size="sm" aria-label={translate('auto.components.reviewMap.c4.container', 'Container')} className="w-56">
        <SelectValue placeholder={translate('auto.components.reviewMap.c4.container', 'Container')} />
      </SelectTrigger>
      <SelectContent>
        {containers.map((c) => (
          <SelectItem key={c.id} value={c.id}>
            {c.name}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  )
}
