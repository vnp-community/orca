/**
 * Steps of the Outcome Review UI flow (FE-OR-T330). Setup that is not a product step (turning the review on for the throwaway
 * project, which has no screen in this build) goes through the admin channel and is put back afterwards.
 */
import { expect, type Locator, type Page } from '@playwright/test'
import { detailPane } from './live-request-ui-steps'
import type { LiveFlow } from './live-flow-harness'

type StoredEngineSettings = {
  engine?: string
  openspecMinVersion?: string
  solutionAgent?: boolean
  executionMode?: string
  planAutoGenerate?: boolean
  outcomeReview?: string
  outcomeReviewEnabled?: boolean
  version: number
}

/**
 * Test setup: the project's choice for the Outcome Review ('off', 'manual' or 'default'), everything else written back as read.
 * Returns what was stored, so the spec can put it back.
 */
export async function setOutcomeReview(
  f: LiveFlow,
  mode: 'off' | 'manual' | 'default'
): Promise<string> {
  const current = await f.rpc.call<StoredEngineSettings>('request.engineSettings', {
    projectId: f.project.id
  })
  const next = await f.rpc.call<StoredEngineSettings>('request.engineSettingsSet', {
    projectId: f.project.id,
    engine: current.engine ?? 'native',
    openspecMinVersion: current.openspecMinVersion ?? '',
    solutionAgent: current.solutionAgent ?? false,
    executionMode: current.executionMode ?? 'step_by_step',
    outcomeReview: mode,
    expectedVersion: current.version
  })
  expect(next.outcomeReview ?? '', 'the project choice was stored').toBe(
    mode === 'default' ? '' : mode
  )
  return current.outcomeReview ?? ''
}

/** The tabs of the open Request, in order. */
export async function detailTabNames(page: Page): Promise<string[]> {
  const names = await detailPane(page).getByRole('tab').allTextContents()
  return names.map((n) => n.trim())
}

export const outcomeReviewTab = (page: Page): Locator =>
  detailPane(page).getByRole('tab', { name: 'Outcome review' })

export async function openOutcomeReviewTab(page: Page): Promise<void> {
  await outcomeReviewTab(page).click()
  await expect(
    page.getByTestId('outcome-review-tab').or(page.getByTestId('outcome-review-invite')).first()
  ).toBeVisible({ timeout: 30_000 })
}

export const reviewInvite = (page: Page): Locator => page.getByTestId('outcome-review-invite')
export const reviewTab = (page: Page): Locator => page.getByTestId('outcome-review-tab')
