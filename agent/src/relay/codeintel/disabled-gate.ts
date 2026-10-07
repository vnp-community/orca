import { RuntimeSwitches } from './runtime-switches'

export interface DisabledStatusResult {
  workspaceRoot: string
  binding: null
  tools: {
    gitnexus: { available: boolean; supported: boolean; version: null }
    codegraph: { available: boolean; supported: boolean; version: null }
  }
  indexes: {
    gitnexus: { state: string }
    codegraph: { state: string }
  }
  warnings: string[]
}

export function buildDisabledStatusResult(workspaceRoot: string): DisabledStatusResult {
  return {
    workspaceRoot,
    binding: null,
    tools: {
      gitnexus: { available: false, supported: false, version: null },
      codegraph: { available: false, supported: false, version: null }
    },
    indexes: {
      gitnexus: { state: 'unknown' },
      codegraph: { state: 'unknown' }
    },
    warnings: ['codeintel_disabled']
  }
}

export function gateDisabled(
  rpc: { method: string; id?: any; params?: any },
  switches: RuntimeSwitches
): any | null {
  const method = rpc.method

  // 1. Full kill switch: ORCA_CODEINTEL_DISABLED
  if (switches.codeintelDisabled) {
    if (method === 'codeintel.status') {
      return {
        jsonrpc: '2.0',
        id: rpc.id ?? null,
        result: buildDisabledStatusResult(rpc.params?.workspaceRoot ?? '')
      }
    }

    if (method.startsWith('codeintel.') || method.startsWith('quality.')) {
      return {
        jsonrpc: '2.0',
        id: rpc.id ?? null,
        error: {
          code: -32000,
          message: 'Code intelligence is disabled',
          data: {
            code: 'CODEINTEL_TOOL_UNAVAILABLE',
            reason: 'codeintel_disabled'
          }
        }
      }
    }
  }

  // 2. Reindex-only switch: ORCA_CODEINTEL_REINDEX=off
  if (switches.reindexDisabled && method.startsWith('codeintel.reindex')) {
    return {
      jsonrpc: '2.0',
      id: rpc.id ?? null,
      error: {
        code: -32000,
        message: 'Code intelligence reindexing is disabled',
        data: {
          code: 'CODEINTEL_TOOL_UNAVAILABLE',
          reason: 'reindex_disabled'
        }
      }
    }
  }

  // 3. Quality-only switch: ORCA_QUALITY_RUN=off
  if (switches.qualityDisabled && method.startsWith('quality.')) {
    return {
      jsonrpc: '2.0',
      id: rpc.id ?? null,
      error: {
        code: -32000,
        message: 'Quality running is disabled',
        data: {
          code: 'CODEINTEL_TOOL_UNAVAILABLE',
          reason: 'quality_disabled'
        }
      }
    }
  }

  return null
}
