// Why: a pull-to-refresh can overlap the initial load; only the newest request
// may write state, so a slow older response never overwrites a fresher one.
export type MobileReviewSummaryRequestGuard = {
  begin: () => () => boolean
}

export function createMobileReviewSummaryRequestGuard(): MobileReviewSummaryRequestGuard {
  let generation = 0
  return {
    begin() {
      generation += 1
      const mine = generation
      return () => mine === generation
    }
  }
}
