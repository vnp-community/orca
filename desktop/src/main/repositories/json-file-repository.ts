/**
 * JSON File State Repository
 *
 * A lightweight IStateRepository backed by a JSON file.
 * Used in server mode when no SQL backend is configured,
 * or as a fallback for simple deployments.
 *
 * NOT used in Electron/desktop mode (which uses the full persistence.ts store).
 *
 * @module repositories/json-file-repository
 */

import { readFileSync, writeFileSync, existsSync, mkdirSync } from 'node:fs'
import { dirname } from 'node:path'
import { randomUUID } from 'node:crypto'
import type {
  IStateRepository,
  IProjectRepository,
  IRepoRepository,
  ISshTargetRepository,
  IGlobalSettingsRepository,
  GlobalSettings
} from './types'
import type { Project, Repo } from '../../shared/types'
import type { SshTarget } from '../../shared/ssh-types'

/** Minimal state structure stored in the JSON file */
interface JsonState {
  projects: Project[]
  repos: Repo[]
  sshTargets: SshTarget[]
  globalSettings: Partial<GlobalSettings>
}

function defaultState(): JsonState {
  return {
    projects: [],
    repos: [],
    sshTargets: [],
    globalSettings: {}
  }
}

export class JsonFileStateRepository implements IStateRepository {
  private state: JsonState
  private flushTimer: ReturnType<typeof setTimeout> | null = null
  private readonly DEBOUNCE_MS = 200

  constructor(private readonly dataFile: string) {
    this.state = this.load()
  }

  private load(): JsonState {
    if (!existsSync(this.dataFile)) {
      return defaultState()
    }
    try {
      const raw = readFileSync(this.dataFile, 'utf-8')
      const parsed = JSON.parse(raw) as Partial<JsonState>
      return {
        projects: parsed.projects ?? [],
        repos: parsed.repos ?? [],
        sshTargets: parsed.sshTargets ?? [],
        globalSettings: parsed.globalSettings ?? {}
      }
    } catch {
      return defaultState()
    }
  }

  private scheduleSave(): void {
    if (this.flushTimer) clearTimeout(this.flushTimer)
    this.flushTimer = setTimeout(() => this.flush(), this.DEBOUNCE_MS)
  }

  private flush(): void {
    try {
      const dir = dirname(this.dataFile)
      if (!existsSync(dir)) mkdirSync(dir, { recursive: true })
      writeFileSync(this.dataFile, JSON.stringify(this.state, null, 2), 'utf-8')
    } catch (err) {
      console.error('[JsonFileStateRepository] Failed to flush state:', err)
    }
  }

  // ── IProjectRepository ─────────────────────────────────────────────────────

  get projects(): IProjectRepository {
    const state = this.state
    const save = (): void => this.scheduleSave()
    return {
      findById: async (id) => state.projects.find((p) => p.id === id) ?? null,
      findAll: async () => [...state.projects],
      create: async (input) => {
        const item = { ...input, id: randomUUID() } as Project
        state.projects.push(item)
        save()
        return { ...item }
      },
      update: async (id, patch) => {
        const idx = state.projects.findIndex((p) => p.id === id)
        if (idx === -1) throw new Error(`Project not found: ${id}`)
        state.projects[idx] = { ...state.projects[idx]!, ...(patch as Partial<Project>) }
        save()
        return { ...state.projects[idx]! }
      },
      delete: async (id) => {
        state.projects = state.projects.filter((p) => p.id !== id)
        save()
      },
      findByGroup: async (groupId) =>
        state.projects.filter((p) => (p as Record<string, unknown>)['projectGroupId'] === groupId)
    }
  }

  // ── IRepoRepository ────────────────────────────────────────────────────────

  get repos(): IRepoRepository {
    const state = this.state
    const save = (): void => this.scheduleSave()
    return {
      findById: async (id) => state.repos.find((r) => r.id === id) ?? null,
      findAll: async () => [...state.repos],
      create: async (input) => {
        const item = { ...input, id: randomUUID() } as Repo
        state.repos.push(item)
        save()
        return { ...item }
      },
      update: async (id, patch) => {
        const idx = state.repos.findIndex((r) => r.id === id)
        if (idx === -1) throw new Error(`Repo not found: ${id}`)
        state.repos[idx] = { ...state.repos[idx]!, ...(patch as Partial<Repo>) }
        save()
        return { ...state.repos[idx]! }
      },
      delete: async (id) => {
        state.repos = state.repos.filter((r) => r.id !== id)
        save()
      },
      findByProject: async (projectId) =>
        state.repos.filter((r) => (r as Record<string, unknown>)['projectId'] === projectId)
    }
  }

  // ── ISshTargetRepository ──────────────────────────────────────────────────

  get sshTargets(): ISshTargetRepository {
    const state = this.state
    const save = (): void => this.scheduleSave()
    return {
      findById: async (id) => state.sshTargets.find((t) => t.id === id) ?? null,
      findAll: async () => [...state.sshTargets],
      create: async (input) => {
        const item = { ...input, id: randomUUID() } as SshTarget
        state.sshTargets.push(item)
        save()
        return { ...item }
      },
      update: async (id, patch) => {
        const idx = state.sshTargets.findIndex((t) => t.id === id)
        if (idx === -1) throw new Error(`SshTarget not found: ${id}`)
        state.sshTargets[idx] = { ...state.sshTargets[idx]!, ...(patch as Partial<SshTarget>) }
        save()
        return { ...state.sshTargets[idx]! }
      },
      delete: async (id) => {
        state.sshTargets = state.sshTargets.filter((t) => t.id !== id)
        save()
      }
    }
  }

  // ── IGlobalSettingsRepository ─────────────────────────────────────────────

  get settings(): IGlobalSettingsRepository {
    const state = this.state
    const save = (): void => this.scheduleSave()
    return {
      get: async () => ({ ...state.globalSettings } as GlobalSettings),
      update: async (patch) => {
        state.globalSettings = { ...state.globalSettings, ...patch }
        save()
        return { ...state.globalSettings } as GlobalSettings
      }
    }
  }

  // ── Lifecycle ─────────────────────────────────────────────────────────────

  async ping(): Promise<boolean> {
    return true
  }

  async close(): Promise<void> {
    if (this.flushTimer) {
      clearTimeout(this.flushTimer)
      this.flushTimer = null
    }
    this.flush()
  }
}
