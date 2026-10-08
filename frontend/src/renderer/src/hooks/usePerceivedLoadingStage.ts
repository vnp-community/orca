import { useEffect, useState } from 'react'

export type PerceivedLoadingStage = 'idle' | 'busy' | 'dimmed' | 'spinner' | 'stages'

/** Thresholds (ms) from STYLEGUIDE "Match in-flight feedback to perceived duration". */
export const PERCEIVED_LOADING_THRESHOLDS = {
  dimmed: 100,
  dimmedRemote: 200,
  spinner: 1000,
  stages: 3000
} as const

/** Pure mapping so the ladder can be tested without timers. */
export function perceivedLoadingStageAt(elapsedMs: number, remote: boolean): PerceivedLoadingStage {
  if (elapsedMs >= PERCEIVED_LOADING_THRESHOLDS.stages) {
    return 'stages'
  }
  if (elapsedMs >= PERCEIVED_LOADING_THRESHOLDS.spinner) {
    return 'spinner'
  }
  const dimAt = remote
    ? PERCEIVED_LOADING_THRESHOLDS.dimmedRemote
    : PERCEIVED_LOADING_THRESHOLDS.dimmed
  return elapsedMs >= dimAt ? 'dimmed' : 'busy'
}

/**
 * `busy` immediately (control disabled, no visual change), then `dimmed`, `spinner`,
 * `stages` as the wait grows. Timers are cancelled when the work finishes first.
 */
export function usePerceivedLoadingStage(
  isPending: boolean,
  opts: { remote?: boolean } = {}
): PerceivedLoadingStage {
  const remote = opts.remote === true
  const [stage, setStage] = useState<PerceivedLoadingStage>('idle')

  useEffect(() => {
    if (!isPending) {
      setStage('idle')
      return
    }
    setStage('busy')
    const t = PERCEIVED_LOADING_THRESHOLDS
    const timers = [
      setTimeout(() => setStage('dimmed'), remote ? t.dimmedRemote : t.dimmed),
      setTimeout(() => setStage('spinner'), t.spinner),
      setTimeout(() => setStage('stages'), t.stages)
    ]
    return () => timers.forEach(clearTimeout)
  }, [isPending, remote])

  return stage
}
