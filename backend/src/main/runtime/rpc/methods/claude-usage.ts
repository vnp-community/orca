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

// Why: server-mode port of desktop/src/main/runtime/rpc/methods/claude-usage.ts
// -- same ClaudeUsageStore public API (backend/src/main/claude-usage/store.ts,
// already ADR-021 Postgres-backed), reached through runtime.getClaudeUsageStore()
// (wired via setClaudeUsageStore() in server-bootstrap.ts).
export const CLAUDE_USAGE_METHODS: readonly RpcAnyMethod[] = [
  defineMethod({
    name: 'claudeUsage.getScanState',
    params: null,
    handler: async (_params, { runtime }) => runtime.getClaudeUsageStore().getScanState()
  }),
  defineMethod({
    name: 'claudeUsage.setEnabled',
    params: SetEnabledParams,
    handler: async (params, { runtime }) => runtime.getClaudeUsageStore().setEnabled(params.enabled)
  }),
  defineMethod({
    name: 'claudeUsage.refresh',
    params: RefreshParams,
    handler: async (params, { runtime }) => runtime.getClaudeUsageStore().refresh(params?.force ?? false)
  }),
  defineMethod({
    name: 'claudeUsage.getSnapshot',
    params: ScopeRangeLimitParams,
    handler: async (params, { runtime }) =>
      runtime.getClaudeUsageStore().getSnapshot(params.scope, params.range, params.limit)
  }),
  defineMethod({
    name: 'claudeUsage.getSummary',
    params: ScopeRangeParams,
    handler: async (params, { runtime }) =>
      runtime.getClaudeUsageStore().getSummary(params.scope, params.range)
  }),
  defineMethod({
    name: 'claudeUsage.getDaily',
    params: ScopeRangeParams,
    handler: async (params, { runtime }) =>
      runtime.getClaudeUsageStore().getDaily(params.scope, params.range)
  }),
  defineMethod({
    name: 'claudeUsage.getBreakdown',
    params: BreakdownParams,
    handler: async (params, { runtime }) =>
      runtime.getClaudeUsageStore().getBreakdown(params.scope, params.range, params.kind)
  }),
  defineMethod({
    name: 'claudeUsage.getRecentSessions',
    params: ScopeRangeLimitParams,
    handler: async (params, { runtime }) =>
      runtime.getClaudeUsageStore().getRecentSessions(params.scope, params.range, params.limit)
  })
]
