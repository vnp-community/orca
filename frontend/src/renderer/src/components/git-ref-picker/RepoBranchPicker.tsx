import { useEffect, useRef } from 'react'
import { RepoBranchListPicker } from './RepoBranchListPicker'
import { RepoRefPicker } from './RepoRefPicker'
import { useRepoBranchList } from './use-repo-branch-list'

export type RepoBranchPickerProps = {
  repoId: string
  /** The ref in effect; empty or undefined means the repository default. */
  value?: string
  /** The repository default as `repo.baseRefDefault` names it; null when it cannot be named. */
  defaultName: string | null
  onSelect: (ref: string) => void
  onUseDefault: () => void
}

/**
 * Lists the repo's branches the moment it mounts. A gateway without `repo.listBranches` gets the search-only picker instead,
 * with no error: it is how every picker worked before the list existed.
 */
export function RepoBranchPicker({
  repoId,
  value,
  defaultName,
  onSelect,
  onUseDefault
}: RepoBranchPickerProps): React.JSX.Element {
  const list = useRepoBranchList(repoId)
  const containerRef = useRef<HTMLDivElement>(null)
  const unsupported = list.status === 'unsupported'

  // The list's search box had focus while loading; the fallback's is a new element, so hand focus over.
  useEffect(() => {
    const container = containerRef.current
    if (unsupported && container && !container.contains(document.activeElement)) {
      container.querySelector('input')?.focus()
    }
  }, [unsupported])

  return (
    <div ref={containerRef}>
      {list.status === 'unsupported' ? (
        <RepoRefPicker
          repoId={repoId}
          value={value || undefined}
          onSelect={onSelect}
          onUseDefault={onUseDefault}
        />
      ) : (
        <RepoBranchListPicker
          repoId={repoId}
          list={list}
          value={value}
          defaultName={defaultName}
          onSelect={onSelect}
          onUseDefault={onUseDefault}
        />
      )}
    </div>
  )
}
