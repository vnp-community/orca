// frontend/src/main/runtime/orca-runtime-automation.ts
// Why (BUG-FE-BIGFILE-002 / TASK-BIGFILE-036): automation CRUD + run-now
// commands extracted from OrcaRuntimeService via the composition pattern
// already established by RuntimeBrowserCommands/RuntimeEmulatorCommands
// (orca-runtime-browser.ts) — host interface with the minimal shape each
// method actually needs, not the full OrcaRuntimeService type surface.
import type {
  Automation,
  AutomationCreateInput,
  AutomationRun,
  AutomationUpdateInput,
  AutomationWorkspaceMode
} from '../../shared/automations-types'
import type { AutomationService } from '../automations/service'
import type {
  RuntimeAutomationCreateInput,
  RuntimeAutomationUpdateInput
} from './orca-runtime-types'

function hasRuntimeAutomationUpdateValue<K extends keyof RuntimeAutomationUpdateInput>(
  updates: RuntimeAutomationUpdateInput,
  key: K
): boolean {
  return Object.hasOwn(updates, key) && updates[key] !== undefined
}

// Why: a lazily-fetched slice of the runtime's Store — a getter (not bound
// methods captured at construction time) so this survives OrcaRuntimeService
// assigning `this.store` inside its constructor body, after field
// initializers (including this command object) already ran.
export type RuntimeAutomationStoreSlice = {
  listAutomations?: () => Automation[]
  listAutomationRuns?: (automationId?: string) => AutomationRun[]
  createAutomation?: (input: AutomationCreateInput) => Automation
  updateAutomation?: (id: string, updates: AutomationUpdateInput) => Automation
  deleteAutomation?: (id: string) => void
}

export type RuntimeAutomationCommandHost = {
  getStore(): RuntimeAutomationStoreSlice | null
  getAutomationService(): AutomationService | null
  showManagedWorktree(selector: string): Promise<{ id: string; repoId: string }>
  showRepo(selector: string): Promise<{ id: string }>
}

export class RuntimeAutomationCommands {
  constructor(private readonly host: RuntimeAutomationCommandHost) {}

  listAutomations(): Automation[] {
    // Why: call listAutomations as a property ON the store, not via a
    // reference extracted from it — persistence.ts's implementation reads
    // `this.state`, and detaching the method loses that `this` binding
    // (live repro: "Cannot read properties of undefined (reading 'state')"
    // on every automation.list call for a devServer/environment-hosted
    // repo). Same fix applied to every sibling method below.
    const store = this.host.getStore()
    if (!store?.listAutomations) {
      throw new Error('runtime_unavailable')
    }
    return store.listAutomations()
  }

  listAutomationRuns(automationId?: string): AutomationRun[] {
    const store = this.host.getStore()
    if (!store?.listAutomationRuns) {
      throw new Error('runtime_unavailable')
    }
    return store.listAutomationRuns(automationId)
  }

  showAutomation(id: string): Automation {
    const automation = this.listAutomations().find((entry) => entry.id === id)
    if (!automation) {
      throw new Error('Automation not found.')
    }
    return automation
  }

  async createAutomation(input: RuntimeAutomationCreateInput): Promise<Automation> {
    const store = this.host.getStore()
    if (!store?.createAutomation) {
      throw new Error('runtime_unavailable')
    }
    const target = await this.resolveAutomationTarget(input)
    if (input.reuseSession && target.workspaceMode !== 'existing') {
      throw new Error('Session reuse requires an existing workspace target.')
    }
    return store.createAutomation({
      name: input.name,
      prompt: input.prompt,
      precheck: input.precheck,
      agentId: input.agentId,
      runContext: input.runContext,
      sourceContext: input.sourceContext,
      projectId: target.projectId,
      workspaceMode: target.workspaceMode,
      workspaceId: target.workspaceId,
      baseBranch: input.baseBranch,
      setupDecision: input.setupDecision,
      reuseSession: input.reuseSession,
      timezone: input.timezone ?? Intl.DateTimeFormat().resolvedOptions().timeZone,
      rrule: input.rrule,
      dtstart: input.dtstart,
      enabled: input.enabled,
      missedRunGraceMinutes: input.missedRunGraceMinutes
    })
  }

