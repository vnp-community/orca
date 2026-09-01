import { callRuntimeRpc } from '@/runtime/runtime-rpc-client'
import type {
  Automation,
  AutomationCreateInput,
  AutomationRun,
  AutomationUpdateInput
} from '../../../../shared/automations-types'
import { parseExecutionHostId } from '../../../../shared/execution-host'
import type { GlobalSettings } from '../../../../shared/types'

type RuntimeAutomationCreateInput = Omit<
  AutomationCreateInput,
  'projectId' | 'workspaceId' | 'timezone'
> & {
  repo?: string
  workspace?: string
  timezone?: string
}

type RuntimeAutomationUpdateInput = Omit<AutomationUpdateInput, 'projectId' | 'workspaceId'> & {
  repo?: string
  workspace?: string
}

export type AutomationHostTarget =
  | { kind: 'local' }
  | { kind: 'environment'; environmentId: string }

export function getAutomationTargetFromHostId(
  hostId: string | null | undefined
): AutomationHostTarget {
  const parsed = parseExecutionHostId(hostId)
  return parsed?.kind === 'runtime'
    ? { kind: 'environment', environmentId: parsed.environmentId }
    : { kind: 'local' }
}

export function getAutomationListTarget(
  settings: Pick<GlobalSettings, 'activeRuntimeEnvironmentId'> | null | undefined
): AutomationHostTarget {
  const environmentId = settings?.activeRuntimeEnvironmentId?.trim()
  return environmentId ? { kind: 'environment', environmentId } : { kind: 'local' }
}

export function getAutomationOwnerTarget(
  automation: Pick<Automation, 'runContext'>,
  sourceTarget?: AutomationHostTarget | null
): AutomationHostTarget {
  if (sourceTarget?.kind === 'environment') {
    return sourceTarget
  }
  return getAutomationTargetFromHostId(automation.runContext?.hostId)
}

export function getAutomationCreateTarget(input: AutomationCreateInput): AutomationHostTarget {
  return getAutomationTargetFromHostId(input.runContext?.hostId)
}

function toRuntimeAutomationCreateInput(
  input: AutomationCreateInput
): RuntimeAutomationCreateInput {
  const { projectId, workspaceId, ...rest } = input
  return {
    ...rest,
    repo: projectId,
    workspace: input.workspaceMode === 'existing' ? (workspaceId ?? undefined) : undefined
  }
}

function toRuntimeAutomationUpdateInput(
  input: AutomationUpdateInput
): RuntimeAutomationUpdateInput {
  const { projectId, workspaceId, ...rest } = input
  return {
    ...rest,
    ...(projectId !== undefined ? { repo: projectId } : {}),
    ...(workspaceId !== undefined ? { workspace: workspaceId ?? undefined } : {})
  }
}

export async function listAutomationsForTarget(
  target: AutomationHostTarget
): Promise<Automation[]> {
  if (target.kind === 'local') {
    return await window.api.automations.list()
  }
  const result = await callRuntimeRpc<{ automations: Automation[] }>(
    target,
    'automation.list',
    undefined,
    { timeoutMs: 15_000 }
  )
  // Why: proto3 `repeated` fields marshal with `omitempty` — a tenant with
  // zero automations gets a response with no `automations` key at all, not
  // an empty array (live-reproduced as "Cannot read properties of undefined
  // (reading 'some')" in AutomationsPage's refresh()).
  return result.automations ?? []
}

export async function listAutomationRunsForTarget(
  target: AutomationHostTarget,
  automationId?: string
): Promise<AutomationRun[]> {
  if (target.kind === 'local') {
    return await window.api.automations.listRuns(automationId ? { automationId } : undefined)
  }
  const result = await callRuntimeRpc<{ runs: AutomationRun[] }>(
    target,
    'automation.runs',
    automationId ? { automationId } : {},
    { timeoutMs: 15_000 }
  )
  // Why: same proto3 omitempty gap as listAutomationsForTarget above — a
  // tenant/automation with zero runs gets no `runs` key at all.
  return result.runs ?? []
}

export async function createAutomationForTarget(input: AutomationCreateInput): Promise<Automation> {
  const target = getAutomationCreateTarget(input)
  if (target.kind === 'local') {
    return await window.api.automations.create(input)
  }
  const result = await callRuntimeRpc<{ automation: Automation }>(
    target,
    'automation.create',
    toRuntimeAutomationCreateInput(input),
    { timeoutMs: 15_000 }
  )
  return result.automation
}

export async function updateAutomationForTarget(
  automation: Automation,
  updates: AutomationUpdateInput,
  sourceTarget?: AutomationHostTarget | null
): Promise<Automation> {
  const target = getAutomationOwnerTarget(automation, sourceTarget)
  if (target.kind === 'local') {
    return await window.api.automations.update({ id: automation.id, updates })
  }
  const result = await callRuntimeRpc<{ automation: Automation }>(
    target,
    'automation.update',
    { id: automation.id, updates: toRuntimeAutomationUpdateInput(updates) },
    { timeoutMs: 15_000 }
  )
  return result.automation
}

// Why: fallback path when the automation being edited could not be re-fetched
// (e.g. list refresh failed) — the caller only has an id, not a full
// Automation to derive the owner target from, so the caller-resolved target
// is used as-is instead of routing through getAutomationOwnerTarget.
export async function updateAutomationByIdForTarget(
  target: AutomationHostTarget | null | undefined,
  id: string,
  updates: AutomationUpdateInput
): Promise<Automation> {
  const resolvedTarget = target ?? { kind: 'local' }
  if (resolvedTarget.kind === 'local') {
    return await window.api.automations.update({ id, updates })
  }
  const result = await callRuntimeRpc<{ automation: Automation }>(
    resolvedTarget,
    'automation.update',
    { id, updates: toRuntimeAutomationUpdateInput(updates) },
    { timeoutMs: 15_000 }
  )
  return result.automation
}

export async function deleteAutomationForTarget(
  automation: Automation,
  sourceTarget?: AutomationHostTarget | null
): Promise<void> {
  const target = getAutomationOwnerTarget(automation, sourceTarget)
  if (target.kind === 'local') {
    await window.api.automations.delete({ id: automation.id })
    return
  }
  await callRuntimeRpc(target, 'automation.delete', { id: automation.id }, { timeoutMs: 15_000 })
}

export async function runAutomationNowForTarget(
  automation: Automation,
  sourceTarget?: AutomationHostTarget | null
): Promise<AutomationRun> {
  const target = getAutomationOwnerTarget(automation, sourceTarget)
  if (target.kind === 'local') {
    return await window.api.automations.runNow({ id: automation.id })
  }
  const result = await callRuntimeRpc<{ run: AutomationRun }>(
    target,
    'automation.runNow',
    { id: automation.id },
    { timeoutMs: 15_000 }
  )
  return result.run
}
