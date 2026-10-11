/**
 * Second half of the add-project spec (TASK-FE-E2E-027): a `question` Request in the project that Add Project made for the bravo
 * repo, answered by the real Claude, read on the Analysis tab (never approved), then cancelled from the UI.
 */
import { expect } from '@playwright/test'
import type { SandboxRepo } from './add-project-two-repos-fixture'
import {
  expectAnswerAboutBravo,
  expectOnlyBravoWasRead
} from './live-add-project-own-project-checks'
import { createRequestInProject, readAnswerOnScreen } from './live-add-project-own-project-steps'
import { plainSnippetOf } from './live-answer-snippet'
import type { LiveFlow } from './live-flow-harness'
import {
  answerClarificationIfAsked,
  cancelThroughTheUi,
  createClassifyAndConfirm
} from './live-flow-steps'
import { requestState } from './live-gateway-crosschecks'

const QUESTION = 'Repo này có những hàm nào và chúng làm gì? Chỉ đọc, không sửa file nào.'

type StoredSolution = { kind: string; status: string; options?: Record<string, unknown> }
type StoredRun = { kind: string; status: string; errorCode?: string; errorMessage?: string }

export async function requestAboutBravoSteps(
  f: LiveFlow,
  o: {
    alpha: SandboxRepo
    bravo: SandboxRepo
    decoy: { id: string; name: string }
    heads: { alpha: string; bravo: string }
  }
): Promise<void> {
  const { rpc, devServerId } = { rpc: f.rpc, devServerId: f.project.devServerId }
  let notes = { decoyNote: '', note: '' }
  const r = await createClassifyAndConfirm(f, {
    titleWords: 'addproj question',
    body: QUESTION,
    typeLabel: 'Question',
    typeKey: 'question',
    size: 'S',
    create: async (title) => {
      notes = await createRequestInProject(f.page, {
        title,
        body: QUESTION,
        decoyProject: o.decoy.name,
        project: f.project.name
      })
    }
  })
  await f.step('Create request named the repo and host of the chosen project', async () => {
    console.log(`[addproject] note for alpha: ${notes.decoyNote}`)
    console.log(`[addproject] note for bravo: ${notes.note}`)
    expect
      .soft(notes.decoyNote, 'the note of the first project picked names alpha')
      .toContain(`repo ${o.alpha.name} (${o.alpha.path})`)
    expect.soft(notes.decoyNote, 'alpha note does not name bravo').not.toContain(o.bravo.name)
    expect
      .soft(notes.note, 'after choosing bravo the note names bravo, its path and the host')
      .toContain(`repo ${o.bravo.name} (${o.bravo.path}) on ${f.cfg.devServerName}`)
    expect.soft(notes.note, 'the bravo note no longer names alpha').not.toContain(o.alpha.name)
    expect
      .soft(
        (await requestState(rpc, r.requestId)).projectId,
        'the Request belongs to the bravo project'
      )
      .toBe(f.project.id)
    expect.soft(devServerId, 'the run is on the dev server of the projects').toBeTruthy()
  })
  await answerClarificationIfAsked(f, r)
  const screen = await readAnswerOnScreen(f, r)
  await f.step('the answer on screen is the stored answer, about bravo', async () => {
    const listed = await rpc.call<{ runs?: StoredRun[]; solutions?: StoredSolution[] }>(
      'solution.list',
      { requestId: r.requestId }
    )
    const run = (listed.runs ?? []).findLast((x) => x.kind === 'answer')
    expect(
      run?.status,
      `the answer run succeeded (${run?.errorCode ?? ''} ${run?.errorMessage ?? ''})`
    ).toBe('succeeded')
    const stored = (listed.solutions ?? []).findLast((x) => x.kind === 'answer')
    expect(stored, 'a Solution of kind answer').toBeTruthy()
    const snippet = plainSnippetOf(String(stored!.options?.answer_markdown ?? ''))
    f.lags.push(`answer ${String(stored!.options?.answer_markdown ?? '').length} chars`)
    console.log(
      `[addproject] answer: stored ${String(stored!.options?.answer_markdown ?? '').length} chars, snippet checked on screen "${snippet}", ${screen.confidence}, citations: ${screen.citations
        .slice(0, 4)
        .map((c) => c.slice(0, 60))
        .join(' | ')}`
    )
    expectAnswerAboutBravo({ screen, stored: stored!, snippet })
    await f.shots.take('answer-on-screen')
  })
  await f.step('the analysis read the bravo repo only, and read-only', async () => {
    f.lags.push(expectOnlyBravoWasRead({ alpha: o.alpha, bravo: o.bravo, heads: o.heads }))
  })
  await cancelThroughTheUi(f, r, /Awaiting Analysis Approval/i)
}
