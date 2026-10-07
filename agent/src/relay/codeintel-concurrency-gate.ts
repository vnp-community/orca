import { CodeIntelError } from './codeintel-errors'

export interface ConcurrencyGateConfig {
  maxTotal: number
  perTool: Record<string, number>
  queueMax: number
  queueWaitMs: number
}

interface QueuedItem {
  tool: string
  resolve: (release: () => void) => void
  reject: (err: Error) => void
  signal?: AbortSignal
  timer: NodeJS.Timeout
  onAbort?: () => void
}

export class CodeIntelConcurrencyGate {
  private activeTotal = 0
  private activePerTool: Record<string, number> = {}
  private queue: QueuedItem[] = []

  constructor(private config: ConcurrencyGateConfig) {}

  async acquire(tool: string, signal?: AbortSignal): Promise<() => void> {
    return new Promise((resolve, reject) => {
      if (signal?.aborted) {
        return reject(new CodeIntelError('CODEINTEL_TIMEOUT', 'aborted', { reason: 'queue_wait', elapsedMs: 0 }))
      }

      if (this.canAcquire(tool)) {
        this.activeTotal++
        this.activePerTool[tool] = (this.activePerTool[tool] || 0) + 1
        
        let released = false
        resolve(() => {
          if (!released) {
            released = true
            this.release(tool)
          }
        })
        return
      }

      if (this.queue.length >= this.config.queueMax) {
        return reject(new CodeIntelError('CODEINTEL_TIMEOUT', 'Queue full', { reason: 'queue_wait', elapsedMs: 0 }))
      }

      const item: QueuedItem = {
        tool,
        resolve,
        reject,
        signal,
        timer: setTimeout(() => {
          this.removeFromQueue(item)
          reject(new CodeIntelError('CODEINTEL_TIMEOUT', 'Queue timeout', { reason: 'queue_wait', elapsedMs: this.config.queueWaitMs }))
        }, this.config.queueWaitMs)
      }

      if (signal) {
        item.onAbort = () => {
          this.removeFromQueue(item)
          reject(new CodeIntelError('CODEINTEL_TIMEOUT', 'aborted', { reason: 'queue_wait', elapsedMs: 0 }))
        }
        signal.addEventListener('abort', item.onAbort)
      }

      this.queue.push(item)
    })
  }

  private canAcquire(tool: string): boolean {
    if (this.activeTotal >= this.config.maxTotal) return false
    const toolLimit = this.config.perTool[tool]
    if (toolLimit !== undefined && (this.activePerTool[tool] || 0) >= toolLimit) return false
    return true
  }

  private release(tool: string) {
    this.activeTotal--
    this.activePerTool[tool]--
    this.pump()
  }

  private removeFromQueue(item: QueuedItem) {
    clearTimeout(item.timer)
    if (item.signal && item.onAbort) {
      item.signal.removeEventListener('abort', item.onAbort)
    }
    const idx = this.queue.indexOf(item)
    if (idx !== -1) {
      this.queue.splice(idx, 1)
    }
  }

  private pump() {
    for (let i = 0; i < this.queue.length; i++) {
      const item = this.queue[i]
      if (this.canAcquire(item.tool)) {
        this.removeFromQueue(item)
        
        this.activeTotal++
        this.activePerTool[item.tool] = (this.activePerTool[item.tool] || 0) + 1
        
        let released = false
        item.resolve(() => {
          if (!released) {
            released = true
            this.release(item.tool)
          }
        })
        return
      }
    }
  }

  resetForTests() {
    for (const item of this.queue) {
      clearTimeout(item.timer)
    }
    this.queue = []
    this.activeTotal = 0
    this.activePerTool = {}
  }
}

let globalGate: CodeIntelConcurrencyGate | undefined

export function getCodeIntelConcurrencyGate(config?: ConcurrencyGateConfig): CodeIntelConcurrencyGate {
  if (!globalGate) {
    globalGate = new CodeIntelConcurrencyGate(config || {
      maxTotal: 3,
      perTool: { gitnexus: 2, codegraph: 3 },
      queueMax: 16,
      queueWaitMs: 10000,
    })
  }
  return globalGate
}

export function resetGlobalCodeIntelConcurrencyGateForTests() {
  if (globalGate) {
    globalGate.resetForTests()
    globalGate = undefined
  }
}

export function createConcurrencyGate(config: ConcurrencyGateConfig): CodeIntelConcurrencyGate {
  return new CodeIntelConcurrencyGate(config)
}
