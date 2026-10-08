/** StorageToolbar.tsx — FE-CV-TASK-058-04. Environment, legacy switch, changed-only filter, kind chips. */

import { Badge } from '@/components/ui/badge'
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'
import { translate } from '@/i18n/i18n'
import { cn } from '@/lib/utils'
import type { StorageEnv } from '../../../hooks/useCodeIntelStorage'

export type StorageToolbarProps = {
  env: StorageEnv
  includeLegacy: boolean
  onlyChanged: boolean
  canFilterChanged: boolean
  kinds: readonly string[]
  selectedKinds: ReadonlySet<string>
  asOfCommit: string | null
  disabled: boolean
  onEnv: (env: StorageEnv) => void
  onLegacy: (value: boolean) => void
  onOnlyChanged: (value: boolean) => void
  onToggleKind: (kind: string) => void
}

export function StorageToolbar(p: StorageToolbarProps): React.JSX.Element {
  const t = translate
  return (
    <div
      className="flex flex-wrap items-center gap-2 border-b px-3 py-2 text-xs"
      role="toolbar"
      aria-label={t('auto.components.reviewMap.StorageToolbar.label', 'Storage controls')}
    >
      <ToggleGroup
        type="single"
        variant="outline"
        size="sm"
        value={p.env}
        disabled={p.disabled}
        aria-label={t('auto.components.reviewMap.StorageToolbar.env', 'Environment')}
        onValueChange={(v) => v && p.onEnv(v as StorageEnv)}
      >
        <ToggleGroupItem value="dev">dev</ToggleGroupItem>
        <ToggleGroupItem value="prod">prod</ToggleGroupItem>
      </ToggleGroup>
      <label className="flex items-center gap-1">
        <input
          type="checkbox"
          checked={p.includeLegacy}
          disabled={p.disabled}
          onChange={(e) => p.onLegacy(e.target.checked)}
        />
        {t('auto.components.reviewMap.StorageToolbar.legacy', 'Show legacy')}
      </label>
      <label className={cn('flex items-center gap-1', !p.canFilterChanged && 'opacity-50')}>
        <input
          type="checkbox"
          checked={p.onlyChanged}
          disabled={p.disabled || !p.canFilterChanged}
          onChange={(e) => p.onOnlyChanged(e.target.checked)}
        />
        {t('auto.components.reviewMap.StorageToolbar.onlyChanged', 'Only changed and related')}
      </label>
      <div
        className="flex flex-wrap gap-1"
        role="group"
        aria-label={t('auto.components.reviewMap.StorageToolbar.kinds', 'Store kinds')}
      >
        {p.kinds.map((k) => (
          <button
            key={k}
            type="button"
            aria-pressed={p.selectedKinds.has(k)}
            onClick={() => p.onToggleKind(k)}
          >
            <Badge variant={p.selectedKinds.has(k) ? 'default' : 'outline'}>{k}</Badge>
          </button>
        ))}
      </div>
      {p.asOfCommit ? (
        <span className="ml-auto text-muted-foreground">
          {t('auto.components.reviewMap.StorageToolbar.asOf', 'As of commit {{commit}}', {
            commit: p.asOfCommit.slice(0, 8)
          })}
        </span>
      ) : null}
    </div>
  )
}
