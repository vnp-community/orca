import { describe, expect, it } from 'vitest'
import {
  RuntimeAutomationCommands,
  type RuntimeAutomationCommandHost
} from './orca-runtime-automation'
import type { Automation, AutomationRun } from '../../shared/automations-types'

// Why: persistence.ts's real listAutomations/listAutomationRuns/createAutomation/
// updateAutomation/deleteAutomation all read `this.state` — a class instance
// method relying on its own `this`. Mimic that exactly (not a plain closure
// captured over a variable) so a regression that goes back to extracting the
// method as a bare reference before calling it (`const fn = store.method; fn()`)
// fails this test the same way it failed live: "Cannot read properties of
// undefined (reading 'state')".
class FakeAutomationStore {
  state = { automations: [] as Automation[], automationRuns: [] as AutomationRun[] }

  listAutomations(): Automation[] {
    return this.state.automations
  }

  listAutomationRuns(automationId?: string): AutomationRun[] {
    return automationId
      ? this.state.automationRuns.filter((run) => run.automationId === automationId)
      : this.state.automationRuns
  }

  createAutomation(input: Partial<Automation> & { name: string }): Automation {
    const automation = { id: 'automation-1', ...input } as Automation
    this.state.automations = [...this.state.automations, automation]
    return automation
  }

  updateAutomation(id: string, updates: Partial<Automation>): Automation {
    const current = this.state.automations.find((entry) => entry.id === id)
    if (!current) {
      throw new Error('Automation not found.')
    }
    const updated = { ...current, ...updates }
    this.state.automations = this.state.automations.map((entry) =>
      entry.id === id ? updated : entry
    )
    return updated
  }

  deleteAutomation(id: string): void {
    this.state.automations = this.state.automations.filter((entry) => entry.id !== id)
  }
}

function createHost(store: FakeAutomationStore): RuntimeAutomationCommandHost {
  return {
    getStore: () => store,
    getAutomationService: () => null,
    showManagedWorktree: async (selector) => ({ id: selector, repoId: 'repo-1' }),
    showRepo: async (selector) => ({ id: selector })
  }
}

describe('RuntimeAutomationCommands', () => {
  it('lists automations without losing the store method`s `this` binding', () => {
    const store = new FakeAutomationStore()
    store.state.automations = [{ id: 'automation-1', name: 'A' } as Automation]
    const commands = new RuntimeAutomationCommands(createHost(store))

    expect(commands.listAutomations()).toEqual(store.state.automations)
  })

  it('lists automation runs without losing the store method`s `this` binding', () => {
    const store = new FakeAutomationStore()
    store.state.automationRuns = [{ id: 'run-1', automationId: 'automation-1' } as AutomationRun]
    const commands = new RuntimeAutomationCommands(createHost(store))

    expect(commands.listAutomationRuns('automation-1')).toEqual(store.state.automationRuns)
  })

  it('creates an automation without losing the store method`s `this` binding', async () => {
    const store = new FakeAutomationStore()
    const commands = new RuntimeAutomationCommands(createHost(store))

    const created = await commands.createAutomation({
      name: 'New automation',
      prompt: 'do work',
      agentId: 'claude',
      repo: 'repo-1',
      workspaceMode: 'new_per_run',
      enabled: true
    } as Parameters<RuntimeAutomationCommands['createAutomation']>[0])

    expect(created.name).toBe('New automation')
    expect(store.state.automations).toHaveLength(1)
  })

  it('updates an automation without losing the store method`s `this` binding', async () => {
    const store = new FakeAutomationStore()
    store.state.automations = [{ id: 'automation-1', name: 'A' } as Automation]
    const commands = new RuntimeAutomationCommands(createHost(store))

    const updated = await commands.updateAutomation('automation-1', { name: 'Renamed' })

    expect(updated.name).toBe('Renamed')
  })

  it('deletes an automation without losing the store method`s `this` binding', () => {
    const store = new FakeAutomationStore()
    store.state.automations = [{ id: 'automation-1', name: 'A' } as Automation]
    const commands = new RuntimeAutomationCommands(createHost(store))

    const result = commands.deleteAutomation('automation-1')

    expect(result).toEqual({ removed: true, id: 'automation-1' })
    expect(store.state.automations).toHaveLength(0)
  })
})
