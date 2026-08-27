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

// Why: additive local-desktop namespace mirroring ipc/codex-usage.ts 1:1 --
// same CodexUsageStore instance the ipcMain channel calls, reached through
// runtime.getCodexUsageStore() (wired in desktop/src/main/index.ts).
export const CODEX_USAGE_METHODS: RpcMethod[] = [
  defineMethod({
    name: 'codexUsage.getScanState',
    params: null,
    handler: async (_params, { runtime }) => runtime.getCodexUsageStore().getScanState()
  }),
  defineMethod({
    name: 'codexUsage.setEnabled',
    params: SetEnabledParams,
    handler: async (params, { runtime }) => runtime.getCodexUsageStore().setEnabled(params.enabled)
  }),
  defineMethod({
    name: 'codexUsage.refresh',
    params: RefreshParams,
    handler: async (params, { runtime }) => runtime.getCodexUsageStore().refresh(params?.force ?? false)
  }),
  defineMethod({
    name: 'codexUsage.getSnapshot',
    params: ScopeRangeLimitParams,
    handler: async (params, { runtime }) =>
      runtime.getCodexUsageStore().getSnapshot(params.scope, params.range, params.limit)
  }),
  defineMethod({
    name: 'codexUsage.getSummary',
    params: ScopeRangeParams,
    handler: async (params, { runtime }) =>
      runtime.getCodexUsageStore().getSummary(params.scope, params.range)
  }),
  defineMethod({
    name: 'codexUsage.getDaily',
    params: ScopeRangeParams,
    handler: async (params, { runtime }) =>
      runtime.getCodexUsageStore().getDaily(params.scope, params.range)
  }),
  defineMethod({
    name: 'codexUsage.getBreakdown',
    params: BreakdownParams,
    handler: async (params, { runtime }) =>
      runtime.getCodexUsageStore().getBreakdown(params.scope, params.range, params.kind)
  }),
  defineMethod({
    name: 'codexUsage.getRecentSessions',
    params: ScopeRangeLimitParams,
    handler: async (params, { runtime }) =>
      runtime.getCodexUsageStore().getRecentSessions(params.scope, params.range, params.limit)
  })
]
