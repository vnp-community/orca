/**
 * useCodeIntelContractDiff.ts — FE-CV-TASK-059-03
 *
 * Contract before/after diff (channel `codeIntel.contractDiff`). `detail: 'summary'` feeds the
 * count chips, `'full'` the table. The UI never classifies compatibility itself.
 *
 * @module hooks/useCodeIntelContractDiff
 */

import { useMemo } from 'react'
import type { ContractDiff } from '../../../shared/code-intel-types'
import { defaultCodeIntelCall, useCodeIntelQuery } from './useCodeIntelQuery'
import type { CodeIntelCallFn, CodeIntelQueryError, CodeIntelQueryStatus } from './useCodeIntelQuery'

export type ContractDiffKindFilter = 'proto' | 'ws-channel' | 'route' | 'migration'

export function parseContractDiff(raw: unknown): ContractDiff {
  const r = (typeof raw === 'object' && raw !== null ? raw : {}) as Partial<ContractDiff>
  const summary = (r.summary ?? {}) as Partial<ContractDiff['summary']>
  const changes = Array.isArray(r.changes) ? r.changes : []
  return {
    scope: r.scope as ContractDiff['scope'],
    summary: {
      breaking: summary.breaking ?? 0,
      risky: summary.risky ?? 0,
      compatible: summary.compatible ?? 0,
      unknown: summary.unknown ?? 0
    },
    changes: changes.map((c) => ({
      ...c,
      details: c.details ?? {},
      files: c.files ?? [],
      consumers: c.consumers ?? [],
      evidence: c.evidence ?? []
    })),
    migrations: Array.isArray(r.migrations) ? r.migrations : [],
    truncated: r.truncated === true,
    totalCount: typeof r.totalCount === 'number' ? r.totalCount : changes.length
  }
}

export type UseCodeIntelContractDiffResult = {
  status: CodeIntelQueryStatus
  error: CodeIntelQueryError | null
  diff: ContractDiff | null
  stale: boolean
  reload: () => void
}

export function useCodeIntelContractDiff(
  worktreeId: string | null,
  environmentId: string | null,
  opts: {
    kinds?: readonly ContractDiffKindFilter[]
    detail?: 'summary' | 'full'
    base?: string
    enabled?: boolean
  } = {},
  callFn: CodeIntelCallFn = defaultCodeIntelCall
): UseCodeIntelContractDiffResult {
  const kindsKey = opts.kinds?.join(',') ?? ''
  const params = useMemo(
    () => ({
      detail: opts.detail ?? 'full',
      ...(kindsKey ? { kinds: kindsKey.split(',') } : {}),
      ...(opts.base ? { base: opts.base } : {})
    }),
    [opts.detail, kindsKey, opts.base]
  )
  const q = useCodeIntelQuery<ContractDiff>(
    worktreeId,
    environmentId,
    { method: 'contractDiff', params, enabled: opts.enabled, parseResult: parseContractDiff },
    callFn
  )
  return { status: q.status, error: q.error, diff: q.data, stale: q.stale, reload: q.refetch }
}
