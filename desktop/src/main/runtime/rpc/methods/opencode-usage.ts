import { z } from 'zod'
import { defineMethod, type RpcMethod } from '../core'

const ScopeSchema = z.enum(['orca', 'all'])
const RangeSchema = z.enum(['7d', '30d', '90d', 'all'])
const BreakdownKindSchema = z.enum(['model', 'project'])

const SetEnabledParams = z.object({ enabled: z.boolean() })
const RefreshParams = z.object({ force: z.boolean().optional() }).nullish()
const ScopeRangeParams = z.object({ scope: ScopeSchema, range: RangeSchema })
const ScopeRangeLimitParams = z.object({
  scope: ScopeSchema,
  range: RangeSchema,
  limit: z.number().optional()
})
const BreakdownParams = z.object({ scope: ScopeSchema, range: RangeSchema, kind: BreakdownKindSchema })

// Why: additive local-desktop namespace mirroring ipc/opencode-usage.ts 1:1 --
// same OpenCodeUsageStore instance the ipcMain channel calls, reached through
// runtime.getOpenCodeUsageStore() (wired in desktop/src/main/index.ts).
export const OPEN_CODE_USAGE_METHODS: RpcMethod[] = [
  defineMethod({
    name: 'openCodeUsage.getScanState',
    params: null,
    handler: async (_params, { runtime }) => runtime.getOpenCodeUsageStore().getScanState()
  }),
  defineMethod({
    name: 'openCodeUsage.setEnabled',
    params: SetEnabledParams,
    handler: async (params, { runtime }) => runtime.getOpenCodeUsageStore().setEnabled(params.enabled)
  }),
  defineMethod({
    name: 'openCodeUsage.refresh',
    params: RefreshParams,
    handler: async (params, { runtime }) =>
      runtime.getOpenCodeUsageStore().refresh(params?.force ?? false)
  }),
  defineMethod({
    name: 'openCodeUsage.getSnapshot',
    params: ScopeRangeLimitParams,
    handler: async (params, { runtime }) =>
      runtime.getOpenCodeUsageStore().getSnapshot(params.scope, params.range, params.limit)
  }),
  defineMethod({
    name: 'openCodeUsage.getSummary',
    params: ScopeRangeParams,
    handler: async (params, { runtime }) =>
      runtime.getOpenCodeUsageStore().getSummary(params.scope, params.range)
  }),
  defineMethod({
    name: 'openCodeUsage.getDaily',
    params: ScopeRangeParams,
    handler: async (params, { runtime }) =>
      runtime.getOpenCodeUsageStore().getDaily(params.scope, params.range)
  }),
  defineMethod({
    name: 'openCodeUsage.getBreakdown',
    params: BreakdownParams,
    handler: async (params, { runtime }) =>
      runtime.getOpenCodeUsageStore().getBreakdown(params.scope, params.range, params.kind)
  }),
  defineMethod({
    name: 'openCodeUsage.getRecentSessions',
    params: ScopeRangeLimitParams,
    handler: async (params, { runtime }) =>
      runtime.getOpenCodeUsageStore().getRecentSessions(params.scope, params.range, params.limit)
  })
]
