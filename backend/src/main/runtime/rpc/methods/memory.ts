import { defineMethod, type RpcAnyMethod } from '../core'
import type { MemorySnapshot } from '../../../../shared/types'

// Why: ports desktop/src/main/runtime/rpc/methods/memory.ts. Reuses
// OrcaRuntimeService.getMemorySnapshot() (backend/.../orca-runtime.ts), which
// already wraps process.memoryUsage() via memory/collector.ts — no
// Electron-specific memory API involved, so this is a near-verbatim port.
export const MEMORY_METHODS: readonly RpcAnyMethod[] = [
  defineMethod({
    name: 'memory.getSnapshot',
    params: null,
    handler: async (_params, { runtime }): Promise<MemorySnapshot> => {
      return runtime.getMemorySnapshot()
    }
  })
]
