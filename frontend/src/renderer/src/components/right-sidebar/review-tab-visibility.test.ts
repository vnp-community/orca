import { describe, expect, it } from 'vitest'
import { Files } from 'lucide-react'
import type { ActivityBarItem } from './activity-bar-buttons'
import { getVisibleRightSidebarActivityItems } from './right-sidebar-activity-visibility'
import { normalizeRightSidebarRoute } from '@/store/right-sidebar-route'

const items: ActivityBarItem[] = [
  { id: 'explorer', icon: Files, title: 'Explorer', shortcut: '' },
  {
    id: 'review',
    icon: Files,
    title: 'Review',
    shortcut: '',
    gitOnly: true,
    codeIntelOnly: true
  }
]
const base = { isFolder: false, isFolderWorkspace: false, isSshRepo: false }

describe('Review sidebar tab', () => {
  it('is visible only with the flag on and a git worktree', () => {
    const ids = (o: Parameters<typeof getVisibleRightSidebarActivityItems>[1]): string[] =>
      getVisibleRightSidebarActivityItems(items, o).map((i) => i.id)
    expect(ids(base)).toEqual(['explorer'])
    expect(ids({ ...base, codeIntelEnabled: true })).toEqual(['explorer', 'review'])
    expect(ids({ ...base, isFolder: true, codeIntelEnabled: true })).toEqual(['explorer'])
  })

  it('normalizeRightSidebarRoute accepts review and leaves old tabs alone', () => {
    expect(normalizeRightSidebarRoute('review').rightSidebarTab).toBe('review')
    expect(normalizeRightSidebarRoute('checks').rightSidebarTab).toBe('checks')
    expect(normalizeRightSidebarRoute('bogus').rightSidebarTab).toBe('explorer')
  })
})
