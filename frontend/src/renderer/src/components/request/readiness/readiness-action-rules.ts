/**
 * Task readiness rules — FE-REQ-TASK-036-07
 *
 * @module components/request/readiness/readiness-action-rules
 */

import { CircleCheck, FileWarning, MessageCircleQuestion, ServerCrash } from 'lucide-react'
import type { LucideIcon } from 'lucide-react'
import type { TaskReadinessFinding, TaskReadinessOutcome, TaskReadinessReport } from '../../../../../shared/request-artifact-types'

const T = 'auto.components.request.readiness.'

export type ReadinessPresentation = { labelKey: string; fallback: string; Icon: LucideIcon; tone: 'success' | 'neutral' | 'warning' | 'destructive' }

const PRESENTATION: Partial<Record<TaskReadinessOutcome, ReadinessPresentation>> = {
  ready: { labelKey: `${T}badge.ready`, fallback: 'Ready', Icon: CircleCheck, tone: 'success' },
  needs_info: { labelKey: `${T}badge.needs_info`, fallback: 'Needs information', Icon: MessageCircleQuestion, tone: 'neutral' },
  spec_defect: { labelKey: `${T}badge.spec_defect`, fallback: 'Spec defect', Icon: FileWarning, tone: 'warning' },
  env_defect: { labelKey: `${T}badge.env_defect`, fallback: 'Environment issue', Icon: ServerCrash, tone: 'destructive' }
}

/** `unknown` has no presentation: unchecked tasks show no badge. */
export function getReadinessPresentation(outcome: TaskReadinessOutcome): ReadinessPresentation | null {
  return PRESENTATION[outcome] ?? null
}

export type ReadinessAction = {
  kind: 'run' | 'answer' | 'regenerate' | 'connect' | 'notify' | 'none'
  labelKey: string
  fallback: string
  noteKey?: string
  noteFallback?: string
}

export function getReadinessAction(report: TaskReadinessReport | null, ctx: { canWrite: boolean; hasDevServer: boolean }): ReadinessAction {
  const none: ReadinessAction = { kind: 'none', labelKey: '', fallback: '' }
  if (!report || !ctx.canWrite) {return none}
  switch (report.outcome) {
    case 'ready':
      return { kind: 'run', labelKey: `${T}action.run`, fallback: 'Run' }
    case 'needs_info':
      return { kind: 'answer', labelKey: `${T}action.answer`, fallback: 'Answer the questions' }
    case 'spec_defect':
      return { kind: 'regenerate', labelKey: `${T}action.regenerate`, fallback: 'Return to the Plan step to regenerate the task' }
    case 'env_defect':
      // Why: an environment defect is not the task's fault, so it never consumes an attempt.
      return ctx.hasDevServer
        ? { kind: 'notify', labelKey: `${T}action.notify`, fallback: 'Notify the operator', noteKey: `${T}notCounted`, noteFallback: 'Does not count as an attempt' }
        : { kind: 'connect', labelKey: `${T}action.connect`, fallback: 'Connect a dev server', noteKey: `${T}notCounted`, noteFallback: 'Does not count as an attempt' }
    default:
      return none
  }
}

/** Missing report or unsupported runtime never blocks (keeps pre-readiness behavior). */
export function isRunBlockedByReadiness(report: TaskReadinessReport | null, supported: boolean): boolean {
  if (!supported || !report) {return false}
  return report.outcome !== 'ready' && report.outcome !== 'unknown'
}

const TIER_ORDER = ['structure', 'semantic', 'environment'] as const
export function groupFindingsByTier(findings: readonly TaskReadinessFinding[]): { tier: string; findings: TaskReadinessFinding[] }[] {
  const by = new Map<string, TaskReadinessFinding[]>()
  for (const f of findings) {by.set(f.tier, [...(by.get(f.tier) ?? []), f])}
  const known = TIER_ORDER.filter((t) => by.has(t)).map((tier) => ({ tier: tier as string, findings: by.get(tier) ?? [] }))
  const other = [...by.keys()].filter((t) => !(TIER_ORDER as readonly string[]).includes(t)).sort().map((tier) => ({ tier, findings: by.get(tier) ?? [] }))
  return [...known, ...other]
}
