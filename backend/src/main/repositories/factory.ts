/**
 * Repository Factory
 *
 * Creates the appropriate IStateRepository based on options:
 *   - pool provided → SqlStateRepository (server mode)
 *   - dataFile provided → JsonFileStateRepository (fallback/simple mode)
 *
 * @module repositories/factory
 */

import type { IStateRepository } from './types'
import type { IConnectionPool } from '../db/pool'
import { SqlStateRepository } from './sql-repository'
import { JsonFileStateRepository } from './json-file-repository'

export type RepositoryFactoryOptions = {
  /** SQL pool — if provided, SqlStateRepository is used */
  pool?: IConnectionPool
  /** JSON file path — if provided (and no pool), JsonFileStateRepository is used */
  dataFile?: string
}

/**
 * Create a state repository backed by either SQL or JSON file.
 *
 * Priority:
 * 1. pool → SqlStateRepository
 * 2. dataFile → JsonFileStateRepository
 *
 * @throws Error if neither pool nor dataFile is provided.
 */
export function createStateRepository(options: RepositoryFactoryOptions): IStateRepository {
  if (options.pool) {
    return new SqlStateRepository(options.pool)
  }

  if (options.dataFile) {
    return new JsonFileStateRepository(options.dataFile)
  }

  throw new Error(
    'createStateRepository: must provide either pool (SQL backend) or dataFile (JSON backend)'
  )
}
