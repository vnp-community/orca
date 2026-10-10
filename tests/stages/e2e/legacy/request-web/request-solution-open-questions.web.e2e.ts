/**
 * E2E for the Solution's AI open questions (TASK-FE-E2E-009 item 6), web SPA + mocked gateway WebSocket.
 * "Ask back" calls clarification.request (CONTRACT 2.6); the Clarification panel then takes the answers.
 * The question keys stay snake_case (CONTRACT-request-ui-api: SolutionView.options is the AI document).
 * Approval is refused with REQUEST_SOLUTION_BLOCKING_QUESTIONS while a blocking question has no answer.
 */
import { expect, test, type Page } from '@playwright/test'
import {
  approvalView,
  bootRequestApp,
  createFakeRequestBackend,
  mockRequestApp,
  openRequestPageViaStore,
  requestView,
  RpcFailure,
  type FakeRequestBackend
} from './support/mock-request-ws'

function solutionWith(openQuestions?: unknown[]): Record<string, unknown> {
  return {
    id: 'sol-1',
    requestId: 'req-1',
    kind: 'solution',
    status: 'proposed',
    version: 1,
    createdAt: '2026-10-08T09:00:00Z',
    options: {
      options: [
        { id: 'opt-a', title: 'Patch the parser', summary: 'Small fix', estimatedEffort: '2h' },
        { id: 'opt-b', title: 'Rewrite the parser', summary: 'Clean design', estimatedEffort: '3d' }
      ],
      recommendation: { optionId: 'opt-a' },
      ...(openQuestions ? { open_questions: openQuestions } : {})
    }
  }
}

const QUESTIONS = [
  { id: 'Q-1', text: 'Which database holds the audit trail?', blocking: true },
  { id: 'Q-2', text: 'Should the old endpoint stay for one release?', blocking: false }
]

function backendFor(solution: Record<string, unknown>): FakeRequestBackend {
  const request = requestView({ type: 'change_request', status: 'awaiting_analysis_approval' })
  return createFakeRequestBackend({
    'request.list': () => ({ requests: [request] }),
    'request.get': () => ({ request }),
    'solution.list': () => ({ solutions: [solution], runs: [] }),
    'approval.list': () => ({ approvals: [approvalView()] }),
    'approval.listPending': () => ({ approvals: [approvalView()] }),
    'solution.choose': (p) => ({
      solution: { ...solution, chosenOptionId: p.optionId },
      approvalDigest: 'digest-2'
    }),
    'approval.approve': () => {
      throw new RpcFailure(
        'runtime_error',
        'REQUEST_SOLUTION_BLOCKING_QUESTIONS: blocking open questions are unanswered: Q-1'
      )
    }
  })
}

async function openAnalysis(page: Page, backend: FakeRequestBackend): Promise<void> {
  await mockRequestApp(page, backend)
  await bootRequestApp(page)
  await openRequestPageViaStore(page, { section: 'requests', requestId: 'req-1' })
  await page.getByRole('tab', { name: 'Analysis' }).click()
}

