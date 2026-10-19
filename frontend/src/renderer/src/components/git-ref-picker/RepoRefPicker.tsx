import { useEffect, useId, useRef, useState } from 'react'
import { Check, Loader2 } from 'lucide-react'
import { Button } from '../ui/button'
import { Input } from '../ui/input'
import { cn } from '@/lib/utils'
import { translate } from '@/i18n/i18n'
import { MiddleTruncatedRef } from './MiddleTruncatedRef'
import {
  useRepoRefSearch,
  type RepoRefDefaultStatus,
  type RepoRefSearchStatus
} from './use-repo-ref-search'

export type RepoRefPickerHeaderContext = {
  defaultBaseRef: string | null
  defaultStatus: RepoRefDefaultStatus
  remoteCount: number
  /** The "multiple remotes" tip, or null; a custom header places it where it wants. */
  remotesHint: React.ReactNode
}

export type RepoRefPickerProps = {
  repoId: string
  /** The ref in effect; marked in the list. Empty or undefined means the repository default. */
  value?: string
  onSelect: (ref: string) => void
  /** Adds a "Default branch" option on top of the idle list, for callers that can fall back to the default. */
  onUseDefault?: () => void
  placeholder?: string
  disabled?: boolean
  /** Replaces the plain tip above the search field (Settings shows what the repo currently follows). */
  renderHeader?: (context: RepoRefPickerHeaderContext) => React.ReactNode
  className?: string
}

type PickerOption = { kind: 'default' } | { kind: 'ref'; ref: string }

function RemotesHint(): React.JSX.Element {
  // Why: static instructional copy, so no aria-live; it would be re-announced on every mount or repo switch.
  return (
    <p className="text-xs text-muted-foreground">
      {translate(
        'auto.components.gitRefPicker.RepoRefPicker.remotesHintPrefix',
        'Multiple remotes detected. Type a remote name (e.g.'
      )}
      <code>
        {translate('auto.components.gitRefPicker.RepoRefPicker.remoteExample', 'upstream')}
      </code>
      {translate(
        'auto.components.gitRefPicker.RepoRefPicker.remotesHintMiddle',
        ') or a full ref (e.g.'
      )}
      <code>
        {translate('auto.components.gitRefPicker.RepoRefPicker.refExample', 'upstream/main')}
      </code>
      {translate(
        'auto.components.gitRefPicker.RepoRefPicker.remotesHintSuffix',
        ') to scope results.'
      )}
    </p>
  )
}

