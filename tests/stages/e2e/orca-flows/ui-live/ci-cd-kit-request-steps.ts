/**
 * First half of the ci-cd-kit run, all through the UI: the Create request dialog with the starting branch picked from its list,
 * the classification the AI proposes and the language of the Analysis (the Clarification is in ci-cd-kit-clarification-steps.ts).
 */
import { expect } from '@playwright/test'
import {
  CICDKIT_BRANCH,
  CICDKIT_PROJECT_NAME,
  CICDKIT_REPO,
  CICDKIT_TITLE,
  requestBody,
  type BacklogItem
} from './ci-cd-kit-backlog'
import { ownRequestIds } from './ci-cd-kit-fixture'
import type { Timeline } from './ci-cd-kit-report'
import { callsTo, type SentRpc } from './live-add-project-own-project-steps'
import {
  branchList,
  chooseStartingBranch,
  openCreateDialog,
  startFromTrigger,
  storedBaseRef
} from './live-base-branch-steps'
import type { LiveFlow } from './live-flow-harness'
import type { RequestUnderTest } from './live-flow-steps'
import { waitForApiStatus } from './live-gateway-crosschecks'
import type { RpcRow } from './gateway-rpc-client'
import {
  confirmTypeInCard,
  detailHeader,
  detailPane,
  openDetailTab,
  openRequestsPage
} from './live-request-ui-steps'
import { expectScreenFollowsService } from './live-status-sync'
import { vietnameseShare } from './live-vietnamese-text-check'

async function requestNumbers(f: LiveFlow): Promise<number[]> {
  const listed = await f.rpc.call<{ requests?: RpcRow[] }>('request.list', {
    projectId: f.project.id
  })
  return (listed.requests ?? []).map((x) => Number(x.number))
}

async function newestWithTitle(
  f: LiveFlow,
  title: string,
  before: number[]
): Promise<RpcRow | undefined> {
  const listed = await f.rpc.call<{ requests?: RpcRow[] }>('request.list', {
    projectId: f.project.id
  })
  return (listed.requests ?? []).find(
    (x) => x.title === title && !before.includes(Number(x.number))
  )
}

export type ClassificationSeen = { type: string; size: string }

/** Create request: title, backlog text, the project by its name, the branch from the dropdown list; one request.create carries them. */
export async function createRequestFromBacklog(
  f: LiveFlow,
  tap: { calls: SentRpc[] },
  item: BacklogItem,
  timeline: Timeline,
  // Why: the rehearsal (ORCA_E2E_CICDKIT_STOP=clarify) must not be taken for the real Request by title.
  title = CICDKIT_TITLE
): Promise<RequestUnderTest> {
  const body = requestBody(item)
  let requestId = ''
  await f.step(
    'create the Request from BL-E07, starting from the branch picked in the list',
    async () => {
      await openRequestsPage(f.page)
      const numbersBefore = await requestNumbers(f)
      const from = tap.calls.length
      const dialog = await openCreateDialog(f.page, {
        title,
        body,
        projectName: CICDKIT_PROJECT_NAME
      })
      await expect(
        dialog.getByTestId('create-request-repo-note'),
        'the note names the repo, its path and the host'
      ).toContainText(`${CICDKIT_PROJECT_NAME} (${CICDKIT_REPO}) on ${f.cfg.devServerName}`, {
        timeout: 15_000
      })
      await chooseStartingBranch(f.page, CICDKIT_BRANCH)
      await expect(startFromTrigger(f.page)).toContainText(CICDKIT_BRANCH)
      // Why: picking a branch leaves its list open; Escape closes only that list, the dialog stays.
      if (await branchList(f.page).isVisible()) {
        await f.page.keyboard.press('Escape')
        await expect(branchList(f.page)).toBeHidden({ timeout: 10_000 })
      }
      await f.shots.take('create-request-dialog')
      await dialog.getByRole('button', { name: 'Create', exact: true }).click()
      await expect(dialog).toBeHidden({ timeout: 30_000 })
      await expect(detailHeader(f.page)).toContainText(title)
      // Why: an older Request (cancelled attempt) can carry the same title; the new one is the one numbered above every earlier one.
      await expect
        .poll(async () => (await newestWithTitle(f, title, numbersBefore))?.id ?? '', {
          timeout: 15_000
        })
        .not.toBe('')
      requestId = (await newestWithTitle(f, title, numbersBefore))!.id
      ownRequestIds.add(requestId)
      const sent = callsTo(tap, 'request.create', from)
      expect(sent, 'the SPA sent one request.create').toHaveLength(1)
      expect(sent[0].params, 'the call carries project, title, body and the branch').toMatchObject({
        projectId: f.project.id,
        title,
        body,
        baseRef: CICDKIT_BRANCH
      })
      expect(await storedBaseRef(f, requestId), 'the service stored the branch').toBe(
        CICDKIT_BRANCH
      )
      timeline.mark('request created')
      await f.shots.take('request-created')
    }
  )
  return { requestId, title }
}

