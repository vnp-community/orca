/**
 * State Repository Interfaces
 *
 * Defines the contract for state persistence regardless of backend
 * (JSON file, SQLite, MySQL, PostgreSQL).
 *
 * Implementations:
 * - JsonFileStateRepository — wraps JSON file store (default Electron/desktop mode)
 * - SqlStateRepository — SQL backend for server mode
 *
 * @module repositories/types
 */

import type { Project, Repo, GlobalSettings } from '../../shared/types'
import type { SshTarget } from '../../shared/ssh-types'

export type { GlobalSettings }

/**
 * Generic CRUD repository interface.
 * T = entity type, CreateInput = input for create,
 * UpdateInput = partial update (id excluded).
 */
export type IRepository<T, CreateInput = Omit<T, 'id'>, UpdateInput = Partial<T>> = {
  findById(id: string): Promise<T | null>
  findAll(): Promise<T[]>
  create(input: CreateInput): Promise<T>
  update(id: string, input: UpdateInput): Promise<T>
  delete(id: string): Promise<void>
}

export type IProjectRepository = {
  findByGroup?(groupId: string): Promise<Project[]>
} & IRepository<Project>

export type IRepoRepository = {
  findByProject?(projectId: string): Promise<Repo[]>
} & IRepository<Repo>

export type ISshTargetRepository = {} & IRepository<SshTarget>

export type IGlobalSettingsRepository = {
  get(): Promise<GlobalSettings>
  update(patch: Partial<GlobalSettings>): Promise<GlobalSettings>
}

/** The unified state repository contract */
export type IStateRepository = {
  readonly projects: IProjectRepository
  readonly repos: IRepoRepository
  readonly sshTargets: ISshTargetRepository
  readonly settings: IGlobalSettingsRepository
  /** Check connectivity — returns true if healthy */
  ping(): Promise<boolean>
  /** Release resources (flush writes, close connections) */
  close(): Promise<void>
}

// Re-export entity types for convenience
export type { Project, Repo, SshTarget }