test.describe('Solution open questions (mocked request-service)', () => {
  test('a: blocking and non-blocking questions are listed with the approval notice', async ({
    page
  }) => {
    await openAnalysis(page, backendFor(solutionWith(QUESTIONS)))
    await expect(page.getByTestId('solution-open-questions')).toBeVisible()
    const blocking = page.getByTestId('solution-blocking-question')
    await expect(blocking).toHaveCount(1)
    await expect(blocking).toHaveAttribute('data-question-id', 'Q-1')
    await expect(blocking).toContainText('Which database holds the audit trail?')
    await expect(page.getByTestId('solution-open-question')).toHaveAttribute(
      'data-question-id',
      'Q-2'
    )
    await expect(page.getByTestId('solution-blocking-notice')).toHaveAttribute(
      'data-refused',
      'false'
    )
  })

  test('b: a refused approval explains the blocking questions instead of "already decided"', async ({
    page
  }) => {
    const backend = backendFor(solutionWith(QUESTIONS))
    await openAnalysis(page, backend)
    const panel = page.getByRole('tabpanel', { name: 'Analysis' })
    await panel.getByRole('radio', { name: /Patch the parser/ }).click()
    await page.getByTestId('solution-approve-option').click()
    await expect.poll(() => backend.callsTo('approval.approve').length).toBe(1)
    const notice = page.getByTestId('solution-blocking-notice')
    await expect(notice).toHaveAttribute('data-refused', 'true')
    await expect(notice).toContainText('Q-1')
    await expect(page.getByTestId('solution-blocking-question')).toHaveAttribute(
      'data-refused',
      'true'
    )
    await expect(page.getByText('This was already decided.')).toHaveCount(0)
  })

  test('c: a Solution without open questions shows no section', async ({ page }) => {
    await openAnalysis(page, backendFor(solutionWith()))
    await expect(page.getByRole('radio', { name: /Patch the parser/ })).toBeVisible()
    await expect(page.getByTestId('solution-open-questions')).toHaveCount(0)
    await expect(page.getByTestId('solution-blocking-notice')).toHaveCount(0)
  })

  test('d: "Ask back" sends clarification.request and moves to the Clarification panel', async ({
    page
  }) => {
    let status = 'awaiting_analysis_approval'
    let asked: Record<string, unknown> | null = null
    const solution = solutionWith(QUESTIONS)
    const view = (): ReturnType<typeof requestView> =>
      requestView({ type: 'change_request', status })
    const backend = createFakeRequestBackend({
      'request.list': () => ({ requests: [view()] }),
      'request.get': () => ({ request: view() }),
      'solution.list': () => ({ solutions: [solution], runs: [] }),
      'approval.list': () => ({
        approvals: status === 'awaiting_analysis_approval' ? [approvalView()] : []
      }),
      'approval.listPending': () => ({
        approvals: status === 'awaiting_analysis_approval' ? [approvalView()] : []
      }),
      'clarification.request': (p) => {
        asked = p
        status = 'awaiting_information'
        return { clarification: { id: 'cl-1' } }
      },
      'clarification.list': () => ({
        clarifications:
          status === 'awaiting_information'
            ? [
                {
                  id: 'cl-1',
                  display_id: 'CL-1',
                  request_id: 'req-1',
                  source: 'solution_open_question',
                  source_ref: 'sol-1',
                  status: 'open',
                  version: 1,
                  round: 1,
                  resume_status: 'analyzing',
                  assignee_ids: ['u1'],
                  questions: [
                    {
                      id: 'cq1',
                      seq: 1,
                      question_key: 'open_question:Q-1',
                      kind: 'text',
                      prompt: QUESTIONS[0].text,
                      required: true
                    }
                  ]
                }
              ]
            : []
      })
    })
    await openAnalysis(page, backend)
    await page.getByTestId('solution-ask-back').click()
    const reason = page.getByTestId('solution-ask-back-reason').first()
    await reason.fill('We need the store before choosing an option')
    await page.getByTestId('solution-ask-back-submit').click()
    await expect.poll(() => backend.callsTo('clarification.request').length).toBe(1)
    expect(asked).toMatchObject({
      requestId: 'req-1',
      source: 'solution_open_question',
      sourceRef: 'sol-1'
    })
    const questions = (asked as unknown as { questions: Record<string, unknown>[] }).questions
    expect(questions).toHaveLength(1)
    expect(questions[0]).toEqual({
      questionKey: 'open_question:Q-1',
      kind: 'text',
      prompt: 'Which database holds the audit trail?',
      reason: 'We need the store before choosing an option',
      required: true
    })
    await expect(page.locator('#request-clarification-panel')).toBeVisible()
    await expect(page.getByText('Which database holds the audit trail?').first()).toBeVisible()
    await expect(page.getByTestId('solution-ask-back-dialog')).toHaveCount(0)
  })

  test('e: a refusal to ask keeps the dialog open with the reason', async ({ page }) => {
    const backend = backendFor(solutionWith(QUESTIONS))
    backend.on('clarification.request', () => {
      throw new RpcFailure(
        'runtime_error',
        'REQUEST_CLARIFICATION_STATE_NOT_ALLOWED: an open clarification already exists'
      )
    })
    await openAnalysis(page, backend)
    await page.getByTestId('solution-ask-back').click()
    await page.getByTestId('solution-ask-back-submit').click()
    await expect(page.getByTestId('solution-ask-back-error')).toContainText(
      'another question round'
    )
    await expect(page.getByTestId('solution-ask-back-dialog')).toBeVisible()
  })

  test('f: after the Clarification is answered the screen follows the NEW Solution without a reload', async ({
    page
  }) => {
    // UI-STALE-SOLUTION: no event is ever pushed here; the screen must re-read on its own.
    let phase: 'before' | 'asked' | 'regenerating' | 'ready' = 'before'
    let readyAt = 0
    const old = solutionWith(QUESTIONS)
    const fresh = { ...solutionWith([]), id: 'sol-2', version: 2 }
    const view = (): ReturnType<typeof requestView> => {
      if (phase === 'regenerating' && Date.now() >= readyAt) {
        phase = 'ready'
      }
      const status =
        phase === 'before' || phase === 'ready'
          ? 'awaiting_analysis_approval'
          : phase === 'asked'
            ? 'awaiting_information'
            : 'analyzing'
      return requestView({
        type: 'change_request',
        status,
        updatedAt: `2026-10-10T00:00:0${['before', 'asked', 'regenerating', 'ready'].indexOf(phase)}Z`
      })
    }
    const solutions = (): Record<string, unknown>[] => {
      view()
      return phase === 'before'
        ? [old]
        : phase === 'ready'
          ? [{ ...old, status: 'superseded' }, fresh]
          : [{ ...old, status: 'superseded' }]
    }
    const backend = createFakeRequestBackend({
      'request.list': () => ({ requests: [view()] }),
      'request.get': () => ({ request: view() }),
      'solution.list': () => ({ solutions: solutions(), runs: [] }),
      'approval.list': () => ({
        approvals:
          phase === 'before'
            ? [approvalView()]
            : phase === 'ready'
              ? [approvalView({ id: 'ap-2', subjectId: 'sol-2' })]
              : []
      }),
      'approval.listPending': () => ({
        approvals: phase === 'ready' ? [approvalView({ id: 'ap-2', subjectId: 'sol-2' })] : []
      }),
      'clarification.request': () => {
        phase = 'asked'
        return { clarification: { id: 'cl-1' } }
      },
      'clarification.list': () => ({
        clarifications:
          phase === 'asked'
            ? [
                {
                  id: 'cl-1',
                  display_id: 'CL-1',
                  request_id: 'req-1',
                  source: 'solution_open_question',
                  source_ref: 'sol-1',
                  status: 'open',
                  version: 1,
                  round: 1,
                  resume_status: 'analyzing',
                  assignee_ids: ['u1'],
                  questions: [
                    {
                      id: 'cq1',
                      seq: 1,
                      question_key: 'open_question:Q-1',
                      kind: 'text',
                      prompt: QUESTIONS[0].text,
                      required: true
                    }
                  ]
                }
              ]
            : []
      }),
      'clarification.answer': () => {
        phase = 'regenerating'
        readyAt = Date.now() + 4000
        return { still_missing: false, request_status: 'analyzing' }
      }
    })
    await openAnalysis(page, backend)
    await expect(page.getByTestId('solution-blocking-question')).toHaveCount(1)
    await page.getByTestId('solution-ask-back').click()
    await page.getByTestId('solution-ask-back-submit').click()
    await expect(page.locator('#request-clarification-panel')).toBeVisible()
    await page
      .getByTestId('clarification-panel')
      .getByRole('textbox')
      .fill('The audit trail lives in Postgres')
    await page.getByRole('button', { name: 'Submit answers' }).click()
    // The old version and its question disappear, then the new version shows up, all without a reload.
    await expect(page.getByTestId('solution-blocking-question')).toHaveCount(0)
    await expect(page.getByTestId('solution-approve-option')).toBeVisible({ timeout: 20_000 })
    await expect(page.getByTestId('solution-status-banner')).toHaveAttribute('data-status', 'ready')
    await expect(page.getByTestId('solution-open-questions')).toHaveCount(0)
  })
})
