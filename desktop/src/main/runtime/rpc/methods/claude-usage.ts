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

// Why: additive local-desktop namespace mirroring ipc/claude-usage.ts 1:1 --
// same ClaudeUsageStore instance the ipcMain channel calls, reached through
// runtime.getClaudeUsageStore() (wired in desktop/src/main/index.ts).
export const CLAUDE_USAGE_METHODS: RpcMethod[] = [
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
