/**
 * The readiness Clarification (and the ask-back one) answered through the UI from the backlog text: every question block is read,
 * its control recognised and filled, the pointer sentence used only where no line of the item fits.
 */
import { expect, type Locator } from '@playwright/test'
import { pickOption, textAnswer, type BacklogItem } from './ci-cd-kit-backlog'
import type { Timeline } from './ci-cd-kit-report'
import type { LiveFlow } from './live-flow-harness'
import type { RequestUnderTest } from './live-flow-steps'
import { requestState } from './live-gateway-crosschecks'
import { expectScreenFollowsService } from './live-status-sync'

export type QuestionAnswered = { prompt: string; control: string; answer: string }

const cleanLabel = (text: string): string =>
  text
    .replace(/\s*\*\s*/g, ' ')
    .replace(/\bRequired\b/g, '')
    .replace(/\s+/g, ' ')
    .trim()

/** One question block of the Clarification panel filled the way its control asks for; returns what was answered. */
async function answerBlock(block: Locator, item: BacklogItem): Promise<QuestionAnswered> {
  const prompt = cleanLabel(await block.locator('label').first().innerText())
  const reason = (await block.locator('p').allInnerTexts()).join(' ')
  const area = block.locator('textarea')
  if ((await area.count()) > 0) {
    const answer = textAnswer(item, prompt, reason)
    await area.first().fill(answer)
    return { prompt, control: 'text', answer }
  }
  const number = block.locator('input[data-value-type="number"]')
  if ((await number.count()) > 0) {
    await number.first().fill('1')
    return { prompt, control: 'number', answer: '1' }
  }
  const radios = block.getByRole('radio')
  if ((await radios.count()) > 0) {
    const labels = (await radios.allInnerTexts()).map((t) => t.trim())
    const yesNo = labels.length === 2 && labels[0] === 'Yes' && labels[1] === 'No'
    const index = yesNo ? 0 : pickOption(item, labels)
    await radios.nth(index).click()
    return { prompt, control: yesNo ? 'boolean' : 'choice', answer: labels[index] }
  }
  const boxes = block.getByRole('checkbox')
  if ((await boxes.count()) > 0) {
    const labels = (
      await block
        .locator('label')
        .filter({ has: block.getByRole('checkbox') })
        .allInnerTexts()
    ).map((t) => t.trim())
    const index = Math.max(0, pickOption(item, labels))
    await boxes.nth(index).click()
    return { prompt, control: 'multi', answer: labels[index] ?? '' }
  }
  return {
    prompt,
    control: 'file or unknown',
    answer: '(no answer: the control cannot be filled from text)'
  }
}

/** Every question block of an open Clarification panel filled from the backlog text, logged one by one. */
async function fillQuestions(panel: Locator, item: BacklogItem): Promise<QuestionAnswered[]> {
  const blocks = panel.locator('[data-question-id]')
  const done: QuestionAnswered[] = []
  for (let i = 0; i < (await blocks.count()); i++) {
    const one = await answerBlock(blocks.nth(i), item)
    done.push(one)
    console.log(
      `[ci-cd-kit] question ${i + 1} (${one.control}): "${one.prompt.slice(0, 140)}" -> "${one.answer.replace(/\s+/g, ' ').slice(0, 140)}"`
    )
  }
  return done
}

/**
 * The Clarification an ask-back opens (the AI's own blocking question) answered the same way and submitted. Passed to approveAnalysis
 * so the question is not answered with the harness placeholder. Returns whether Submit answers was enabled.
 */
export function answerAskBackFromBacklog(
  item: BacklogItem,
  asked: QuestionAnswered[]
): (f: LiveFlow) => Promise<boolean> {
  return async (f) => {
    const panel = f.page.getByTestId('clarification-panel')
    await expect(panel.locator('[data-question-id]').first()).toBeVisible({ timeout: 30_000 })
    asked.push(...(await fillQuestions(panel, item)))
    const submit = panel.getByRole('button', { name: /submit answers/i })
    const enabled = await expect(submit)
      .toBeEnabled({ timeout: 10_000 })
      .then(
        () => true,
        () => false
      )
    if (enabled) {
      await submit.click()
    }
    return enabled
  }
}

/** Reads the questions on screen and answers each from the backlog text; the pointer sentence only where nothing fits. */
export async function answerClarificationFromBacklog(
  f: LiveFlow,
  r: RequestUnderTest,
  item: BacklogItem,
  timeline: Timeline,
  options: { submit: boolean } = { submit: true }
): Promise<QuestionAnswered[]> {
  const answered: QuestionAnswered[] = []
  const status = (await requestState(f.rpc, r.requestId)).status
  if (status !== 'awaiting_information') {
    console.log(`[ci-cd-kit] no readiness Clarification (status ${status})`)
    return answered
  }
  await f.step('answer the readiness Clarification on screen, from the backlog text', async () => {
    await expectScreenFollowsService(f.page, f.rpc, f.gaps, {
      requestId: r.requestId,
      title: r.title,
      step: 'AWAITING-INFORMATION',
      apiStatus: ['awaiting_information'],
      chip: /Awaiting Information/i,
      apiWaitMs: 15_000
    })
    const panel = f.page.getByTestId('clarification-panel')
    await expect(panel.locator('[data-question-id]').first()).toBeVisible({ timeout: 30_000 })
    await f.shots.take('clarification-open')
    answered.push(...(await fillQuestions(panel, item)))
    await f.shots.take('clarification-filled')
    const submit = panel.getByRole('button', { name: /submit answers/i })
    await expect(submit, 'Submit answers is enabled once every question is answered').toBeEnabled({
      timeout: 10_000
    })
    if (!options.submit) {
      return
    }
    await submit.click()
    await expectScreenFollowsService(f.page, f.rpc, f.gaps, {
      requestId: r.requestId,
      title: r.title,
      step: 'AFTER-CLARIFICATION',
      apiStatus: ['analyzing', 'awaiting_analysis_approval'],
      chip: /Analyzing|Awaiting Analysis Approval/i,
      apiWaitMs: 30_000
    })
    const recorded = await f.rpc.call<{ clarifications: unknown[] }>('clarification.list', {
      requestId: r.requestId,
      status: 'answered'
    })
    expect(recorded.clarifications.length, 'the answers were recorded').toBeGreaterThan(0)
    timeline.mark('clarification answered')
    await f.shots.take('clarification-answered')
  })
  return answered
}
