import os from 'os'

export class HeavyGateTimeoutError extends Error {
  constructor() {
    super('Heavy job queue wait timeout')
    this.name = 'HeavyGateTimeoutError'
  }
}

export interface HeavyJobGate {
  acquire(signal: AbortSignal, waitMs: number): Promise<() => void>
  stats(): { running: number; waiting: number }
}

interface Waiter {
  resolve: (release: () => void) => void
  reject: (err: Error) => void
  timer: NodeJS.Timeout
  onAbort: () => void
  signal: AbortSignal
}

export function createHeavyJobGate(opts: { max?: number; now?: () => number } = {}): HeavyJobGate {
  let max = opts.max
  if (max === undefined) {
    const envVal = process.env.ORCA_HEAVY_JOBS
    const parsed = parseInt(envVal || '', 10)
    const cores = os.availableParallelism ? os.availableParallelism() : os.cpus().length
    if (parsed && !isNaN(parsed) && parsed >= 1 && parsed <= cores) {
      max = parsed
    } else {
      if (envVal) {
        console.warn(`Invalid ORCA_HEAVY_JOBS value "${envVal}". Defaulting to 1.`)
      }
      max = 1
    }
  }

  let running = 0
  const queue: Waiter[] = []
  const nowFn = opts.now || Date.now

  function pump() {
    while (running < max! && queue.length > 0) {
      const w = queue.shift()!
      clearTimeout(w.timer)
      w.signal.removeEventListener('abort', w.onAbort)
      
      running++
      let released = false
      const release = () => {
        if (released) return
        released = true
        running--
        pump()
      }
      w.resolve(release)
    }
  }

  return {
    acquire(signal: AbortSignal, waitMs: number): Promise<() => void> {
      return new Promise((resolve, reject) => {
        if (signal.aborted) {
          const err = new Error('Aborted')
          err.name = 'AbortError'
          return reject(err)
        }

        if (running < max! && queue.length === 0) {
          running++
          let released = false
          const release = () => {
            if (released) return
            released = true
            running--
            pump()
          }
          return resolve(release)
        }

        const w: Partial<Waiter> = { signal, resolve, reject }

        const onAbort = () => {
          const idx = queue.indexOf(w as Waiter)
          if (idx !== -1) queue.splice(idx, 1)
          clearTimeout(w.timer)
          w.signal!.removeEventListener('abort', onAbort)
          const err = new Error('Aborted')
          err.name = 'AbortError'
          reject(err)
        }

        w.onAbort = onAbort
        signal.addEventListener('abort', onAbort)

        w.timer = setTimeout(() => {
          const idx = queue.indexOf(w as Waiter)
          if (idx !== -1) queue.splice(idx, 1)
          signal.removeEventListener('abort', onAbort)
          reject(new HeavyGateTimeoutError())
        }, waitMs)

        queue.push(w as Waiter)
      })
    },
    stats() {
      return { running, waiting: queue.length }
    }
  }
}

let singleton: HeavyJobGate | null = null

export function getHeavyJobGate(): HeavyJobGate {
  if (!singleton) {
    singleton = createHeavyJobGate()
  }
  return singleton
}
