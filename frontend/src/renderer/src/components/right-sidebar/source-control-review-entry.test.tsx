import { renderToStaticMarkup } from 'react-dom/server'
import { describe, expect, it, vi } from 'vitest'
import { TooltipProvider } from '@/components/ui/tooltip'

const storeState = {
  codeIntelSupportState: { state: 'enabled' as string },
  worktreesByRepo: { r1: [{ id: 'r1::/p', repoId: 'r1', projectId: 'p1' }] },
  repos: [{ id: 'r1', projectId: 'p1' }],
  agentStatusByPaneKey: {},
  retainedAgentsByPaneKey: {},
  tabsByWorktree: {}
}
vi.mock('@/store', () => ({
  useAppStore: Object.assign(
    (sel: (s: typeof storeState) => unknown) => sel(storeState),
    { getState: () => storeState }
  )
}))
vi.mock('@/lib/worktree-runtime-owner', () => ({
  getRuntimeEnvironmentIdForWorktree: () => null
}))

import { SourceControlHeaderOverflowMenu } from './source-control-header-overflow-menu'
import { SourceControlBranchContextRow } from './source-control-branch-context-row'
import { useSourceControlReviewEntry } from './source-control-review-entry'

function Probe({ id }: { id: string | null }): React.JSX.Element {
  const e = useSourceControlReviewEntry({ worktreeId: id })
  return <span data-visible={String(e.visible)} />
}

describe('useSourceControlReviewEntry', () => {
  it('is visible only with the flag on and an addressable worktree', () => {
    storeState.codeIntelSupportState = { state: 'enabled' }
    expect(renderToStaticMarkup(<Probe id="r1::/p" />)).toContain('data-visible="true"')
    expect(renderToStaticMarkup(<Probe id="folder:x" />)).toContain('data-visible="false"')
    expect(renderToStaticMarkup(<Probe id={null} />)).toContain('data-visible="false"')
    storeState.codeIntelSupportState = { state: 'disabled' }
    expect(renderToStaticMarkup(<Probe id="r1::/p" />)).toContain('data-visible="false"')
  })
})

describe('branch context row Review button', () => {
  const summary = {
    baseRef: 'origin/main',
    baseOid: 'a',
    compareRef: 'feat',
    headOid: 'b',
    mergeBase: 'a',
    changedFiles: 3,
    status: 'ready' as const
  }
  const render = (changed: number, onReviewChanges?: () => void): string =>
    renderToStaticMarkup(
      <TooltipProvider>
        <SourceControlBranchContextRow
          summary={{ ...summary, changedFiles: changed }}
          compareBaseRef="origin/main"
          onChangeBaseRef={() => {}}
          onRetry={() => {}}
          onReviewChanges={onReviewChanges}
        />
      </TooltipProvider>
    )
  it('renders only with a handler and changed files', () => {
    expect(render(3, () => {})).toContain('Review changes')
    expect(render(0, () => {})).not.toContain('Review changes')
    expect(render(3)).not.toContain('Review changes')
  })
})

describe('overflow menu', () => {
  it('compiles with the optional review handler', () => {
    expect(typeof SourceControlHeaderOverflowMenu).toBe('function')
  })
})
