import type { RelayDispatcher } from './dispatcher'
import type { AgentConfig } from './agent-config'
import type { AgentLogger } from './agent-logger'
import { toErrorPayload, CodeIntelError } from './codeintel-errors'

// We only import types statically
import type { CodeIntelRequestContext, CodeIntelNotificationSink } from './codeintel-method-table'

export function toRelayThrowable(err: unknown): Error & { code: number; data?: Record<string, unknown> } {
  if (err instanceof CodeIntelError) {
    const payload = toErrorPayload(err)
    return Object.assign(new Error(payload.message), {
      code: payload.code,
      data: payload.data
    })
  }

  const payload = toErrorPayload(err)
  return Object.assign(new Error(payload.message), {
    code: payload.code,
    data: payload.data
  })
}

// Known method names to register synchronously to avoid dropping requests during dynamic load
export const CODEINTEL_METHOD_NAMES = [
  'codeintel.status',
  'codeintel.overview',
  'codeintel.processes',
  'codeintel.process',
  'codeintel.routes',
  'codeintel.subgraph',
  'codeintel.impact',
  'codeintel.symbol',
  'codeintel.codegraphSearch',
  'codeintel.files',
  'codeintel.structuralFacts',
  'codeintel.reindex',
  'codeintel.reindexStatus',
  'codeintel.reindexCancel',
  'codeintel.watch',
  'codeintel.detectChanges'
]

export function registerCodeIntelHandlers(
  dispatcher: RelayDispatcher,
  config: AgentConfig,
  log: AgentLogger
): void {
  // We register placeholders immediately
  for (const methodName of CODEINTEL_METHOD_NAMES) {
    dispatcher.onRequest(methodName, async (params, context) => {
      let module: any
      try {
        module = await import('./codeintel-method-table')
      } catch (err: any) {
        throw toRelayThrowable(new CodeIntelError('CODEINTEL_TOOL_FAILED', 'Failed to load codeintel methods: ' + err.message))
      }

      const methodDef = module.CODEINTEL_METHODS[methodName]
      if (!methodDef) {
        throw toRelayThrowable(new Error('Method not found: ' + methodName))
      }

      try {
        // Wire global sink to dispatcher.notify lazily
        const { setCodeIntelNotifier } = await import('./codeintel-notification-sink')
        setCodeIntelNotifier((m, p) => dispatcher.notify(m, p))
        
        const { PerfCollector } = await import('./codeintel-result-envelope')
        
        const validatedParams = methodDef.validate(params)

        const ac = new AbortController()
        const timeoutId = setTimeout(() => ac.abort(), methodDef.timeoutMs)

        if (context?.signal) {
          if (context.signal.aborted) {
            ac.abort()
          } else {
            context.signal.addEventListener('abort', () => ac.abort(), { once: true })
          }
        }

        const notifier: CodeIntelNotificationSink = {
          notify: (method: string, nParams: any) => {
            dispatcher.notify(method, nParams)
          }
        }

        const perf = new PerfCollector()
        perf.start()

        const ctx: CodeIntelRequestContext = {
          config,
          log,
          signal: ac.signal,
          deadline: Date.now() + methodDef.timeoutMs,
          notifier,
          perf
        }

        try {
          return await methodDef.handle(validatedParams, ctx)
        } finally {
          clearTimeout(timeoutId)
        }
      } catch (err: any) {
        throw toRelayThrowable(err)
      }
    })
  }
}
