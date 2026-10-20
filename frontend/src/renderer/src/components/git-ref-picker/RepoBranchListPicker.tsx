import { useEffect, useId, useRef, useState } from 'react'
import { Check } from 'lucide-react'
import { Input } from '../ui/input'
import { cn } from '@/lib/utils'
import { translate } from '@/i18n/i18n'
import { MiddleTruncatedRef } from './MiddleTruncatedRef'
import { buildBranchSections, type BranchOption } from './branch-list-sections'
import { HEADING_LABEL, OptionTag, StatusLine, optionLabel } from './repo-branch-list-parts'
import { useRepoRefSearch } from './use-repo-ref-search'
import type { RepoBranchList } from './use-repo-branch-list'

export type RepoBranchListPickerProps = {
  repoId: string
  list: RepoBranchList
  /** The ref in effect; empty or undefined means the repository default. */
  value?: string
  /** The repository default as `repo.baseRefDefault` names it; null when it cannot be named. */
  defaultName: string | null
  onSelect: (ref: string) => void
  onUseDefault: () => void
}

/**
 * The branch dropdown: the repository default pinned on top, then local and remote-only branches, with a search box that
 * filters the loaded list in place and, only when the host cut the list, also searches the host.
 */
export function RepoBranchListPicker({
  repoId,
  list,
  value,
  defaultName,
  onSelect,
  onUseDefault
}: RepoBranchListPickerProps): React.JSX.Element {
  const ready = list.status === 'ready'
  const branches = list.status === 'ready' ? list.branches : []
  const truncated = list.status === 'ready' && list.truncated
  const search = useRepoRefSearch(repoId, { searchEnabled: truncated, lookupDefault: false })
  const { query, setQuery } = search
  const baseId = useId()
  const listId = `${baseId}-list`
  const optionId = (index: number): string => `${baseId}-opt-${index}`
  const listRef = useRef<HTMLDivElement>(null)

  const sections = buildBranchSections({
    branches,
    defaultName,
    query,
    searchMatches: search.results
  })
  const options = sections.flatMap((section) => section.options)
  const usingDefault = !value || value === defaultName

  const optionsKey = options.map((o) => o.key).join('\n')
  const [highlight, setHighlight] = useState<{ key: string; index: number } | null>(null)
  const activeIndex =
    options.length === 0 ? -1 : highlight?.key === optionsKey ? highlight.index : 0

  // Why: Radix Dialog scroll-lock cancels wheel events on in-dialog scroll regions, so the list scrolls itself.
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

  const isCurrent = (option: BranchOption): boolean =>
    option.kind === 'default' ? usingDefault : value === option.ref

  const choose = (option: BranchOption): void => {
    setQuery('')
    if (option.kind === 'default') {
      onUseDefault()
    } else if (option.ref !== null) {
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

  let emptyText: string | null = null
  if (ready && search.status !== 'searching') {
    if (query.trim() !== '') {
      emptyText =
        options.length === 0
          ? translate(
              'auto.components.gitRefPicker.RepoRefPicker.noMatches',
              'No matching branches found.'
            )
          : null
    } else if (branches.length === 0) {
      emptyText = translate(
        'auto.components.gitRefPicker.RepoBranchPicker.noBranches',
        'No branches found'
      )
    }
  }

  // No search box yet: the answer may swap this picker for the search-only one, which would drop what was typed.
  if (list.status === 'loading') {
    return (
      <div data-slot="repo-branch-list-picker">
        <StatusLine
          loading
          failed={false}
          searching={false}
          searchFailed={false}
          emptyText={null}
          onRetry={list.retry}
        />
      </div>
    )
  }

  let flatIndex = -1
  return (
    <div className="space-y-2.5" data-slot="repo-branch-list-picker">
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
        // Why: the popover opens on the list, so typing must start at once; focus would otherwise sit on the popover frame.
        autoFocus
        value={query}
        onChange={(e) => setQuery(e.target.value)}
        onKeyDown={onKeyDown}
        placeholder={translate(
          'auto.components.gitRefPicker.RepoRefPicker.placeholder',
          'Search branches by name...'
        )}
        className="max-w-md"
      />

      <StatusLine
        loading={false}
        failed={list.status === 'error'}
        searching={search.status === 'searching'}
        searchFailed={search.status === 'error' || search.status === 'offline'}
        emptyText={emptyText}
        onRetry={list.status === 'error' ? list.retry : search.retry}
      />

      {options.length > 0 ? (
        <div
          ref={listRef}
          className="max-h-[min(14rem,40vh)] overflow-y-auto overflow-x-hidden rounded-md border border-border/50 scrollbar-sleek"
        >
          <div
            id={listId}
            role="listbox"
            aria-label={translate(
              'auto.components.gitRefPicker.RepoBranchPicker.listLabel',
              'Branches'
            )}
            className="p-1"
          >
            {sections.map((section) => {
              const headingId = `${baseId}-${section.id}`
              return (
                <div
                  key={section.id}
                  role={section.id === 'default' ? 'presentation' : 'group'}
                  aria-labelledby={section.id === 'default' ? undefined : headingId}
                >
                  {section.id === 'default' ? null : (
                    <div
                      id={headingId}
                      className="px-3 pb-1 pt-2 text-[10px] uppercase tracking-[0.18em] text-muted-foreground"
                    >
                      {HEADING_LABEL[section.id]()}
                    </div>
                  )}
                  {section.options.map((option) => {
                    flatIndex += 1
                    const index = flatIndex
                    const current = isCurrent(option)
                    return (
                      // Focus stays in the search field (aria-activedescendant), so the rows are pointer-only targets.
                      <div
                        key={option.key}
                        id={optionId(index)}
                        role="option"
                        // Why: the name must be the whole ref plus its tag; read from content, the split label would sound like two words.
                        aria-label={optionLabel(option)}
                        aria-selected={current}
                        onMouseDown={(e) => e.preventDefault()}
                        onMouseMove={() =>
                          index !== activeIndex && setHighlight({ key: optionsKey, index })
                        }
                        onClick={() => choose(option)}
                        className={cn(
                          'flex w-full cursor-pointer items-center justify-between gap-2 rounded-sm px-3 py-2 text-left text-sm transition-colors',
                          current ? 'bg-accent text-accent-foreground' : 'text-foreground',
                          index === activeIndex && 'bg-muted/60 ring-1 ring-inset ring-ring/40'
                        )}
                      >
                        <span className="flex min-w-0 flex-1 items-center gap-2">
                          {option.ref === null ? (
                            <span className="truncate">{optionLabel(option)}</span>
                          ) : (
                            <span className="flex min-w-0 flex-1 font-mono text-xs">
                              <MiddleTruncatedRef value={option.ref} />
                            </span>
                          )}
                          <OptionTag kind={option.kind} />
                        </span>
                        {current ? (
                          <Check className="size-3.5 shrink-0" aria-hidden="true" />
                        ) : null}
                      </div>
                    )
                  })}
                </div>
              )
            })}
          </div>
        </div>
      ) : null}
    </div>
  )
}
