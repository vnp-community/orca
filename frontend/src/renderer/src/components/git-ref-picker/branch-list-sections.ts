import type { RepoBranchEntry } from '../../../../shared/types'

/** `ref` is null for the default row of a repo whose default branch could not be named. */
export type BranchOption = {
  key: string
  kind: 'default' | 'local' | 'remote' | 'match'
  ref: string | null
}

/** The default row is pinned with no heading; `match` holds search hits beyond a truncated list. */
export type BranchSection = {
  id: 'default' | 'local' | 'remote' | 'match'
  options: BranchOption[]
}

type Input = {
  branches: readonly RepoBranchEntry[]
  /** The repository default as `repo.baseRefDefault` names it; null when it cannot be named. */
  defaultName: string | null
  query: string
  /** Host search hits, only used when the list was truncated. */
  searchMatches: readonly string[]
}

/**
 * Pinned default, then local branches, then remote-only ones, then search hits the loaded list lacks. Typing filters the
 * loaded list in place; a branch that is the default is shown once, in the pinned row.
 */
export function buildBranchSections({
  branches,
  defaultName,
  query,
  searchMatches
}: Input): BranchSection[] {
  const needle = query.trim().toLowerCase()
  const matches = (ref: string): boolean => needle === '' || ref.toLowerCase().includes(needle)

  const sections: BranchSection[] = []
  const pinned: BranchOption[] = []
  if (defaultName === null) {
    // An unnamed default is only a way back to "no choice"; it means nothing while the user is searching.
    if (needle === '') {
      pinned.push({ key: '\u0000default', kind: 'default', ref: null })
    }
  } else if (matches(defaultName)) {
    pinned.push({ key: defaultName, kind: 'default', ref: defaultName })
  }
  sections.push({ id: 'default', options: pinned })

  const seen = new Set<string>(defaultName === null ? [] : [defaultName])
  const take = (kind: 'local' | 'remote' | 'match', refs: Iterable<string>): BranchOption[] => {
    const options: BranchOption[] = []
    for (const ref of refs) {
      if (!seen.has(ref) && matches(ref)) {
        seen.add(ref)
        options.push({ key: ref, kind, ref })
      }
    }
    return options
  }
  const names = (remote: boolean): string[] =>
    // The gateway omits `remote` for local branches (proto3 zero value), so absent means local.
    branches.filter((b) => (b.remote === true) === remote).map((b) => b.name)
  sections.push({ id: 'local', options: take('local', names(false)) })
  sections.push({ id: 'remote', options: take('remote', names(true)) })
  // Taken last so a hit already in the list keeps its place there.
  sections.push({ id: 'match', options: take('match', needle === '' ? [] : searchMatches) })
  return sections.filter((section) => section.options.length > 0)
}
