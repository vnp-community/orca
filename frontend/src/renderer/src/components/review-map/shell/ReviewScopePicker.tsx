import { useState } from 'react'
import { GitCompare } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'
import { translate } from '@/i18n/i18n'
import { reviewScopeLabel, type ReviewScope } from '../review-scope-model'

export type HostedReviewOption = Extract<ReviewScope, { kind: 'hostedReview' }>

type Props = {
  scope: ReviewScope | null
  open: boolean
  onOpenChange: (open: boolean) => void
  onChange: (scope: ReviewScope) => void
  /** Suggested base for the "Branch" choice (branch-compare base). */
  defaultBaseRef: string | null
  /** Submitted review of this branch, when the host has one (labelled "Submitted review", not "PR"). */
  hostedReview: HostedReviewOption | null
}

type Kind = ReviewScope['kind']

export function ReviewScopePicker({
  scope,
  open,
  onOpenChange,
  onChange,
  defaultBaseRef,
  hostedReview
}: Props): React.JSX.Element {
  const [kind, setKind] = useState<Kind>(scope?.kind ?? 'branch')
  const [baseRef, setBaseRef] = useState(
    scope?.kind === 'branch' ? scope.baseRef : (defaultBaseRef ?? '')
  )
  const [includeUncommitted, setIncludeUncommitted] = useState(
    scope?.kind === 'branch' ? scope.includeUncommitted : true
  )
  const [baseCommit, setBaseCommit] = useState(scope?.kind === 'range' ? scope.baseCommit : '')
  const [headCommit, setHeadCommit] = useState(scope?.kind === 'range' ? scope.headCommit : '')

  const canApply =
    kind === 'branch'
      ? baseRef.trim() !== ''
      : kind === 'range'
        ? baseCommit.trim() !== '' && headCommit.trim() !== ''
        : hostedReview !== null

  function apply(): void {
    if (kind === 'branch') {
      onChange({ kind, baseRef: baseRef.trim(), includeUncommitted })
    } else if (kind === 'range') {
      onChange({ kind, baseCommit: baseCommit.trim(), headCommit: headCommit.trim() })
    } else if (hostedReview) {
      onChange(hostedReview)
    }
    onOpenChange(false)
  }

  return (
    <Popover open={open} onOpenChange={onOpenChange}>
      <PopoverTrigger asChild>
        <Button type="button" size="xs" variant="outline" data-testid="review-scope-trigger">
          <GitCompare aria-hidden />
          {scope
            ? translate('auto.components.reviewMap.shell.scope.current', 'vs {{label}}', {
                label: reviewScopeLabel(scope)
              })
            : translate('auto.components.reviewMap.shell.scope.choose', 'Scope')}
        </Button>
      </PopoverTrigger>
      <PopoverContent align="start" className="w-80 space-y-3 text-xs">
        <ToggleGroup
          type="single"
          value={kind}
          onValueChange={(v) => v && setKind(v as Kind)}
          variant="outline"
          size="sm"
          className="w-full"
        >
          <ToggleGroupItem value="branch">
            {translate('auto.components.reviewMap.shell.scope.branch', 'Base branch')}
          </ToggleGroupItem>
          <ToggleGroupItem value="range">
            {translate('auto.components.reviewMap.shell.scope.range', 'Commit range')}
          </ToggleGroupItem>
          <ToggleGroupItem value="hostedReview" disabled={hostedReview === null}>
            {translate('auto.components.reviewMap.shell.scope.hosted', 'Submitted review')}
          </ToggleGroupItem>
        </ToggleGroup>
        {kind === 'branch' ? (
          <div className="space-y-2">
            <Input
              value={baseRef}
              onChange={(e) => setBaseRef(e.target.value)}
              placeholder={translate(
                'auto.components.reviewMap.shell.scope.basePlaceholder',
                'origin/main'
              )}
              aria-label={translate(
                'auto.components.reviewMap.shell.scope.baseAria',
                'Base branch'
              )}
            />
            <label className="flex items-center gap-2">
              <Checkbox
                checked={includeUncommitted}
                onCheckedChange={(c) => setIncludeUncommitted(c === true)}
              />
              {translate(
                'auto.components.reviewMap.shell.scope.uncommitted',
                'Include uncommitted changes'
              )}
            </label>
          </div>
        ) : null}
        {kind === 'range' ? (
          <div className="space-y-2">
            <Input
              value={baseCommit}
              onChange={(e) => setBaseCommit(e.target.value)}
              placeholder={translate(
                'auto.components.reviewMap.shell.scope.fromCommit',
                'From commit'
              )}
              aria-label={translate(
                'auto.components.reviewMap.shell.scope.fromCommit',
                'From commit'
              )}
            />
            <Input
              value={headCommit}
              onChange={(e) => setHeadCommit(e.target.value)}
              placeholder={translate('auto.components.reviewMap.shell.scope.toCommit', 'To commit')}
              aria-label={translate('auto.components.reviewMap.shell.scope.toCommit', 'To commit')}
            />
          </div>
        ) : null}
        {kind === 'hostedReview' && hostedReview ? <p>#{hostedReview.number}</p> : null}
        <Button type="button" size="sm" disabled={!canApply} onClick={apply}>
          {translate('auto.components.reviewMap.shell.scope.apply', 'Apply')}
        </Button>
      </PopoverContent>
    </Popover>
  )
}
