import { AgentConfig } from './agent-config'
import { AgentLogger } from './agent-logger'
import { PerfCollector } from './codeintel-result-envelope'
import { CodeIntelError } from './codeintel-errors'
import { validateCodeIntelParams, assertGitRef, assertSafeClientString } from './codeintel-params-validation'

export type CodeIntelNotificationSink = {
  notify(method: string, params: any): void
}

export type CodeIntelRequestContext = {
  config: AgentConfig
  log: AgentLogger
  signal: AbortSignal
  deadline: number
  notifier: CodeIntelNotificationSink
  perf: PerfCollector
}

export type CodeIntelMethodDefinition = {
  validate(params: any): any
  handle(params: any, ctx: CodeIntelRequestContext): Promise<any>
  timeoutMs: number
}

type IndexProbe = (workspaceRoot: string, ctx: CodeIntelRequestContext) => Promise<any>

const indexProbes = new Map<string, IndexProbe>()

export const CODEINTEL_METHODS: Record<string, CodeIntelMethodDefinition> = {
  'codeintel.status': {
    timeoutMs: 25000,
    validate: (params: any) => {
      const validated = validateCodeIntelParams(params, {
        baseRef: { kind: 'string', required: false, maxLen: 256 }
      })
      if (validated.baseRef !== undefined) {
        assertGitRef(validated.baseRef, 'baseRef')
      }
      return validated
    },
    handle: async (params, ctx) => {
      try {
        const module = await import('./codeintel-status')
        return module.handleStatus(params, ctx)
      } catch (err: any) {
        if (err instanceof CodeIntelError) throw err
        throw new CodeIntelError('CODEINTEL_TOOL_FAILED', `Failed to load or execute codeintel.status: ${err.message}`)
      }
    }
  },
  'codeintel.overview': {
    timeoutMs: 25000,
    validate: (params: any) => {
      return validateCodeIntelParams(params, {
        topN: { kind: 'int', required: false, min: 1, max: 1000, default: 200 },
        maxEdges: { kind: 'int', required: false, min: 1, max: 20000, default: 5000 },
        edgeKinds: { kind: 'stringList', required: false, default: ['CALLS', 'IMPORTS'] },
        withTopFiles: { kind: 'bool', required: false, default: true }
      })
    },
    handle: async (params, ctx) => {
      const m = await import('./codeintel-gitnexus-overview')
      return m.handleOverview(params, ctx)
    }
  },
  'codeintel.processes': {
    timeoutMs: 25000,
    validate: (params: any) => {
      return validateCodeIntelParams(params, {
        offset: { kind: 'int', required: false, min: 0, default: 0 },
        limit: { kind: 'int', required: false, min: 1, max: 500, default: 50 }
      })
    },
    handle: async (params, ctx) => {
      const m = await import('./codeintel-gitnexus-processes')
      return m.handleProcesses(params, ctx)
    }
  },
  'codeintel.process': {
    timeoutMs: 25000,
    validate: (params: any) => {
      const validated = validateCodeIntelParams(params, {
        processId: { kind: 'string', required: true, maxLen: 512 }
      })
      assertSafeClientString(validated.processId, 'processId')
      return validated
    },
    handle: async (params, ctx) => {
      const m = await import('./codeintel-gitnexus-process')
      return m.handleProcess(params, ctx)
    }
  },
  'codeintel.routes': {
    timeoutMs: 25000,
    validate: (params: any) => {
      return validateCodeIntelParams(params, {
        offset: { kind: 'int', required: false, min: 0, default: 0 },
        limit: { kind: 'int', required: false, min: 1, max: 500, default: 200 }
      })
    },
    handle: async (params, ctx) => {
      const m = await import('./codeintel-gitnexus-routes')
      return m.handleRoutes(params, ctx)
    }
  },
  'codeintel.subgraph': {
    timeoutMs: 25000,
    validate: (params: any) => {
      return validateCodeIntelParams(params, {
        center: { kind: 'object', required: true },
        depth: { kind: 'int', required: false, min: 1, max: 10, default: 1 },
        kinds: { kind: 'stringList', required: false, default: ['CALLS', 'IMPORTS', 'EXTENDS', 'IMPLEMENTS'] },
        limit: { kind: 'int', required: false, min: 1, max: 2000, default: 800 }
      })
    },
    handle: async (params, ctx) => {
      const m = await import('./codeintel-gitnexus-subgraph')
      return m.handleSubgraph(params, ctx)
    }
  },
  'codeintel.impact': {
    timeoutMs: 25000,
    validate: (params: any) => {
      return validateCodeIntelParams(params, {
        target: { kind: 'object', required: true },
        direction: { kind: 'enum', required: false, enum: ['upstream', 'downstream'], default: 'upstream' },
        depth: { kind: 'int', required: false, min: 1, max: 10, default: 2 },
        limit: { kind: 'int', required: false, min: 1, max: 1000, default: 300 },
        includeTests: { kind: 'bool', required: false, default: false }
      })
    },
    handle: async (params, ctx) => {
      const m = await import('./codeintel-gitnexus-impact')
      return m.handleImpact(params, ctx)
    }
  },
  'codeintel.symbol': {
    timeoutMs: 25000,
    validate: (params: any) => {
      const validated = validateCodeIntelParams(params, {
        uid: { kind: 'string', required: false, maxLen: 512 },
        name: { kind: 'string', required: false, maxLen: 512 },
        file: { kind: 'string', required: false, maxLen: 512 },
        includeSource: { kind: 'bool', required: false, default: true },
        relationLimit: { kind: 'int', required: false, min: 1, max: 100, default: 20 }
      })
      if (!validated.uid && !(validated.name && validated.file)) {
        throw new CodeIntelError('CODEINTEL_INVALID_PARAMS', 'Missing uid or name/file')
      }
      if (validated.uid) assertSafeClientString(validated.uid, 'uid')
      if (validated.name) assertSafeClientString(validated.name, 'name')
      if (validated.file) assertSafeClientString(validated.file, 'file')
      return validated
    },
    handle: async (params, ctx) => {
      const m = await import('./codeintel-gitnexus-symbol')
      return m.handleSymbol(params, ctx)
    }
  },
  'codeintel.codegraphSearch': {
    timeoutMs: 25000,
    validate: (params: any) => {
      const validated = validateCodeIntelParams(params, {
        search: { kind: 'string', required: true, maxLen: 256 },
        limit: { kind: 'int', required: false, min: 1, max: 50, default: 50, clamp: true },
        kind: { kind: 'string', required: false, maxLen: 128 }
      })
      assertSafeClientString(validated.search, 'search')
      if (validated.kind) assertSafeClientString(validated.kind, 'kind')
      return validated
    },
    handle: async (params, ctx) => {
      const m = await import('./codeintel-codegraph-search')
      return m.handleCodegraphSearch(params, ctx)
    }
  },
  'codeintel.files': {
    timeoutMs: 25000,
    validate: (params: any) => {
      const validated = validateCodeIntelParams(params, {
        filter: { kind: 'string', required: false, maxLen: 512, default: '' },
        limit: { kind: 'int', required: false, min: 1, max: 5000, default: 5000, clamp: true }
      })
      if (validated.filter) {
        assertSafeClientString(validated.filter, 'filter')
        if (validated.filter.includes('..')) {
          throw new CodeIntelError('CODEINTEL_INVALID_PARAMS', `Param 'filter' cannot contain '..'`, { field: 'filter' })
        }
      }
      return validated
    },
    handle: async (params, ctx) => {
      const m = await import('./codeintel-codegraph-files')
      return m.handleFiles(params, ctx)
    }
  },
  'codeintel.structuralFacts': {
    timeoutMs: process.env.ORCA_CODEINTEL_DETECT_TIMEOUT_MS ? parseInt(process.env.ORCA_CODEINTEL_DETECT_TIMEOUT_MS, 10) : 55000,
    validate: (params: any) => {
      const { validateStructuralFacts } = require('./codeintel-structural-facts')
      return validateStructuralFacts(params)
    },
    handle: async (params, ctx) => {
      const m = await import('./codeintel-structural-facts')
      return m.handleStructuralFacts(params, ctx)
    }
  },
  'codeintel.reindex': {
    timeoutMs: 25000,
    validate: (params: any) => {
      const { validateReindex } = require('./codeintel-reindex-methods')
      return validateReindex(params)
    },
    handle: async (params, ctx) => {
      const m = await import('./codeintel-reindex-methods')
      return m.handleReindex(params, ctx)
    }
  },
  'codeintel.reindexStatus': {
    timeoutMs: 25000,
    validate: (params: any) => {
      const { validateReindexStatus } = require('./codeintel-reindex-methods')
      return validateReindexStatus(params)
    },
    handle: async (params, ctx) => {
      const m = await import('./codeintel-reindex-methods')
      return m.handleReindexStatus(params, ctx)
    }
  },
  'codeintel.reindexCancel': {
    timeoutMs: 25000,
    validate: (params: any) => {
      const { validateReindexCancel } = require('./codeintel-reindex-methods')
      return validateReindexCancel(params)
    },
    handle: async (params, ctx) => {
      const m = await import('./codeintel-reindex-methods')
      return m.handleReindexCancel(params, ctx)
    }
  },
  'codeintel.watch': {
    timeoutMs: 25000,
    validate: (params: any) => {
      const { validateWatch } = require('./codeintel-reindex-methods')
      return validateWatch(params)
    },
    handle: async (params, ctx) => {
      const m = await import('./codeintel-reindex-methods')
      return m.handleWatch(params, ctx)
    }
  },
  'codeintel.detectChanges': {
    timeoutMs: process.env.ORCA_CODEINTEL_DETECT_TIMEOUT_MS ? parseInt(process.env.ORCA_CODEINTEL_DETECT_TIMEOUT_MS, 10) : 55000,
    validate: (params: any) => {
      const validated = validateCodeIntelParams(params, {
        base: { kind: 'string', required: false, maxLen: 256 },
        head: { kind: 'string', required: false, maxLen: 256 },
        includeUntracked: { kind: 'bool', required: false },
        withClusters: { kind: 'bool', required: false },
        crossCheck: { kind: 'bool', required: false }
      })
      if (validated.base !== undefined) assertGitRef(validated.base, 'base')
      if (validated.head !== undefined) assertGitRef(validated.head, 'head')
      return validated
    },
    handle: async (params, ctx) => {
      const m = await import('./codeintel-detect-changes')
      return m.handleDetectChanges(params, ctx)
    }
  }
}

export function registerIndexProbe(tool: string, probe: IndexProbe) {
  indexProbes.set(tool, probe)
}

export function getIndexProbes(): Array<{ tool: string, probe: IndexProbe }> {
  const arr: Array<{ tool: string, probe: IndexProbe }> = []
  for (const [tool, probe] of indexProbes.entries()) {
    arr.push({ tool, probe })
  }
  return arr
}