  async updateAutomation(id: string, updates: RuntimeAutomationUpdateInput): Promise<Automation> {
    if (!this.host.getStore()?.updateAutomation) {
      throw new Error('runtime_unavailable')
    }
    const current = this.showAutomation(id)
    const patch: AutomationUpdateInput = {}
    if (hasRuntimeAutomationUpdateValue(updates, 'name')) {
      patch.name = updates.name
    }
    if (hasRuntimeAutomationUpdateValue(updates, 'prompt')) {
      patch.prompt = updates.prompt
    }
    if (hasRuntimeAutomationUpdateValue(updates, 'precheck')) {
      patch.precheck = updates.precheck
    }
    if (hasRuntimeAutomationUpdateValue(updates, 'agentId')) {
      patch.agentId = updates.agentId
    }
    if (hasRuntimeAutomationUpdateValue(updates, 'runContext')) {
      patch.runContext = updates.runContext
    }
    if (hasRuntimeAutomationUpdateValue(updates, 'sourceContext')) {
      patch.sourceContext = updates.sourceContext
    }
    if (hasRuntimeAutomationUpdateValue(updates, 'baseBranch')) {
      patch.baseBranch = updates.baseBranch
    }
    if (hasRuntimeAutomationUpdateValue(updates, 'setupDecision')) {
      patch.setupDecision = updates.setupDecision
    }
    if (hasRuntimeAutomationUpdateValue(updates, 'reuseSession')) {
      patch.reuseSession = updates.reuseSession
    }
    if (hasRuntimeAutomationUpdateValue(updates, 'timezone')) {
      patch.timezone = updates.timezone
    }
    if (hasRuntimeAutomationUpdateValue(updates, 'rrule')) {
      patch.rrule = updates.rrule
    }
    if (hasRuntimeAutomationUpdateValue(updates, 'dtstart')) {
      patch.dtstart = updates.dtstart
    }
    if (hasRuntimeAutomationUpdateValue(updates, 'enabled')) {
      patch.enabled = updates.enabled
    }
    if (hasRuntimeAutomationUpdateValue(updates, 'missedRunGraceMinutes')) {
      patch.missedRunGraceMinutes = updates.missedRunGraceMinutes
    }
    const targetChanged =
      hasRuntimeAutomationUpdateValue(updates, 'repo') ||
      hasRuntimeAutomationUpdateValue(updates, 'workspace') ||
      hasRuntimeAutomationUpdateValue(updates, 'workspaceMode')
    if (targetChanged) {
      const target = await this.resolveAutomationTarget(updates, current)
      if (patch.reuseSession === true && target.workspaceMode !== 'existing') {
        throw new Error('Session reuse requires an existing workspace target.')
      }
      patch.projectId = target.projectId
      patch.workspaceMode = target.workspaceMode
      patch.workspaceId = target.workspaceId
      if (target.workspaceMode !== 'existing') {
        patch.reuseSession = false
      }
    }
    if (!targetChanged && patch.reuseSession && current.workspaceMode !== 'existing') {
      throw new Error('Session reuse requires an existing workspace target.')
    }
    const store = this.host.getStore()
    if (!store?.updateAutomation) {
      throw new Error('runtime_unavailable')
    }
    return store.updateAutomation(id, patch)
  }

  deleteAutomation(id: string): { removed: boolean; id: string } {
    const store = this.host.getStore()
    if (!store?.deleteAutomation) {
      throw new Error('runtime_unavailable')
    }
    this.showAutomation(id)
    store.deleteAutomation(id)
    return { removed: true, id }
  }

  async runAutomationNow(id: string): Promise<AutomationRun> {
    const automationService = this.host.getAutomationService()
    if (!automationService) {
      throw new Error('runtime_unavailable')
    }
    return await automationService.runNow(id)
  }

  private async resolveAutomationTarget(
    input: {
      repo?: string
      workspace?: string
      workspaceMode?: AutomationWorkspaceMode
      baseBranch?: string | null
    },
    current?: Automation
  ): Promise<{
    projectId: string
    workspaceMode: AutomationWorkspaceMode
    workspaceId?: string | null
  }> {
    const hasRepo = input.repo !== undefined
    const hasWorkspace = input.workspace !== undefined
    if (
      current?.workspaceMode === 'existing' &&
      hasRepo &&
      !hasWorkspace &&
      input.workspaceMode !== 'new_per_run'
    ) {
      throw new Error(
        'Repo updates for existing-workspace automation require workspaceMode new_per_run.'
      )
    }
    const workspace = input.workspace ? await this.host.showManagedWorktree(input.workspace) : null
    const repo = input.repo ? await this.host.showRepo(input.repo) : null
    const workspaceMode =
      input.workspaceMode ??
      (workspace
        ? 'existing'
        : input.repo && !current
          ? 'new_per_run'
          : (current?.workspaceMode ?? 'new_per_run'))
    if (workspaceMode === 'existing') {
      const workspaceId = workspace?.id ?? current?.workspaceId
      const projectId = workspace?.repoId ?? current?.projectId
      if (repo && repo.id !== projectId) {
        throw new Error('Selected workspace belongs to a different repo.')
      }
      if (!workspaceId || !projectId) {
        throw new Error('Existing-workspace automation requires --workspace.')
      }
      return { projectId, workspaceMode, workspaceId }
    }
    const projectId = repo?.id ?? workspace?.repoId ?? current?.projectId
    if (!projectId) {
      throw new Error('Automation requires --repo or --workspace.')
    }
    return { projectId, workspaceMode: 'new_per_run', workspaceId: null }
  }
}
