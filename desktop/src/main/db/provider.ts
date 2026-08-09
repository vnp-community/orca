/**
 * Database Provider Registry
 *
 * Adapters tự register khi được import (side-effect):
 *   import '../db/sqlite/sqlite-adapter'  // registers 'sqlite' provider
 *   import '../db/mysql/mysql-adapter'    // registers 'mysql' provider
 *
 * @module db/provider
 */

import type { IDatabase, DatabaseProvider, IDatabaseCapabilities, DatabaseConfig } from './types'

type Dialect = IDatabaseCapabilities['dialect']

const _registry = new Map<Dialect, DatabaseProvider>()

/**
 * Register a database provider for a dialect.
 * If a provider already exists for that dialect, it will be overwritten (last-write-wins).
 */
export function registerDatabaseProvider(provider: DatabaseProvider): void {
  _registry.set(provider.dialect, provider)
}

/**
 * Retrieve a registered provider by dialect.
 * @throws Error if no provider registered for the requested dialect.
 */
export function getDatabaseProvider(dialect: Dialect): DatabaseProvider {
  const provider = _registry.get(dialect)
  if (!provider) {
    const available = [..._registry.keys()].join(', ') || '(none registered)'
    throw new Error(
      `No database provider registered for dialect: "${dialect}". ` +
        `Available dialects: ${available}. ` +
        `Make sure to import the adapter before calling createDatabase().`
    )
  }
  return provider
}

/**
 * Create a database connection using the registered provider for config.dialect.
 * @throws Error if no provider registered for config.dialect.
 */
export async function createDatabase(config: DatabaseConfig): Promise<IDatabase> {
  const provider = getDatabaseProvider(config.dialect as Dialect)
  return provider.connect(config)
}

/**
 * FOR TESTING ONLY — reset all registered providers.
 * Do not call in production code.
 * @internal
 */
export function clearProviderRegistry(): void {
  _registry.clear()
}

/**
 * FOR TESTING ONLY — get a snapshot of registered dialects.
 * @internal
 */
export function getRegisteredDialects(): Dialect[] {
  return [..._registry.keys()]
}
