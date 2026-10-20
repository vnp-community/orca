import { Loader2 } from 'lucide-react'
import { Badge } from '../ui/badge'
import { Button } from '../ui/button'
import { translate } from '@/i18n/i18n'
import type { BranchOption } from './branch-list-sections'

export const HEADING_LABEL = {
  local: () =>
    translate('auto.components.gitRefPicker.RepoBranchPicker.localHeading', 'Local branches'),
  remote: () =>
    translate('auto.components.gitRefPicker.RepoBranchPicker.remoteHeading', 'Remote branches'),
  match: () =>
    translate('auto.components.gitRefPicker.RepoBranchPicker.moreHeading', 'More matches')
} as const

export function optionLabel(option: BranchOption): string {
  if (option.ref === null) {
    return translate('auto.components.gitRefPicker.RepoRefPicker.defaultUnnamed', 'Default branch')
  }
  if (option.kind === 'default') {
    return translate(
      'auto.components.gitRefPicker.RepoBranchPicker.optionDefault',
      '{{name}} (default)',
      {
        name: option.ref
      }
    )
  }
  if (option.kind === 'remote') {
    return translate(
      'auto.components.gitRefPicker.RepoBranchPicker.optionRemote',
      '{{name}} (remote only)',
      {
        name: option.ref
      }
    )
  }
  return option.ref
}

export function OptionTag({ kind }: { kind: BranchOption['kind'] }): React.JSX.Element | null {
  const text =
    kind === 'default'
      ? translate('auto.components.gitRefPicker.RepoBranchPicker.defaultTag', 'default')
      : kind === 'remote'
        ? translate('auto.components.gitRefPicker.RepoBranchPicker.remoteOnly', 'remote only')
        : null
  return text ? (
    <Badge variant="outline" className="shrink-0 px-1.5 py-0 text-[10px] font-normal">
      {text}
    </Badge>
  ) : null
}

export function StatusLine({
  loading,
  failed,
  searching,
  searchFailed,
  emptyText,
  onRetry
}: {
  loading: boolean
  failed: boolean
  searching: boolean
  searchFailed: boolean
  emptyText: string | null
  onRetry: () => void
}): React.JSX.Element {
  // Always mounted so a screen reader announces a change of state; the visible text doubles as the announcement.
  return (
    <div role="status" aria-live="polite" className="text-xs text-muted-foreground">
      {loading || searching ? (
        <p className="flex items-center gap-1.5">
          <Loader2 className="size-3 animate-spin" aria-hidden="true" />
          {loading
            ? translate(
                'auto.components.gitRefPicker.RepoBranchPicker.loading',
                'Loading branches...'
              )
            : translate(
                'auto.components.gitRefPicker.RepoRefPicker.searching',
                'Searching branches...'
              )}
        </p>
      ) : null}
      {failed || searchFailed ? (
        <div className="flex flex-wrap items-center gap-2">
          <p className="text-destructive">
            {failed
              ? translate(
                  'auto.components.gitRefPicker.RepoBranchPicker.loadFailed',
                  'Could not load the branches. Try again.'
                )
              : translate(
                  'auto.components.gitRefPicker.RepoRefPicker.searchFailed',
                  'Branch search failed. Try again.'
                )}
          </p>
          <Button variant="outline" size="xs" onClick={onRetry}>
            {translate('auto.components.gitRefPicker.RepoRefPicker.retry', 'Retry')}
          </Button>
        </div>
      ) : null}
      {emptyText ? <p>{emptyText}</p> : null}
    </div>
  )
}
