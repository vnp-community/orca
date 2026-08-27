import { defineMethod, type RpcMethod } from '../core'
import type { MemorySnapshot } from '../../../../shared/types'

// Why: reuses OrcaRuntimeService.getMemorySnapshot(), the exact function the
// `memory:getSnapshot` ipcMain handler (desktop/src/main/ipc/memory.ts) and
// the unrelated `diagnostics.memory` RPC method already call — one snapshot
// collector, three call surfaces.
export const MEMORY_METHODS: RpcMethod[] = [
  defineMethod({
    name: 'memory.getSnapshot',
    params: null,
    handler: async (_params, { runtime }): Promise<MemorySnapshot> => {
      return runtime.getMemorySnapshot()
    }
  })
]
