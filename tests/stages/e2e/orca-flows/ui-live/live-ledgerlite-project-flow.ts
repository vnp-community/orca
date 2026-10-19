/**
 * A live UI flow whose project sits on a fresh LedgerLite repo (fixture-repo-ledgerlite.ts): main, a release/1.x branch, and a
 * third branch the primary checkout is left on, so "the default branch" cannot be confused with "whatever is checked out".
 * The repo is the caller's: runLiveFlow only registers it as the project's repo, and it is removed here after the project is gone.
 */
import type { BrowserContext, Page, TestInfo } from '@playwright/test'
import {
  createLedgerliteRepo,
  type LedgerliteOptions,
  type LedgerliteRepo
} from './fixture-repo-ledgerlite'
import { runLiveFlow, type LiveFlow } from './live-flow-harness'

export async function runLedgerliteFlow(
  args: {
    page: Page
    context: BrowserContext
    testInfo: TestInfo
    slug: string
    repo?: LedgerliteOptions
  },
  body: (flow: LiveFlow, repo: LedgerliteRepo) => Promise<void>
): Promise<void> {
  const repo = createLedgerliteRepo(args.slug, args.repo)
  try {
    await runLiveFlow(
      {
        page: args.page,
        context: args.context,
        testInfo: args.testInfo,
        slug: args.slug,
        repoPath: repo.path
      },
      (flow) => body(flow, repo)
    )
  } finally {
    repo.cleanup()
  }
}