/** The AI proposes a type and a size; they are read off the card, then change_request / M is confirmed. */
export async function classifyAndConfirm(
  f: LiveFlow,
  r: RequestUnderTest,
  timeline: Timeline
): Promise<ClassificationSeen> {
  const seen: ClassificationSeen = { type: '', size: '' }
  await f.step('the AI classifies; change_request, size M, is confirmed on the card', async () => {
    const lag = await expectScreenFollowsService(f.page, f.rpc, f.gaps, {
      requestId: r.requestId,
      title: r.title,
      step: 'CLASSIFY',
      apiStatus: ['awaiting_type_confirmation'],
      chip: /Awaiting Type Confirmation/i,
      apiWaitMs: f.budgets.classifyMs
    })
    f.lags.push(`classification ${lag}ms`)
    timeline.mark('classified')
    const card = f.page.getByTestId('type-confirmation-card')
    await expect(card).toBeVisible()
    // The box also carries the type's description on a second line.
    seen.type = (await card.getByRole('combobox').first().innerText()).split('\n')[0].trim()
    seen.size = (await card.getByRole('combobox', { name: 'Size' }).innerText()).trim()
    console.log(`[ci-cd-kit] the AI proposed type "${seen.type}", size "${seen.size}"`)
    await f.shots.take('type-confirmation')
    await confirmTypeInCard(f.page, 'Change request', 'M')
    const after = await waitForApiStatus(
      f.rpc,
      r.requestId,
      ['awaiting_information', 'analyzing', 'planning'],
      30_000
    )
    expect(after.type).toBe('change_request')
    timeline.mark('type confirmed')
    await f.shots.take('type-confirmed')
  })
  return seen
}

export type LanguageFinding = {
  service: ReturnType<typeof vietnameseShare>
  screen: ReturnType<typeof vietnameseShare>
  isVietnamese: boolean
  sample: string
}

function stringLeaves(value: unknown, out: string[] = []): string[] {
  if (typeof value === 'string') {
    out.push(value)
  } else if (Array.isArray(value)) {
    value.forEach((v) => stringLeaves(v, out))
  } else if (value && typeof value === 'object') {
    Object.values(value).forEach((v) => stringLeaves(v, out))
  }
  return out
}

const looksVietnamese = (share: ReturnType<typeof vietnameseShare>): boolean =>
  share.accentedLetterRatio >= 0.15 &&
  share.vietnameseStopWords >= 3 &&
  share.englishStopWordRatio < 0.1

/** What language the Analysis is written in, from the stored document and from the Analysis tab. Recorded, never a reason to stop. */
export async function recordAnalysisLanguage(
  f: LiveFlow,
  r: RequestUnderTest
): Promise<LanguageFinding> {
  const listed = await f.rpc.call<{ solutions?: { status: string; options?: unknown }[] }>(
    'solution.list',
    { requestId: r.requestId }
  )
  const live = (listed.solutions ?? []).findLast((s) => s.status !== 'superseded')
  const text = stringLeaves(live?.options).join('\n')
  await openDetailTab(f.page, 'Analysis')
  const onScreen = await detailPane(f.page).innerText()
  const finding: LanguageFinding = {
    service: vietnameseShare(text),
    screen: vietnameseShare(onScreen),
    isVietnamese: false,
    sample: text.replace(/\s+/g, ' ').slice(0, 300)
  }
  finding.isVietnamese = looksVietnamese(finding.service)
  console.log(
    `[ci-cd-kit] Analysis language: ${finding.isVietnamese ? 'Vietnamese' : 'NOT Vietnamese'}; stored document accented ${(finding.service.accentedLetterRatio * 100).toFixed(1)}%, Vietnamese words ${finding.service.vietnameseStopWords}, English ratio ${(finding.service.englishStopWordRatio * 100).toFixed(1)}%; sample: ${finding.sample}`
  )
  return finding
}
