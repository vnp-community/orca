import { z } from 'zod'
import { defineMethod, type RpcAnyMethod } from '../core'

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

// Why: server-mode port of desktop/src/main/runtime/rpc/methods/opencode-usage.ts
// -- same OpenCodeUsageStore public API (backend/src/main/opencode-usage/store.ts,
// ADR-021 Postgres-backed like claude-usage.ts/codex-usage.ts), reached through
// runtime.getOpenCodeUsageStore() (wired via setOpenCodeUsageStore() in server-bootstrap.ts).
export const OPEN_CODE_USAGE_METHODS: readonly RpcAnyMethod[] = [
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
