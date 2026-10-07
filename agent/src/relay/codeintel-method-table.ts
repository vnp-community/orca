import { AgentConfig } from './agent-config'
import { AgentLogger } from './agent-logger'
import { PerfCollector } from './codeintel-result-envelope'
import { CodeIntelError } from './codeintel-errors'

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

export const CODEINTEL_METHODS: Record<string, CodeIntelMethodDefinition> = {
  'codeintel.status': {
    timeoutMs: 25000,
    validate: (params: any) => {
      if (typeof params !== 'object' || params === null) {
        throw new CodeIntelError('CODEINTEL_INVALID_PARAMS', 'Params must be an object', { field: 'params' })
      }
      const { workspaceRoot, baseRef } = params
      if (typeof workspaceRoot !== 'string' || !workspaceRoot.trim()) {
        throw new CodeIntelError('CODEINTEL_INVALID_PARAMS', 'Missing or invalid workspaceRoot', { field: 'workspaceRoot' })
      }
      if (baseRef !== undefined && typeof baseRef !== 'string') {
        throw new CodeIntelError('CODEINTEL_INVALID_PARAMS', 'Invalid baseRef', { field: 'baseRef' })
      }
      // "assertGitRef" validation could go here, but for now just regex check or accept string
      if (baseRef && baseRef.startsWith('-')) {
         throw new CodeIntelError('CODEINTEL_INVALID_PARAMS', 'Invalid baseRef format', { field: 'baseRef' })
      }
      return { workspaceRoot, baseRef }
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
      const p = params || {}
      return {
        workspaceRoot: p.workspaceRoot,
        topN: typeof p.topN === 'number' ? p.topN : 200,
        maxEdges: typeof p.maxEdges === 'number' ? p.maxEdges : 5000,
        edgeKinds: Array.isArray(p.edgeKinds) ? p.edgeKinds : ['CALLS', 'IMPORTS'],
        withTopFiles: typeof p.withTopFiles === 'boolean' ? p.withTopFiles : true
      }
    },
    handle: async (params, ctx) => {
      const m = await import('./codeintel-gitnexus-overview')
      return m.handleOverview(params, ctx)
    }
  },
  'codeintel.processes': {
    timeoutMs: 25000,
    validate: (params: any) => {
      const p = params || {}
      return {
        workspaceRoot: p.workspaceRoot,
        offset: typeof p.offset === 'number' ? p.offset : 0,
        limit: typeof p.limit === 'number' ? p.limit : 50
      }
    },
    handle: async (params, ctx) => {
      const m = await import('./codeintel-gitnexus-processes')
      return m.handleProcesses(params, ctx)
    }
  },
  'codeintel.process': {
    timeoutMs: 25000,
    validate: (params: any) => {
      const p = params || {}
      if (typeof p.processId !== 'string' || !p.processId) {
        throw new CodeIntelError('CODEINTEL_INVALID_PARAMS', 'Missing processId')
      }
      return {
        workspaceRoot: p.workspaceRoot,
        processId: p.processId
      }
    },
    handle: async (params, ctx) => {
      const m = await import('./codeintel-gitnexus-process')
      return m.handleProcess(params, ctx)
    }
  },
  'codeintel.routes': {
    timeoutMs: 25000,
    validate: (params: any) => {
      const p = params || {}
      return {
        workspaceRoot: p.workspaceRoot,
        offset: typeof p.offset === 'number' ? p.offset : 0,
        limit: typeof p.limit === 'number' ? p.limit : 200
      }
    },
    handle: async (params, ctx) => {
      const m = await import('./codeintel-gitnexus-routes')
      return m.handleRoutes(params, ctx)
    }
  },
  'codeintel.subgraph': {
    timeoutMs: 25000,
    validate: (params: any) => {
      const p = params || {}
      return {
        workspaceRoot: p.workspaceRoot,
        center: p.center,
        depth: typeof p.depth === 'number' ? p.depth : 1,
        kinds: Array.isArray(p.kinds) ? p.kinds : ['CALLS', 'IMPORTS', 'EXTENDS', 'IMPLEMENTS'],
        limit: typeof p.limit === 'number' ? p.limit : 800
      }
    },
    handle: async (params, ctx) => {
      const m = await import('./codeintel-gitnexus-subgraph')
      return m.handleSubgraph(params, ctx)
    }
  },
  'codeintel.impact': {
    timeoutMs: 25000,
    validate: (params: any) => {
      const p = params || {}
      if (!p.target || typeof p.target !== 'object') {
        throw new CodeIntelError('CODEINTEL_INVALID_PARAMS', 'Missing or invalid target')
      }
      return {
        workspaceRoot: p.workspaceRoot,
        target: p.target,
        direction: p.direction === 'downstream' ? 'downstream' : 'upstream',
        depth: typeof p.depth === 'number' ? p.depth : 2,
        limit: typeof p.limit === 'number' ? p.limit : 300,
        includeTests: !!p.includeTests
      }
    },
    handle: async (params, ctx) => {
      const m = await import('./codeintel-gitnexus-impact')
      return m.handleImpact(params, ctx)
    }
  },
  'codeintel.symbol': {
    timeoutMs: 25000,
    validate: (params: any) => {
      const p = params || {}
      if (!p.uid && !(p.name && p.file)) {
        throw new CodeIntelError('CODEINTEL_INVALID_PARAMS', 'Missing uid or name/file')
      }
      return {
        workspaceRoot: p.workspaceRoot,
        uid: p.uid,
        name: p.name,
        file: p.file,
        includeSource: p.includeSource !== false,
        relationLimit: typeof p.relationLimit === 'number' ? p.relationLimit : 20
      }
    },
    handle: async (params, ctx) => {
      const m = await import('./codeintel-gitnexus-symbol')
      return m.handleSymbol(params, ctx)
    }
  },
  'codeintel.codegraphSearch': {
    timeoutMs: 25000,
    validate: (params: any) => {
      const p = params || {}
      if (typeof p.search !== 'string' || !p.search || p.search.length > 256) {
        throw new CodeIntelError('CODEINTEL_INVALID_PARAMS', 'Invalid or missing search string (max 256)')
      }
      return {
        workspaceRoot: p.workspaceRoot,
        search: p.search,
        limit: typeof p.limit === 'number' ? Math.min(50, Math.max(1, p.limit)) : 50,
        kind: p.kind
      }
    },
    handle: async (params, ctx) => {
      const m = await import('./codeintel-codegraph-search')
      return m.handleCodegraphSearch(params, ctx)
    }
  },
  'codeintel.files': {
    timeoutMs: 25000,
    validate: (params: any) => {
      const p = params || {}
      if (typeof p.filter === 'string') {
        if (p.filter.startsWith('-') || p.filter.includes('..') || p.filter.length > 512) {
          throw new CodeIntelError('CODEINTEL_INVALID_PARAMS', 'Invalid filter')
        }
      }
      return {
        workspaceRoot: p.workspaceRoot,
        filter: typeof p.filter === 'string' ? p.filter : '',
        limit: typeof p.limit === 'number' ? Math.min(5000, Math.max(1, p.limit)) : 5000
      }
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
  }
}

type IndexProbe = (workspaceRoot: string, ctx: CodeIntelRequestContext) => Promise<any>

const indexProbes = new Map<string, IndexProbe>()

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
