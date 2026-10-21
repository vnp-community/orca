/**
 * The Architecture and Flow tabs of an Outcome Review, in both modes, from the screen: each must show a drawing. A tab that shows
 * none comes back as a line for the report, not as a failed run.
 */
import type { LiveFlow } from './live-flow-harness'

/** Why: the review was once delivered with Architecture and Flow empty; each tab must show a drawing or say plainly why it cannot. */
export async function drawingsShowInTheArchitectureAndFlowTabs(f: LiveFlow): Promise<string[]> {
  const modes = f.page.getByTestId('outcome-mode-switch')
  const empty: string[] = []
  for (const mode of ['design', 'after_code']) {
    await modes.locator(`[data-mode="${mode}"]`).click()
    for (const tab of ['Architecture', 'Flow']) {
      await f.page.getByRole('tab', { name: new RegExp(`^${tab}`) }).click()
      const drawn = await f.page
        .getByTestId('review-artifact-iframe')
        .first()
        .waitFor({ state: 'visible', timeout: 15_000 })
        .then(
          () => true,
          () => false
        )
      await f.shots.take(`review-${mode}-${tab.toLowerCase()}`)
      if (!drawn) {
        empty.push(`${tab}/${mode}`)
      }
    }
  }
  return empty.map((x) => `no drawing in the review tab ${x}`)
}