function SearchStatusLine({
  status,
  resultCount,
  onRetry
}: {
  status: RepoRefSearchStatus
  resultCount: number
  onRetry: () => void
}): React.JSX.Element {
  // Always mounted so a screen reader announces a change of state; the visible text doubles as the announcement.
  return (
    <div role="status" aria-live="polite" className="text-xs text-muted-foreground">
      {status === 'searching' ? (
        <p className="flex items-center gap-1.5">
          <Loader2 className="size-3 animate-spin" aria-hidden="true" />
          {translate(
            'auto.components.gitRefPicker.RepoRefPicker.searching',
            'Searching branches...'
          )}
        </p>
      ) : null}
      {status === 'empty' ? (
        <p>
          {translate(
            'auto.components.gitRefPicker.RepoRefPicker.noMatches',
            'No matching branches found.'
          )}
        </p>
      ) : null}
      {status === 'results' ? (
        <p className="sr-only">
          {translate(
            'auto.components.gitRefPicker.RepoRefPicker.resultCount',
            '{{number}} matching branches',
            { number: resultCount }
          )}
        </p>
      ) : null}
      {status === 'error' || status === 'offline' ? (
        <div className="flex flex-wrap items-center gap-2">
          <p className="text-destructive">
            {status === 'offline'
              ? translate(
                  'auto.components.gitRefPicker.RepoRefPicker.offline',
                  'Cannot reach the host to search branches. Check the connection and try again.'
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
    </div>
  )
}

export function RepoRefPicker({
  repoId,
  value,
  onSelect,
  onUseDefault,
  placeholder,
  disabled,
  renderHeader,
  className
}: RepoRefPickerProps): React.JSX.Element {
  const search = useRepoRefSearch(repoId)
  const { query, setQuery, results, status, defaultBaseRef, defaultStatus, remoteCount } = search
  const baseId = useId()
  const listId = `${baseId}-list`
  const optionId = (index: number): string => `${baseId}-opt-${index}`
  const listRef = useRef<HTMLDivElement>(null)
  const effectiveRef = value || defaultBaseRef
  const usingDefault = !value

  const options: PickerOption[] = []
  if (onUseDefault && query.trim() === '') {
    options.push({ kind: 'default' })
  }
  for (const ref of results) {
    options.push({ kind: 'ref', ref })
  }

  // The highlighted row follows the option list: a new list starts at its first row, so Enter picks the top match.
  const optionsKey = options.map((o) => (o.kind === 'default' ? '\u0000default' : o.ref)).join('\n')
  const [highlight, setHighlight] = useState<{ key: string; index: number } | null>(null)
  const activeIndex =
    options.length === 0 ? -1 : highlight?.key === optionsKey ? highlight.index : 0

  // Why: Radix Dialog scroll-lock cancels wheel events on in-dialog scroll regions, so we scroll the list manually
  // (same pattern as CommandList).
  useEffect(() => {
    const el = listRef.current
    if (!el) {
      return
    }
    const onWheel = (event: WheelEvent): void => {
      if (el.scrollHeight <= el.clientHeight) {
        return
      }
      event.preventDefault()
      el.scrollTop += event.deltaY
    }
    el.addEventListener('wheel', onWheel, { passive: false })
    return () => el.removeEventListener('wheel', onWheel)
  }, [options.length])

  const activeOptionId = activeIndex >= 0 ? optionId(activeIndex) : undefined
  useEffect(() => {
    if (activeOptionId) {
      document.getElementById(activeOptionId)?.scrollIntoView?.({ block: 'nearest' })
    }
  }, [activeOptionId])

  const choose = (option: PickerOption): void => {
    // Why: back to the resting state after a pick, so the old query does not look like a pending choice.
    setQuery('')
    if (option.kind === 'default') {
      onUseDefault?.()
    } else {
      onSelect(option.ref)
    }
  }

  const moveHighlight = (delta: number): void => {
    setHighlight({
      key: optionsKey,
      index: (activeIndex + delta + options.length) % options.length
    })
  }

  const onKeyDown = (event: React.KeyboardEvent<HTMLInputElement>): void => {
    if (event.nativeEvent.isComposing) {
      return
    }
    if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
      if (options.length > 0) {
        event.preventDefault()
        moveHighlight(event.key === 'ArrowDown' ? 1 : -1)
      }
    } else if (event.key === 'Enter') {
      // A half-typed query must not submit the form around the picker.
      if (activeIndex >= 0) {
        event.preventDefault()
        choose(options[activeIndex])
      } else if (query !== '') {
        event.preventDefault()
      }
    } else if (event.key === 'Escape' && query !== '') {
      // Esc first clears the search; only an untouched field lets the surrounding dialog or popover close.
      event.preventDefault()
      event.stopPropagation()
      setQuery('')
    }
  }

  const remotesHint = remoteCount > 1 ? <RemotesHint /> : null
  const defaultLabel = defaultBaseRef
    ? translate(
        'auto.components.gitRefPicker.RepoRefPicker.defaultNamed',
        'Default branch ({{value0}})',
        { value0: defaultBaseRef }
      )
    : translate('auto.components.gitRefPicker.RepoRefPicker.defaultUnnamed', 'Default branch')

  return (
    <div className={cn('space-y-2.5', className)} data-slot="repo-ref-picker">
      {renderHeader
        ? renderHeader({ defaultBaseRef, defaultStatus, remoteCount, remotesHint })
        : remotesHint}

      <Input
        role="combobox"
        aria-autocomplete="list"
        aria-expanded={options.length > 0}
        aria-controls={listId}
        aria-activedescendant={activeOptionId}
        aria-label={translate(
          'auto.components.gitRefPicker.RepoRefPicker.searchLabel',
          'Search branches'
        )}
        autoComplete="off"
        spellCheck={false}
        value={query}
        disabled={disabled}
        onChange={(e) => setQuery(e.target.value)}
        onKeyDown={onKeyDown}
        placeholder={
          placeholder ??
          translate(
            'auto.components.gitRefPicker.RepoRefPicker.placeholder',
            'Search branches by name...'
          )
        }
        className="max-w-md"
      />

      <SearchStatusLine status={status} resultCount={results.length} onRetry={search.retry} />

      {options.length > 0 ? (
        <div
          ref={listRef}
          className="max-h-[min(12rem,40vh)] overflow-y-auto overflow-x-hidden rounded-md border border-border/50 scrollbar-sleek"
        >
          <div
            id={listId}
            role="listbox"
            aria-label={translate(
              'auto.components.gitRefPicker.RepoRefPicker.listLabel',
              'Matching branches'
            )}
            className="p-1"
          >
            {options.map((option, index) => {
              const label = option.kind === 'default' ? defaultLabel : option.ref
              const current = option.kind === 'default' ? usingDefault : effectiveRef === option.ref
              return (
                // Focus stays in the search field (aria-activedescendant), so the rows are pointer-only targets.
                <div
                  key={option.kind === 'default' ? '\u0000default' : option.ref}
                  id={optionId(index)}
                  role="option"
                  // Why: the name must be the whole ref; read from content, the split label would sound like two words.
                  aria-label={label}
                  aria-selected={current}
                  aria-disabled={disabled || undefined}
                  onMouseDown={(e) => e.preventDefault()}
                  onMouseMove={() =>
                    index !== activeIndex && setHighlight({ key: optionsKey, index })
                  }
                  onClick={() => !disabled && choose(option)}
                  className={cn(
                    'flex w-full cursor-pointer items-center justify-between gap-2 rounded-sm px-3 py-2 text-left text-sm transition-colors',
                    current ? 'bg-accent text-accent-foreground' : 'text-foreground',
                    index === activeIndex && 'bg-muted/60 ring-1 ring-inset ring-ring/40'
                  )}
                >
                  {option.kind === 'default' ? (
                    <span className="truncate">{label}</span>
                  ) : (
                    <MiddleTruncatedRef value={label} />
                  )}
                  {current ? (
                    <span className="flex shrink-0 items-center gap-1 text-[10px] uppercase tracking-[0.18em]">
                      <Check className="size-3" aria-hidden="true" />
                      {translate('auto.components.gitRefPicker.RepoRefPicker.current', 'Current')}
                    </span>
                  ) : null}
                </div>
              )
            })}
          </div>
        </div>
      ) : null}
    </div>
  )
}
