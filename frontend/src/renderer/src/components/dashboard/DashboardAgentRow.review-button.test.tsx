import { renderToStaticMarkup } from 'react-dom/server'
import { describe, expect, it, vi } from 'vitest'
import type { AgentStatusEntry } from '../../../../shared/agent-status-types'
import type { TerminalTab } from '../../../../shared/types'
import { TooltipProvider } from '../ui/tooltip'
import DashboardAgentRow from './DashboardAgentRow'
import { CompactAgentRow } from '../sidebar/worktree-card-compact-agent-row'
import type { DashboardAgentRow as DashboardAgentRowData } from './useDashboardData'

function makeAgent(
  overrides: Partial<DashboardAgentRowData> = {},
  entryOverrides: Partial<AgentStatusEntry> = {}
): DashboardAgentRowData {
  const tab = {
    id: 'tab-1',
    ptyId: null,
    worktreeId: 'wt-1',
    title: 'T',
    customTitle: null,
    color: null,
    sortOrder: 0,
    createdAt: 1
  } as TerminalTab
  const entry: AgentStatusEntry = {
    state: 'done',
    prompt: 'Do it',
    updatedAt: 60_000,
    stateStartedAt: 60_000,
    agentType: 'codex',
    paneKey: 'tab-1:leaf-1',
    stateHistory: [],
    ...entryOverrides
  }
  return {
    paneKey: entry.paneKey,
    entry,
    tab,
    agentType: 'codex',
    state: entry.state,
    startedAt: 60_000,
    ...overrides
  }
}

const renderDashboard = (
  agent: DashboardAgentRowData,
  extra: Partial<React.ComponentProps<typeof DashboardAgentRow>> = {}
): string =>
  renderToStaticMarkup(
    <TooltipProvider>
      <DashboardAgentRow
        agent={agent}
        onDismiss={vi.fn()}
        onActivate={vi.fn()}
        now={120_000}
        hideIdentityIcon
        hideExpand
        onReview={vi.fn()}
        {...extra}
      />
    </TooltipProvider>
  )

const renderCompact = (
  agent: DashboardAgentRowData,
  extra: Partial<React.ComponentProps<typeof CompactAgentRow>> = {}
): string =>
  renderToStaticMarkup(
    <CompactAgentRow agent={agent} now={120_000} onActivate={vi.fn()} onReview={vi.fn()} {...extra} />
  )

describe('Review button on agent rows', () => {
  it('shows for done rows (dashboard and compact)', () => {
    expect(renderDashboard(makeAgent())).toContain('data-agent-review-button')
    expect(renderCompact(makeAgent())).toContain('data-agent-review-button')
  })

  it('hides for working and subagent rows', () => {
    const working = makeAgent({}, { state: 'working' })
    expect(renderDashboard(working)).not.toContain('data-agent-review-button')
    expect(renderCompact(working)).not.toContain('data-agent-review-button')
    const sub = makeAgent({ rowSource: 'subagent' })
    expect(renderDashboard(sub)).not.toContain('data-agent-review-button')
    expect(renderCompact(sub)).not.toContain('data-agent-review-button')
  })

  it('hides in send-target mode', () => {
    expect(renderDashboard(makeAgent(), { sendTargetStatus: 'eligible' })).not.toContain(
      'data-agent-review-button'
    )
    expect(renderCompact(makeAgent(), { sendTargetStatus: 'eligible' })).not.toContain(
      'data-agent-review-button'
    )
  })

  it('does not render without onReview', () => {
    expect(renderDashboard(makeAgent(), { onReview: undefined })).not.toContain(
      'data-agent-review-button'
    )
    expect(renderCompact(makeAgent(), { onReview: undefined })).not.toContain(
      'data-agent-review-button'
    )
  })

  it('uses the interrupted tooltip and is always visible when unvisited', () => {
    const markup = renderDashboard(makeAgent({}, { interrupted: true }), { isUnvisited: true })
    expect(markup).toContain('agent was interrupted')
    const btn = markup.match(/<button[^>]*data-agent-review-button[^>]*>/)?.[0] ?? ''
    expect(btn).not.toContain('group-hover')
    const hovered = renderDashboard(makeAgent())
    expect(hovered.match(/<button[^>]*data-agent-review-button[^>]*>/)?.[0]).toContain(
      'group-hover/agent-row:opacity-100'
    )
  })
})
