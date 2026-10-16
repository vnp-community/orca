/**
 * Steps shared by the QA specs of 2026-10-16 (specs/frontend/bugs/qa-2026-10-16): what the screen shows for a Request's flow,
 * and which directory a terminal opened. Everything a user does goes through the UI; the gateway client only reads.
 */
import { expect, type Page } from '@playwright/test'
import { execFileSync } from 'node:child_process'
import { realpathSync } from 'node:fs'
import type { GatewayRpcClient } from './gateway-rpc-client'
import type { LiveFlow } from './live-flow-harness'
import type { RequestUnderTest } from './live-flow-steps'
import { detailPane } from './live-request-ui-steps'

/** The stage ids of the timeline, in order, as the screen renders them. */
export async function stagesOnScreen(page: Page): Promise<string[]> {
  return detailPane(page)
    .getByTestId('request-stage-timeline')
    .locator('li[data-step-id]')
    .evaluateAll((els) => els.map((e) => (e as HTMLElement).dataset.stepId ?? ''))
}

export async function tabsOnScreen(page: Page): Promise<string[]> {
  const names = await detailPane(page).getByRole('tab').allInnerTexts()
  // Why: a tab can carry an attention dot with its own accessible name; keep the visible word only.
  return names.map((n) => n.replace(/\s+/g, ' ').trim().split(' ')[0])
}

/** The flow of the Request's type as the user sees it: stages, tabs and whether a Handover row exists. */
export async function expectStagesAndTabs(
  f: LiveFlow,
  expected: { stages: string[]; tabs: string[]; engine: 'native' | 'openspec'; requestId: string }
): Promise<void> {
  await f.step(
    `the screen shows stages ${expected.stages.join(' > ')} and tabs ${expected.tabs.join(', ')}`,
    async () => {
      await expect
        .poll(() => stagesOnScreen(f.page), { timeout: 20_000, message: 'stages in the timeline' })
        .toEqual(expected.stages)
      await expect
        .poll(() => tabsOnScreen(f.page), { timeout: 20_000, message: 'tabs of the detail pane' })
        .toEqual(expected.tabs)
      const req = await f.rpc.call<{ engine?: string }>('request.get', { id: expected.requestId })
      const got = (req as { request?: { engine?: string } }).request?.engine ?? req.engine
      expect(got, `engine of the Request reported by the service`).toBe(expected.engine)
      if (expected.engine === 'native') {
        await expect(
          f.page.locator('[data-step-id="handover"]'),
          'a native Request has no Handover stage'
        ).toHaveCount(0)
      }
      await f.shots.take(`stages-${expected.engine}`)
    }
  )
}

/** Plan tab of a non-continuous project: the approval bar names what is approved. */
export async function expectApprovalLabel(
  f: LiveFlow,
  r: RequestUnderTest,
  label: RegExp
): Promise<void> {
  await f.step(`the Plan tab offers "${label}"`, async () => {
    await detailPane(f.page).getByRole('tab', { name: /^Plan/ }).click()
    await expect(detailPane(f.page).getByTestId('plan-approve')).toHaveText(label, {
      timeout: 30_000
    })
  })
}

type TerminalRow = { ptyId?: string; cwd?: string; connectionId?: string; createdAt?: number }

export async function listTerminals(rpc: GatewayRpcClient): Promise<TerminalRow[]> {
  const rows = await rpc.call<TerminalRow[] | { terminals?: TerminalRow[] }>('terminal.list', {})
  return Array.isArray(rows) ? rows : (rows.terminals ?? [])
}

/** The terminal sessions created after `knownPtyIds` was taken. The cwd is what the agent actually opened. */
export async function newTerminalsSince(
  rpc: GatewayRpcClient,
  known: Set<string>
): Promise<TerminalRow[]> {
  return (await listTerminals(rpc)).filter((t) => t.ptyId && !known.has(t.ptyId))
}

export function sameDirectory(a: string, b: string): boolean {
  try {
    return realpathSync(a) === realpathSync(b)
  } catch {
    return a === b
  }
}

/** `git worktree add` next to the repo, for a sidebar row with a branch name nobody else uses. */
export function addWorktree(repoPath: string, branch: string): string {
  const path = `${repoPath}-${branch.replace(/[^a-zA-Z0-9]+/g, '-')}`
  execFileSync('git', ['worktree', 'add', '-q', '-b', branch, path], { cwd: repoPath })
  return path
}

export function removeWorktree(repoPath: string, worktreePath: string): void {
  try {
    execFileSync('git', ['worktree', 'remove', '--force', worktreePath], {
      cwd: repoPath,
      stdio: 'ignore'
    })
  } catch {
    /* already gone */
  }
}

/** The path of the project's repo as the dev server sees it (repo.list carries it in `url`). */
export async function repoPathOf(
  rpc: GatewayRpcClient,
  projectId: string
): Promise<{ id: string; path: string }> {
  const res = await rpc.call<{ repos?: { id: string; projectId: string; url: string }[] }>(
    'repo.list',
    {}
  )
  const repo = (res.repos ?? []).find((r) => r.projectId === projectId)
  expect(repo, `repo.list has a repo for project ${projectId}`).toBeTruthy()
  return { id: repo!.id, path: repo!.url }
}
